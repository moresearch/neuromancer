package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gdamore/tcell"
)

const (
	TyperComplete = iota
	TyperSigInt
	TyperEscape
	TyperPrevious
	TyperNext
	TyperResize
)

// layoutSplitPad is the blank rows kept between the practice text and the
// keyboard/chatsubo block: one, so the block reads as one tight stack.
const layoutSplitPad = 1

// statsHistoryMax caps how many WPM samples the equalizer keeps.
const statsHistoryMax = 120

type segment struct {
	Text        string `json:"text"`
	Attribution string `json:"attribution"`
}

type mistake struct {
	Word  string `json:"word"`
	Typed string `json:"typed"`
}

type typer struct {
	Scr              tcell.Screen
	OnStart          func()
	SkipWord         bool
	DisableBackspace bool
	BlockCursor      bool
	tty              io.Writer

	// When > 0, highlight this many non-space characters ahead of the cursor
	// instead of the current and next word.
	HighlightChars int

	// WordByWord highlights only the word under the cursor, leaving the
	// following word at the default style.
	WordByWord bool

	currentWordStyle    tcell.Style
	nextWordStyle       tcell.Style
	incorrectSpaceStyle tcell.Style
	incorrectStyle      tcell.Style
	correctStyle        tcell.Style
	defaultStyle        tcell.Style

	// Most recent incorrect key, highlighted red on the keyboard matrix.
	wrongKey   rune
	wrongUntil time.Time

	// Optional Vial keymap; when set it replaces the built-in Colemak-DH matrix.
	vil *vilKeyboard
	// vilFallback is set when the Vial layout is too wide for the window and
	// the built-in Colemak-DH matrix is drawn instead.
	vilFallback  bool
	previewLayer int
	blinkOn      bool
	chatsubo     *chatsubo
	sprint       bool
	panelLabel   string
	// sprintMessage is the status line shown in the box's commentary row while
	// the next sprint summary is being generated.
	sprintMessage string
	layoutTop     int
	statsY        int
	// boxTop and boxH are the Chatsubo box's place in the centred block,
	// recorded by keyboardMetrics so the box and board share one frame.
	boxTop int
	boxH   int
	// statsTop is the equalizer row of the block; the readout is the row
	// under it.
	statsTop int
	// parts is the block split into its three parts (box, text, keys, stats),
	// recorded by keyboardMetrics so every band draws inside its own rows.
	parts        partLayout
	hintText     string
	layerTracker *layerTracker

	// Live dashboard state: WPM sampled once per frame, used to detect speed
	// changes so the stats can flash.
	lastWpm      int
	lastStatsT   time.Time
	lastStatsDir int
	statsPulse   time.Time

	// Latest live readings, shared by the floating HUD and the panel.
	curWpm, curCpm int
	curAcc         float64
	curErrs        int

	// Prior results for the current mode, loaded at test start. bestWpm
	// tracks the live reading so a PB shows up the moment it is beaten.
	scorePath string
	scoreKey  string
	lastScore scoreRecord
	bestWpm   int
	bestAcc   float64

	// Rolling WPM history for the equalizer, oldest first.
	wpmHistory []int

	// Cyberpunk accents for the live dashboard.
	errColor  tcell.Color
	neonCyan  tcell.Color
	neonPink  tcell.Color
	neonGreen tcell.Color
	neonAmber tcell.Color
}

// wordAheadStyle picks the style for a not-yet-typed character at or past the
// cursor, given the word index the cursor is currently in (inword). The current
// word is highlighted; the next word is highlighted too unless WordByWord is
// set, in which case only the current word stands out. It returns the updated
// word index and writes the chosen style through style.
func (t *typer) wordAheadStyle(c rune, inword int, style *tcell.Style) int {
	switch {
	case c == ' ':
		inword++
	case inword == 0:
		*style = t.currentWordStyle
	case inword == 1 && !t.WordByWord:
		*style = t.nextWordStyle
	default:
		*style = t.defaultStyle
	}
	return inword
}

