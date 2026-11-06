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

func fgOf(s tcell.Style) tcell.Color {
	fg, _, _ := s.Decompose()
	return fg
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
	m, ok := kbBuiltinMetrics(80, 24)
	if !ok || m.keyWidth != 4 {
		t.Fatalf("keyboard not visible: %+v %v", m, ok)
	}
	width := kbSplitWidth(m.keyWidth)
	leftWidth := kbSplitLeftWidth(m.keyWidth)
	x0 := (80 - width) / 2
	keyX := func(row, col int) int {
		if col < kbSplitCol {
			return x0 + kbOffset(row, m.keyWidth) + col*(m.keyWidth+m.keyGap)
		}
		return x0 + leftWidth + kbSplitGap + (col-kbSplitCol)*(m.keyWidth+m.keyGap)
	}
	keyY := func(row int) int {
		return m.top + row*(m.keyH+m.rowGap)
	}

	kb.drawKeyboard('f')
	checkBg := func(label string, row, col int, want tcell.Color) {
		_, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row))
		_, bg, _ := style.Decompose()
		if bg != want {
			t.Errorf("%s: bg=%v want %v", label, bg, want)
		}
	}
	checkFg := func(label string, row, col int, want tcell.Color) {
		_, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row))
		if got := fgOf(style); got != want {
			t.Errorf("%s: fg=%v want %v", label, got, want)
		}
	}
	checkFg("target f border", 1, 2, tcell.ColorGreen)
	_, _, style, _ := scr.GetContent(keyX(1, 2)+m.keyWidth/2, keyY(1)+1)
	if got := bgOf(style); got != bgOf(kb.defaultStyle) {
		t.Errorf("target f interior bg=%v, want %v", got, bgOf(kb.defaultStyle))
	}
	checkBg("non-target q", 1, 0, bgOf(kb.defaultStyle))

	kb.blinkOn = false
	kb.drawKeyboard('f')
	checkFg("target f blink-off border", 1, 2, tcell.ColorWhite)
	kb.blinkOn = true

	_, _, style, _ = scr.GetContent(keyX(1, 6)+m.keyWidth/2, keyY(1))
	_, _, attrs := style.Decompose()
	if attrs&tcell.AttrDim == 0 {
		t.Error("inactive right half is not dimmed")
	}

	kb.wrongKey = 'j'
	kb.wrongUntil = time.Now().Add(time.Second)
	kb.drawKeyboard('f')
	checkFg("target f border", 1, 2, tcell.ColorGreen)
	checkBg("wrong j", 1, 5, bgOf(kbErrorStyle))

	kb.wrongUntil = time.Time{}
	kb.drawKeyboard('j')
	checkFg("target j border", 1, 5, tcell.ColorGreen)
	_, _, style, _ = scr.GetContent(keyX(1, 0)+m.keyWidth/2, keyY(1))
	_, _, attrs = style.Decompose()
	if attrs&tcell.AttrDim == 0 {
		t.Error("inactive left half is not dimmed")
	}
}

func TestBuiltinHomeKeys(t *testing.T) {
	if !kbBuiltinHomeKey(2, 3) {
		t.Error("t should be a home key")
	}
	if kbBuiltinHomeKey(2, 4) {
		t.Error("g should not be a home key")
	}
	if !kbBuiltinHomeKey(2, 6) {
		t.Error("n should be a home key")
	}
	if kbBuiltinHomeKey(2, 5) {
		t.Error("m should not be a home key")
	}

	if r, c, ok := kbBuiltinTargetHome(2, 4); !ok || r != 2 || c != 3 {
		t.Fatalf("g home = %d,%d,%v; want 2,3", r, c, ok)
	}
	if r, c, ok := kbBuiltinTargetHome(2, 5); !ok || r != 2 || c != 6 {
		t.Fatalf("m home = %d,%d,%v; want 2,6", r, c, ok)
	}
}

func TestBlinkStyle(t *testing.T) {
	if targetStyle(true, tcell.StyleDefault) != kbTargetStyle {
		t.Error("blink-on target style is wrong")
	}
	if targetStyle(false, tcell.StyleDefault) != kbTargetBlinkStyle {
		t.Error("blink-off target style is wrong")
	}
}

func TestBuiltinTargetFingerHome(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(80, 24)

	typ := newTestTyper(scr)
	m, ok := kbBuiltinMetrics(80, 24)
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	width := kbSplitWidth(m.keyWidth)
	leftWidth := kbSplitLeftWidth(m.keyWidth)
	x0 := (80 - width) / 2
	keyX := func(row, col int) int {
		if col < kbSplitCol {
			return x0 + kbOffset(row, m.keyWidth) + col*(m.keyWidth+m.keyGap)
		}
		return x0 + leftWidth + kbSplitGap + (col-kbSplitCol)*(m.keyWidth+m.keyGap)
	}
	keyY := func(row int) int {
		return m.top + row*(m.keyH+m.rowGap)
	}
	typ.drawKeyboard('g')
	_, _, style, _ := scr.GetContent(keyX(2, 3)+m.keyWidth/2, keyY(2))
	if got, want := fgOf(style), fgOf(typ.defaultStyle); got != want {
		t.Errorf("t resting border fg = %v, want normal %v", got, want)
	}
	_, _, style, _ = scr.GetContent(keyX(2, 3)+m.keyWidth/2, keyY(2)+1)
	_, _, attrs := style.Decompose()
	if fg := fgOf(style); fg != tcell.ColorGreen {
		t.Errorf("t label fg = %v, want green", fg)
	}
	if attrs&tcell.AttrDim == 0 {
		t.Error("t label should stay dim green")
	}
	_, _, style, _ = scr.GetContent(keyX(2, 4)+m.keyWidth/2, keyY(2))
	if got := fgOf(style); got != tcell.ColorGreen {
		t.Errorf("g target border fg = %v, want green", got)
	}
}
