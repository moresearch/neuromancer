package main

import (
	"fmt"
	"io"
	"io/ioutil"
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
// keyboard/chatsubo block.
const layoutSplitPad = 3

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
	vil          *vilKeyboard
	previewLayer int
	blinkOn      bool
	chatsubo     *chatsubo
	sprint       bool
	panelLabel   string

	// panelDrawn reports whether the chatsubo panel was actually painted
	// this frame; when it is not (e.g. too narrow a terminal) the live
	// stats fall back to the floating HUD above the text.
	panelDrawn   bool
	layoutTop    int
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
		currentWordStyle:    def.Foreground(hicol2),
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

// drawLiveStats renders the cyberpunk speed readout centered on cx at row y.
// The WPM figure is colour-zoned by speed and flashes on the moment the
// reading changes, like a car HUD.
func (t *typer) drawLiveStats(cx, y, wpm, cpm int, acc float64, errs int) {
	segs := t.statsSegments(wpm, cpm, acc, errs, t.statsFlashOn(), true, true)
	fieldW := 0
	for _, sg := range segs {
		fieldW += len([]rune(sg.s))
	}
	startX := cx - fieldW/2

	// Wipe the whole band (a little wider than the content) first: cx can
	// shift between frames and a wide rune leaves a trailing cell behind.
	for bx := startX - 1; bx <= startX+fieldW; bx++ {
		t.Scr.SetContent(bx, y, ' ', nil, t.defaultStyle)
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

	// Equalizer above the numbers.
	t.drawEqualizer(startX, y-1, fieldW, 1, t.defaultStyle)
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
		{" · ", sep},
	}
	if showCPM {
		segs = append(segs,
			statsSeg{fmt.Sprintf("%d CPM", cpm), sep},
			statsSeg{" · ", sep},
		)
	}
	segs = append(segs,
		statsSeg{fmt.Sprintf("%.0f%%", acc), t.defaultStyle.Foreground(accColor).Bold(true)},
		statsSeg{" · ", sep},
	)
	if showErrs {
		segs = append(segs,
			statsSeg{fmt.Sprintf("%d ERR", errs), t.defaultStyle.Foreground(t.neonAmber)},
		)
	}
	return append(segs, t.scoreSegments()...)
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
// to maxCols. If it still does not fit, it retries without the CPM figure and
// then without the error count so LAST/BEST stay visible.
func (t *typer) drawPanelStats(x, row, maxCols, wpm, cpm int, acc float64, errs int) {
	variants := []struct {
		showCPM, showErrs bool
	}{{false, true}, {false, false}}
	for _, v := range variants {
		if t.paintPanelStats(x, row, maxCols, t.statsSegments(wpm, cpm, acc, errs, t.statsFlashOn(), v.showCPM, v.showErrs)) {
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
	text := []rune(s)
	typed := make([]rune, len(text))

	if len(text) == 0 {
		rc = TyperComplete
		return
	}

	sw, sh := scr.Size()
	textW, nr := calcStringDimensions(s)

	t.layoutTop = -1
	m, kbOK := t.keyboardMetrics()

	nc := textW
	x := 0
	y := (sh - nr) / 2
	statsY := y - 2
	placed := false

	if t.sprint && kbOK {
		// Sprint mode: center the keyboard/chatsubo block and type inside the
		// chatsubo body.
		kbTop := (sh - m.height) / 2
		if kbTop < 2 {
			kbTop = 2
		}
		t.layoutTop = kbTop
		if m2, ok := t.keyboardMetrics(); ok {
			if tx, ty, tw, rows, ok := t.sprintTextArea(m2); ok {
				nc = tw
				x = tx
				if nr > rows {
					nr = rows
				}
				y = ty + (rows-nr)/2
				placed = true
			}
		}
	}

	if !placed {
		t.layoutTop = -1

		// The text area spans the full keyboard width, so its left and right
		// edges line up with the outer keycaps of both splits.
		if kbOK {
			if w := t.keyboardWidth(m); w > 0 {
				nc = w
			}
		}
		if nc > sw {
			nc = sw
		}
		if nc < 1 {
			nc = 1
		}
		x = (sw - nc) / 2

		// Center the whole block (text, attribution/timer, gap, keyboard) both
		// horizontally and vertically. The keyboard itself is centered by the
		// metrics code; here we position the text and the vertical offset.
		y = (sh - nr) / 2
		if kbOK {
			extra := 0
			if attribution != "" {
				_, ah := calcStringDimensions(attribution)
				extra += ah
			}
			if timeLimit != -1 {
				extra++
			}
			// Reserve room for the floating HUD unless the chatsubo panel
			// will carry the stats instead.
			statsRows := 0
			_, _, _, _, panelOK := t.chatPanelRect(m)
			if !t.sprint && !(t.chatsubo != nil && panelOK) {
				statsRows = 3 // blank spacer + equalizer + live stats line
			}
			contentH := statsRows + nr + extra + layoutSplitPad + m.height
			if contentH <= sh {
				y = (sh-contentH)/2 + statsRows
				statsY = y - 2
				t.layoutTop = y + nr + extra + layoutSplitPad
			} else {
				y = 0
				statsY = nr + 1
				t.layoutTop = sh - m.height
				if t.layoutTop < nr+extra+layoutSplitPad {
					t.layoutTop = nr + extra + layoutSplitPad
				}
			}
		}
		if y < 0 {
			y = 0
		}
		// Keep the stats block (equalizer + line) clear of the keyboard and
		// on screen when space is tight.
		if kbOK && statsY-1+m.height > sh {
			statsY = sh - m.height
		}
		if statsY < 1 {
			statsY = 1
		}
	}

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

			if i >= idx && t.HighlightChars > 0 {
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
				if text[i] == ' ' {
					inword++
				} else if inword == 0 {
					style = t.currentWordStyle
				} else if inword == 1 {
					style = t.nextWordStyle
				} else {
					style = t.defaultStyle
				}
			} else if text[i] != typed[i] {
				if text[i] == ' ' {
					style = t.incorrectSpaceStyle
				} else {
					style = t.incorrectStyle
				}
			} else {
				style = t.correctStyle
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

		// Show the HUD unless the chatsubo panel is carrying the stats.
		if !t.panelDrawn {
			hudX, hudY := x+nc/2, statsY
			if t.sprint {
				// Sprint draws its text inside the panel, so the HUD goes
				// just above the panel frame, centered on the screen.
				sw, _ := t.Scr.Size()
				hudX = sw / 2
				if m, ok := t.keyboardMetrics(); ok && m.top > 0 {
					hudY = m.top - 1
				} else {
					hudY = 1
				}
			}
			t.drawLiveStats(hudX, hudY, t.curWpm, t.curCpm, t.curAcc, t.curErrs)
		}

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
