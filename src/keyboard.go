package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell"
)

// kbRows describes the Colemak-DH (matrix) layout. Keys are listed in physical
// order, so the same tables can be used both to draw the layout and to translate
// a character back to the key which produces it. offset is the stagger of the
// row, in half key widths.
var kbRows = []struct {
	offset int
	keys   []rune
}{
	{0, []rune("1234567890-=")},
	{1, []rune("qwfpbjluy;[]\\")},
	{2, []rune("arstgmneio'")},
	{3, []rune("zxcdvkh,./")},
}

// kbShifted maps characters which require shift on the Colemak-DH keymap back
// to the key which produces them.
var kbShifted = map[rune]rune{
	'!': '1', '@': '2', '#': '3', '$': '4', '%': '5',
	'^': '6', '&': '7', '*': '8', '(': '9', ')': '0',
	'_': '-', '+': '=',
	'{': '[', '}': ']', '|': '\\',
	':': ';', '"': '\'',
	'<': ',', '>': '.', '?': '/',
}

const (
	kbSpaceRow    = 4
	kbKeyGap      = 1
	kbMaxKeyGap   = 2
	kbBottomPad   = 3
	kbMinTextRows = 4
)

type kbMetrics struct {
	keyWidth int
	keyGap   int
	splitGap int
	keyH     int
	rowGap   int
	top      int
	height   int
}

var (
	kbTargetStyle = tcell.StyleDefault.
			Foreground(tcell.ColorLightGreen).
			Bold(true)
	kbErrorStyle = tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.NewRGBColor(0xc6, 0x28, 0x28)).
			Bold(true)
	kbModStyle = tcell.StyleDefault.
			Foreground(tcell.ColorBlack).
			Background(tcell.NewRGBColor(0x00, 0x89, 0x86)).
			Bold(true)
	kbTargetBlinkStyle = tcell.StyleDefault.
				Foreground(tcell.ColorDarkGreen).
				Bold(true)
)

func hrmTag(mod string) string {
	switch mod {
	case "Ctrl":
		return "C"
	case "Alt":
		return "A"
	case "Gui":
		return "G"
	case "Shift":
		return "S"
	}
	return ""
}

func targetStyle(blinkOn bool, def tcell.Style) tcell.Style {
	if blinkOn {
		return def.Foreground(tcell.ColorLightGreen).Bold(true)
	}
	return def.Foreground(tcell.ColorDarkGreen).Bold(true)
}

// kbFind returns the row and column of the key which produces r.
func kbFind(r rune) (row, col int, ok bool) {
	if shifted, found := kbShifted[r]; found {
		r = shifted
	}

	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}

	for i := range kbRows {
		for j, k := range kbRows[i].keys {
			if k == r {
				return i, j, true
			}
		}
	}

	return 0, 0, false
}

func chooseKeyMetrics(rows, screenHeight, bottomPad, minTextRows int) (keyH, rowGap int) {
	for h := 3; h >= 1; h -= 2 {
		keyArea := rows * h
		height := keyArea + 1 + bottomPad
		if screenHeight-height-1 >= minTextRows {
			return h, 0
		}
	}
	return 1, 0
}

func kbBuiltinMetrics(screenWidth, screenHeight, splitGap int) (kbMetrics, bool) {
	keyWidth := 0
	split := false
	// Prefer full-size caps on the single keyboard, then the split layout at
	// full size, and only then shrink the caps to keep the split.
	for kw := 5; kw >= 1; kw-- {
		if kbWidth(kw, kbKeyGap)+2 <= screenWidth {
			keyWidth = kw
			break
		}
		if kbSplitWidth(kw, kbKeyGap, splitGap)+2 <= screenWidth {
			keyWidth = kw
			split = true
			break
		}
	}
	if keyWidth == 0 && screenWidth >= kbWideScreen {
		for kw := 4; kw >= 1; kw-- {
			if kbSplitWidth(kw, kbKeyGap, splitGap)+2 <= screenWidth {
				keyWidth = kw
				split = true
				break
			}
		}
	}
	if keyWidth == 0 {
		return kbMetrics{}, false
	}

	// Keep the cap size chosen above and spend any spare width on wider
	// columns. Narrow terminals keep the original spacing.
	keyGap := kbKeyGap
	for g := kbMaxKeyGap; g > kbKeyGap; g-- {
		w := kbWidth(keyWidth, g)
		if split {
			w = kbSplitWidth(keyWidth, g, splitGap)
		}
		if w+2 <= screenWidth {
			keyGap = g
			break
		}
	}

	rows := len(kbRows) + 1 // four key rows plus the space row
	keyH, rowGap := chooseKeyMetrics(rows, screenHeight, kbBottomPad, kbMinTextRows)
	keyArea := rows*keyH + (rows-1)*rowGap
	height := keyArea + 1 + kbBottomPad
	top := screenHeight - height
	if top < 2 {
		return kbMetrics{}, false
	}

	return kbMetrics{
		keyWidth: keyWidth,
		keyGap:   keyGap,
		splitGap: splitGap,
		keyH:     keyH,
		rowGap:   rowGap,
		top:      top,
		height:   height,
	}, true
}

