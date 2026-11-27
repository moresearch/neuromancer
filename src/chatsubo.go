package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell"
)

// The box is a band above the keys, below the practice text's band. It carries
// the guidance line and the LLM commentary; the stats (equalizer plus readout)
// render as a separate band beneath the keys. A book sprint uses the same box,
// with the countdown in its rule instead of commentary.
const (
	// chatsuboBoxWidth is the upper bound on the box in its standard place; it
	// is drawn exactly as wide as the keyboard, so the two share one rectangle.
	// 60 leaves 56 usable columns for the guidance and commentary; the stats
	// are a separate centred line below.
	chatsuboBoxWidth = 60
	// chatsuboBoxMinWidth is the narrowest readable box; below it the box is
	// dropped and only the guidance line remains.
	chatsuboBoxMinWidth   = 12
	chatsuboCompactHeight = 4

	// Two columns of horizontal padding inside each border.
	chatsuboBodyCols = chatsuboBoxWidth - 6

	// Guidance uses one body row; the LLM can fill the rest of the panel.
	chatsuboReplyRunes = 7 * chatsuboBodyCols
)

type chatsuboSnapshot struct {
	Seconds  int
	WPM      int
	Accuracy float64
	Errors   int
	Chars    int
	Recent   string
	Hint     string
	Target   string
	Finger   string
}

type chatsubo struct {
	mu    sync.Mutex
	url   string
	model string

	interval time.Duration
	comment  string
	busy     bool
	layout   string
	history  []string

	lastCorrect int
	lastErrors  int
	lastTime    time.Time
}

func newChatsubo(url, model string, interval time.Duration, layout ...string) *chatsubo {
	if interval <= 0 {
		interval = 120 * time.Second
	}
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	c := &chatsubo{
		url:      url,
		model:    model,
		interval: interval,
		comment:  "jacked in. listening.",
		lastTime: time.Now(),
	}
	if len(layout) > 0 {
		c.layout = layout[0]
	}
	return c
}

// chatsuboLayoutSummary gives the model a compact description of the loaded
// .vil, including every layer matrix and the key which activates each layer.
func chatsuboLayoutSummary(kb *vilKeyboard) string {
	if kb == nil {
		return "built-in Colemak-DH matrix (no .vil loaded)\n" + fingerLegend
	}

	var b strings.Builder
	b.WriteString(kb.info())
	b.WriteString("\nlayer activators:\n")
	for i := 1; i < len(kb.layers); i++ {
		if a := kb.findLayerActivator(i); a != nil {
			fmt.Fprintf(&b, "- %s (layer %d): hold or tap %s (row %d, col %d)\n",
				kb.hintLayerName(i), i, a.action.label, a.row, a.col)
		} else {
			fmt.Fprintf(&b, "- %s (layer %d): no activator key found\n", kb.hintLayerName(i), i)
		}
	}
	b.WriteString("labels: C/A/G/S prefixes are Ctrl/Alt/Gui/Shift home-row mods; " +
		"MO/LT/TG/TO/DF/TT are layer keys; ___ is transparent; . is unused.\n")
	b.WriteString(fingerLegend + "\n")
	return b.String()
}

func (c *chatsubo) resetSegment() {
	c.mu.Lock()
	c.lastCorrect = 0
	c.lastErrors = 0
	c.lastTime = time.Now()
	c.mu.Unlock()
}

// chatsuboClipRunes trims s to at most max runes. When it truncates, the
// ellipsis is included in max so the result never exceeds the box capacity.
func chatsuboClipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return string(r[:1])
	}
	return string(r[:max-1]) + "…"
}