func NewTyper(scr tcell.Screen, emboldenTypedText bool, fgcol, bgcol, hicol, hicol2, hicol3, errcol tcell.Color) *typer {
	var tty io.Writer
	def := tcell.StyleDefault.
		Foreground(fgcol).
		Background(bgcol)

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	//Will fail on windows, but nm is still mostly usable via tcell
	if err != nil {
		tty = ioutil.Discard
	}

	correctStyle := def.Foreground(hicol)
	if emboldenTypedText {
		correctStyle = correctStyle.Bold(true)
	}

	return &typer{
		Scr:          scr,
		SkipWord:     true,
		tty:          tty,
		previewLayer: -1,
		blinkOn:      true,
		layoutTop:    -1,

		defaultStyle:        def,
		correctStyle:        correctStyle,
		currentWordStyle:    def.Foreground(ensureReadable(hicol2, bgcol)),
		nextWordStyle:       def.Foreground(hicol3),
		incorrectStyle:      def.Foreground(errcol),
		errColor:            errcol,
		incorrectSpaceStyle: def.Background(errcol),

		neonCyan:  tcell.NewHexColor(0x00F0FF),
		neonPink:  tcell.NewHexColor(0xFF2BD6),
		neonGreen: tcell.NewHexColor(0x39FF14),
		neonAmber: tcell.NewHexColor(0xFFB000),
	}
}