func kbOffset(row, keyWidth int) int {
	if keyWidth > 1 {
		return kbRows[row].offset * (keyWidth - 1)
	}
	return kbRows[row].offset
}

// kbWidth is the width of the un-split matrix. The full-width layout keeps the
// home-column gaps of both halves, so the halves stay distinguishable.
func kbWidth(keyWidth, keyGap int) int {
	width := 0
	for i, row := range kbRows {
		last := len(row.keys) - 1
		rowWidth := kbOffset(i, keyWidth) + kbFullKeyX(last, keyWidth, keyGap) + keyWidth
		if rowWidth > width {
			width = rowWidth
		}
	}

	return width
}

// kbFullKeyX returns the x offset of column col when the matrix is drawn as a
// single block: the left half's gaps, then the right half's.
func kbFullKeyX(col, keyWidth, keyGap int) int {
	if col < kbSplitCol {
		return homeColX(kbSplitCol, kbLeftHome, col, keyWidth, keyGap)
	}
	rightCols := len(kbRows[0].keys) - kbSplitCol
	return lenWithGap(kbSplitCol, kbLeftHome, keyWidth, keyGap) +
		homeColX(rightCols, kbRightHome, col-kbSplitCol, keyWidth, keyGap)
}

// keyboardVisible reports whether the matrix fits on screen and where it starts.
func (t *typer) keyboardVisible() (keyWidth, top int, ok bool) {
	m, ok := t.keyboardMetrics()
	return m.keyWidth, m.top, ok
}

// keyboardWidth is the total width of the drawn matrix, including both
// halves and the split gap.
func (t *typer) keyboardWidth(m kbMetrics) int {
	if t.vil != nil {
		return t.vil.width(m.keyWidth, m.keyGap, m.splitGap)
	}
	if t.builtinSplit(m) {
		return kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)
	}
	return kbWidth(m.keyWidth, m.keyGap)
}

// builtinSplit reports whether the built-in matrix is drawn as two halves. The
// split layout keeps full-size caps on wide terminals even when there is no
// room for it, so this cannot be decided from the widths alone.
func (t *typer) builtinSplit(m kbMetrics) bool {
	sw, _ := t.Scr.Size()
	return kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)+2 <= sw ||
		(sw >= kbWideScreen && kbWidth(m.keyWidth, m.keyGap)+2 <= sw)
}

func (t *typer) keyboardMetrics() (kbMetrics, bool) {
	sw, sh := t.Scr.Size()
	splitGap := kbSplitGap
	if sw >= kbWideScreen {
		splitGap = kbWideSplitGap
	}
	if (t.chatsubo != nil || t.sprint) && sw >= chatsuboMinScreenWidth {
		splitGap = chatsuboBoxWidth + 4
	}

	var m kbMetrics
	var ok bool
	if t.vil != nil {
		m, ok = t.vil.metrics(sw, sh, splitGap)
	} else {
		m, ok = kbBuiltinMetrics(sw, sh, splitGap)
	}
	if !ok {
		return m, false
	}
	if t.layoutTop >= 2 && t.layoutTop+m.height <= sh {
		m.top = t.layoutTop
	}
	return m, true
}