func (c *chatsubo) ask(snap chatsuboSnapshot) (string, error) {
	c.mu.Lock()
	layout := c.layout
	history := append([]string(nil), c.history...)
	c.mu.Unlock()

	system := fmt.Sprintf(`You are Chatsubo, a laconic Gibson-style AI typing coach.
The typist's Vial layout is supplied in every request. Study it and teach one
concrete thing they can do on a specific layer: name the layer, how to
activate it, and the key to press. Always name the finger for the target in
plain terms: left or right pinky, ring, middle, index, or thumb. Do not use
numeric finger codes. Give ergonomic Miryoku/Colemak-DH advice in that
vocabulary: the home row, the finger's home column, inner or outer column
reaches, home-row mods, thumb keys, and layers. Keep the hand relaxed and
prefer a layer or thumb key over stretching. Then add one short remark about
the last interval if useful. Never invent keys, layers, or shortcuts not
present in the layout. Keep the whole reply to one to three short sentences,
max %d characters.
Be noir/cyberpunk, specific, and not cruel.
No markdown, no emoji, no quotation marks.`, chatsuboReplyRunes)

	if layout == "" {
		layout = "no layout information loaded"
	}
	tips := "none"
	if len(history) > 0 {
		tips = "- " + strings.Join(history, "\n- ")
	}

	user := fmt.Sprintf(
		"layout_summary:\n%s\n\ncurrent_guidance: %s\ncurrent_target: %q\ntarget_finger: %s\n\ninterval_seconds=%d\nwpm=%d\naccuracy=%.1f\nerrors=%d\nchars=%d\nrecent_typed=%q\n\nprevious_tips:\n%s\n\nDo not repeat a previous tip.",
		layout, snap.Hint, snap.Target, snap.Finger,
		snap.Seconds, snap.WPM, snap.Accuracy, snap.Errors, snap.Chars, snap.Recent,
		tips,
	)

	text, err := requestOllamaText(c.url, c.model, system, user, 512)
	if err != nil {
		return "", err
	}

	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		text = "silence on the wire. keep typing."
	}
	return chatsuboClipRunes(text, chatsuboReplyRunes), nil
}

func (c *chatsubo) request(snap chatsuboSnapshot, scr tcell.Screen) {
	comment, err := c.ask(snap)

	c.mu.Lock()
	if err != nil {
		c.comment = chatsuboClipRunes("link lost: "+err.Error(), chatsuboReplyRunes)
	} else {
		c.comment = chatsuboClipRunes(comment, chatsuboReplyRunes)
		c.history = append(c.history, c.comment)
		if len(c.history) > 3 {
			c.history = c.history[len(c.history)-3:]
		}
	}
	c.busy = false
	c.mu.Unlock()

	if scr != nil {
		_ = scr.PostEvent(nil)
	}
}

func recentTyped(typed []rune, n int) string {
	var b []rune
	for _, r := range typed {
		switch r {
		case 0, '\n', '\r', '\t':
			b = append(b, ' ')
		default:
			b = append(b, r)
		}
	}
	s := strings.TrimSpace(string(b))
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return string(r)
}

func (t *typer) maybeChatsubo(text, typed []rune, idx int) {
	c := t.chatsubo
	if c == nil || idx <= 0 || idx > len(text) || idx > len(typed) {
		return
	}

	now := time.Now()
	c.mu.Lock()
	if c.busy || now.Sub(c.lastTime) < c.interval {
		c.mu.Unlock()
		return
	}

	nerrs, ncorrect := 0, 0
	for i := 0; i < idx; i++ {
		if text[i] != '\n' {
			if text[i] == typed[i] {
				ncorrect++
			} else {
				nerrs++
			}
		}
	}

	dCorrect := ncorrect - c.lastCorrect
	dErrors := nerrs - c.lastErrors
	elapsed := now.Sub(c.lastTime)
	c.lastCorrect = ncorrect
	c.lastErrors = nerrs
	c.lastTime = now

	// Backspacing can make the deltas negative; only report progress made
	// during this interval.
	if dCorrect < 0 {
		dCorrect = 0
	}
	if dErrors < 0 {
		dErrors = 0
	}
	if dCorrect+dErrors == 0 || elapsed <= 0 {
		c.mu.Unlock()
		return
	}
	c.busy = true
	c.mu.Unlock()

	chars := dCorrect + dErrors
	wpm := int((float64(dCorrect) / 5) / elapsed.Minutes())
	acc := float64(dCorrect) / float64(chars) * 100

	target := rune(0)
	if idx < len(text) {
		target = text[idx]
	}
	targetText := ""
	if target != 0 {
		targetText = string(target)
	}
	hint := t.hintFor(target)
	if t.layerTracker != nil && t.vil != nil {
		if layer := t.layerTracker.Layer(); layer >= 0 && layer < len(t.vil.layers) {
			hint = t.withFinger(t.vil.hintForLayer("held", layer, target), target)
		}
	}
	snap := chatsuboSnapshot{
		Seconds:  int(elapsed.Seconds()),
		WPM:      wpm,
		Accuracy: acc,
		Errors:   dErrors,
		Chars:    chars,
		Recent:   recentTyped(typed[:idx], 32),
		Hint:     hint,
		Target:   targetText,
		Finger:   fingerLabel(t.fingerForTarget(target)),
	}

	go c.request(snap, t.Scr)
}

