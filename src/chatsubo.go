package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell"
)

// chatsuboMinAbove is the minimum keyboard top row where a three-row box
// plus attribution/timer can fit between the text and the keyboard.
const chatsuboMinAbove = 8

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

func (c *chatsubo) ask(snap chatsuboSnapshot) (string, error) {
	system := `You are Chatsubo, a laconic Gibson-style AI typing coach.
Comment on the typist's last interval in 1-2 short sentences, max 200 characters.
Be noir/cyberpunk, specific, and not cruel.
No markdown, no emoji, no quotation marks.`

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
		return "silence on the wire. keep typing.", nil
	}
	r := []rune(text)
	if len(r) > 200 {
		text = string(r[:199]) + "…"
	}
	return text, nil
}

func (c *chatsubo) request(snap chatsuboSnapshot, scr tcell.Screen) {
	comment, err := c.ask(snap)

	c.mu.Lock()
	if err != nil {
		msg := err.Error()
		if r := []rune(msg); len(r) > 70 {
			msg = string(r[:70]) + "…"
		}
		c.comment = "link lost: " + msg
	} else {
		c.comment = comment
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

// chatsuboPlacement centers the three-row Chatsubo box in the gap between
// the text block and the keyboard. It returns -1 when there is not enough
// room to draw the box without overlapping the text.
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

	const boxHeight = 3
	gapTop := blockBottom + 1
	maxTop := top - boxHeight
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
	width := sw - 8
	if width > 72 {
		width = 72
	}
	if width < 30 || sh < 6 {
		return
	}
	x := (sw - width) / 2

	// Prefer the gap computed for the current text block. When drawing
	// without a started segment (for example in tests), fall back to sitting
	// directly above the keyboard, or at the bottom when no keyboard fits.
	y := sh - 3
	if _, top, ok := t.keyboardVisible(); ok {
		y = top - 3
	}
	if t.chatPlaced {
		if t.chatTop < 0 {
			// Not enough room above the keyboard; use the keyboard's
			// bottom padding so the box stays visible.
			y = sh - 3
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
	label := " chatsubo "
	top := "╭" + label + strings.Repeat("─", width-2-len([]rune(label))) + "╮"
	drawString(t.Scr, x, y, top, -1, border)

	inner := width - 2
	r := []rune(strings.Join(strings.Fields(comment), " "))
	if len(r) > inner-2 {
		r = r[:inner-2]
	}
	line := "│ " + string(r) + strings.Repeat(" ", inner-2-len(r)) + " │"
	drawString(t.Scr, x, y+1, line, -1, t.defaultStyle)

	bottom := "╰" + strings.Repeat("─", width-2) + "╯"
	drawString(t.Scr, x, y+2, bottom, -1, border)
}
