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
	leftWidth := kbSplitLeftWidth(m.keyWidth, m.keyGap) + kbSplitLeftPad(m.keyWidth, m.keyGap)
	x0 := (140 - width) / 2
	keyX := func(row, col int) int {
		gx, right := kbSplitKeyX(row, col, m.keyWidth, m.keyGap)
		if right {
			return x0 + leftWidth + m.splitGap + gx
		}
		return x0 + gx
	}
	keyY := func(row int) int {
		return rowY(m.top, row, m.keyH, m.rowGap, row == kbSpaceRow)
	}

	// tcell's simulation screen keeps cell runes and attributes faithfully,
	// but Dim(true) strips the colour bits from a style, so assert on runes
	// and the dim/bold attributes instead of named colours.
	cell := func(row, col, dy int) (rune, tcell.AttrMask) {
		ch, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row)+dy)
		_, _, attrs := style.Decompose()
		return ch, attrs
	}
	// A cap's label row: one row below the top border when the cap is tall
	// enough for a separate label row, otherwise the cap's only row.
	labelRow := 0
	if m.keyH >= 3 {
		labelRow = 1
	}
	// focusOf reads the cap's centre and reports whether it draws as the
	// target (bold) or as the inactive half (dim). At one-row caps the cap's
	// single row mixes border and label runes, so the test asserts on the
	// style the target gives the key rather than on one glyph.
	focusOf := func(label string, row, col int) (target, dim bool) {
		t.Helper()
		_, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row)+labelRow)
		_, _, attrs := style.Decompose()
		return attrs&tcell.AttrBold != 0, attrs&tcell.AttrDim != 0
	}
	checkTarget := func(label string, row, col int) {
		t.Helper()
		if target, _ := focusOf(label, row, col); !target {
			t.Errorf("%s: key is not drawn as the target", label)
		}
	}

	kb.drawKeyboard('f')
	// f is the target: its centre still shows the label, like any other key.
	checkTarget("target f", 1, 2)
	// The whole half in play stays readable: other columns are not dimmed.
	if _, attrs := cell(1, 0, labelRow); attrs&tcell.AttrDim != 0 {
		t.Error("keys outside the finger column are dimmed")
	}
	if _, attrs := cell(2, 2, labelRow); attrs&tcell.AttrDim != 0 {
		t.Error("active finger column is dimmed")
	}

	kb.blinkOn = false
	kb.drawKeyboard('f')
	checkTarget("target f blink-off", 1, 2)
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
	if _, dim := focusOf("wrong j", jr, jc); dim {
		t.Error("wrong j is dimmed")
	}

	kb.wrongUntil = time.Time{}
	kb.drawKeyboard('j')
	checkTarget("target j", jr, jc)
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
	leftWidth := kbSplitLeftWidth(m.keyWidth, m.keyGap) + kbSplitLeftPad(m.keyWidth, m.keyGap)
	x0 := (140 - width) / 2
	keyX := func(row, col int) int {
		gx, right := kbSplitKeyX(row, col, m.keyWidth, m.keyGap)
		if right {
			return x0 + leftWidth + m.splitGap + gx
		}
		return x0 + gx
	}
	keyY := func(row int) int {
		return rowY(m.top, row, m.keyH, m.rowGap, row == kbSpaceRow)
	}
	// Named colours do not survive the simulation screen (see
	// TestDrawKeyboardHighlights), so check the home/dim attributes instead.
	attrsAt := func(row, col, dy int) tcell.AttrMask {
		_, _, style, _ := scr.GetContent(keyX(row, col)+m.keyWidth/2, keyY(row)+dy)
		_, _, attrs := style.Decompose()
		return attrs
	}

	// The label sits on the row below the top border when the caps are tall
	// enough for it, otherwise on the cap's only row.
	labelRow := 0
	if m.keyH >= 3 {
		labelRow = 1
	}
	typ.drawKeyboard('g')
	// t is the finger home of the g target: its label stays undimmed while the
	// rest of the matrix dims.
	if attrs := attrsAt(2, 3, labelRow); attrs&tcell.AttrDim != 0 {
		t.Error("active finger home label should not be dimmed")
	}
	if attrs := attrsAt(2, 1, labelRow); attrs&tcell.AttrDim == 0 {
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
	// A commonly sized layout (five-wide caps, two-wide key gaps).
	const kw, kg = 5, 2
	step := kw + kg
	cell := homeGapXIn(kw, kg) // the home gap, 1.5 key cells

	// Column offsets, independent of the per-row stagger.
	colX := func(halfCols int, block kbHomeBlock, col int) int {
		return homeColX(halfCols, block, col, kw, kg)
	}

	// The home blocks are a r s t (left) and n e i o (right); each is set off
	// from the rest by a home gap. The block itself may start at column 0, in
	// which case there is no column to its left to measure against.
	checkBlock := func(name string, halfCols int, block kbHomeBlock) {
		for col := block.start + 1; col < block.start+block.cols; col++ {
			if d := colX(halfCols, block, col) - colX(halfCols, block, col-1); d != step {
				t.Errorf("%s: home columns are not contiguous at %d: %d", name, col, d)
			}
		}
		if block.before && block.start > 0 {
			if d := colX(halfCols, block, block.start) - colX(halfCols, block, block.start-1); d != step+cell {
				t.Errorf("%s: gap before the block is %d, want %d", name, d, step+cell)
			}
		}
		if end := block.start + block.cols; block.after && end < halfCols {
			if d := colX(halfCols, block, end) - colX(halfCols, block, end-1); d != step+cell {
				t.Errorf("%s: gap after the block is %d, want %d", name, d, step+cell)
			}
		}
	}
	checkBlock("left", kbSplitCol, kbLeftHome)
	checkBlock("right", 8, kbRightHome)

	// The block columns hold the home keys on the home row.
	homeRow := kbRows[2].keys
	for i, want := range []rune{'a', 'r', 's', 't'} {
		if got := homeRow[kbLeftHome.start+i]; got != want {
			t.Errorf("left block column %d is %c, want %c", i, got, want)
		}
	}
	for i, want := range []rune{'n', 'e', 'i', 'o'} {
		if got := homeRow[kbSplitCol+kbRightHome.start+i]; got != want {
			t.Errorf("right block column %d is %c, want %c", i, got, want)
		}
	}
}

