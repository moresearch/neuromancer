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
	kbSpaceRow = 4
	// kbKeyGap is the gap between neighbouring caps: zero, so the board is a
	// contiguous grid with no spaces between keys.
	kbKeyGap = 0
	// kbMaxKeyGap and kbMaxKeyH bound the board's vertical rhythm: how many
	// blank rows go between key rows, and how tall a single cap may grow.
	kbMaxKeyGap = 1
	kbMaxKeyH   = 3
	// boardPad is the blank row below the board, before the stats band, so the
	// keys never run into the equalizer.
	boardPad = 1
	// kbMinTextRows is the fewest practice-text rows the block is planned
	// around, so the board's share of the window leaves room for the words.
	kbMinTextRows = 4
)

type kbMetrics struct {
	keyWidth int
	keyGap   int
	// splitGap is the gap between the hands, always boardHalfGap.
	splitGap int
	keyH     int
	rowGap   int
	// height is the total board height, already including the blank boardPad
	// row below the last key row and before the stats band.
	top    int
	height int
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

// thumbRowGap is the blank line between the alpha rows and the thumb row, so
// the thumb keys read as a separate cluster.
const thumbRowGap = 1

// rowY is the top of display row r, with the thumb row pushed down by
// thumbRowGap.
func rowY(top, r, keyH, rowGap int, thumbRow bool) int {
	y := top + r*(keyH+rowGap)
	if thumbRow {
		y += thumbRowGap
	}
	return y
}

// gridHeight is the height of rows key rows, or rows-1 rows when the last of
// them is a thumb row drawn further down.
func gridHeight(rows, keyH, rowGap int, thumbRow bool) int {
	if rows <= 0 {
		return 0
	}
	h := rows*keyH + (rows-1)*rowGap
	if thumbRow {
		h += thumbRowGap
	}
	return h
}

// chooseKeyMetrics picks the tallest key cap that fits a board of avail rows,
// where grid reports the board's height for its keyH and rowGap. It then
// spends any leftover rows on blank rows between the key rows, as many as fit.
// The board decides how many rows each gap actually adds, since some layouts
// leave the thumb row unpadded.
func chooseKeyMetrics(rows, avail int, grid func(keyH, rowGap int) int) (keyH, rowGap int) {
	// A board with the room is a keyboard, not a stack of flat labels: with
	// every row on screen, the tallest cap that still leaves the rows apart
	// wins even if it loses the blank line between them.
	if rows <= 0 {
		return 1, 0
	}
	// Completed keycaps first: a cap with a top border, a label row and a
	// bottom border reads as a key, so the tallest closed cap wins even with
	// no blank line between rows, and a two-row cap (closed, with a gap
	// between rows) beats a flat one-row label.
	for _, h := range []int{kbMaxKeyH, 2, 1} {
		if h == 1 && grid(kbMaxKeyH, 0) <= avail {
			// There is room for closed caps: a flat row of labels would only
			// crowd more rows onto the board.
			break
		}
		maxGap := 0
		if rows >= 2 && h == 2 {
			maxGap = kbMaxKeyGap
		}
		// A cap only counts when it really fits the rows the board is given:
		// the full grid, not just its tight form.
		for g := maxGap; g >= 0; g-- {
			if grid(h, g) <= avail {
				return h, g
			}
		}
	}
	keyH = 1
	for h := kbMaxKeyH; h >= 1; h-- {
		if grid(h, 0) <= avail {
			keyH = h
			break
		}
	}
	return keyH, 0
}

// builtinRows is the number of key rows in the built-in matrix: the letter
// rows plus the space row.
func builtinRows() int { return len(kbRows) + 1 }

// builtinBareRows is the thinnest the built-in board draws: one-row caps, no
// row gaps. It is the floor when the whole block has to fit a short window.
func builtinBareRows() int { return gridHeight(builtinRows(), 1, 0, true) }

// kbBuiltinMetrics chooses the built-in board for the rows it may occupy:
// availHeight is that allowance, and the board grows to fill it, no further.
func kbBuiltinMetrics(screenWidth, availHeight, splitGap int) (kbMetrics, bool) {
	keyWidth := 0
	// Caps are chosen against the whole matrix, so the full-size keyboard is
	// preferred; the two-half board is only used when it fits beside the
	// joiner (see builtinSplit).
	for _, kw := range capLadder() {
		if kbWidth(kw, kbKeyGap)+2 <= screenWidth {
			keyWidth = kw
			break
		}
	}
	if keyWidth == 0 {
		return kbMetrics{}, false
	}

	// Keys sit close together: the columns are packed at the minimum gap and
	// any spare width goes to the home cluster, which is what should stand
	// apart. Widening every column instead makes the whole board loose.
	keyGap := kbKeyGap

	rows := builtinRows()
	keyH, rowGap := chooseKeyMetrics(rows, availHeight, func(kh, rg int) int {
		return gridHeight(rows, kh, rg, true)
	})

	return kbMetrics{
		keyWidth: keyWidth,
		keyGap:   keyGap,
		splitGap: splitGap,
		keyH:     keyH,
		rowGap:   rowGap,
		height:   gridHeight(rows, keyH, rowGap, true) + boardPad,
	}, true
}

// blockChromeRows is the block's rows outside the board: the Chatsubo box and
// its padding, the practice text, and the equalizer plus readout.
func (t *typer) blockChromeRows(sw, sh int) int {
	n := layoutBandPad*2 + t.textRows() + layoutStatsRows
	if t.usesChatBox() {
		n += t.blockBoxHeight(sh)
	}
	return n
}

// boardRowsAllowed is the rows the board may draw in: the shape of a keyboard,
// or on a short window the rows the box, the text and the stats can spare
// (their minimum sizes, the stats keeping one row). Never below the thinnest
// board.
func boardRowsAllowed(sh, nonBoardRows int) int {
	// The board takes the shape of a keyboard; the block's other parts are
	// laid out around that height. A window too short for the shape draws
	// smaller caps, down to the thinnest board.
	available := sh - 2 - blockBoxMinHeight - layoutBandPad*2 - 2 - 1
	if allowed := boardMaxRows(); allowed < available {
		return allowed
	}
	if floor := boardMinRows(); available < floor {
		return floor
	}
	return available
}

// planBoardHeight is the board's drawn height for a window, without laying the
// block out (keyboardMetrics calls the block layout, so it cannot be used
// here). A window too short for the shape is planned around its own drawn
// rows rather than around the thinnest possible board.
// screenHeight is the window's height.
func (t *typer) screenHeight() int {
	_, sh := t.Scr.Size()
	return sh
}

func (t *typer) planBoardHeight() int {
	rows := builtinRows()
	if t.vil != nil && t.vil.hasBoard {
		rows = t.vil.displayRows()
	}
	if rows <= 0 {
		return boardMinRows()
	}
	if !t.usesChatBox() {
		return boardMinRows()
	}
	// The window the caller is asking about is the screen's height. The tight
	// levels are measured against the rows the block can really spare (the
	// board's allowance), so the plan and the drawn board agree.
	sh := boardRowsAllowed(t.screenHeight(), 0) + 3 + 2 + 2 + 2 + 2
	shape := 0
	if t.vil != nil && t.vil.hasBoard {
		shape = t.vil.vilGridHeight(rows, kbMaxKeyH, kbMaxKeyGap) + boardPad
	} else {
		shape = 5*kbMaxKeyH + 4*kbMaxKeyGap + boardPad
	}
	if sh >= shape+blockBoxMinHeight+2+layoutTextRows+1+2 {
		return shape
	}
	// Closed caps first, then the same caps with a blank line between the
	// rows, so the board keeps whole keys before it keeps air between them.
	type caps struct{ h, gap int }
	for _, c := range []caps{{kbMaxKeyH, 0}, {2, 0}, {2, kbMaxKeyGap}, {1, 0}} {
		h, rowGap := c.h, c.gap
		if rows < 2 {
			rowGap = 0
		}
		var height int
		if t.vil != nil && t.vil.hasBoard {
			height = t.vil.vilGridHeight(rows, h, rowGap) + boardPad
		} else {
			height = rows*h + (rows-1)*rowGap + boardPad
		}
		if sh >= height+3+2+2+2+2 {
			return height
		}
	}
	return boardMinRows()
}

// blockBoxMinHeight is the shortest Chatsubo box: its border, one body row and
// its border, enough for the title and the guidance.
const blockBoxMinHeight = 3

// boardMinRows is the shortest the board draws: its rows at one-row caps,
// still keeping a blank line between them, plus the pad.
func boardMinRows() int {
	return builtinBareRows() + boardPad
}

// blockRoom is how many rows the block may take: half the window plus the
// rows a short window can still spare, so a compact board and a small box fit
// on a laptop terminal without a single band running off.
func blockRoom(sh int) int {
	return sh/2 + sh/10
}

// boardMaxRows is the tallest the board draws: five key rows of three-row
// caps with a blank line between them, plus the pad. Taller caps turn the keys
// into blocks, so spare rows go to the other bands instead.
func boardMaxRows() int {
	return 5*kbMaxKeyH + 4*kbMaxKeyGap + boardPad
}

// boardMaxHeight caps the board's height at its drawn rows at the tallest
// cap that is not a block: four key rows plus the thumb row. Spare rows go to
// the box and the text instead, so the board keeps its keyboard shape on every
// terminal.

// kbOffset is the stagger of a row. The matrix is drawn as a clean grid, so
// every row starts in the same column; the physical stagger of the printed
// keyboard is deliberately not reproduced.
func kbOffset(row, keyWidth int) int {
	return 0
}

// kbWidth is the width of the single-matrix board. The layout keeps the
// home-column gaps of both hands, so the hands stay distinguishable.
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

// fullHalfGap separates the two hands when the matrix is drawn as a single
// block, so the hands stay recognisable on a narrow terminal. It is the same
// joiner the two-hand board uses.
const fullHalfGap = boardHalfGap

// kbFullKeyX returns the x offset of column col when the matrix is drawn as a
// single block: the left half, a small gap, then the right half.
func kbFullKeyX(col, keyWidth, keyGap int) int {
	// The left half is kbHalfCols wide, matching kbSplitLeftWidth; using the
	// split column here made the single-block layout disagree with the split
	// one about where the right half starts.
	if col < kbSplitCol {
		return homeColX(kbHalfCols, kbLeftHome, col, keyWidth, keyGap)
	}
	rightCols := len(kbRows[0].keys) - kbSplitCol
	return lenWithGap(kbHalfCols, kbLeftHome, keyWidth, keyGap) + fullHalfGap +
		homeColX(rightCols, kbRightHome, col-kbSplitCol, keyWidth, keyGap)
}

// keyboardVisible reports whether the matrix fits on screen and where it starts.
func (t *typer) keyboardVisible() (keyWidth, top int, ok bool) {
	m, ok := t.keyboardMetrics()
	return m.keyWidth, m.top, ok
}

// useVil reports whether the loaded Vial layout is actually drawn this frame;
// a layout too wide for the window falls back to the built-in matrix.
func (t *typer) useVil() bool { return t.vil != nil && !t.vilFallback }

// usesChatBox reports whether the block draws the Chatsubo box as its top band.
// A sprint uses the same box: its panel is a band above the keys, like the
// normal block's.
func (t *typer) usesChatBox() bool { return t.chatsubo != nil || t.sprint }

// keyboardWidth is the total width of the drawn matrix, including both
// hands and the joiner between them.
func (t *typer) keyboardWidth(m kbMetrics) int {
	if t.useVil() {
		return t.vil.width(m.keyWidth, m.keyGap, m.splitGap)
	}
	if t.builtinSplit(m) {
		return kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)
	}
	return kbWidth(m.keyWidth, m.keyGap)
}