func buttonLabel(label string, max int) string {
	r := []rune(label)
	if len(r) <= max {
		return label
	}
	if max <= 0 {
		return ""
	}
	if len(r) >= 2 && r[0] == 'F' {
		tail := r[1:]
		if len(tail) > max {
			tail = tail[len(tail)-max:]
		}
		return string(tail)
	}
	return string(r[:max])
}

func drawKBButton(scr tcell.Screen, x, y, width, height int, label, top string, style, border tcell.Style) {
	if width <= 0 || height <= 0 {
		return
	}

	fillRow := func(yy int) {
		for i := 0; i < width; i++ {
			scr.SetContent(x+i, yy, ' ', nil, style)
		}
	}

	if height == 1 {
		fillRow(y)
		shown := label
		if top != "" {
			combined := top + " " + label
			if len([]rune(combined)) > width-2 {
				combined = top + label
			}
			shown = combined
		}
		shown = buttonLabel(shown, width-2)
		labelRunes := []rune(shown)
		start := x + (width-len(labelRunes))/2
		if width >= 2 {
			scr.SetContent(x, y, '[', nil, border)
			scr.SetContent(x+width-1, y, ']', nil, border)
		}
		for i, c := range labelRunes {
			scr.SetContent(start+i, y, c, nil, style)
		}
		return
	}

	// Rounded three-row keycap.
	for yy := y; yy < y+height; yy++ {
		fillRow(yy)
	}
	if width >= 2 {
		scr.SetContent(x, y, '╭', nil, border)
		scr.SetContent(x+width-1, y, '╮', nil, border)
		for i := 1; i < width-1; i++ {
			scr.SetContent(x+i, y, '─', nil, border)
		}
		if top != "" {
			tr := []rune(top)
			if len(tr) > width-2 {
				tr = tr[:width-2]
			}
			start := x + (width-len(tr))/2
			for i, c := range tr {
				scr.SetContent(start+i, y, c, nil, border)
			}
		}

		by := y + height - 1
		scr.SetContent(x, by, '╰', nil, border)
		scr.SetContent(x+width-1, by, '╯', nil, border)
		for i := 1; i < width-1; i++ {
			scr.SetContent(x+i, by, '─', nil, border)
		}
	}

	mid := y + height/2
	shown := buttonLabel(label, width-2)
	labelRunes := []rune(shown)
	start := x + (width-len(labelRunes))/2
	if width >= 2 {
		scr.SetContent(x, mid, '│', nil, border)
		scr.SetContent(x+width-1, mid, '│', nil, border)
	}
	for i, c := range labelRunes {
		scr.SetContent(start+i, mid, c, nil, style)
	}
}

// drawKeyboard renders the keyboard matrix, marking the key needed for the
// next character in green and the most recent wrong key in red.
func (t *typer) drawKeyboard(target rune) {
	t.hintText = ""
	if t.vil != nil {
		t.drawVilKeyboard(target)
		return
	}
	t.drawBuiltinKeyboard(target)
}

// hintFor returns the key/layer guidance for target using the loaded Vial
// keymap, or the built-in Colemak-DH matrix when no .vil is loaded.
func (t *typer) hintFor(target rune) string {
	if target == 0 || target == '\n' {
		return ""
	}
	if t.vil != nil {
		hint, found := t.vil.findRune(target)
		if !found {
			return noKeyHint(target)
		}
		return t.withFinger(t.vil.hintText(hint, found), target)
	}
	return t.withFinger(builtinHint(target), target)
}

// builtinHint describes the key needed for target on the built-in Colemak-DH
// matrix. The Chatsubo box uses this guidance when no .vil is loaded.
func builtinHint(target rune) string {
	if target == 0 || target == '\n' {
		return ""
	}
	if target == ' ' {
		return "press SPACE"
	}

	row, col, ok := kbFind(target)
	if !ok {
		return ""
	}

	shift := false
	if target >= 'A' && target <= 'Z' {
		shift = true
	}
	if _, ok := kbShifted[target]; ok {
		shift = true
	}

	key := kbRows[row].keys[col]
	if shift {
		return fmt.Sprintf("hold SHIFT + press %c", key)
	}
	return fmt.Sprintf("press %c", key)
}