// The single-block layout must measure the left half exactly as the split one
// does, or the right half starts at a different place in each.
func TestFullWidthMatchesSplitHalf(t *testing.T) {
	for _, kw := range []int{3, 5} {
		for _, kg := range []int{1, 2} {
			if got, want := lenWithGap(kbHalfCols, kbLeftHome, kw, kg), kbSplitLeftWidth(kw, kg); got != want {
				t.Errorf("kw=%d kg=%d: full-width left half %d, split %d", kw, kg, got, want)
			}
			// The first right-half key starts there plus the separator.
			if got, want := kbFullKeyX(kbSplitCol, kw, kg), kbSplitLeftWidth(kw, kg)+fullHalfGap; got != want {
				t.Errorf("kw=%d kg=%d: right half starts at %d, want %d", kw, kg, got, want)
			}
		}
	}
}

// The home block is set off from the columns beside it, so reaching for a key
// outside arst / neio shows as a clear jump.
func TestBuiltinHomeGapBothSides(t *testing.T) {
	const kw, kg = 5, 1
	step := kw + kg
	gap := step + homeGapXIn(kw, kg)

	// a is the first column, so it starts flush; the gap shows on the other
	// side of the cluster.
	if got := homeColX(kbSplitCol, kbLeftHome, 0, kw, kg); got != 0 {
		t.Errorf("a starts at %d, want 0", got)
	}
	right := homeColX(kbHalfCols, kbLeftHome, kbLeftHome.start+kbLeftHome.cols, kw, kg) -
		homeColX(kbHalfCols, kbLeftHome, kbLeftHome.start+kbLeftHome.cols-1, kw, kg)
	if right != gap {
		t.Errorf("left block: gap after t is %d, want %d", right, gap)
	}
	after := homeColX(len(kbRows[0].keys)-kbSplitCol, kbRightHome, kbRightHome.start+kbRightHome.cols, kw, kg) -
		homeColX(len(kbRows[0].keys)-kbSplitCol, kbRightHome, kbRightHome.start+kbRightHome.cols-1, kw, kg)
	if after != gap {
		t.Errorf("right block: gap after o is %d, want %d", after, gap)
	}
}

// The thumb row is drawn a line below the alpha rows, so it reads as thumbs
// rather than as a fifth letter row. That must hold at any cap height.
func TestThumbRowHasABlankLineAbove(t *testing.T) {
	for _, size := range [][2]int{{200, 44}, {140, 24}, {120, 18}} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(size[0], size[1])
		typ := newTestTyper(sim)
		typ.drawKeyboard(0)
		m, ok := typ.keyboardMetrics()
		if !ok || !typ.builtinSplit(m) {
			sim.Fini()
			continue
		}
		lastAlpha := 2
		alphaY := rowY(m.top, lastAlpha, m.keyH, m.rowGap, false)
		thumbY := rowY(m.top, lastAlpha+1, m.keyH, m.rowGap, true)
		if thumbY-alphaY != m.keyH+m.rowGap+thumbRowGap {
			t.Errorf("%dx%d: thumb row %d rows below the last alpha row, want %d",
				size[0], size[1], thumbY-alphaY, m.keyH+m.rowGap+thumbRowGap)
		}
		sim.Fini()
	}
}