// blockBoxHeight is the box's height in the centred block: the same ladder as
// blockBoxHeight is the box's height in the centred block: the same ladder as
// chatsuboBoxHeight, spending the block's spare rows on the empty body first,
// so the parenthesised box grows before the board does.
func (t *typer) blockBoxHeight(sh int) int {
	return t.blockBoxHeightFor(sh, boardMinRows())
}

// blockBoxHeightFor is blockBoxHeight with the board's drawn height known, so
// the box can grow into the rows the block leaves before the caps do. It takes
// the height as an argument because keyboardMetrics needs the box to size the
// board.
func (t *typer) blockBoxHeightFor(sh, boardH int) int {
	boxH := chatsuboBoxHeight(sh)
	if boxH > sh-2 {
		boxH = sh - 2
	}
	if boxH < 3 {
		boxH = 3
	}
	used := boxH + layoutBandPad*2 + layoutTextRows + boardH + layoutStatsRows
	if spare := blockRoom(sh) - used; spare > 0 {
		if room := chatsuboMaxHeight - boxH; spare > room {
			spare = room
		}
		boxH += spare
	}
	return boxH
}

// chatsuboMaxHeight is the tallest box body: the title and bottom border rows
// plus room for the practice text and its padding.
const chatsuboMaxHeight = 8

// chatsuboBoxHeight sizes the box to the window: room for the padded practice
// text in a tall window, a compact body in a short one, and never more than a
// third of it so the board keeps its room.
func chatsuboBoxHeight(screenHeight int) int {
	h := 3 // title, one body row
	if screenHeight >= 36 {
		h = chatsuboMaxHeight
	} else if screenHeight >= 24 {
		h = chatsuboCompactHeight
	}
	if max := screenHeight / 3; h > max {
		h = max
	}
	if h < 3 {
		h = 3 // title, one body row
	}
	return h
}

// chatBand returns the rectangle of the Chatsubo box in the centred block:
// the block's top band, as wide as the board and centred on it, so the box and
// the keys read as one rectangle.
func (t *typer) chatBand() (x, y, width, height int, ok bool) {
	sw, sh := t.Scr.Size()
	// The box follows the board: exactly as wide, on the same columns, so the
	// two read as one block on any terminal. On a very narrow window the box
	// gives up its sides before the board does.
	width = chatsuboBoxWidth
	x = (sw - width) / 2
	if bx, bw, kbOK := t.boardBand(); kbOK && bw > 0 {
		// The box is exactly as wide as the board (both halves plus the split
		// gap) and sits on the same columns, so the panel and the keys read as
		// one rectangle.
		x = bx
		width = bw
	}
	if width > sw-2 {
		width = sw - 2
		x = 1
	}
	if width < chatsuboBoxMinWidth {
		return 0, 0, 0, 0, false
	}
	y = t.boxTop
	height = t.parts.chat.H
	if height < 3 {
		height = 3
	}
	if y < 0 {
		y = 0
	}
	if y+height > sh {
		height = sh - y
	}
	if height < 3 {
		return 0, 0, 0, 0, false
	}
	return x, y, width, height, true
}

// boardSplit reports whether the board is drawn as two halves on the matrix's
// own rows rather than through the compact single-line path.
func (t *typer) boardSplit(m kbMetrics) bool {
	return t.vil != nil && t.vil.hasBoard && !t.compactBoard(m)
}

