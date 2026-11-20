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
	scr.SetSize(140, 30)

	kb := newTestTyper(scr)
	m, ok := kb.keyboardMetrics()
	if !ok || kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)+2 > 140 || m.keyWidth != 5 {
		t.Fatalf("keyboard not visible: %+v %v", m, ok)
	}
	width := kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)
	leftWidth := kbSplitLeftWidth(m.keyWidth, m.keyGap)
	x0 := (140 - width) / 2
	keyX := func(row, col int) int {
		gx, right := kbSplitKeyX(col, m.keyWidth, m.keyGap)
		if right {
			return x0 + leftWidth + m.splitGap + gx
		}
		return x0 + gx
	}
	keyY := func(row int) int {
		return m.top + row*(m.keyH+m.rowGap)
	}

	// tcell's simulation screen keeps cell runes and attributes faithfully,
	// but Dim(true) strips the colour bits from a style, so assert on runes
	// and the dim/bold attributes instead of named colours.
	cell := func(row, col, dy int) (rune, tcell.AttrMask) {
		ch, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row)+dy)
		_, _, attrs := style.Decompose()
		return ch, attrs
	}
	checkRune := func(label string, row, col int, want rune) {
		t.Helper()
		ch, _, _, _ := scr.GetContent(keyX(row, col), keyY(row))
		if ch != want {
			t.Errorf("%s: rune %q want %q", label, string(ch), string(want))
		}
	}

	kb.drawKeyboard('f')
	// f is the target: its centre still shows the label, like any other key.
	checkRune("target f border", 1, 2, '╭')
	// q is in the inactive finger column and is dimmed.
	if _, attrs := cell(1, 0, 1); attrs&tcell.AttrDim == 0 {
		t.Error("inactive finger column is not dimmed")
	}
	// t sits in the active (target) finger column and must not be dimmed.
	if _, attrs := cell(2, 2, 1); attrs&tcell.AttrDim != 0 {
		t.Error("active finger column is dimmed")
	}

	kb.blinkOn = false
	kb.drawKeyboard('f')
	checkRune("target f blink-off border", 1, 2, '╭')
	kb.blinkOn = true

	// The right half is not focused for f and is dimmed.
	if _, attrs := cell(1, 6, 1); attrs&tcell.AttrDim == 0 {
		t.Error("inactive right half is not dimmed")
	}
	kb.wrongKey = 'j'
	kb.wrongUntil = time.Now().Add(time.Second)
	kb.drawKeyboard('f')
	// j is the fourth key of the outer block in the built-in table.
	jr, jc := 1, 5
	checkRune("wrong j border", jr, jc, '╭')

	kb.wrongUntil = time.Time{}
	kb.drawKeyboard('j')
	checkRune("target j border", jr, jc, '╭')
	if _, attrs := cell(1, 0, 1); attrs&tcell.AttrDim == 0 {
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
	scr.SetSize(140, 30)

	typ := newTestTyper(scr)
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	width := kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)
	leftWidth := kbSplitLeftWidth(m.keyWidth, m.keyGap)
	x0 := (140 - width) / 2
	keyX := func(row, col int) int {
		gx, right := kbSplitKeyX(col, m.keyWidth, m.keyGap)
		if right {
			return x0 + leftWidth + m.splitGap + gx
		}
		return x0 + gx
	}
	keyY := func(row int) int {
		return m.top + row*(m.keyH+m.rowGap)
	}
	// Named colours do not survive the simulation screen (see
	// TestDrawKeyboardHighlights), so check the home/dim attributes instead.
	attrsAt := func(row, col, dy int) tcell.AttrMask {
		_, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row)+dy)
		_, _, attrs := style.Decompose()
		return attrs
	}

	typ.drawKeyboard('g')
	// t is the finger home of the g target: its label stays undimmed while the
	// rest of the matrix dims.
	if attrs := attrsAt(2, 3, 1); attrs&tcell.AttrDim != 0 {
		t.Error("active finger home label should not be dimmed")
	}
	if attrs := attrsAt(2, 1, 1); attrs&tcell.AttrDim == 0 {
		t.Error("inactive finger home should be dimmed")
	}
}

