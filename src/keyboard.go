package main

import (
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
	kbBottomPad   = 3
	kbMinTextRows = 4
)

type kbMetrics struct {
	keyWidth int
	keyGap   int
	keyH     int
	rowGap   int
	top      int
	height   int
}

var (
	kbTargetStyle = tcell.StyleDefault.
			Foreground(tcell.ColorGreen).
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
				Foreground(tcell.ColorWhite).
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
		return def.Foreground(tcell.ColorGreen).Bold(true)
	}
	return def.Foreground(tcell.ColorWhite).Bold(true)
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

func kbBuiltinMetrics(screenWidth, screenHeight int) (kbMetrics, bool) {
	keyWidth := 0
	for kw := 5; kw >= 1; kw-- {
		if kbSplitWidth(kw)+2 <= screenWidth {
			keyWidth = kw
			break
		}
	}
	if keyWidth == 0 {
		for kw := 5; kw >= 1; kw-- {
			if kbWidth(kw)+2 <= screenWidth {
				keyWidth = kw
				break
			}
		}
	}
	if keyWidth == 0 {
		return kbMetrics{}, false
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
		keyGap:   kbKeyGap,
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

func kbWidth(keyWidth int) int {
	width := 0
	for i, row := range kbRows {
		rowWidth := kbOffset(i, keyWidth) + len(row.keys)*keyWidth + (len(row.keys)-1)*kbKeyGap
		if rowWidth > width {
			width = rowWidth
		}
	}

	return width
}

// keyboardVisible reports whether the matrix fits on screen and where it starts.
func (t *typer) keyboardVisible() (keyWidth, top int, ok bool) {
	if t.vil != nil {
		m, ok := t.vil.metrics(t.Scr.Size())
		return m.keyWidth, m.top, ok
	}

	m, ok := kbBuiltinMetrics(t.Scr.Size())
	return m.keyWidth, m.top, ok
}

func drawKBBox(scr tcell.Screen, x, y, width int, label string, style tcell.Style) {
	for i := 0; i < width; i++ {
		scr.SetContent(x+i, y, ' ', nil, style)
	}

	runes := []rune(label)
	if len(runes) > width {
		runes = runes[:width]
	}

	start := x + (width-len(runes))/2
	for i, c := range runes {
		scr.SetContent(start+i, y, c, nil, style)
	}
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
	if t.vil != nil {
		t.drawVilKeyboard(target)
		return
	}
	t.drawBuiltinKeyboard(target)
}

const (
	kbSplitCol = 5
	kbSplitGap = 5
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

func kbSplitLeftWidth(keyWidth int) int {
	return kbSplitMaxOffset(keyWidth) + kbSplitCol*keyWidth + (kbSplitCol-1)*kbKeyGap
}

func kbSplitRightWidth(keyWidth int) int {
	max := 0
	for _, row := range kbRows {
		if n := len(row.keys) - kbSplitCol; n > max {
			max = n
		}
	}
	if max == 0 {
		return 0
	}
	return max*keyWidth + (max-1)*kbKeyGap
}

func kbSplitWidth(keyWidth int) int {
	return kbSplitLeftWidth(keyWidth) + kbSplitGap + kbSplitRightWidth(keyWidth)
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
	sw, sh := t.Scr.Size()
	m, ok := kbBuiltinMetrics(sw, sh)
	if !ok {
		return
	}

	split := kbSplitWidth(m.keyWidth)+2 <= sw
	width := kbWidth(m.keyWidth)
	leftWidth := 0
	if split {
		width = kbSplitWidth(m.keyWidth)
		leftWidth = kbSplitLeftWidth(m.keyWidth)
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

	for i, row := range kbRows {
		y := m.top + i*(m.keyH+m.rowGap)
		offset := kbOffset(i, m.keyWidth)

		for j, ch := range row.keys {
			x := x0 + offset + j*(m.keyWidth+m.keyGap)
			focused := true
			if split {
				if j < kbSplitCol {
					focused = focusLeft
				} else {
					x = x0 + leftWidth + kbSplitGap + (j-kbSplitCol)*(m.keyWidth+m.keyGap)
					focused = focusRight
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
				style = t.defaultStyle
				border = targetStyle(t.blinkOn, t.defaultStyle)
				highlighted = true
			case kbBuiltinHomeKey(i, j):
				style = t.defaultStyle.Foreground(tcell.ColorGreen).Dim(true)
				border = t.defaultStyle
			}
			if !highlighted && !focused {
				style = style.Dim(true)
				border = border.Dim(true)
			}

			drawKBButton(t.Scr, x, y, m.keyWidth, m.keyH, string(ch), "", style, border)
		}
	}

	spaceY := m.top + len(kbRows)*(m.keyH+m.rowGap)

	if split {
		rightWidth := width - leftWidth - kbSplitGap
		leftSpace, rightSpace := 11, 11
		if m.keyWidth == 1 {
			leftSpace, rightSpace = 5, 5
		}
		if leftSpace > leftWidth {
			leftSpace = leftWidth
		}
		if rightSpace > rightWidth {
			rightSpace = rightWidth
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
			leftStyle, leftBorder = t.defaultStyle, targetStyle(t.blinkOn, t.defaultStyle)
			rightStyle, rightBorder = t.defaultStyle, targetStyle(t.blinkOn, t.defaultStyle)
		}

		leftX := x0 + (leftWidth-leftSpace)/2
		rightX := x0 + leftWidth + kbSplitGap + (rightWidth-rightSpace)/2
		drawKBButton(t.Scr, leftX, spaceY, leftSpace, m.keyH, "space", "", leftStyle, leftBorder)
		drawKBButton(t.Scr, rightX, spaceY, rightSpace, m.keyH, "space", "", rightStyle, rightBorder)
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
			spaceStyle = t.defaultStyle
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