// boardBand is the board's drawn column band and width.
func (t *typer) boardBand() (x, width int, ok bool) {
	sw, _ := t.Scr.Size()
	m, mOK := t.keyboardMetrics()
	if !mOK {
		return 0, 0, false
	}
	width = t.keyboardWidth(m)
	if width <= 0 {
		return 0, 0, false
	}
	return (sw - width) / 2, width, true
}

// sprintTextArea is the typing area in the practice text band under the
// sprint panel: the text's own band, inset by one column each side.
func (t *typer) sprintTextArea(m kbMetrics) (x, y, width, rows int, ok bool) {
	b := t.parts.text
	rows = b.H
	if rows < 1 {
		return 0, 0, 0, 0, false
	}
	boardW := t.keyboardWidth(m)
	if boardW <= 0 {
		return 0, 0, 0, 0, false
	}
	width = boardW - 2
	if width < 1 {
		return 0, 0, 0, 0, false
	}
	x = (t.screenWidth() - width) / 2
	y = b.Y
	return x, y, width, rows, true
}

// screenWidth returns the window's width.
func (t *typer) screenWidth() int {
	sw, _ := t.Scr.Size()
	return sw
}

// showSprintMessage draws the sprint block with a status line in the box's
// commentary row while the next summary is being generated.
func (t *typer) showSprintMessage(msg string) {
	if t.Scr == nil || !t.sprint {
		return
	}
	t.layoutTop = -1
	t.panelLabel = " chatsubo "
	t.sprintMessage = msg
	t.drawKeyboard(0)
	t.drawChatsubo()
	t.Scr.Show()
}

func (t *typer) drawChatsubo() {
	if t.Scr == nil {
		return
	}
	if t.chatsubo == nil && !t.sprint {
		return
	}

	x, y, width, height, ok := t.chatBand()
	if !ok {
		return
	}

	contentRows := height - 2
	if contentRows < 1 {
		return
	}

	border := t.defaultStyle.Foreground(tcell.ColorTeal)
	// The rule runs through the title, so the label is always readable even
	// when the box is only as wide as a short board.
	label := " chatsubo"
	if width >= 24 {
		label = " chatsubo "
	}
	drawTopRule(t.Scr, x, y, width, label, border)
	// In a sprint the countdown sits right-aligned in the rule: there is room
	// above the board now that the panel is a band, not a gap between hands.
	// panelLabel is " chatsubo Ns "; the countdown is what follows the title.
	if t.sprint {
		countdown := strings.TrimSpace(t.panelLabel)
		countdown = strings.TrimSpace(strings.TrimPrefix(countdown, "chatsubo"))
		if countdown != "" {
			tail := []rune(countdown)
			tx := x + width - 2 - len(tail)
			minX := x + 1 + len([]rune(label))
			if tx >= minX {
				drawString(t.Scr, tx, y, countdown, -1, border.Bold(true))
			}
		}
	}

	drawBlank := func(row int) {
		line := "│" + strings.Repeat(" ", width-2) + "│"
		drawString(t.Scr, x, row, line, -1, t.defaultStyle)
	}

	// The box body holds the practice text, which the typing loop draws over
	// these rows after the frame. Leave the interior blank with its side
	// borders so the text lands inside the box.
	for row := y + 1; row < y+height-1; row++ {
		drawBlank(row)
	}
	drawString(t.Scr, x, y+height-1, "╰"+strings.Repeat("─", width-2)+"╯", -1, border)
}

// drawTopRule draws a panel's top border with the title set into the rule.
func drawTopRule(scr tcell.Screen, x, y, width int, title string, style tcell.Style) {
	if width < 2 {
		return
	}
	line := make([]rune, width)
	line[0], line[width-1] = '╭', '╮'
	for i := 1; i < width-1; i++ {
		line[i] = '─'
	}
	// A narrow box keeps the end of the title, which is the part that names
	// the panel.
	title = "─" + title + "─"
	if tr := []rune(title); len(tr) > width-2 {
		title = string(tr[len(tr)-(width-2):])
	}
	for i, r := range []rune(title) {
		if 1+i >= width-1 {
			break
		}
		line[1+i] = r
	}
	drawString(scr, x, y, string(line), -1, style)
}