const (
	kbSplitCol     = 5
	kbSplitGap     = 5
	kbWideSplitGap = 9
	kbWideScreen   = 100
)

func kbSplitMaxOffset(keyWidth int) int {
	max := 0
	for i := range kbRows {
		if o := kbOffset(i, keyWidth); o > max {
			max = o
		}
	}
	return max
}

// homeGapCells is the blank gap, in key cells, placed between each split's
// home block (arst / oien) and the columns outside it. In cell units the gap
// is keyWidth+keyGap wide, i.e. one extra column step.
const homeGapCells = 1

// kbHomeBlock describes the four home columns (a r s t / o i e n) of one
// half: start is its first column and cols its width.
type kbHomeBlock struct{ start, cols int }

// built-in home columns, taken from the home row of kbRows:
//
//	col 0 1 2 3 4   5 6 7 8 9 10
//	key a r s t g   m n e i o  '
//
// so a r s t are the left half's columns 0..3 and o i e n the right half's
// columns 1..4 (the jutting ; [ ] \ tail and the outer index columns stay
// outside the block).
var (
	kbLeftHome  = kbHomeBlock{start: 0, cols: 4}
	kbRightHome = kbHomeBlock{start: 1, cols: 4}
)

// homeColX returns the x offset of column col relative to its half's origin,
// counting the home-column gaps. Widths and drawing both use it.
func homeColX(halfCols int, block kbHomeBlock, col, keyWidth, keyGap int) int {
	if col < 0 {
		return 0
	}
	if block.cols == 0 || block.cols >= halfCols {
		return col * (keyWidth + keyGap)
	}
	// The home block is an island: a blank cell on each side of it.
	x := col * (keyWidth + keyGap)
	if col >= block.start {
		x += homeGapCells * (keyWidth + keyGap)
	}
	if col >= block.start+block.cols {
		x += homeGapCells * (keyWidth + keyGap)
	}
	return x
}

// lenWithGap is the width of halfCols key columns plus the home gaps, if any.
func lenWithGap(halfCols int, block kbHomeBlock, keyWidth, keyGap int) int {
	return homeColX(halfCols, block, halfCols, keyWidth, keyGap) - keyGap
}

func kbSplitLeftWidth(keyWidth, keyGap int) int {
	return kbSplitMaxOffset(keyWidth) + lenWithGap(kbSplitCol, kbLeftHome, keyWidth, keyGap)
}

func kbSplitRightWidth(keyWidth, keyGap int) int {
	max := 0
	for _, row := range kbRows {
		if n := len(row.keys) - kbSplitCol; n > max {
			max = n
		}
	}
	if max == 0 {
		return 0
	}
	return lenWithGap(max, kbRightHome, keyWidth, keyGap)
}

// kbSplitKeyX returns the x offset of key col of matrix row row (relative to
// the split's own origin) and whether the key is in the right half. The row
// stagger and the home-column gaps are included, so drawing, widths and hit
// positions all agree.
func kbSplitKeyX(row, col, keyWidth, keyGap int) (x int, right bool) {
	if col < 0 {
		return 0, false
	}
	if col < kbSplitCol {
		return kbOffset(row, keyWidth) + homeColX(kbSplitCol, kbLeftHome, col, keyWidth, keyGap), false
	}
	max := 0
	for _, r := range kbRows {
		if n := len(r.keys) - kbSplitCol; n > max {
			max = n
		}
	}
	return homeColX(max, kbRightHome, col-kbSplitCol, keyWidth, keyGap), true
}

// thumbIndentCells is how far the thumb row is pulled in from each half's
// outer edge, so the thumbs sit under the home block rather than the whole
// half.
const thumbIndentCells = 2

// thumbX shifts a thumb row position inward by the indent, keeping it inside
// the half.
func thumbX(halfCols int, block kbHomeBlock, x, keyWidth, keyGap int) int {
	half := lenWithGap(halfCols, block, keyWidth, keyGap)
	indent := thumbIndentCells * (keyWidth + keyGap)
	if x+indent+keyWidth <= half {
		return x + indent
	}
	return x
}