// gapColumns returns the screen columns which are blank between two
// neighbours of the matrix rows, and asserts none of them falls on a key.
func blankGapColumns(scr tcell.Screen, m kbMetrics, keyX func(row, col int) int, x0, width int) []int {
	var gaps []int
	y := m.top + (m.keyH-1)/2
	for x := x0; x < x0+width; x++ {
		covered := false
		for row := range kbRows {
			for col := range kbRows[row].keys {
				kx := keyX(row, col)
				if x >= kx && x < kx+m.keyWidth {
					covered = true
				}
			}
		}
		if covered {
			continue
		}
		ch, _, _, _ := scr.GetContent(x, y)
		if ch == 0 || ch == ' ' {
			gaps = append(gaps, x)
		}
	}
	return gaps
}

// drawnKeyX returns the screen x of the label of key ch on the row containing
// the given anchor key, or -1.
func drawnKeyX(scr tcell.Screen, y, width int, ch rune) int {
	for x := 0; x < width; x++ {
		if c, _, _, _ := scr.GetContent(x, y); c == ch {
			return x
		}
	}
	return -1
}

func TestBuiltinHomeGap(t *testing.T) {
	if kbLeftHomeCol != 1 || kbRightHomeCol != 4 {
		t.Fatalf("home columns %d/%d want 1/4", kbLeftHomeCol, kbRightHomeCol)
	}

	const sw, sh = 140, 30
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(sw, sh)

	typ := newTestTyper(scr)
	typ.drawKeyboard('g')

	// Locate the row holding the q..p home keys.
	homeY := -1
	for y := 0; y < sh && homeY < 0; y++ {
		if drawnKeyX(scr, y, sw, 'p') >= 0 {
			homeY = y
		}
	}
	if homeY < 0 {
		t.Fatal("home row not drawn")
	}
	pos := func(ch rune) int {
		x := drawnKeyX(scr, homeY, sw, ch)
		if x < 0 {
			t.Fatalf("key %q not drawn", string(ch))
		}
		return x
	}
	q, w, f := pos('q'), pos('w'), pos('f')
	// The left outer column (q) is separated from the home block by one extra
	// key cell, while the four home columns stay contiguous.
	if w-q <= f-w {
		t.Errorf("no home gap on the left half: q=%d w=%d f=%d", q, w, f)
	}
	if p := pos('p'); p-f != f-w {
		t.Errorf("left home columns are not contiguous: w=%d f=%d p=%d", w, f, p)
	}

	j, l, u := pos('j'), pos('l'), pos('u')
	if u-l != l-j {
		t.Errorf("right outer columns are not contiguous: j=%d l=%d u=%d", j, l, u)
	}
	if semicolon := pos(';'); semicolon-u <= u-l {
		t.Errorf("no home gap on the right half: u=%d ;=%d", u, semicolon)
	}
}

func TestVilHomeGap(t *testing.T) {
	kb := loadWideTestVil(t)
	if !kb.splitCols || kb.halfCols != 6 {
		t.Fatalf("split=%v halfCols=%d", kb.splitCols, kb.halfCols)
	}

	const sw, sh = 120, 24
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(sw, sh)

	typ := newTestTyper(sim)
	typ.vil = kb
	typ.drawKeyboard(0)
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}

	// Collect the x of every keycap's top-left corner on the first row.
	var caps []int
	for x := 0; x < sw; x++ {
		if ch, _, _, _ := sim.GetContent(x, m.top); ch == '╭' {
			caps = append(caps, x)
		}
	}
	if len(caps) < 6 {
		t.Fatalf("found %d keycaps", len(caps))
	}
	// First half: two outer columns, then the gap, then four home columns.
	step := caps[1] - caps[0]
	gapStep := caps[2] - caps[1]
	if gapStep <= step {
		t.Fatalf("no home gap after the outer block: step=%d gapStep=%d", step, gapStep)
	}
	if gapStep != step+m.keyWidth {
		t.Errorf("gap step=%d want one key cell more: %d", gapStep, step+m.keyWidth)
	}
	if caps[3]-caps[2] != step || caps[4]-caps[3] != step {
		t.Errorf("home columns are not contiguous: %v", caps[:5])
	}
}
