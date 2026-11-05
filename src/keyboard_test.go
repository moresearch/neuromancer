package main

import (
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

func TestKbFind(t *testing.T) {
	tests := []struct {
		r        rune
		row, col int
		ok       bool
	}{
		{'q', 1, 0, true},
		{'f', 1, 2, true},
		{';', 1, 9, true},
		{':', 1, 9, true},
		{'[', 1, 10, true},
		{'{', 1, 10, true},
		{'\\', 1, 12, true},
		{'|', 1, 12, true},
		{'a', 2, 0, true},
		{'g', 2, 4, true},
		{'m', 2, 5, true},
		{'o', 2, 9, true},
		{'\'', 2, 10, true},
		{'"', 2, 10, true},
		{'z', 3, 0, true},
		{'d', 3, 3, true},
		{'h', 3, 6, true},
		{'/', 3, 9, true},
		{'?', 3, 9, true},
		{'A', 2, 0, true},
		{'!', 0, 0, true},
		{'?', 3, 9, true},
		{'~', 0, 0, false},
	}
	for _, c := range tests {
		r, col, ok := kbFind(c.r)
		if ok != c.ok || (ok && (r != c.row || col != c.col)) {
			t.Errorf("kbFind(%q) = %d,%d,%v want %d,%d,%v", c.r, r, col, ok, c.row, c.col, c.ok)
		}
	}
}

func bgOf(s tcell.Style) tcell.Color {
	_, bg, _ := s.Decompose()
	return bg
}

func newTestTyper(scr tcell.Screen) *typer {
	return NewTyper(scr, true, tcell.ColorDefault, tcell.ColorDefault,
		tcell.ColorWhite, tcell.ColorGreen, tcell.ColorGreen, tcell.ColorMaroon)
}

func TestDrawKeyboardHighlights(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(80, 24)

	kb := newTestTyper(scr)
	keyWidth, top, ok := kb.keyboardVisible()
	if !ok || keyWidth != 3 {
		t.Fatalf("keyboard not visible: %d %d %v", keyWidth, top, ok)
	}
	width := kbWidth(keyWidth)
	x0 := (80 - width) / 2

	kb.drawKeyboard('f')
	checkBg := func(label string, row, col int, want tcell.Color) {
		x := x0 + kbOffset(row, keyWidth) + col*(keyWidth+1)
		_, _, style, _ := scr.GetContent(x+1, top+1+row)
		_, bg, _ := style.Decompose()
		if bg != want {
			t.Errorf("%s: bg=%v want %v", label, bg, want)
		}
	}
	checkBg("target f", 1, 2, bgOf(kbTargetStyle))
	checkBg("non-target q", 1, 0, bgOf(kb.defaultStyle))

	kb.wrongKey = 'j'
	kb.wrongUntil = time.Now().Add(time.Second)
	kb.drawKeyboard('f')
	checkBg("target f", 1, 2, bgOf(kbTargetStyle))
	checkBg("wrong j", 1, 5, bgOf(kbErrorStyle))
}