// homeSpaceRect returns the left edge and width of a half's space bar. The bar
// sits on the indented thumb row, starting under the home gap and spanning the
// home block (arst / oien).
func homeSpaceRect(halfCols int, block kbHomeBlock, keyWidth, keyGap int) (x, width int) {
	if block.cols == 0 || block.cols >= halfCols {
		x = thumbX(halfCols, block, 0, keyWidth, keyGap)
		return x, halfCols*keyWidth + (halfCols-1)*keyGap
	}
	// Start at the gap before the home block and cover the home columns.
	width = block.cols*keyWidth + (block.cols-1)*keyGap + homeGapCells*keyGap
	x = homeColX(halfCols, block, block.start, keyWidth, keyGap)
	return thumbX(halfCols, block, x-homeGapCells*(keyWidth+keyGap), keyWidth, keyGap), width
}

func kbSplitWidth(keyWidth, keyGap, splitGap int) int {
	return kbSplitLeftWidth(keyWidth, keyGap) + splitGap + kbSplitRightWidth(keyWidth, keyGap)
}

func kbBuiltinHomeKey(row, col int) bool {
	if row != 2 {
		return false
	}
	if col < kbSplitCol {
		return isHomeDistance(kbSplitCol, kbSplitCol-1-col)
	}
	rightCols := len(kbRows[0].keys) - kbSplitCol
	return isHomeDistance(rightCols, col-kbSplitCol)
}

func kbBuiltinTargetHome(row, col int) (int, int, bool) {
	if row == kbSpaceRow {
		return 0, 0, false
	}
	if col < kbSplitCol {
		hd := homeDistanceFor(kbSplitCol, kbSplitCol-1-col)
		return 2, kbSplitCol - 1 - hd, true
	}
	rightCols := len(kbRows[0].keys) - kbSplitCol
	hd := homeDistanceFor(rightCols, col-kbSplitCol)
	return 2, kbSplitCol + hd, true
}

