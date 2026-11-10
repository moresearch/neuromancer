package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell"
)

// The box is a panel in the gap between the keyboard halves.
const (
	chatsuboBoxWidth       = 40
	chatsuboMinScreenWidth = 100
	chatsuboCompactHeight  = 4
	chatsuboFullHeight     = 8
	chatsuboMaxKeyRows     = 3
	chatsuboExtraRows      = 3

	// Two columns of horizontal padding inside each border, and one blank
	// padding row above and below the full-size body.
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

// chatPanelGap returns the left edge of the gap between the keyboard halves
// and whether the current layout is split.
func (t *typer) chatPanelGap(m kbMetrics) (int, bool) {
	sw, _ := t.Scr.Size()
	if t.vil != nil {
		kb := t.vil
		width := kb.width(m.keyWidth, m.keyGap, m.splitGap)
		x0 := (sw - width) / 2
		switch {
		case kb.splitCols:
			left := kb.halfCols*m.keyWidth + (kb.halfCols-1)*m.keyGap
			return x0 + left, true
		case kb.halfRows > 0:
			left := kb.halfWidth(m.keyWidth, m.keyGap)
			return x0 + left, true
		}
		return 0, false
	}

	if kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)+2 > sw {
		return 0, false
	}
	width := kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)
	x0 := (sw - width) / 2
	return x0 + kbSplitLeftWidth(m.keyWidth, m.keyGap), true
}

func (t *typer) drawChatsubo() {
	if t.chatsubo == nil || t.Scr == nil {
		return
	}

	sw, sh := t.Scr.Size()
	m, ok := t.keyboardMetrics()
	if !ok {
		return
	}
	gapX, split := t.chatPanelGap(m)
	if !split {
		return
	}

	width := chatsuboBoxWidth
	if width > m.splitGap-2 {
		width = m.splitGap - 2
	}
	if width > sw-8 {
		width = sw - 8
	}
	if width < 12 {
		return
	}

	height := chatsuboMaxKeyRows*m.keyH + (chatsuboMaxKeyRows-1)*m.rowGap + chatsuboExtraRows
	if height < chatsuboCompactHeight {
		height = chatsuboCompactHeight
	}
	if height > m.height {
		height = m.height
	}
	if height > sh-m.top {
		height = sh - m.top
	}
	if height < chatsuboCompactHeight+1 {
		return
	}

	y := m.top + (m.height-height)/2
	x := gapX + (m.splitGap-width)/2
	if x < 1 || x+width > sw-1 {
		return
	}

	contentRows := height - 2
	topPad, bottomPad := 0, 0
	if height >= chatsuboFullHeight {
		contentRows = height - 4
		topPad, bottomPad = 1, 1
	}
	if contentRows < 1 {
		return
	}

	t.chatsubo.mu.Lock()
	comment := t.chatsubo.comment
	t.chatsubo.mu.Unlock()
	if comment == "" {
		comment = "jacked in. listening."
	}

	border := t.defaultStyle.Foreground(tcell.ColorTeal)
	guideStyle := t.defaultStyle.Foreground(tcell.ColorTeal).Bold(true)
	label := " chatsubo "
	top := "╭" + label + strings.Repeat("─", width-2-len([]rune(label))) + "╮"
	drawString(t.Scr, x, y, top, -1, border)

	inner := width - 2
	bodyCols := inner - 4 // two columns of padding on each side
	if bodyCols < 1 {
		return
	}
	drawBoxLine := func(row int, text string, style tcell.Style) {
		r := []rune(strings.Join(strings.Fields(text), " "))
		if len(r) > bodyCols {
			r = r[:bodyCols]
		}
		line := "│  " + string(r) + strings.Repeat(" ", bodyCols-len(r)) + "  │"
		drawString(t.Scr, x, row, line, -1, style)
	}
	drawBlank := func(row int) {
		line := "│" + strings.Repeat(" ", width-2) + "│"
		drawString(t.Scr, x, row, line, -1, t.defaultStyle)
	}

	row := y + 1
	if topPad > 0 {
		drawBlank(row)
		row++
	}

	rowsLeft := contentRows
	guidance := strings.Join(strings.Fields(t.hintText), " ")
	if guidance != "" && rowsLeft > 0 {
		drawBoxLine(row, chatsuboClipRunes(guidance, bodyCols), guideStyle)
		row++
		rowsLeft--
	}

	comment = strings.Join(strings.Fields(comment), " ")
	if comment == "" {
		comment = "jacked in. listening."
	}
	if rowsLeft > 0 {
		for _, line := range strings.Split(wordWrap(comment, bodyCols), "\n") {
			if rowsLeft == 0 {
				break
			}
			drawBoxLine(row, strings.TrimSpace(line), t.defaultStyle)
			row++
			rowsLeft--
		}
	}
	for rowsLeft > 0 {
		drawBoxLine(row, "", t.defaultStyle)
		row++
		rowsLeft--
	}

	if bottomPad > 0 {
		drawBlank(row)
		row++
	}

	bottom := "╰" + strings.Repeat("─", width-2) + "╯"
	drawString(t.Scr, x, y+height-1, bottom, -1, border)
}