// builtinSplit reports whether the built-in matrix is drawn as two hands. The
// joiner keeps full-size caps on wide terminals; a terminal too narrow for
// them falls back to the single matrix at the same cap size.
func (t *typer) builtinSplit(m kbMetrics) bool {
	sw, _ := t.Scr.Size()
	// The split layout is used only when it fits at the chosen cap size; a
	// terminal too narrow for it falls back to the single keyboard.
	return kbSplitWidth(m.keyWidth, m.keyGap, m.splitGap)+2 <= sw
}

func (t *typer) keyboardMetrics() (kbMetrics, bool) {
	sw, sh := t.Scr.Size()

	// The board gets what the rest of the block leaves of its share of the
	// window (about half the height), so the block stays compact.
	// The board's size does not depend on what else the block holds: it takes
	// the shape of a keyboard in a tall window and its minimum in a short one,
	// and the other bands take the rest. Sizing it from the box's height would
	// make it a flat row of labels on exactly the terminals that have room.
	availSh := boardRowsAllowed(sh, 0)

	// One board, one joiner: the caps are sized against the fixed gap between
	// the hands, and the window never trades that gap for a wider cap.
	compute := func(avail int) (kbMetrics, bool) {
		if t.vil != nil {
			return t.vil.metrics(sw, avail, boardHalfGap)
		}
		return kbBuiltinMetrics(sw, avail, boardHalfGap)
	}
	m, ok := compute(availSh)
	if !ok {
		// A Vial layout too wide for the window cannot be drawn; fall back to
		// the built-in Colemak-DH matrix rather than showing an empty board.
		if t.vil != nil {
			if bm, bok := kbBuiltinMetrics(sw, availSh, boardHalfGap); bok {
				m, ok = bm, true
				t.vilFallback = true
			}
		}
		if !ok {
			return m, false
		}
	} else {
		t.vilFallback = false
	}

	// The window is one centred block of three parts, top to bottom: the
	// Chatsubo box, the keys, the stats; the practice text has its own band
	// between the box and the keys. Every part must start where the previous
	// one ends and stay on screen, so the block is planned twice: the first
	// pass fixes the heights, the second places them.
	parts := t.layoutParts(sh, boardMinRows())
	room := sh - 2 - parts.chat.H - layoutBandPad*2 - parts.text.H - parts.stats.H
	if room > boardMaxRows() {
		room = boardMaxRows()
	}
	if room > availSh {
		room = availSh
	}
	// Rows the board does not need make room for the box's extra body lines,
	// so the box is taller on a small window instead of the block sitting with
	// empty rows around it.
	if grow := room - m.height; grow > 0 {
		if limit := chatsuboMaxHeight - parts.chat.H; limit < grow {
			grow = limit
		}
		if grow > 0 {
			parts.chat.H += grow
		}
	}
	if room >= boardMinRows() && room < availSh {
		// A tight window draws smaller caps so the board fits its part. The
		// block is laid out again with the board that will really be drawn,
		// so the box, the text and the stats take exactly the rows left over.
		if sm, sok := compute(room); sok && sm.height <= room {
			m = sm
			parts = t.layoutParts(sh, m.height)
		}
	}
	parts = t.layoutParts(sh, m.height)
	parts.keys.H = m.height

	// The block must fit the window: give up the box's extra body rows, then
	// the text's second row, then the equalizer, before anything is pushed off
	// the bottom. Each cut is taken from a part's height, so the parts still
	// touch and nothing overlaps.
	// Every part must fit the window. Heights come from what the window can
	// really give: the box from its own ladder, the text from the rows left
	// after it, and the board from those left after the text and the stats.
	// Heights never grow into rows another part needs, so nothing overlaps and
	// nothing is cut.
	parts.chat.H = 0
	if t.usesChatBox() {
		parts.chat.H = chatsuboBoxHeight(sh)
		if parts.chat.H > sh/4 {
			parts.chat.H = sh / 4
		}
		if parts.chat.H < blockBoxMinHeight {
			parts.chat.H = blockBoxMinHeight
		}
	}
	parts.stats.H = 2
	if sh-parts.chat.H-layoutBandPad*2-2-parts.keys.H-parts.stats.H < 0 {
		if sh-parts.chat.H-layoutBandPad*2-2-parts.keys.H-1 >= 0 {
			parts.stats.H = 1
		}
	}
	// The practice text lives inside the chatsubo box, so the box takes the
	// text's rows as part of its body. There is no separate text band between
	// the box and the keys.
	parts.text.H = sh - 2 - parts.chat.H - layoutBandPad - parts.keys.H - parts.stats.H
	if parts.text.H > layoutTextRows {
		parts.text.H = layoutTextRows
	}
	if parts.text.H < 2 {
		parts.text.H = 2
	}
	// The box body holds the text rows: one title row, the text, one border.
	parts.chat.H += parts.text.H

	// The parts start where the previous one ends, so no part can overlap
	// another: the box (holding the text) on top, then a blank line, the keys,
	// and the stats under them.
	p := partLayout{}
	p.chat.H, p.text.H = parts.chat.H, parts.text.H
	p.keys.H, p.stats.H = parts.keys.H, parts.stats.H
	total := p.chat.H + layoutBandPad + p.keys.H + p.stats.H
	y := (sh - total) / 2
	if y < 0 {
		y = 0
	}
	p.chat.Y = y
	// The text sits inside the box: below the title rule, above the bottom
	// border, at the box's interior columns.
	p.text.Y = p.chat.Y + 1
	y += p.chat.H + layoutBandPad
	p.keys.Y = y
	p.stats.Y = y + p.keys.H
	parts = p

	t.parts = parts
	t.boxTop, t.boxH = parts.chat.Y, parts.chat.H
	m.top = parts.keys.Y
	t.statsTop = parts.stats.Y
	if t.layoutTop >= 0 {
		m.top = t.layoutTop
		parts.keys.Y = t.layoutTop
		parts.stats.Y = t.layoutTop + parts.keys.H
		t.parts = parts
		t.statsTop = parts.stats.Y
		t.boxTop = parts.chat.Y
	}
	return m, true
}

