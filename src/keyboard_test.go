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
		gx, right := kbSplitKeyX(row, col, m.keyWidth, m.keyGap)
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
		gx, right := kbSplitKeyX(row, col, m.keyWidth, m.keyGap)
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

// drawnKeyX returns the screen x of the label of key ch on screen row y, or
// -1. The label must sit inside a keycap ("│ c │"), so letters which also
// appear in running text are ignored.
func drawnKeyX(scr tcell.Screen, y, width int, ch rune) int {
	for x := 1; x < width-1; x++ {
		c, _, _, _ := scr.GetContent(x, y)
		if c != ch {
			continue
		}
		// A one-cell label is centred in its keycap: "│ c │".
		before, _, _, _ := scr.GetContent(x-1, y)
		after, _, _, _ := scr.GetContent(x+1, y)
		if before != ' ' || after != ' ' {
			continue
		}
		if x < 2 || x+2 >= width {
			continue
		}
		edgeL, _, _, _ := scr.GetContent(x-2, y)
		edgeR, _, _, _ := scr.GetContent(x+2, y)
		if edgeL == '│' && edgeR == '│' {
			return x
		}
	}
	return -1
}

func TestBuiltinHomeGap(t *testing.T) {
	const sw, sh = 140, 30
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(sw, sh)

	typ := newTestTyper(scr)
	// Target a key outside the rows under test so every label is drawn.
	typ.drawKeyboard('d')

	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	// keyX returns the left edge of key ch, verified from the matrix geometry:
	// the label must be drawn at the centre of that keycap.
	keyX := func(ch rune) int {
		// Find the key in the built-in table directly; kbFind would also match
		// characters of the guidance text.
		row, col := -1, -1
		for i := range kbRows {
			for j, k := range kbRows[i].keys {
				if k == ch {
					row, col = i, j
				}
			}
		}
		if row < 0 {
			t.Fatalf("key %q not in the matrix", string(ch))
		}
		y := m.top + row*(m.keyH+m.rowGap) + m.keyH/2
		x, right := kbSplitKeyX(row, col, m.keyWidth, m.keyGap)
		if right {
			x += kbSplitLeftWidth(m.keyWidth, m.keyGap) + m.splitGap
		}
		x += (sw - kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)) / 2
		centre := x + m.keyWidth/2
		got, _, _, _ := scr.GetContent(centre, y)
		if got != ch {
			t.Fatalf("%q expected at %d, found %q", string(ch), centre, string(got))
		}
		return x
	}

	step := m.keyWidth + m.keyGap
	cell := m.keyWidth + m.keyGap // one whole key cell

	// Left half, upper row: q is the only outer column, so the gap comes right
	// after it and w f p b are the home block.
	if gap := keyX('w') - keyX('q'); gap != step+cell {
		t.Errorf("left gap after q: %d, want %d", gap, step+cell)
	}
	if keyX('f')-keyX('w') != step || keyX('p')-keyX('f') != step {
		t.Errorf("left home columns are not contiguous")
	}
	// Left half, home row: a is the outer key and is separated from the home
	// block r s t.
	if gap := keyX('r') - keyX('a'); gap != step+cell {
		t.Errorf("left gap after a: %d, want %d", gap, step+cell)
	}
	if keyX('s')-keyX('r') != step || keyX('t')-keyX('s') != step {
		t.Errorf("left home columns are not contiguous")
	}
	if keyX('g')-keyX('t') != step {
		t.Errorf("left home columns are not contiguous")
	}

	// Right half, upper row: j l are the outer columns, then the gap, then the
	// four home columns u y ; [.
	if keyX('l')-keyX('j') != step {
		t.Errorf("right outer columns are not contiguous")
	}
	if gap := keyX('u') - keyX('l'); gap != step+cell {
		t.Errorf("right gap after l: %d, want %d", gap, step+cell)
	}
	if keyX('y')-keyX('u') != step || keyX(';')-keyX('y') != step {
		t.Errorf("right home columns are not contiguous")
	}
	// Right half, home row: m n are outside the block, then the gap, then
	// e i o '. The gap sits between n and e.
	if keyX('n')-keyX('m') != step {
		t.Errorf("right home row outer columns are not contiguous")
	}
	if gap := keyX('e') - keyX('n'); gap != step+cell {
		t.Errorf("right gap after n: %d, want %d", gap, step+cell)
	}
	if keyX('i')-keyX('e') != step || keyX('o')-keyX('i') != step {
		t.Errorf("right home columns are not contiguous")
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

	// Row 0 holds the two outer columns and the four home columns of each
	// half. Every key is drawn as "╭…╮"; record each cap's left edge.
	var caps []int
	open := false
	for x := 0; x < sw; x++ {
		ch, _, _, _ := sim.GetContent(x, m.top)
		if ch == '╭' && !open {
			caps = append(caps, x)
			open = true
		}
		if ch == '╮' {
			open = false
		}
	}
	if len(caps) != 2*kb.halfCols {
		t.Fatalf("found %d keycaps, want %d: %v", len(caps), 2*kb.halfCols, caps)
	}
	left := caps[:kb.halfCols]
	right := caps[kb.halfCols:]

	step := m.keyWidth + m.keyGap
	outer := homeBlockStart(kb.halfCols) // first home column of a half

	check := func(name string, caps []int) {
		// The outer columns stay contiguous, the four home columns stay
		// contiguous, and the block is set off by one blank cell on each side.
		for i := 1; i < outer; i++ {
			if caps[i]-caps[i-1] != step {
				t.Errorf("%s: outer columns are not contiguous: %v", name, caps)
			}
		}
		if gap := caps[outer] - caps[outer-1]; gap != step+m.keyWidth {
			t.Errorf("%s: gap before the home block is %d, want %d", name, gap, step+m.keyWidth)
		}
		for i := outer + 1; i < outer+4; i++ {
			if caps[i]-caps[i-1] != step {
				t.Errorf("%s: home columns are not contiguous: %v", name, caps)
			}
		}
		if len(caps) > outer+4 {
			if gap := caps[outer+4] - caps[outer+3]; gap != step+m.keyWidth {
				t.Errorf("%s: gap after the home block is %d, want %d", name, gap, step+m.keyWidth)
			}
		}
	}
	check("left", left)
	check("right", right)
	if right[0] != left[0]+kb.halfWidthWithGap(kb.halfCols, m.keyWidth, m.keyGap)+m.splitGap {
		t.Errorf("right half starts at %d, want %d", right[0],
			left[0]+kb.halfWidthWithGap(kb.halfCols, m.keyWidth, m.keyGap)+m.splitGap)
	}

	// The thumb row is pulled in from the outer edge, so its leftmost cap is
	// right of the outer column above it.
	thumbY := m.top + (len(kb.layers[0].matrix)-1)*(m.keyH+m.rowGap)
	if !kb.isThumbRow(len(kb.layers[0].matrix) - 1) {
		return
	}
	if ch, _, _, _ := sim.GetContent(left[0], thumbY); ch != ' ' {
		t.Errorf("thumb row is not indented: cell %d is %q", left[0], string(ch))
	}
}
