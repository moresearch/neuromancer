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

const kbSpaceRow = 4
const kbHeight = 7 // title + four key rows + spacebar + one blank line

var (
	kbTargetStyle = tcell.StyleDefault.
			Foreground(tcell.ColorBlack).
			Background(tcell.NewRGBColor(0x2e, 0x7d, 0x32)).
			Bold(true)
	kbErrorStyle = tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.NewRGBColor(0xc6, 0x28, 0x28)).
			Bold(true)
)

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

func kbKeyWidth(screenWidth int) int {
	switch {
	case screenWidth >= 57:
		return 3
	case screenWidth >= 30:
		return 1
	default:
		return 0
	}
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
		rowWidth := kbOffset(i, keyWidth) + len(row.keys)*keyWidth + len(row.keys) - 1
		if rowWidth > width {
			width = rowWidth
		}
	}

	return width
}

// keyboardVisible reports whether the matrix fits on screen and where it starts.
func (t *typer) keyboardVisible() (keyWidth, top int, ok bool) {
	sw, sh := t.Scr.Size()

	keyWidth = kbKeyWidth(sw)
	if keyWidth == 0 {
		return 0, 0, false
	}

	top = sh - kbHeight
	if top < 1 {
		return 0, 0, false
	}

	return keyWidth, top, true
}

func drawKBBox(scr tcell.Screen, x, y, width int, label string, style tcell.Style) {
	for i := 0; i < width; i++ {
		scr.SetContent(x+i, y, ' ', nil, style)
	}

	if len(label) > width {
		label = label[:width]
	}

	start := x + (width-len(label))/2
	for i, c := range label {
		scr.SetContent(start+i, y, c, nil, style)
	}
}

// drawKeyboard renders the Colemak-DH matrix, marking the key needed for the
// next character in green and the most recent wrong key in red.
func (t *typer) drawKeyboard(target rune) {
	keyWidth, top, ok := t.keyboardVisible()
	if !ok {
		return
	}

	sw, _ := t.Scr.Size()
	width := kbWidth(keyWidth)
	x0 := (sw - width) / 2

	const title = "colemak-dh matrix"
	drawString(t.Scr, (sw-len(title))/2, top, title, -1, t.defaultStyle)

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

	for i, row := range kbRows {
		y := top + 1 + i
		offset := kbOffset(i, keyWidth)

		for j, ch := range row.keys {
			x := x0 + offset + j*(keyWidth+1)
			style := t.defaultStyle

			switch {
			case i == wrongRow && j == wrongCol:
				style = kbErrorStyle
			case i == targetRow && j == targetCol:
				style = kbTargetStyle
			}

			drawKBBox(t.Scr, x, y, keyWidth, string(ch), style)
		}
	}

	spaceWidth := 17
	if keyWidth == 1 {
		spaceWidth = 9
	}
	if spaceWidth > width {
		spaceWidth = width
	}
	spaceX := x0 + (width-spaceWidth)/2
	spaceStyle := t.defaultStyle
	switch {
	case wrongRow == kbSpaceRow:
		spaceStyle = kbErrorStyle
	case targetRow == kbSpaceRow:
		spaceStyle = kbTargetStyle
	}

	drawKBBox(t.Scr, spaceX, top+1+len(kbRows), spaceWidth, "space", spaceStyle)
}

func (t *typer) markWrong(r rune) {
	t.wrongKey = r
	t.wrongUntil = time.Now().Add(1200 * time.Millisecond)
}

func (t *typer) wrongKeyValid() bool {
	return !t.wrongUntil.IsZero() && time.Now().Before(t.wrongUntil)
}