// band is one horizontal part of the window: its top row and height.
type band struct{ Y, H int }

// partLayout is the block split into its parts, top to bottom: the box (which
// holds the practice text), the keys and the stats. text is where the practice
// text draws, inside the box's body. Every part starts where the previous one
// ends, so nothing overlaps.
type partLayout struct {
	chat, text, keys, stats band
}

// layoutParts divides the window into the three parts. The box takes its own
// height (the window's quarter, clamped), the text its band, the stats their
// two rows (one on a very short window), and the keys everything that is
// left, so the board is sized by the room the other parts leave it.
func (t *typer) layoutParts(sh, boardH int) partLayout {
	const pad = layoutBandPad
	// The box, the text and the stats take their rows from the block's room,
	// smallest first; the board is drawn at the caps the window can hold and
	// takes the plan's height, so the three parts never disagree.
	chat := t.usesChatBox()
	plan := t.planBoardHeight()
	if plan <= 0 {
		plan = boardMinRows()
	}
	textMin := layoutTextRows
	if sh < blockBoxMinHeight+2+layoutTextRows+boardMinRows() {
		textMin = 2
	}
	statsMin := 2
	if sh < 2+blockBoxMinHeight+2+textMin+boardMinRows()+2 {
		statsMin = 1
	}
	room := sh - 2
	if room < 1 {
		room = 1
	}
	chatH := 0
	if chat {
		chatH = chatsuboBoxHeight(sh)
		if chatH > sh/4 {
			chatH = sh / 4
		}
		if chatH < blockBoxMinHeight {
			chatH = blockBoxMinHeight
		}
	}
	// Read the plan first; the box then grows only into rows the text and the
	// stats do not need.
	textRows := textMin
	statsH := statsMin
	{
		keysPlan := plan
		if keysPlan < boardMinRows() {
			keysPlan = boardMinRows()
		}
		if spare := room - chatH - 2 - textRows - keysPlan - statsH; spare > 0 {
			growText := layoutTextRows - textRows
			if growText > spare {
				growText = spare
			}
			textRows += growText
			spare -= growText
			if chat {
				if grow := chatsuboMaxHeight - chatH; spare > 0 && grow > 0 {
					if spare > grow {
						spare = grow
					}
					chatH += spare
				}
			}
		}
	}
	// A very short window drops the equalizer before a text row.
	if room-chatH-2-textRows-plan-statsH < 0 && statsH > 1 {
		statsH = 1
	}
	keysH := plan
	if keysH < boardMinRows() {
		keysH = boardMinRows()
	}
	if keysH < boardH {
		keysH = boardH
	}
	total := chatH + 2 + textRows + keysH + statsH
	y0 := (sh - total) / 2
	if y0+total > sh-2 {
		y0 = sh - 2 - total
	}
	if y0 < 0 {
		y0 = 0
	}
	p := partLayout{}
	y := y0
	p.chat = band{y, chatH}
	y += chatH
	p.text = band{y + pad, textRows}
	y += pad + textRows + pad
	p.keys = band{y, keysH}
	y += keysH
	p.stats = band{y, statsH}
	return p
}