func (t *typer) drawBuiltinKeyboard(target rune) {
	t.hintText = t.withFinger(builtinHint(target), target)

	sw, _ := t.Scr.Size()
	m, ok := t.keyboardMetrics()
	if !ok {
		return
	}

	split := t.builtinSplit(m)
	width := kbWidth(m.keyWidth, m.keyGap)
	leftWidth := 0
	if split {
		width = kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)
		leftWidth = kbSplitLeftWidth(m.keyWidth, m.keyGap)
	}
	x0 := (sw - width) / 2

	targetRow, targetCol := -1, -1
	if target == ' ' {
		targetRow = kbSpaceRow
	} else if target != 0 && target != '\n' {
		if r, c, found := kbFind(target); found {
			targetRow, targetCol = r, c
		}
	}

	wrongRow, wrongCol := -1, -1
	if t.wrongKeyValid() {
		if t.wrongKey == ' ' {
			wrongRow = kbSpaceRow
		} else if r, c, found := kbFind(t.wrongKey); found {
			wrongRow, wrongCol = r, c
		}
	}

	focusLeft, focusRight := true, true
	if split && targetRow >= 0 && targetRow != kbSpaceRow {
		if targetCol < kbSplitCol {
			focusRight = false
		} else {
			focusLeft = false
		}
	}

	// Light the column of the finger which should reach this key. The rest of
	// the matrix dims so the active column stands out.
	activeCol := -1
	if targetRow >= 0 && targetRow != kbSpaceRow {
		if _, c, ok := kbBuiltinTargetHome(targetRow, targetCol); ok {
			activeCol = c
		}
	}

	for i, row := range kbRows {
		y := m.top + i*(m.keyH+m.rowGap)
		offset := kbOffset(i, m.keyWidth)

		for j, ch := range row.keys {
			x := x0 + offset + kbFullKeyX(j, m.keyWidth, m.keyGap)
			focused := true
			if split {
				var gx int
				var right bool
				gx, right = kbSplitKeyX(i, j, m.keyWidth, m.keyGap)
				if right {
					x = x0 + leftWidth + m.splitGap + gx
					focused = focusRight
				} else {
					x = x0 + gx
					focused = focusLeft
				}
			}

			style := t.defaultStyle
			border := style
			highlighted := false
			switch {
			case i == wrongRow && j == wrongCol:
				style = kbErrorStyle
				border = kbErrorStyle
				highlighted = true
			case i == targetRow && j == targetCol:
				style = targetStyle(t.blinkOn, t.defaultStyle)
				border = style
				highlighted = true
			case kbBuiltinHomeKey(i, j):
				if j == activeCol {
					style = t.defaultStyle.Foreground(tcell.ColorGreen).Bold(true)
				} else {
					style = t.defaultStyle.Foreground(tcell.ColorGreen).Dim(true)
				}
				border = t.defaultStyle
			}
			inFingerCol := activeCol < 0 || j == activeCol
			if !highlighted && !(focused && inFingerCol) {
				style = style.Dim(true)
				border = border.Dim(true)
			}

			drawKBButton(t.Scr, x, y, m.keyWidth, m.keyH, string(ch), "", style, border)
		}
	}

	spaceY := m.top + len(kbRows)*(m.keyH+m.rowGap)

	if split {
		// Each thumb bar sits under its half's home block, aligned with the
		// arst / oien columns and the home gap in front of them.
		leftX, leftSpace := homeSpaceRect(kbSplitCol, kbLeftHome, m.keyWidth, m.keyGap)
		rightCols := len(kbRows[0].keys) - kbSplitCol
		rightX, rightSpace := homeSpaceRect(rightCols, kbRightHome, m.keyWidth, m.keyGap)
		rightX += leftWidth + m.splitGap
		if m.keyWidth == 1 {
			leftSpace, rightSpace = 5, 5
			leftX, rightX = x0+(leftWidth-leftSpace)/2, x0+leftWidth+m.splitGap+((width-leftWidth-m.splitGap)-rightSpace)/2
		}

		homeStyle := t.defaultStyle.Foreground(tcell.ColorGreen).Dim(true)
		leftStyle, rightStyle := homeStyle, homeStyle
		leftBorder, rightBorder := t.defaultStyle, t.defaultStyle
		if !focusLeft {
			leftStyle = leftStyle.Dim(true)
			leftBorder = leftBorder.Dim(true)
		}
		if !focusRight {
			rightStyle = rightStyle.Dim(true)
			rightBorder = rightBorder.Dim(true)
		}
		if wrongRow == kbSpaceRow {
			leftStyle, rightStyle = kbErrorStyle, kbErrorStyle
			leftBorder, rightBorder = kbErrorStyle, kbErrorStyle
		} else if targetRow == kbSpaceRow {
			leftStyle, leftBorder = targetStyle(t.blinkOn, t.defaultStyle), targetStyle(t.blinkOn, t.defaultStyle)
			rightStyle, rightBorder = targetStyle(t.blinkOn, t.defaultStyle), targetStyle(t.blinkOn, t.defaultStyle)
		}

		drawKBButton(t.Scr, x0+leftX, spaceY, leftSpace, m.keyH, "space", "", leftStyle, leftBorder)
		drawKBButton(t.Scr, x0+rightX, spaceY, rightSpace, m.keyH, "space", "", rightStyle, rightBorder)
	} else {
		spaceWidth := 17
		if m.keyWidth == 1 {
			spaceWidth = 9
		}
		if spaceWidth > width {
			spaceWidth = width
		}
		spaceX := x0 + (width-spaceWidth)/2
		spaceStyle := t.defaultStyle.Foreground(tcell.ColorGreen).Dim(true)
		spaceBorder := t.defaultStyle
		switch {
		case wrongRow == kbSpaceRow:
			spaceStyle = kbErrorStyle
			spaceBorder = kbErrorStyle
		case targetRow == kbSpaceRow:
			spaceStyle = targetStyle(t.blinkOn, t.defaultStyle)
			spaceBorder = targetStyle(t.blinkOn, t.defaultStyle)
		}
		drawKBButton(t.Scr, spaceX, spaceY, spaceWidth, m.keyH, "space", "", spaceStyle, spaceBorder)
	}
}

func (t *typer) markWrong(r rune) {
	t.wrongKey = r
	t.wrongUntil = time.Now().Add(1200 * time.Millisecond)
}

func (t *typer) wrongKeyValid() bool {
	return !t.wrongUntil.IsZero() && time.Now().Before(t.wrongUntil)
}