// relativeLuminance is the WCAG relative luminance of c, or -1 for colours which
// cannot be measured (the terminal default).
func relativeLuminance(c tcell.Color) float64 {
	r, g, b := c.RGB()
	lin := func(v int32) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// contrastRatio is the WCAG contrast between two colours, or -1 when either is
// the terminal default and therefore unknowable.
func contrastRatio(a, b tcell.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < 0 || lb < 0 {
		return -1
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// minTextContrast is the WCAG AA threshold for large text. The current word is
// the one the typist is reading, so it must stay legible; a theme whose
// highlight colour is too close to the background would otherwise hide it.
const minTextContrast = 3.0

// ensureReadable returns fg, or the theme's normal text colour when fg would
// be hard to read on bg. Themes may pick a dim highlight, but a word the user
// is meant to type must never disappear into the background.
func ensureReadable(fg, bg tcell.Color) tcell.Color {
	if c := contrastRatio(fg, bg); c >= 0 && c < minTextContrast {
		return tcell.ColorDefault
	}
	return fg
}

// segsWidth is the total rendered width of a stats readout in cells.
func segsWidth(segs []statsSeg) int {
	w := 0
	for _, sg := range segs {
		w += len([]rune(sg.s))
	}
	return w
}

// drawLiveStats renders the cyberpunk speed readout centered on cx at row y.
// The WPM figure is colour-zoned by speed and flashes on the moment the
// reading changes, like a car HUD.
func (t *typer) drawLiveStats(cx, y, wpm, cpm int, acc float64, errs int) {
	sw, _ := t.Scr.Size()
	flash := t.statsFlashOn()
	// On a narrow window drop the least-useful figures (CPM, errors, then the
	// last/best tail) until the readout fits on one line, so it never wraps:
	// first the stats band's width, then the window itself.
	// The band's rows are the window's, less one clear cell on each side.
	fit := sw - 2
	if fit < 1 {
		fit = 1
	}
	segs := t.statsSegmentsFull(wpm, cpm, acc, errs, flash, true, true, true)
	if w := segsWidth(segs); w > fit {
		segs = t.statsSegmentsFull(wpm, cpm, acc, errs, flash, false, true, true)
	}
	if w := segsWidth(segs); w > fit {
		segs = t.statsSegmentsFull(wpm, cpm, acc, errs, false, false, false, true)
	}
	if w := segsWidth(segs); w > fit {
		segs = t.statsSegmentsFull(wpm, cpm, acc, errs, false, false, false, false)
	}
	if w := segsWidth(segs); w > fit {
		// Still too wide for the last usable variant: keep the speed figure.
		segs = segs[:1]
	}
	fieldW := segsWidth(segs)
	startX := cx - fieldW/2
	if startX < 0 {
		startX = 0
	}
	if startX+fieldW > sw {
		startX = sw - fieldW
	}
	if startX < 0 {
		startX = 0
	}

	// Wipe the whole band (a little wider than the content) first: cx can
	// shift between frames and a wide rune leaves a trailing cell behind.
	for bx := startX - 1; bx <= startX+fieldW; bx++ {
		if bx >= 0 && bx < sw {
			t.Scr.SetContent(bx, y, ' ', nil, t.defaultStyle)
		}
	}

	x := startX
	for _, sg := range segs {
		r := []rune(sg.s)
		remaining := fieldW - (x - startX)
		if len(r) > remaining {
			r = r[:remaining]
		}
		drawString(t.Scr, x, y, string(r), -1, sg.style)
		x += len(r)
	}

	// Equalizer above the numbers, when the stats band has a row for it.
	if t.parts.stats.H == 2 {
		t.drawEqualizer(startX, y-1, fieldW, 1, t.defaultStyle)
	}
}

// drawEqualizer renders the rolling WPM history as a block equalizer. Bars
// grow from the last `rows` of the sample history, scaled to the recent peak,
// and are tinted by the same speed zones as the WPM figure.
func (t *typer) drawEqualizer(x, y, w, rows int, style tcell.Style) {
	if w <= 0 || rows <= 0 {
		return
	}
	samples := t.wpmHistory
	if len(samples) > w {
		samples = samples[len(samples)-w:]
	}

	// Scale to the recent peak so the graph stays lively; keep a floor so a
	// short burst does not blow up into a full-height wall.
	peak := 1
	for _, v := range samples {
		if v > peak {
			peak = v
		}
	}
	if peak < 20 {
		peak = 20
	}

	const blocks = "▁▂▃▄▅▆▇█"
	nb := []rune(blocks)

	startX := x + w - len(samples) // right-align: newest sample on the right
	for i, v := range samples {
		level := (v*len(nb)*rows + peak - 1) / peak // 1..rows*len(nb)
		full, rem := level/len(nb), level%len(nb)
		if rem > 0 {
			full++
		}
		col := startX + i
		color := t.neonAmber
		switch {
		case v >= 100:
			color = t.neonPink
		case v >= 70:
			color = t.neonGreen
		case v >= 45:
			color = t.neonCyan
		}
		barStyle := t.defaultStyle.Foreground(color)
		// Fill solid blocks from the bottom up, then a partial block on top.
		// `full` counts the partial cell, so start one row higher for it.
		partial := rem > 0
		solid := full
		if partial {
			solid = full - 1
		}
		bottom := y + rows - 1
		row := bottom
		for i := 0; i < solid; i++ {
			t.Scr.SetContent(col, row, nb[len(nb)-1], nil, barStyle)
			row--
		}
		if partial && row >= y {
			t.Scr.SetContent(col, row, nb[rem-1], nil, barStyle)
		}
	}
}

// sampleWpm records the reading a few times a second; any change arms a short
// flash. Sampling (rather than per-frame) keeps the average drifting from
// retriggering the flash constantly.
func (t *typer) sampleWpm(wpm int) {
	now := time.Now()
	if !t.lastStatsT.IsZero() && now.Sub(t.lastStatsT) < 250*time.Millisecond {
		return
	}
	if t.lastWpm != 0 && wpm != t.lastWpm {
		t.statsPulse = now
		t.lastStatsDir = 1
		if wpm < t.lastWpm {
			t.lastStatsDir = -1
		}
	}
	t.lastWpm = wpm
	t.lastStatsT = now

	// Keep a short rolling history for the equalizer. Only advance once the
	// timer has actually been running, so the idle 0s do not fill the graph.
	if !t.lastStatsT.IsZero() && wpm > 0 {
		t.wpmHistory = append(t.wpmHistory, wpm)
		if len(t.wpmHistory) > statsHistoryMax {
			t.wpmHistory = t.wpmHistory[len(t.wpmHistory)-statsHistoryMax:]
		}
	}
}

// statsFlashOn reports whether the readout is in its blink-on phase.
func (t *typer) statsFlashOn() bool {
	if t.statsPulse.IsZero() || time.Since(t.statsPulse) >= 450*time.Millisecond {
		return false
	}
	return int(time.Now().UnixMilli()/140)%2 == 0
}

// statsSeg is one styled chunk of the speed readout.
type statsSeg struct {
	s     string
	style tcell.Style
}

// statsSegments builds the colour-zoned readout: WPM (tinted by speed, and
// flashing when flashOn), then CPM, accuracy, errors and the last/best score
// in neon accents. The panel is narrower than the standalone HUD, so showCPM
// drops the CPM figure there and keeps the score figures instead.
func (t *typer) statsSegments(wpm, cpm int, acc float64, errs int, flashOn, showCPM, showErrs bool) []statsSeg {
	return t.statsSegmentsFull(wpm, cpm, acc, errs, flashOn, showCPM, showErrs, true)
}

// statsSegmentsFull is statsSegments with control over the LAST/BEST tail, so
// the narrow chatsubo body can drop it when it would not fit.
func (t *typer) statsSegmentsFull(wpm, cpm int, acc float64, errs int, flashOn, showCPM, showErrs, showScore bool) []statsSeg {
	wpmColor, arrow := t.neonCyan, "▸"
	switch {
	case wpm >= 100:
		wpmColor, arrow = t.neonPink, "◆"
	case wpm >= 70:
		wpmColor, arrow = t.neonGreen, "▴"
	case wpm >= 45:
		wpmColor, arrow = t.neonCyan, "▸"
	default:
		wpmColor, arrow = t.neonAmber, "▾"
	}

	accColor := t.neonGreen
	if acc < 98 {
		accColor = t.neonAmber
	}

	wpmStyle := t.defaultStyle.Foreground(wpmColor).Bold(true)
	if flashOn {
		// Flash the speed figure itself: white-hot on a jump up, errcol on a
		// drop, so a change reads instantly without disturbing the rest.
		if t.lastStatsDir < 0 {
			wpmStyle = t.defaultStyle.Foreground(t.errColor).Bold(true)
		} else {
			wpmStyle = t.defaultStyle.Foreground(tcell.ColorWhite).Reverse(true).Bold(true)
		}
	}

	sep := t.defaultStyle.Foreground(t.neonPink)
	segs := []statsSeg{
		{fmt.Sprintf("%s %d WPM", arrow, wpm), wpmStyle},
	}
	if showCPM {
		segs = append(segs,
			statsSeg{" · ", sep},
			statsSeg{fmt.Sprintf("%d CPM", cpm), sep},
		)
	}
	segs = append(segs,
		statsSeg{" · ", sep},
		statsSeg{fmt.Sprintf("%.0f%%", acc), t.defaultStyle.Foreground(accColor).Bold(true)},
	)
	if showErrs {
		segs = append(segs,
			statsSeg{" · ", sep},
			statsSeg{fmt.Sprintf("%d ERR", errs), t.defaultStyle.Foreground(t.neonAmber)},
		)
	}
	if showScore {
		segs = append(segs, t.scoreSegments()...)
	}
	return segs
}

// scoreSegments renders the last and best score. Best is relative to the WPM
// being beaten, so it updates mid-test and pops white-hot on a new record.
func (t *typer) scoreSegments() []statsSeg {
	labelStyle := t.defaultStyle.Foreground(t.neonPink)
	valueStyle := t.defaultStyle.Foreground(t.neonGreen).Bold(true)
	bestStyle := valueStyle
	if t.curWpm > 0 && t.curWpm >= t.bestWpm {
		bestStyle = t.defaultStyle.Foreground(tcell.ColorWhite).Reverse(true).Bold(true)
	}

	best := "—"
	if t.bestWpm > 0 {
		best = fmt.Sprintf("%d%%%.0f", t.bestWpm, t.bestAcc)
	}
	return []statsSeg{
		{" · ", labelStyle},
		{"LAST ", labelStyle},
		{fmt.Sprintf("%d%%%.0f", t.lastScore.LastWpm, t.lastScore.LastAcc), valueStyle},
		{" · ", labelStyle},
		{"BEST ", labelStyle},
		{best, bestStyle},
	}
}

// saveScore commits a finished test to the stored last/best. Tests ended
// early (escape, arrows, ctrl-c) return before this, so a partial run never
// overwrites a real score.
func (t *typer) saveScore() {
	if t.scorePath == "" {
		return
	}
	t.lastScore = recordScore(t.scorePath, t.scoreKey, t.curWpm, t.curAcc)
	t.bestWpm, t.bestAcc = t.lastScore.BestWpm, t.lastScore.BestAcc
}

// drawPanelStats paints the readout inside the chatsubo body on row, clipped
// to maxCols. In a narrow panel the figures drop one at a time, least useful
// first: the error count, then CPM, then the LAST/BEST tail. The flash is off
// in the try-outs because it decides the width of the WPM figure itself.
func (t *typer) drawPanelStats(x, row, maxCols, wpm, cpm int, acc float64, errs int) {
	flash := t.statsFlashOn()
	type variant struct {
		flash, showCPM, showErrs, showScore bool
	}
	variants := []variant{
		{flash, true, true, true},
		{flash, false, true, true},
		{false, true, true, true},
		{false, false, true, true},
		{false, false, false, true},
		{false, false, false, false},
	}
	for _, v := range variants {
		segs := t.statsSegmentsFull(wpm, cpm, acc, errs, v.flash, v.showCPM, v.showErrs, v.showScore)
		if t.paintPanelStats(x, row, maxCols, segs) {
			return
		}
	}
}

// paintPanelStats draws segs and reports whether they fit in maxCols.
func (t *typer) paintPanelStats(x, row, maxCols int, segs []statsSeg) bool {
	w := 0
	for _, sg := range segs {
		w += len([]rune(sg.s))
	}
	if w > maxCols {
		return false
	}
	col := 0
	for _, sg := range segs {
		for _, c := range sg.s {
			t.Scr.SetContent(x+col, row, c, nil, sg.style)
			col++
		}
	}
	return true
}

func (t *typer) Start(text []segment, timeout time.Duration) (nerrs, ncorrect int, duration time.Duration, rc int, mistakes []mistake) {
	timeLeft := timeout
	// A run ends with (or without) a layer still held: fall back to the target
	// layer so the results screen never shows a physically-held layer.
	defer func() {
		if t.layerTracker != nil {
			t.layerTracker.reset()
		}
	}()

	for i, s := range text {
		startImmediately := true
		var d time.Duration
		var e, c int
		var m []mistake

		if i == 0 {
			startImmediately = false
		}

		e, c, rc, d, m = t.start(s.Text, timeLeft, startImmediately, s.Attribution)

		nerrs += e
		ncorrect += c
		duration += d
		mistakes = append(mistakes, m...)

		if timeout != -1 {
			timeLeft -= d
			if timeLeft <= 0 {
				return
			}
		}

		if rc != TyperComplete {
			return
		}
	}

	return
}

func extractMistypedWords(text []rune, typed []rune) (mistakes []mistake) {
	var w []rune
	var t []rune
	f := false

	for i := range text {
		if text[i] == ' ' {
			if f {
				mistakes = append(mistakes, mistake{string(w), string(t)})
			}

			w = w[:0]
			t = t[:0]
			f = false
			continue
		}

		if text[i] != typed[i] {
			f = true
		}

		if text[i] == 0 {
			w = append(w, '_')
		} else {
			w = append(w, text[i])
		}

		if typed[i] == 0 {
			t = append(t, '_')
		} else {
			t = append(t, typed[i])
		}
	}

	if f {
		mistakes = append(mistakes, mistake{string(w), string(t)})
	}

	return
}

func (t *typer) start(s string, timeLimit time.Duration, startImmediately bool, attribution string) (nerrs int, ncorrect int, rc int, duration time.Duration, mistakes []mistake) {
	var startTime time.Time

	sw, sh := scr.Size()

	t.layoutTop = -1
	t.keyboardMetrics()

	nc := 0
	x := 0
	y := 0
	statsY := 0

	// The practice text draws inside the chatsubo box, padded away from the
	// border on every side so it never hugs the frame: five columns in from the
	// border horizontally and a blank row above.
	textTop, _, _, _ := t.layoutFrame()
	bx, _, bw, _, boxOK := t.chatBand()
	if boxOK {
		x = bx + 6
		nc = bw - 12
		if nc < 1 {
			nc = 1
		}
	} else {
		nc = sw - 2
		if nc < 1 {
			nc = 1
		}
		x = (sw - nc) / 2
		if x < 1 {
			x = 1
		}
	}

	// Wrap the practice text to the interior width so every line stays inside
	// the padding. The user never types the inserted newlines: the typer skips
	// them automatically.
	s = wordWrap(s, nc)
	text := []rune(s)
	typed := make([]rune, len(text))
	if len(text) == 0 {
		rc = TyperComplete
		return
	}
	_, nr := calcStringDimensions(s)
	// boxTextPad blank rows above the text keep it off the title rule; the
	// same padding below keeps the last line clear of the bottom border.
	y = textTop + boxTextPad
	if y < 0 {
		y = 0
	}
	// Lines the padded box cannot hold are clipped, so a wrapped paragraph
	// never spills past the padding or the bottom border.
	if nr > t.textRows()-2*boxTextPad {
		nr = t.textRows() - 2*boxTextPad
	}
	if nr < 1 {
		nr = 1
	}
	statsY = t.statsTop + 1
	if statsY > sh-1 {
		statsY = sh - 1
	}

	t.statsY = statsY

	if t.chatsubo != nil {
		t.chatsubo.resetSegment()
	}

	t.wrongKey = 0
	t.wrongUntil = time.Time{}
	t.lastWpm = 0
	t.lastStatsT = time.Time{}
	t.statsPulse = time.Time{}
	t.wpmHistory = t.wpmHistory[:0]

	// Prior results for this mode ride along in the readout so the current
	// test is always measured against the last and best one.
	if t.scorePath != "" {
		t.lastScore = loadScore(t.scorePath, t.scoreKey)
		t.bestWpm, t.bestAcc = t.lastScore.BestWpm, t.lastScore.BestAcc
	}

	if !t.BlockCursor {
		t.tty.Write([]byte("\033[5 q"))

		//Assumes original cursor shape was a block (the one true cursor shape), there doesn't appear to be a
		//good way to save/restore the shape if the user has changed it from the otcs.
		defer t.tty.Write([]byte("\033[2 q"))
	}

	t.Scr.SetStyle(t.defaultStyle)
	idx := 0

	calcStats := func() {
		nerrs = 0
		ncorrect = 0

		mistakes = extractMistypedWords(text[:idx], typed[:idx])

		for i := 0; i < idx; i++ {
			if text[i] != '\n' {
				if text[i] != typed[i] {
					nerrs++
				} else {
					ncorrect++
				}
			}
		}

		rc = TyperComplete
		duration = time.Now().Sub(startTime)
	}

	redraw := func() {
		var target rune
		if idx < len(text) {
			target = text[idx]
		}

		if t.sprint {
			t.panelLabel = " chatsubo "
			t.sprintMessage = ""
			if timeLimit != -1 && !startTime.IsZero() {
				remaining := timeLimit - time.Now().Sub(startTime)
				secs := int(remaining/time.Second) + 1
				if secs < 0 {
					secs = 0
				}
				t.panelLabel = fmt.Sprintf(" chatsubo %ds ", secs)
			}
		}

		// Compute live stats up front so the panel and the HUD share them.
		calcStats()
		total := ncorrect + nerrs
		cpm := 0
		acc := 100.0
		if duration > 1e7 { //Avoid flashing large numbers on test start.
			cpm = int(float64(ncorrect) / (float64(duration) / 60e9))
		}
		if total > 0 {
			acc = float64(ncorrect) / float64(total) * 100
		}
		t.curWpm, t.curCpm, t.curAcc, t.curErrs = cpm/5, cpm, acc, nerrs
		// Show a live PB the instant it is beaten, so the BEST figure is
		// never stale while the test is still going.
		if t.bestWpm == 0 || t.curWpm > t.bestWpm {
			t.bestWpm, t.bestAcc = t.curWpm, t.curAcc
		}
		// Sample here so the equalizer, which draws before the numbers, sees
		// the current bucket rather than the previous frame's.
		t.sampleWpm(t.curWpm)

		// Draw the matrix and panel frame before the text so sprint text lands
		// inside the chatsubo body instead of under its border.
		t.drawKeyboard(target)
		t.drawChatsubo()

		cx := x
		cy := y
		inword := -1
		ahead := 0
		bottom := y + nr - 1

		for i := range text {
			style := t.defaultStyle

			if text[i] == '\n' {
				cy++
				cx = x
				if inword != -1 {
					inword++
				}
				continue
			}

			if i == idx {
				scr.ShowCursor(cx, cy)
				inword = 0
			}

			if i >= idx && t.HighlightChars > 0 && !t.WordByWord {
				if text[i] == ' ' {
					style = t.defaultStyle
				} else if ahead < t.HighlightChars {
					if ahead == 0 {
						style = t.currentWordStyle
					} else {
						style = t.nextWordStyle
					}
					ahead++
				}
			} else if i >= idx {
				inword = t.wordAheadStyle(text[i], inword, &style)
			} else if text[i] != typed[i] {
				if text[i] == ' ' {
					style = t.incorrectSpaceStyle
				} else {
					style = t.incorrectStyle
				}
			} else {
				style = t.correctStyle
			}

			// Keep every character inside the text's band: past its last row
			// the text is clipped, never drawn over another part.
			if cy > bottom {
				break
			}
			scr.SetContent(cx, cy, text[i], nil, style)
			cx++
		}

		if !t.sprint {
			aw, ah := calcStringDimensions(attribution)
			drawString(t.Scr, x+nc-aw, y+nr+1, attribution, -1, t.defaultStyle)

			if timeLimit != -1 && !startTime.IsZero() {
				remaining := timeLimit - time.Now().Sub(startTime)
				drawString(t.Scr, x+nc/2, y+nr+ah+1, "      ", -1, t.defaultStyle)
				drawString(t.Scr, x+nc/2, y+nr+ah+1, strconv.Itoa(int(remaining/1e9)+1), -1, t.defaultStyle)
			}

		}

		// The live stats render as their own line below the keys, so they
		// always draw.
		hudX, hudY := x+nc/2, statsY
		t.drawLiveStats(hudX, hudY, t.curWpm, t.curCpm, t.curAcc, t.curErrs)

		//Potentially inefficient, but seems to be good enough

		t.Scr.Show()
	}

	deleteWord := func() {
		if idx == 0 {
			return
		}

		idx--

		for idx > 0 && (text[idx] == ' ' || text[idx] == '\n') {
			idx--
		}

		for idx > 0 && text[idx] != ' ' && text[idx] != '\n' {
			idx--
		}

		if text[idx] == ' ' || text[idx] == '\n' {
			typed[idx] = text[idx]
			idx++
		}
	}

	tickerCloser := make(chan bool)

	//Inject nil events into the main event loop at regular invervals to force an update
	ticker := func() {
		for {
			select {
			case <-tickerCloser:
				return
			default:
			}

			time.Sleep(time.Duration(5e8))
			t.Scr.PostEventWait(nil)
		}
	}

	go ticker()
	defer close(tickerCloser)

	if startImmediately {
		startTime = time.Now()
	}

	t.Scr.Clear()
	for {
		redraw()

		ev := t.Scr.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventResize:
			rc = TyperResize
			return
		case *tcell.EventInterrupt:
			// Physical layer changed; the next loop redraws the matrix.
		case *tcell.EventKey:
			if runtime.GOOS != "windows" && ev.Key() == tcell.KeyBackspace { //Control+backspace on unix terms
				if !t.DisableBackspace {
					deleteWord()
				}
				continue
			}

			if startTime.IsZero() {
				startTime = time.Now()
			}

			switch key := ev.Key(); key {
			case tcell.KeyCtrlC:
				rc = TyperSigInt

				return
			case tcell.KeyEscape:
				rc = TyperEscape

				return
			case tcell.KeyCtrlL:
				t.Scr.Sync()

			case tcell.KeyRight:
				rc = TyperNext
				return

			case tcell.KeyLeft:
				rc = TyperPrevious
				return

			case tcell.KeyCtrlW:
				if !t.DisableBackspace {
					deleteWord()
				}

			case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3:
				if t.vil != nil {
					n := len(t.vil.layers)
					switch key {
					case tcell.KeyF1:
						if t.previewLayer < 0 {
							t.previewLayer = 0
						} else {
							t.previewLayer = (t.previewLayer + 1) % n
						}
					case tcell.KeyF2:
						if t.previewLayer < 0 {
							t.previewLayer = 0
						} else {
							t.previewLayer = (t.previewLayer - 1 + n) % n
						}
					case tcell.KeyF3:
						t.previewLayer = -1
					}
				}

			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if !t.DisableBackspace {
					if ev.Modifiers() == tcell.ModAlt || ev.Modifiers() == tcell.ModCtrl {
						deleteWord()
					} else {
						if idx == 0 {
							break
						}

						idx--

						for idx > 0 && text[idx] == '\n' {
							idx--
						}
					}
				}
			case tcell.KeyRune:
				if idx < len(text) {
					if t.SkipWord && ev.Rune() == ' ' {
						if idx > 0 && text[idx-1] == ' ' && text[idx] != ' ' { //Do nothing on word boundaries.
							break
						}

						for idx < len(text) && text[idx] != ' ' && text[idx] != '\n' {
							typed[idx] = 0
							idx++
						}

						if idx < len(text) {
							typed[idx] = text[idx]
							idx++
						}
					} else {
						if ev.Rune() != text[idx] {
							t.markWrong(ev.Rune())
						}
						typed[idx] = ev.Rune()
						idx++
					}

					for idx < len(text) && text[idx] == '\n' {
						typed[idx] = text[idx]
						idx++
					}
				}

				if idx == len(text) {
					calcStats()
					t.saveScore()
					return
				}
			}
		default: //tick
			if timeLimit != -1 && !startTime.IsZero() && timeLimit <= time.Now().Sub(startTime) {
				calcStats()
				t.saveScore()
				return
			}

			t.maybeChatsubo(text, typed, idx)
			t.blinkOn = !t.blinkOn
			redraw()
		}
	}
}