// layoutFrame reports where the block sits: the first practice
// text row, the board's top row and the board's width. The text has its own
// band between the box and the board, so it never lands inside the box.
func (t *typer) layoutFrame() (textTop, boardTop, width int, kbOK bool) {
	m, kbOK := t.keyboardMetrics()
	if !kbOK {
		return 0, 0, 0, false
	}
	width = t.keyboardWidth(m)
	textTop = t.parts.text.Y
	return textTop, m.top, width, true
}

// textRows is how many rows the practice text band has.
func (t *typer) textRows() int {
	if t.parts.text.H > 0 {
		return t.parts.text.H
	}
	return layoutTextRows
}

// modifierAbbrev is the one-letter form used when a key cap is too narrow for
// the three-letter name (e.g. LCT → C, LSF → S).
var modifierAbbrev = map[string]string{
	"LCT": "C", "RCT": "C",
	"LSF": "S", "RSF": "S",
	"LAL": "A", "RAL": "A",
	"LGU": "G", "RGU": "G",
	"ALG": "A",
}

func buttonLabel(label string, max int) string {
	r := []rune(label)
	if len(r) <= max {
		return label
	}
	// A cap narrower than its label still shows one recognisable cell (the
	// modifier abbreviation, a function index, or the first rune) rather than
	// collapsing to nothing on a tiny terminal.
	if max <= 0 {
		max = 1
	}
	if abbrev, ok := modifierAbbrev[label]; ok && len([]rune(abbrev)) <= max {
		return abbrev
	}
	if len(r) >= 2 && r[0] == 'F' {
		tail := r[1:]
		if len(tail) > max {
			tail = tail[len(tail)-max:]
		}
		return string(tail)
	}
	if len(r) > max {
		return string(r[:max])
	}
	return label
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
		// A one-row cap is a labelled cell; brackets only when they leave room
		// for a real label, so a single-cell key still shows its character.
		inlet := width - 2
		if width < 3 {
			inlet = width
		}
		shown := label
		if top != "" {
			combined := top + " " + label
			if len([]rune(combined)) > inlet {
				combined = top + label
			}
			shown = combined
		}
		shown = buttonLabel(shown, inlet)
		fillRow(y)
		labelRunes := []rune(shown)
		start := x + (width-len(labelRunes))/2
		if width >= 3 {
			scr.SetContent(x, y, '[', nil, border)
			scr.SetContent(x+width-1, y, ']', nil, border)
		}
		for i, c := range labelRunes {
			scr.SetContent(start+i, y, c, nil, style)
		}
		return
	}

	// A two-row cap is a closed key with the label on its only body row. A cap
	// too narrow for its label still shows its first character (never an empty
	// key between the sides), and the label is re-fitted to the cells the
	// sides leave.
	if height == 2 && width >= 2 {
		fillRow(y)
		fillRow(y + 1)
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
		scr.SetContent(x, y+1, '│', nil, border)
		scr.SetContent(x+width-1, y+1, '│', nil, border)
		for i := 1; i < width-1; i++ {
			scr.SetContent(x+i, y+1, ' ', nil, style)
		}
		inlet := width - 2
		if inlet < 1 {
			inlet = 1
		}
		lr := []rune(buttonLabel(label, inlet))
		start := x + (width-len(lr))/2
		for i, c := range lr {
			scr.SetContent(start+i, y+1, c, nil, style)
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

		// Only draw the bottom border when there is a separate row for it, so
		// a two-row cap keeps its label row intact instead of overwriting the
		// border with the text; the label row still gets its sides.
		if height >= 3 {
			by := y + height - 1
			scr.SetContent(x, by, '╰', nil, border)
			scr.SetContent(x+width-1, by, '╯', nil, border)
			for i := 1; i < width-1; i++ {
				scr.SetContent(x+i, by, '─', nil, border)
			}
		}
	}

	mid := y + height/2
	if height == 2 {
		mid = y + 1
	}
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
	if t.useVil() {
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
	if t.useVil() {
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
	kbSplitCol = 5
	// boardHalfGap is the blank columns between the two hands of the board:
	// one continuous grid, with the hands kept readable by their home gaps
	// and outer macro columns, not by a split keyboard's gap. It is fixed, so
	// no window trades it away for a wider cap.
	boardHalfGap = 2
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

// The home gap is measured as a fraction of a key column, not as a fixed
// number of cells: a column count that ignores the cap width vanishes next to
// wide caps (four blank cells beside an eight-wide key is ordinary letter
// spacing, not a gap).
const (
	homeGapNumer = 1
	homeGapDenom = 2
)

// The block's vertical rhythm: one blank row around the practice text, the
// lines the text is planned to take, and the equalizer plus readout under the
// board.
const (
	layoutBandPad   = 1
	layoutTextRows  = 4
	layoutStatsRows = 2
)

// homeGapStep returns the step of one key column.
func homeGapStep(keyWidth, keyGap int) int { return keyWidth + keyGap }

// homeGapXIn returns the blank space taken by one home gap. The board is
// drawn with keys touching, so this is just the inter-key gap (zero for the
// built-in board).
func homeGapXIn(keyWidth, keyGap int) int {
	return keyGap
}

// kbHomeBlock describes the four home columns (a r s t / n e i o) of one half
// and which of their sides are opened up by a home gap.
type kbHomeBlock struct {
	start, cols   int
	before, after bool
}

// built-in home columns, taken from the home row of kbRows:
//
//	col 0 1 2 3 4   5 6 7 8 9 10
//	key a r s t g   m n e i o  '
//
// so a r s t are the left half's columns 0..3 and n e i o the right half's
// columns 1..4. In the split each half draws from its inner edge, where the
// split gap already separates the two halves, so the gap goes on the side with
// a column beside it: after t and after o.
var (
	kbLeftHome  = kbHomeBlock{start: 0, cols: 4, after: true}
	kbRightHome = kbHomeBlock{start: 1, cols: 4, after: true}
)

// halfBlock returns block if it fits a half of halfCols columns, otherwise a
// block covering the whole half (no home gap).
func halfBlock(halfCols int, block kbHomeBlock) (kbHomeBlock, bool) {
	if block.cols == 0 || block.start+block.cols > halfCols {
		return kbHomeBlock{start: 0, cols: halfCols}, false
	}
	// before/after describe padding, not the existence of a neighbouring
	// column: a block starting at column 0 is still padded on its outer side.
	block.after = block.after && block.start+block.cols < halfCols
	return block, true
}

// homeColX returns the x offset of column col relative to its half's origin,
// counting the home-column gaps. Widths and drawing both use it.
func homeColX(halfCols int, block kbHomeBlock, col, keyWidth, keyGap int) int {
	if col < 0 {
		return 0
	}
	block, framed := halfBlock(halfCols, block)
	if !framed {
		return col * homeGapStep(keyWidth, keyGap)
	}
	gap := homeGapXIn(keyWidth, keyGap)
	x := col * homeGapStep(keyWidth, keyGap)
	if block.before && col >= block.start {
		x += gap
	}
	if block.after && col >= block.start+block.cols {
		x += gap
	}
	return x
}

// lenWithGap is the width of halfCols key columns plus the home gaps, if any.
func lenWithGap(halfCols int, block kbHomeBlock, keyWidth, keyGap int) int {
	return homeColX(halfCols, block, halfCols, keyWidth, keyGap) - keyGap
}

// kbHalfCols is the number of key columns in each half of the grid. The right
// half is wider because its rows carry the jutting ; [ ] \ tail; the left half
// is padded to the same width so the grid is symmetric.
const kbHalfCols = 8

func kbSplitLeftWidth(keyWidth, keyGap int) int {
	return lenWithGap(kbHalfCols, kbLeftHome, keyWidth, keyGap)
}

// kbSplitLeftPad is blank space added to the right of the left half so both
// halves take the same width. The right half's rows jut out (; [ ] \ and ')
// which makes it wider; the padding goes inside the split gap, next to the
// left half, so the halves and the Chatsubo box stay centred without shifting
// the left half's keys.
func kbSplitLeftPad(keyWidth, keyGap int) int {
	pad := kbSplitRightWidth(keyWidth, keyGap) - kbSplitLeftWidth(keyWidth, keyGap)
	if pad < 0 {
		return 0
	}
	return pad
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
		return kbOffset(row, keyWidth) + homeColX(kbHalfCols, kbLeftHome, col, keyWidth, keyGap), false
	}
	max := 0
	for _, r := range kbRows {
		if n := len(r.keys) - kbSplitCol; n > max {
			max = n
		}
	}
	return homeColX(max, kbRightHome, col-kbSplitCol, keyWidth, keyGap), true
}

// thumbCells is how many grid columns a half's thumb cluster spans. The
// cluster is flush with the inner edge of the half, so it lines up with the
// same column grid as every other row.
const thumbCells = 3

// thumbStart returns the first home column the thumb cluster sits under. It
// favours the inner three home columns (r s t / n e i) so the cluster tucks
// toward the split rather than the outer edge, falling back to the first three
// when the inner ones would run past the half.
func thumbStart(halfCols int, block kbHomeBlock) int {
	block, _ = halfBlock(halfCols, block)
	start := block.start + 1
	if start+thumbCells > block.start+block.cols && block.cols >= thumbCells {
		start = block.start + block.cols - thumbCells
	}
	if start < 0 {
		start = 0
	}
	return start
}

// homeSpaceRect returns the left edge and width of a half's thumb bar. The bar
// starts at the same grid column as the first home key it sits under and spans
// thumbCells uniform keys, so its left edge lines up with that key while the
// thumb keys themselves stay evenly spaced like a real thumb cluster.
func homeSpaceRect(halfCols int, block kbHomeBlock, keyWidth, keyGap int) (x, width int) {
	start := thumbStart(halfCols, block)
	x = homeColX(halfCols, block, start, keyWidth, keyGap)
	width = thumbCells*keyWidth + (thumbCells-1)*keyGap
	return x, width
}

func kbSplitWidth(keyWidth, keyGap, splitGap int) int {
	left := kbSplitLeftWidth(keyWidth, keyGap) + kbSplitLeftPad(keyWidth, keyGap)
	return left + splitGap + kbSplitRightWidth(keyWidth, keyGap)
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
		leftWidth = kbSplitLeftWidth(m.keyWidth, m.keyGap) + kbSplitLeftPad(m.keyWidth, m.keyGap)
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
		y := rowY(m.top, i, m.keyH, m.rowGap, false)
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
					// Centre the left half's keys in its region so the pad
					// taken to match the wider right half splits evenly,
					// leaving the hands symmetric about the centre line.
					x = x0 + kbSplitLeftPad(m.keyWidth, m.keyGap)/2 + gx
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
			// Dim only the half which is not in play; the active finger column
			// is already picked out by the brighter home-key colour.
			if !highlighted && !focused {
				style = style.Dim(true)
				border = border.Dim(true)
			}

			drawKBButton(t.Scr, x, y, m.keyWidth, m.keyH, string(ch), "", style, border)
		}
	}

	spaceY := rowY(m.top, len(kbRows), m.keyH, m.rowGap, true)

	if split {
		// Each thumb bar sits under its half's home block, aligned with the
		// arst / oien columns and the home gap in front of them.
		rightCols := len(kbRows[0].keys) - kbSplitCol
		leftX, leftSpace := homeSpaceRect(kbSplitCol, kbLeftHome, m.keyWidth, m.keyGap)
		rightX, rightSpace := homeSpaceRect(rightCols, kbRightHome, m.keyWidth, m.keyGap)
		rightX += leftWidth + m.splitGap
		// Match the keys' centring so the bar stays under the left half.
		leftX += kbSplitLeftPad(m.keyWidth, m.keyGap) / 2

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
