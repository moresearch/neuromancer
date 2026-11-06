package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell"
)

// chatsuboMinAbove is the minimum keyboard top row where a four-row box
// plus attribution/timer can fit between the text and the keyboard.
const chatsuboMinAbove = 8

// The box has a fixed size. Its body holds exactly two text lines: the key
// guidance line and the model commentary line. Each line has
// chatsuboBoxWidth-4 usable runes; every string is clipped to that capacity.
const (
	chatsuboBoxWidth   = 72
	chatsuboBoxHeight  = 4
	chatsuboReplyRunes = chatsuboBoxWidth - 4
)

type chatsuboSnapshot struct {
	Seconds  int
	WPM      int
	Accuracy float64
	Errors   int
	Chars    int
	Recent   string
}

type chatsubo struct {
	mu    sync.Mutex
	url   string
	model string

	interval time.Duration
	comment  string
	busy     bool

	lastCorrect int
	lastErrors  int
	lastTime    time.Time
}

func newChatsubo(url, model string, interval time.Duration) *chatsubo {
	if interval <= 0 {
		interval = 120 * time.Second
	}
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	return &chatsubo{
		url:      url,
		model:    model,
		interval: interval,
		comment:  "jacked in. listening.",
		lastTime: time.Now(),
	}
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
	system := fmt.Sprintf(`You are Chatsubo, a laconic Gibson-style AI typing coach.
Comment on the typist's last interval in one short sentence, max %d characters.
Be noir/cyberpunk, specific, and not cruel.
No markdown, no emoji, no quotation marks.`, chatsuboReplyRunes)

	user := fmt.Sprintf(
		"interval_seconds=%d\nwpm=%d\naccuracy=%.1f\nerrors=%d\nchars=%d\nrecent_typed=%q",
		snap.Seconds, snap.WPM, snap.Accuracy, snap.Errors, snap.Chars, snap.Recent,
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
	snap := chatsuboSnapshot{
		Seconds:  int(elapsed.Seconds()),
		WPM:      wpm,
		Accuracy: acc,
		Errors:   dErrors,
		Chars:    chars,
		Recent:   recentTyped(typed[:idx], 32),
	}

	go c.request(snap, t.Scr)
}

// chatsuboPlacement centers the Chatsubo box in the gap between the text
// block and the keyboard. It returns -1 when there is not enough room to draw
// the box without overlapping the text.
func (t *typer) chatsuboPlacement(textY, textLines int, attribution string, timeLimit time.Duration) int {
	_, sh := t.Scr.Size()
	_, attribLines := calcStringDimensions(attribution)

	blockBottom := textY + textLines - 1
	if attribution != "" {
		blockBottom = textY + textLines + attribLines
	}
	if timeLimit != -1 {
		if row := textY + textLines + attribLines + 1; row > blockBottom {
			blockBottom = row
		}
	}

	top := sh
	if _, kt, ok := t.keyboardVisible(); ok {
		top = kt
	}

	gapTop := blockBottom + 1
	maxTop := top - chatsuboBoxHeight
	if maxTop < gapTop || maxTop < 1 {
		return -1
	}

	y := gapTop + (maxTop-gapTop)/2
	if y < 1 {
		y = 1
	}
	if y > maxTop {
		y = maxTop
	}
	return y
}

func (t *typer) drawChatsubo() {
	if t.chatsubo == nil || t.Scr == nil {
		return
	}

	sw, sh := t.Scr.Size()
	width := chatsuboBoxWidth
	if width > sw-8 {
		width = sw - 8
	}
	if width < 30 || sh < chatsuboBoxHeight+1 {
		return
	}
	x := (sw - width) / 2

	// Prefer the gap computed for the current text block. When drawing
	// without a started segment (for example in tests), fall back to sitting
	// directly above the keyboard, or at the bottom when no keyboard fits.
	y := sh - chatsuboBoxHeight
	if _, top, ok := t.keyboardVisible(); ok {
		y = top - chatsuboBoxHeight
	}
	if t.chatPlaced {
		if t.chatTop < 0 {
			// Not enough room above the keyboard; use the keyboard's
			// bottom padding so the box stays visible.
			y = sh - chatsuboBoxHeight
		} else {
			y = t.chatTop
		}
	}
	if y < 1 {
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
	bodyCols := inner - 2
	drawBoxLine := func(row int, text string, style tcell.Style) {
		r := []rune(strings.Join(strings.Fields(text), " "))
		if len(r) > bodyCols {
			r = r[:bodyCols]
		}
		line := "│ " + string(r) + strings.Repeat(" ", bodyCols-len(r)) + " │"
		drawString(t.Scr, x, row, line, -1, style)
	}

	drawBoxLine(y+1, t.hintText, guideStyle)
	drawBoxLine(y+2, comment, t.defaultStyle)

	bottom := "╰" + strings.Repeat("─", width-2) + "╯"
	drawString(t.Scr, x, y+chatsuboBoxHeight-1, bottom, -1, border)
}
