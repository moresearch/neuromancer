package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell"
)

// Vial stores a keymap as a stack of 2D matrices. Each cell is a QMK keycode
// string such as "KC_Q", "MO(2)", "LT(1,KC_SPC)" or -1 for a missing key.
type vilFile struct {
	Layout   []interface{} `json:"layout"`
	TapDance []interface{} `json:"tap_dance"`
}

type vilAction struct {
	raw         string
	label       string
	display     string // keycap label (may include an HRM prefix)
	output      rune   // character produced directly by this key
	shifted     rune   // character produced when shift is held
	layer       int
	layerMode   string // MO, LT, OSL, TG, TO, DF, TT, LM
	mods        string // mod-tap modifier: Ctrl, Shift, Alt, Gui
	isShift     bool
	transparent bool
	none        bool
}

type vilLayer struct {
	matrix  [][]string
	actions [][]vilAction
}

type vilKeyboard struct {
	name       string
	layers     []*vilLayer
	layerNames []string
	tapDances  []tapDance
	halfRows   int
	halfCols   int
	splitCols  bool
	homeRow    int
	maxCols    int

	thumbLeftRow, thumbLeftCol   int
	thumbRightRow, thumbRightCol int
	thumbLeftOK, thumbRightOK    bool

	homeKeys  map[[2]int]bool
	homeLeft  []homePos
	homeRight []homePos

	// thumbX holds the laid-out thumb columns of each thumb row, tucked
	// against the inner edge of its half and evenly spaced, so they never
	// inherit an alpha row's home gap.
	thumbX map[int][]clusterKey

	// peripheralCols is the set of matrix columns holding only the edge
	// macro/function keys (the outer column on each side), and peripheralRows
	// the top function row of each half. They are drawn with a small gap from
	// the central typing block so the keys in active use stand apart.
	peripheralCols map[int]bool
	peripheralRows map[int]bool

	// hasBoard reports whether a row-split matrix is drawn as the board (one
	// drawn row per matrix row). boardHalfCols is the columns per half.
	hasBoard      bool
	boardHalfCols int
	// boardRowOf maps each drawn row to the matrix row it shows; the thumb
	// row's entry is -1.
	boardRowOf []int

	// pressedAt reports whether a physical matrix cell is held, set for the
	// duration of a draw so a key with no action still lights when pressed.
	pressedAt func(row, col int) bool
}

// clusterKey is the x offset of one key in a row laid out on its own, in
// columns from the start of its half.
type clusterKey struct {
	col int
	x   int
}

type homePos struct {
	row, col int
}

type vilHint struct {
	row, col  int
	action    vilAction
	layer     int
	shift     bool
	shiftRow  int
	shiftCol  int
	activator *vilActivator
}

type vilActivator struct {
	row, col int
	action   vilAction
}

type vilKeyRef struct {
	row, col int
	action   vilAction
}

var vilCharMap = map[string]rune{
	"SPC": ' ', "SPACE": ' ',
	"MINS": '-', "MINUS": '-', "EQL": '=', "EQUAL": '=',
	"LBRC": '[', "RBRC": ']', "LEFT_BRACKET": '[', "RIGHT_BRACKET": ']',
	"BSLS": '\\', "BACKSLASH": '\\',
	"SCLN": ';', "SCOLON": ';', "SEMICOLON": ';',
	"QUOT": '\'', "QUOTE": '\'', "APOSTROPHE": '\'',
	"GRV": '`', "GRAVE": '`', "COMM": ',', "COMMA": ',',
	"DOT": '.', "PERIOD": '.', "SLSH": '/', "SLASH": '/',
	"EXLM": '!', "AT": '@', "HASH": '#', "DLR": '$', "PERC": '%',
	"CIRC": '^', "AMPR": '&', "ASTR": '*', "LPRN": '(', "RPRN": ')',
	"UNDS": '_', "PLUS": '+', "LCBR": '{', "RCBR": '}', "PIPE": '|',
	"COLN": ':', "DQUO": '"', "DOUBLE_QUOTE": '"',
	"TILD": '~', "LABK": '<', "RABK": '>',
	"QUES": '?',
}

var vilShiftMap = map[rune]rune{
	'1': '!', '2': '@', '3': '#', '4': '$', '5': '%',
	'6': '^', '7': '&', '8': '*', '9': '(', '0': ')',
	'-': '_', '=': '+', '[': '{', ']': '}', '\\': '|',
	';': ':', '\'': '"', '`': '~', ',': '<', '.': '>', '/': '?',
}

var vilSpecialMap = map[string]string{
	"ESC": "ESC", "ESCAPE": "ESC", "ENT": "ENT", "ENTER": "ENT",
	"BSPC": "BSP", "BSPACE": "BSP", "BACKSPACE": "BSP",
	"TAB": "TAB", "DEL": "DEL", "DELETE": "DEL",
	"HOME": "HOM", "END": "END", "PGUP": "PGU", "PGDN": "PGD",
	"LEFT": "←", "RGHT": "→", "DOWN": "↓", "UP": "↑",
	"INS": "INS", "INSERT": "INS",
	"LSFT": "SFT", "LSHIFT": "SFT", "RSFT": "SFT", "RSHIFT": "SFT",
	"LCTL": "CTL", "LCTRL": "CTL", "RCTL": "CTL", "RCTRL": "CTL",
	"LALT": "ALT", "RALT": "ALT", "LGUI": "GUI", "RGUI": "GUI",
	"CAPS": "CAP", "CAPSLOCK": "CAP",
	"PSCR": "PSC", "PSCREEN": "PSC", "SCRL": "SCR", "SCROLLLOCK": "SCR",
	"PAUS": "PAU", "PAUSE": "PAU",
	"MPLY": "PLY", "MUTE": "MUT", "VOLU": "V+", "VOLD": "V-",
	"MNXT": "NXT", "MPRV": "PRV", "MSTP": "STP",
	"MS_BTN1": "BT1", "MS_BTN2": "BT2", "MS_BTN3": "BT3",
	"MS_BTN4": "BT4", "MS_BTN5": "BT5",
	"MS_UP": "M↑", "MS_DOWN": "M↓", "MS_LEFT": "M←", "MS_RIGHT": "M→",
	"MS_WH_UP": "W↑", "MS_WH_DOWN": "W↓", "MS_WH_LEFT": "W←", "MS_WH_RIGHT": "W→",
}

func loadVil(path, layerNames string) (*vilKeyboard, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var f vilFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("invalid .vil JSON: %v", err)
	}
	if len(f.Layout) == 0 {
		return nil, fmt.Errorf("layout is empty")
	}

	kb := &vilKeyboard{name: filepath.Base(path)}
	for _, n := range strings.Split(layerNames, ",") {
		if n = strings.TrimSpace(n); n != "" {
			kb.layerNames = append(kb.layerNames, n)
		}
	}
	kb.tapDances = parseVilTapDances(f.TapDance)

	for li, layerVal := range f.Layout {
		rawLayer, ok := layerVal.([]interface{})
		if !ok || len(rawLayer) == 0 {
			return nil, fmt.Errorf("layer %d is empty", li)
		}

		lay := &vilLayer{}
		for _, rawRow := range vilLayerRows(rawLayer) {
			row := make([]string, len(rawRow))
			for ci, cell := range rawRow {
				row[ci] = vilCellString(cell)
			}
			lay.matrix = append(lay.matrix, row)
			if len(row) > kb.maxCols {
				kb.maxCols = len(row)
			}
		}

		lay.actions = make([][]vilAction, len(lay.matrix))
		for ri, row := range lay.matrix {
			lay.actions[ri] = make([]vilAction, len(row))
			for ci, raw := range row {
				lay.actions[ri][ci] = parseVilKey(raw, kb.tapDances, 0)
			}
		}

		kb.layers = append(kb.layers, lay)
	}

	rows := len(kb.layers[0].matrix)
	switch {
	case kb.maxCols >= 10:
		// Full-width matrix: left and right halves share each row.
		kb.splitCols = true
		kb.halfCols = kb.maxCols / 2
	case rows >= 6 && rows%2 == 0:
		// Split matrix: the left half's rows are followed by the right half's.
		kb.halfRows = rows / 2
	case rows <= 5 && kb.maxCols >= 6:
		// A short, wide matrix is a full-width split keyboard.
		kb.splitCols = true
		kb.halfCols = kb.maxCols / 2
	}

	if kb.halfRows > 0 {
		kb.homeRow = (kb.halfRows - 1) / 2
	} else {
		kb.homeRow = (rows - 1) / 2
	}
	kb.findHomeKeys()
	kb.findThumbHomes()
	kb.placeOnBoard()
	kb.buildPeripheral()

	return kb, nil
}

func (kb *vilKeyboard) hrmLegend() string {
	seen := map[string]bool{}
	var parts []string
	for r := range kb.layers[0].matrix {
		for c := range kb.layers[0].matrix[r] {
			a := kb.actionAt(0, r, c)
			if a.mods == "" || a.output == 0 {
				continue
			}
			part := fmt.Sprintf("%c=%s", a.output, a.mods)
			if seen[part] {
				continue
			}
			seen[part] = true
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

func (kb *vilKeyboard) info() string {
	var b strings.Builder
	base := kb.layers[0].matrix

	split := "none"
	switch {
	case kb.splitCols:
		split = "cols"
	case kb.halfRows > 0:
		split = "rows"
	}

	fmt.Fprintf(&b, "layers: %d\n", len(kb.layers))
	fmt.Fprintf(&b, "matrix: %d rows x %d cols\n", len(base), kb.maxCols)
	fmt.Fprintf(&b, "split: %s\n", split)
	fmt.Fprintf(&b, "home row: %d\n", kb.homeRow)
	fmt.Fprintf(&b, "home keys: %d\n", len(kb.homeKeys))
	if hrm := kb.hrmLegend(); hrm != "" {
		fmt.Fprintf(&b, "HRM: %s\n", hrm)
	}
	if kb.thumbLeftOK {
		fmt.Fprintf(&b, "thumb left: row %d col %d\n", kb.thumbLeftRow, kb.thumbLeftCol)
	}
	if kb.thumbRightOK {
		fmt.Fprintf(&b, "thumb right: row %d col %d\n", kb.thumbRightRow, kb.thumbRightCol)
	}
	b.WriteString("purposes:")
	for i := range kb.layers {
		fmt.Fprintf(&b, " %d=%s", i, kb.hintLayerName(i))
	}
	b.WriteString("\n")

	for li, lay := range kb.layers {
		fmt.Fprintf(&b, "\nlayer %d (%s):\n", li, kb.hintLayerName(li))
		for _, row := range lay.actions {
			for _, a := range row {
				label := a.label
				if a.display != "" {
					label = a.display
				}
				if a.transparent {
					label = "___"
				} else if label == "" {
					label = "."
				}
				fmt.Fprintf(&b, "%-4s", label)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (kb *vilKeyboard) sideFor(row, col int) int {
	if kb.halfRows > 0 {
		if row < kb.halfRows {
			return -1
		}
		return 1
	}
	if kb.splitCols {
		if col < kb.halfCols {
			return -1
		}
		return 1
	}
	return 0
}

func (kb *vilKeyboard) onHomeRow(row, col int) bool {
	if kb.halfRows > 0 {
		if row < kb.halfRows {
			return row == kb.homeRow
		}
		return row == kb.halfRows+kb.homeRow
	}
	return row == kb.homeRow
}

func (kb *vilKeyboard) findHomeKeys() {
	kb.homeKeys = map[[2]int]bool{}
	kb.homeLeft = nil
	kb.homeRight = nil

	wanted := map[rune]bool{
		'a': true, 'r': true, 's': true, 't': true,
		'n': true, 'e': true, 'i': true, 'o': true,
	}
	seen := map[rune]bool{}

	add := func(homeRowOnly bool) {
		for r := range kb.layers[0].matrix {
			for c := range kb.layers[0].matrix[r] {
				if homeRowOnly && !kb.onHomeRow(r, c) {
					continue
				}
				a := kb.actionAt(0, r, c)
				if !wanted[a.output] || seen[a.output] {
					continue
				}
				seen[a.output] = true
				key := [2]int{r, c}
				kb.homeKeys[key] = true
				switch kb.sideFor(r, c) {
				case -1:
					kb.homeLeft = append(kb.homeLeft, homePos{r, c})
				case 1:
					kb.homeRight = append(kb.homeRight, homePos{r, c})
				}
			}
		}
	}

	add(true)
	if len(kb.homeKeys) < 8 {
		add(false)
	}

	sort.Slice(kb.homeLeft, func(i, j int) bool { return kb.homeLeft[i].col < kb.homeLeft[j].col })
	sort.Slice(kb.homeRight, func(i, j int) bool { return kb.homeRight[i].col < kb.homeRight[j].col })
}

func (kb *vilKeyboard) nearestHome(homes []homePos, col int) (homePos, bool) {
	if len(homes) == 0 {
		return homePos{}, false
	}
	best := homes[0]
	bestDist := best.col - col
	if bestDist < 0 {
		bestDist = -bestDist
	}
	for _, h := range homes[1:] {
		d := h.col - col
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			best, bestDist = h, d
		}
	}
	return best, true
}

func (kb *vilKeyboard) innerThumbCol(row, start, end int, left bool) (int, bool) {
	if row < 0 || row >= len(kb.layers[0].matrix) {
		return 0, false
	}
	rowKeys := kb.layers[0].matrix[row]
	if end > len(rowKeys) {
		end = len(rowKeys)
	}

	if left {
		for c := end - 1; c >= start; c-- {
			if rowKeys[c] != "" {
				return c, true
			}
		}
	} else {
		for c := start; c < end; c++ {
			if rowKeys[c] != "" {
				return c, true
			}
		}
	}
	return 0, false
}

func (kb *vilKeyboard) thumbHomeCol(row, start, end int, left bool) (int, bool) {
	if row >= 0 && row < len(kb.layers[0].matrix) {
		rowKeys := kb.layers[0].matrix[row]
		for c := start; c < end && c < len(rowKeys); c++ {
			if rowKeys[c] == "" {
				continue
			}
			if a := kb.actionAt(0, row, c); a.output == ' ' {
				return c, true
			}
		}
	}
	return kb.innerThumbCol(row, start, end, left)
}

func (kb *vilKeyboard) findThumbHomes() {
	if kb.halfRows >= 4 {
		leftRow := kb.halfRows - 1
		if c, ok := kb.thumbHomeCol(leftRow, 0, kb.maxCols, true); ok {
			kb.thumbLeftRow, kb.thumbLeftCol, kb.thumbLeftOK = leftRow, c, true
		}

		rightRow := 2*kb.halfRows - 1
		if c, ok := kb.thumbHomeCol(rightRow, 0, kb.maxCols, false); ok {
			kb.thumbRightRow, kb.thumbRightCol, kb.thumbRightOK = rightRow, c, true
		}
		return
	}

	if kb.splitCols {
		row := len(kb.layers[0].matrix) - 1
		if c, ok := kb.thumbHomeCol(row, 0, kb.halfCols, true); ok {
			kb.thumbLeftRow, kb.thumbLeftCol, kb.thumbLeftOK = row, c, true
		}
		if c, ok := kb.thumbHomeCol(row, kb.halfCols, kb.maxCols, false); ok {
			kb.thumbRightRow, kb.thumbRightCol, kb.thumbRightOK = row, c, true
		}
	}
}

func homeDistanceFor(halfCols, d int) int {
	if halfCols <= 4 {
		if d < 0 {
			return 0
		}
		if d >= halfCols {
			return halfCols - 1
		}
		return d
	}
	switch {
	case d <= 0:
		return 1 // inner index key (g/m) rests on the index home
	case d >= 5:
		return 4 // outer macro columns use the pinky home
	default:
		return d
	}
}

func isHomeDistance(halfCols, d int) bool {
	if halfCols <= 4 {
		return d >= 0 && d < halfCols
	}
	return d >= 1 && d <= 4
}

// cellPresent reports whether row has a key at col.
func (kb *vilKeyboard) cellPresent(row, col int) bool {
	if row < 0 || row >= len(kb.layers[0].matrix) {
		return false
	}
	r := kb.layers[0].matrix[row]
	return col >= 0 && col < len(r) && r[col] != ""
}

// halfFor returns the number of columns of the half holding matrix column col.
func (kb *vilKeyboard) halfFor(col int) (halfCols int) {
	switch {
	case kb.splitCols && col < kb.halfCols:
		return kb.halfCols
	case kb.splitCols:
		return kb.maxCols - kb.halfCols
	}
	return kb.maxCols
}

// vilHomeBlock is the home block of a Vial half. The four home keys (a r s t /
// n e i o) sit one column in from the inner edge on the left half and two in
// on the right, because the right half carries its outer macro column past the
// ' tail:
//
//	outer | a r s t | rest        inner | n e i o | outer
//
// The gap goes on the open sides, between the home keys and the resting outer
// and stretch index columns. In a matrix split by columns the two halves sit
// side by side, so the split edge is on the inner side and carries the gap
// itself. A half which is no wider than the block (plus its outer column) has
// no gap.
func vilHomeBlock(halfCols int, right, rowSplit bool) kbHomeBlock {
	if halfCols <= 4 {
		return kbHomeBlock{start: 0, cols: halfCols}
	}
	// The home columns are the ones the fingers rest on: a r s t on the left
	// half and n e i o on the right. The right half starts one column later
	// because its first column is the outer index key (m, beside the split).
	// g, m(->l) and the jutting keys are not home columns.
	start := 1
	if right {
		start = 2
	}
	return kbHomeBlock{start: start, cols: 4, before: !rowSplit, after: true}
}

// homeBlock is the home block of one half of the matrix.
func (kb *vilKeyboard) homeBlock(right bool) kbHomeBlock {
	return vilHomeBlock(kb.maxCols-kb.halfCols, right, kb.halfRows > 0)
}

// boardHomeBlock is the Sofle board's home block of one half: the four finger
// columns (a r s t / n e i o), one column in from the outer edge, with the gap
// on the outer side only so the alphas stay adjacent to their own macro
// column and the split.
// boardHomeBlockFor is the home block of a keymap whose home keys follow the
// alpha columns: the first four columns (a r s t / n e i o), the gap going
// toward the outer macro column.
func boardHomeBlockFor(cols int) kbHomeBlock {
	if cols <= 4 {
		return kbHomeBlock{start: 0, cols: cols}
	}
	return kbHomeBlock{start: 0, cols: 4, after: true}
}

// homeBlockAt is the home block of the half holding column col. A matrix split
// by rows puts the right half in its lower rows, so the row decides the side.
func (kb *vilKeyboard) homeBlockAt(row, col int) kbHomeBlock {
	return kb.homeBlock(kb.rightHalfAt(row, col))
}

// rightHalfAt reports whether (row, col) is in the right half.
func (kb *vilKeyboard) rightHalfAt(row, col int) bool {
	if kb.halfRows > 0 {
		return row >= kb.halfRows
	}
	if kb.splitCols {
		return col >= kb.halfCols
	}
	return false
}

// colX returns the x offset of column col relative to the half which contains
// it, including the home-column gap. Drawing and width both go through it.
func (kb *vilKeyboard) colX(row, col, keyWidth, keyGap int) int {
	return kb.colXSide(row, col, false, keyWidth, keyGap)
}

// colXSide is colX with the side known. A row-split matrix draws each half from
// its own inner edge, so a column cannot be attributed to a half on its own.
func (kb *vilKeyboard) colXSide(row, col int, right bool, keyWidth, keyGap int) int {
	// Keys are packed edge to edge: the board is drawn as one contiguous grid
	// with no gap between caps and no extra home/peripheral gaps. Only the
	// split between the two halves keeps a gap.
	step := keyWidth
	if kb.hasBoard {
		if keys, ok := kb.thumbX[row]; ok {
			maxX := 0
			for _, k := range keys {
				if k.x > maxX {
					maxX = k.x
				}
			}
			for _, k := range keys {
				if k.col == col {
					if right {
						return (maxX - k.x) * step
					}
					return k.x * step
				}
			}
			return col * step
		}
		return col * step
	}
	if keys, ok := kb.thumbX[row]; ok {
		want := col
		if kb.splitCols {
			want = col % kb.halfCols
		}
		for _, k := range keys {
			if k.col == want {
				return k.x * step
			}
		}
		return col * step
	}
	return col * step
}

// peripheralGapIn is the blank space between the edge macro/function columns
// (or the top function row) and the central typing block. It is a fraction of
// a key column, like the home gap, so it stays visible at any cap size.
func peripheralGapIn(keyWidth, keyGap int) int {
	gap := homeGapStep(keyWidth, keyGap) / 4
	if gap < 1 {
		gap = 1
	}
	return keyGap + gap
}

// buildPeripheral marks the matrix columns and rows that hold only the edge
// macro/function keys — the outer column on each side and the top function row
// — so they can be drawn a little apart from the central block the user types
// on. A column or row is peripheral when none of its base-layer cells produces
// a typing character (they are modifiers, layer keys, tap-dances or F-keys),
// so the call is content-driven rather than a fixed column guess.
func (kb *vilKeyboard) buildPeripheral() {
	kb.peripheralCols = map[int]bool{}
	kb.peripheralRows = map[int]bool{}
	if kb.halfRows <= 0 || kb.maxCols < 5 {
		return
	}
	// A cell is a typing key when it produces a character directly.
	typing := func(row, col int) bool {
		if !kb.cellPresent(row, col) {
			return false
		}
		return kb.actionAt(0, row, col).output != 0
	}
	// A column is peripheral if no cell in it types a character.
	for c := 0; c < kb.maxCols; c++ {
		anyType := false
		for r := 0; r < len(kb.layers[0].matrix); r++ {
			if typing(r, c) {
				anyType = true
				break
			}
		}
		if !anyType {
			kb.peripheralCols[c] = true
		}
	}
	// A row is peripheral if no cell in it types a character; only the top row
	// of each half is considered, so the thumb row is never marked this way.
	// The top row only stands apart when the drawing shows it apart: a board
	// whose alphas start below it (a 4-row half with a macro row keeps its
	// alphas on the board's alpha rows; a 5-row Miryoku half is drawn with no
	// macro row).
	if !kb.hasTypingRow(0) && kb.alphasFitBelowTopRow() {
		kb.peripheralRows[0] = true
	}
	if kb.halfRows > 0 && !kb.hasTypingRow(kb.halfRows) && kb.alphasFitBelowTopRow() {
		kb.peripheralRows[kb.halfRows] = true
	}
}

// alphasFitBelowTopRow reports whether the board can keep the keymap's
// non-typing top row as a macro row above four alpha rows: the keymap then
// needs five rows above its thumbs, or has blank filler in the top row.
func (kb *vilKeyboard) alphasFitBelowTopRow() bool {
	if kb.halfRows <= 0 {
		return true
	}
	alphaRows := kb.halfRows
	if kb.isThumbRow(kb.halfRows - 1) {
		alphaRows--
	}
	// Four alpha rows plus the macro row need five matrix rows; the thumb
	// row is one of them, so a half of five rows before the thumbs has it.
	return alphaRows >= sofleRows
}

// hasTypingRow reports whether half row r has at least one typing key.
func (kb *vilKeyboard) hasTypingRow(r int) bool {
	if r < 0 || r >= len(kb.layers[0].matrix) {
		return false
	}
	for c := 0; c < kb.maxCols; c++ {
		if kb.cellPresent(r, c) && kb.actionAt(0, r, c).output != 0 {
			return true
		}
	}
	return false
}

// peripheralRow reports whether display row r is a peripheral (function) row.
func (kb *vilKeyboard) peripheralRow(r int) bool {
	return kb.peripheralRows[r]
}

// isPeripheral reports whether the key at (row, col) is a peripheral key.
func (kb *vilKeyboard) isPeripheral(row, col int) bool {
	if kb.peripheralRows[row] {
		return true
	}
	return kb.peripheralCols[col]
}

// Sofle-shaped board frame. The board keeps this shape whatever the .vil
// contains: six columns per hand, four rows, five thumb keys, and a knob at
// each hand's top-outer corner. A 5-column keymap (Miryoku's 36 keys with the
// macro column) is placed on the inner five columns, a full 6-column Sofle
// keymap keeps its outer macro column. A half of five rows gets a blank knob
// row at the top; four alpha rows follow, then the two thumb rows.
const (
	sofleCols       = 6
	sofleRows       = 4 // alpha rows
	sofleThumbs     = 5
	boardKnobMarker = '⌾'
)

// placeOnBoard places the matrix's rows on the board's drawn rows, one for
// one: a row-split matrix draws both halves row by row, so row r of the board
// is row r of the matrix on both halves (the right half reads its own rows,
// halfRows later). Only a column-split matrix goes unplaced: it already draws
// its halves from its own columns.
func (kb *vilKeyboard) placeOnBoard() {
	kb.hasBoard = false
	kb.boardHalfCols = 0
	kb.thumbX = map[int][]clusterKey{}
	if kb.halfRows <= 0 {
		kb.buildThumbRows()
		return
	}
	kb.hasBoard = true
	kb.boardHalfCols = kb.maxCols
	kb.buildThumbRows()
}

// buildThumbRows lays out each thumb row's keys as a cluster per half:
// evenly spaced, right-aligned to the half's last column so the cluster sits
// against its inner edge, with the same gaps as the alpha rows.
func (kb *vilKeyboard) buildThumbRows() {
	kb.thumbX = map[int][]clusterKey{}
	for r := range kb.layers[0].matrix {
		if !kb.isThumbRow(r) {
			continue
		}
		if kb.splitCols {
			kb.layoutThumbHalf(r, 0, kb.halfCols)
			kb.layoutThumbHalf(r, kb.halfCols, kb.maxCols)
			continue
		}
		kb.layoutThumbHalf(r, 0, kb.maxCols)
	}
}

// layoutThumbHalf records the thumb keys of row r between start and end.
func (kb *vilKeyboard) layoutThumbHalf(row, start, end int) {
	if end > len(kb.layers[0].matrix[row]) {
		end = len(kb.layers[0].matrix[row])
	}
	ref := kb.maxCols
	if kb.splitCols {
		ref = kb.halfCols
	}
	if kb.hasBoard {
		// The board draws every column of the half (seven on the Miryoku);
		// the thumb cluster starts at the inner column beside the split.
		ref = kb.boardHalfCols
	}
	count := 0
	for c := start; c < end; c++ {
		if kb.cellPresent(row, c) {
			count++
		}
	}
	// x counts from the half's origin in the drawn board: the keys hug the
	// inner edge first, then extend outward, as on the real board. The board is
	// drawn mirrored, so this x is for the left half; colXSide flips it for the
	// right half.
	inner := ref
	idx := 0
	for c := start; c < end; c++ {
		if !kb.cellPresent(row, c) {
			continue
		}
		x := inner - (count - idx)
		if x < 0 {
			x = 0
		}
		kb.thumbX[row] = append(kb.thumbX[row], clusterKey{col: c, x: x})
		idx++
	}
}

// halfWidth is the laid-out width of one half of a row-split matrix. It counts
// the home gap, the peripheral column gaps, and the right half's extra inner
// column, all through the same colXSide used for drawing.
func (kb *vilKeyboard) halfWidth(right bool, keyWidth, keyGap int) int {
	halfCols := kb.maxCols
	if kb.hasBoard {
		halfCols = kb.boardHalfCols
	} else if kb.halfRows > 0 {
		halfCols = kb.maxCols - kb.halfCols
	}
	if halfCols <= 0 {
		return 0
	}
	// The width is the x of the column one past the end, minus the trailing
	// key gap, so it covers every key plus its peripheral gaps.
	return kb.colXSide(0, halfCols, right, keyWidth, keyGap) - keyGap
}

// halfWidthWithGap is halfWidth for a matrix independent half of halfCols
// columns, left-sided.
func (kb *vilKeyboard) halfWidthWithGap(halfCols, keyWidth, keyGap int) int {
	if halfCols <= 0 {
		return 0
	}
	// Contiguous grid: halfCols caps edge to edge, no gaps.
	return halfCols * keyWidth
}

func (kb *vilKeyboard) targetHome(row, col int) (int, int, bool) {
	if kb.isThumbRow(row) {
		if kb.hasBoard {
			// The resting thumb is the keymap's thumb home, on its drawn
			// thumb key.
			if kb.thumbLeftOK {
				return row, kb.thumbLeftCol, true
			}
			if kb.thumbRightOK {
				return row, kb.thumbRightCol, true
			}
			return row, 2, true
		}
		if kb.halfRows >= 4 {
			if kb.thumbLeftOK && row == kb.thumbLeftRow {
				return kb.thumbLeftRow, kb.thumbLeftCol, true
			}
			if kb.thumbRightOK && row == kb.thumbRightRow {
				return kb.thumbRightRow, kb.thumbRightCol, true
			}
		} else if kb.splitCols {
			if col < kb.halfCols && kb.thumbLeftOK {
				return kb.thumbLeftRow, kb.thumbLeftCol, true
			}
			if kb.thumbRightOK {
				return kb.thumbRightRow, kb.thumbRightCol, true
			}
		}
		return 0, 0, false
	}

	if len(kb.homeKeys) > 0 {
		switch kb.sideFor(row, col) {
		case -1:
			if h, ok := kb.nearestHome(kb.homeLeft, col); ok {
				return h.row, h.col, true
			}
		case 1:
			if h, ok := kb.nearestHome(kb.homeRight, col); ok {
				return h.row, h.col, true
			}
		}
	}

	if kb.halfRows > 0 {
		if row < kb.halfRows {
			n := kb.maxCols
			hd := homeDistanceFor(n, n-1-col)
			return kb.homeRow, n - 1 - hd, true
		}
		n := kb.maxCols
		hd := homeDistanceFor(n, col)
		return kb.halfRows + kb.homeRow, hd, true
	}

	if kb.splitCols {
		leftCols := kb.halfCols
		if col < leftCols {
			hd := homeDistanceFor(leftCols, leftCols-1-col)
			return kb.homeRow, leftCols - 1 - hd, true
		}
		rightCols := kb.maxCols - leftCols
		hd := homeDistanceFor(rightCols, col-leftCols)
		return kb.homeRow, leftCols + hd, true
	}

	return 0, 0, false
}

func (kb *vilKeyboard) isHomeKey(row, col int) bool {
	if kb.hasBoard {
		// The resting keys follow the keymap's own home row, wherever the
		// placement put it on the board.
		block := boardHomeBlockFor(kb.maxCols)
		return row == kb.homeRow && col >= block.start && col < block.start+block.cols
	}
	if kb.homeKeys[[2]int{row, col}] {
		return true
	}
	if kb.thumbLeftOK && row == kb.thumbLeftRow && col == kb.thumbLeftCol {
		return true
	}
	if kb.thumbRightOK && row == kb.thumbRightRow && col == kb.thumbRightCol {
		return true
	}
	if len(kb.homeKeys) > 0 {
		return false
	}

	if kb.halfRows > 0 {
		if row == kb.homeRow {
			return isHomeDistance(kb.maxCols, kb.maxCols-1-col)
		}
		if row == kb.halfRows+kb.homeRow {
			return isHomeDistance(kb.maxCols, col)
		}
		return false
	}

	if kb.splitCols && row == kb.homeRow {
		leftCols := kb.halfCols
		if col < leftCols {
			return isHomeDistance(leftCols, leftCols-1-col)
		}
		return isHomeDistance(kb.maxCols-leftCols, col-leftCols)
	}

	return false
}

// thumbRowAt reports whether matrix row row is the thumb row, which is drawn a
// line below the alpha rows so the thumbs read as a separate cluster.
func (kb *vilKeyboard) thumbRowAt(row int) bool {
	if kb.hasBoard {
		return row == sofleRows
	}
	if kb.halfRows > 0 {
		// Only the row holding the thumbs is the thumb row. A Sofle matrix
		// carries five thumb rows per half (knob row included); the cluster
		// is a single drawn row.
		first := kb.halfRows - 1
		if kb.thumbLeftOK {
			first = kb.thumbLeftRow
		}
		return row == first
	}
	if row < 0 || row != kb.displayRows()-1 {
		return false
	}
	if kb.halfRows > 0 {
		// A row with no keys (a half's thumb row when the keymap leaves it
		// empty) is not drawn as a thumb row at all.
		return kb.rowHasKeys(row) || kb.rowHasKeys(row+kb.halfRows)
	}
	return kb.rowHasKeys(row)
}

// rowHasKeys reports whether row has a key in either half it is drawn in.
func (kb *vilKeyboard) rowHasKeys(row int) bool {
	if row < 0 || row >= len(kb.layers[0].matrix) {
		return false
	}
	end := kb.maxCols
	if kb.splitCols {
		end = kb.halfCols
	}
	for _, r := range kb.layers[0].matrix[row][:minInt(end, len(kb.layers[0].matrix[row]))] {
		if r != "" {
			return true
		}
	}
	return false
}

func (kb *vilKeyboard) isThumbRow(row int) bool {
	if kb.hasBoard {
		// Each half has its own thumb row: the left at sofleRows, the right
		// halfRows below it.
		return row == sofleRows || (kb.halfRows > 0 && row == sofleRows+kb.halfRows)
	}
	if kb.halfRows > 0 {
		if kb.halfRows < 4 {
			return false
		}
		return row == kb.halfRows-1 || row == 2*kb.halfRows-1
	}

	rows := len(kb.layers[0].matrix)
	return rows >= 4 && row == rows-1
}

func vilLayerRows(raw []interface{}) [][]interface{} {
	if len(raw) == 0 {
		return nil
	}

	if _, ok := raw[0].([]interface{}); ok {
		rows := make([][]interface{}, 0, len(raw))
		for _, r := range raw {
			if row, ok := r.([]interface{}); ok {
				rows = append(rows, row)
			}
		}
		return rows
	}

	return vilArrangeFlat(raw)
}

func vilArrangeFlat(cells []interface{}) [][]interface{} {
	n := len(cells)
	if n >= 58 {
		// Sofle/Lily58-style LAYOUT order: four 12-key rows followed by ten
		// thumb keys (five per hand).
		cells = cells[:58]
		rows := make([][]interface{}, 5)
		for i := 0; i < 4; i++ {
			rows[i] = append([]interface{}{}, cells[i*12:(i+1)*12]...)
		}
		row := append([]interface{}{}, cells[48:53]...)
		row = append(row, nil)
		row = append(row, cells[53:58]...)
		row = append(row, nil)
		rows[4] = row
		return rows
	}

	// Generic fallback: twelve columns per row.
	rows := make([][]interface{}, 0, (n+11)/12)
	for i := 0; i < n; i += 12 {
		end := i + 12
		if end > n {
			end = n
		}
		row := append([]interface{}{}, cells[i:end]...)
		for len(row) < 12 {
			row = append(row, nil)
		}
		rows = append(rows, row)
	}
	return rows
}

func vilCellString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		n := int(x)
		if n < 0 {
			return ""
		}
		return fmt.Sprintf("\x00%d", n)
	}
	return ""
}

// tapDance is one entry of the .vil tap_dance array. Vial writes either the
// tap action on its own or a {"tap": ..., "hold": ...} object.
type tapDance struct {
	tap  string
	hold string
}

func parseVilTapDances(raw []interface{}) []tapDance {
	out := make([]tapDance, len(raw))
	for i, td := range raw {
		cell := func(s string) string { return strings.TrimSpace(s) }
		switch x := td.(type) {
		case string:
			out[i].tap = cell(x)
		case map[string]interface{}:
			if tap, ok := x["tap"].(string); ok {
				out[i].tap = cell(tap)
			}
			if hold, ok := x["hold"].(string); ok {
				out[i].hold = cell(hold)
			}
		case []interface{}:
			// Vial writes a tap-dance as a five-element array:
			//   [tap, hold, double-tap, double-tap-hold, tapping-term]
			// The hold is only meaningful when it is not KC_NO.
			if len(x) >= 1 {
				if s, ok := x[0].(string); ok {
					out[i].tap = cell(s)
				}
			}
			if len(x) >= 2 {
				if s, ok := x[1].(string); ok && s != "KC_NO" {
					out[i].hold = cell(s)
				}
			}
		}
	}
	return out
}

// parseVilCall splits "LT(1,KC_SPC)" into "LT" and ["1", "KC_SPC"].
func parseVilCall(raw string) (name string, args []string, ok bool) {
	i := strings.IndexByte(raw, '(')
	if i < 0 || !strings.HasSuffix(raw, ")") {
		return "", nil, false
	}

	name = strings.TrimSpace(raw[:i])
	inner := raw[i+1 : len(raw)-1]
	if inner == "" {
		return name, nil, true
	}
	for _, arg := range strings.Split(inner, ",") {
		args = append(args, strings.TrimSpace(arg))
	}
	return name, args, true
}

// withTapDanceHold fills in the layer action of a tap-dance whose hold action
// is a layer key, e.g. {"tap": "KC_A", "hold": "MO(2)"}. Without it the key is
// treated as an ordinary key and its layer would never activate.
func withTapDanceHold(td vilAction, hold string, tapDances []tapDance, depth int) vilAction {
	if hold == "" {
		return td
	}
	h := parseVilKey(hold, tapDances, depth+1)
	if h.layerMode == "" || h.layer <= 0 {
		return td
	}
	td.layer = h.layer
	td.layerMode = h.layerMode
	return td
}

func parseVilKey(raw string, tapDances []tapDance, depth int) vilAction {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-1" {
		return vilAction{raw: raw, none: true}
	}
	if strings.HasPrefix(raw, "\x00") {
		if n, err := strconv.Atoi(raw[1:]); err == nil {
			return parseVilNumeric(uint16(n))
		}
	}

	switch raw {
	case "KC_NO", "XXXXXXX":
		return vilAction{raw: raw, none: true}
	case "KC_TRNS", "KC_TRANSPARENT", "_______":
		return vilAction{raw: raw, transparent: true}
	}

	if name, args, ok := parseVilCall(raw); ok {
		switch name {
		case "MO", "TG", "TO", "DF", "OSL", "TT", "LM":
			if len(args) >= 1 {
				if l, err := strconv.Atoi(args[0]); err == nil {
					return vilAction{
						raw:       raw,
						label:     fmt.Sprintf("%s%d", name, l),
						layer:     l,
						layerMode: name,
					}
				}
			}
		case "LT":
			if len(args) == 2 {
				if l, err := strconv.Atoi(args[0]); err == nil {
					tap := parseVilKey(args[1], tapDances, depth+1)
					return vilAction{
						raw:       raw,
						label:     fmt.Sprintf("LT%d", l),
						output:    tap.output,
						shifted:   tap.shifted,
						layer:     l,
						layerMode: name,
					}
				}
			}
		case "LCTL_T", "RCTL_T", "LSFT_T", "RSFT_T", "LALT_T", "RALT_T", "LGUI_T", "RGUI_T":
			if len(args) == 1 {
				tap := parseVilKey(args[0], tapDances, depth+1)
				mods := modTapName(name)
				return vilAction{
					raw:     raw,
					label:   tap.label,
					display: hrmDisplay(mods, tap),
					output:  tap.output,
					shifted: tap.shifted,
					mods:    mods,
					isShift: mods == "Shift",
				}
			}
		case "MT":
			if len(args) == 2 {
				tap := parseVilKey(args[1], tapDances, depth+1)
				mods := modifierFromString(args[0])
				return vilAction{
					raw:     raw,
					label:   tap.label,
					display: hrmDisplay(mods, tap),
					output:  tap.output,
					shifted: tap.shifted,
					mods:    mods,
					isShift: mods == "Shift",
				}
			}
		case "S":
			if len(args) == 1 {
				inner := parseVilKey(args[0], tapDances, depth+1)
				if inner.shifted != 0 {
					return vilAction{raw: raw, label: vilCharLabel(inner.shifted), output: inner.shifted}
				}
				if inner.output != 0 {
					if r := shiftedRune(inner.output); r != 0 {
						return vilAction{raw: raw, label: vilCharLabel(r), output: r}
					}
				}
			}
		case "TD":
			if len(args) == 1 && depth < 8 {
				if n, err := strconv.Atoi(args[0]); err == nil && n >= 0 && n < len(tapDances) && tapDances[n].tap != "" {
					dance := tapDances[n]
					td := parseVilKey(dance.tap, tapDances, depth+1)
					if td.layerMode == "" {
						td = withTapDanceHold(td, dance.hold, tapDances, depth)
					}
					td.raw = raw
					if td.label == "" {
						td.label = fmt.Sprintf("TD%d", n)
					}
					return td
				}
			}
			return vilAction{raw: raw, label: "TD"}
		case "C", "A", "G":
			if len(args) == 1 {
				inner := parseVilKey(args[0], tapDances, depth+1)
				return vilAction{raw: raw, label: inner.label}
			}
		case "MACRO", "M":
			return vilAction{raw: raw, label: "MAC"}
		case "OSM":
			return vilAction{raw: raw, label: "OSM", isShift: len(args) > 0 && strings.Contains(args[0], "LSFT")}
		}

		return vilAction{raw: raw, label: vilAbbrev(name)}
	}

	if ch, ok := vilBasicChar(raw); ok {
		return vilAction{
			raw:     raw,
			label:   vilCharLabel(ch),
			output:  ch,
			shifted: shiftedRune(ch),
		}
	}

	if label, ok := vilSpecialLabel(raw); ok {
		return vilAction{
			raw:     raw,
			label:   label,
			isShift: isVilShiftKeycode(raw),
		}
	}

	return vilAction{raw: raw, label: vilAbbrev(raw)}
}

func hrmDisplay(mod string, tap vilAction) string {
	prefix := ""
	switch mod {
	case "Ctrl":
		prefix = "C"
	case "Alt":
		prefix = "A"
	case "Gui":
		prefix = "G"
	case "Shift":
		prefix = "S"
	}
	if prefix == "" || tap.label == "" {
		return tap.label
	}
	r := []rune(tap.label)
	return prefix + string(r[0])
}

func modTapName(name string) string {
	switch name {
	case "LCTL_T", "RCTL_T":
		return "Ctrl"
	case "LSFT_T", "RSFT_T":
		return "Shift"
	case "LALT_T", "RALT_T":
		return "Alt"
	case "LGUI_T", "RGUI_T":
		return "Gui"
	}
	return ""
}

func modifierFromString(s string) string {
	switch {
	case strings.Contains(s, "LSFT") || strings.Contains(s, "RSFT"):
		return "Shift"
	case strings.Contains(s, "LCTL") || strings.Contains(s, "RCTL"):
		return "Ctrl"
	case strings.Contains(s, "LALT") || strings.Contains(s, "RALT"):
		return "Alt"
	case strings.Contains(s, "LGUI") || strings.Contains(s, "RGUI"):
		return "Gui"
	}
	return ""
}

func modifierFromBits(bits int) string {
	switch {
	case bits&0x22 != 0:
		return "Shift"
	case bits&0x11 != 0:
		return "Ctrl"
	case bits&0x44 != 0:
		return "Alt"
	case bits&0x88 != 0:
		return "Gui"
	}
	return ""
}

func parseVilNumeric(code uint16) vilAction {
	raw := fmt.Sprintf("\x00%d", code)

	if code == 0 {
		return vilAction{raw: raw, none: true}
	}
	if code == 1 {
		return vilAction{raw: raw, transparent: true}
	}

	if code >= 0x5200 && code <= 0x52FF {
		layer := int(code & 0x1F)
		mode := ""
		switch code & 0xFFE0 {
		case 0x5200:
			mode = "TO"
		case 0x5220:
			mode = "MO"
		case 0x5240:
			mode = "DF"
		case 0x5260:
			mode = "TG"
		case 0x5280:
			mode = "OSL"
		case 0x52C0:
			mode = "TT"
		}
		if mode != "" {
			return vilAction{raw: raw, label: fmt.Sprintf("%s%d", mode, layer), layer: layer, layerMode: mode}
		}
	}

	if code&0xF000 == 0x4000 {
		layer := int(code>>8) & 0x0F
		tap := parseVilNumeric(code & 0xFF)
		return vilAction{
			raw:       raw,
			label:     fmt.Sprintf("LT%d", layer),
			output:    tap.output,
			shifted:   tap.shifted,
			layer:     layer,
			layerMode: "LT",
		}
	}

	if code&0xF000 == 0x2000 {
		mods := modifierFromBits(int(code>>8) & 0xFF)
		tap := parseVilNumeric(code & 0xFF)
		return vilAction{
			raw:     raw,
			label:   tap.label,
			display: hrmDisplay(mods, tap),
			output:  tap.output,
			shifted: tap.shifted,
			mods:    mods,
			isShift: mods == "Shift",
		}
	}

	if code&0xFF00 >= 0x0100 && code&0xFF00 <= 0x1F00 {
		mod := int(code>>8) & 0x1F
		base := parseVilNumeric(code & 0xFF)
		if mod&0x22 != 0 {
			if base.shifted != 0 {
				return vilAction{raw: raw, label: vilCharLabel(base.shifted), output: base.shifted}
			}
			if base.output != 0 {
				if r := shiftedRune(base.output); r != 0 {
					return vilAction{raw: raw, label: vilCharLabel(r), output: r}
				}
			}
		}
		return vilAction{raw: raw, label: base.label, isShift: mod&0x22 != 0}
	}

	label := func(name string) vilAction { return vilAction{raw: raw, label: name} }

	switch {
	case code >= 0x04 && code <= 0x1D:
		ch := rune('a' + code - 0x04)
		return vilAction{raw: raw, label: vilCharLabel(ch), output: ch, shifted: shiftedRune(ch)}
	case code >= 0x1E && code <= 0x27:
		ch := rune('1' + code - 0x1E)
		if code == 0x27 {
			ch = '0'
		}
		return vilAction{raw: raw, label: vilCharLabel(ch), output: ch, shifted: shiftedRune(ch)}
	}

	switch code {
	case 0x28:
		return label("ENT")
	case 0x29:
		return label("ESC")
	case 0x2A:
		return label("BSP")
	case 0x2B:
		return label("TAB")
	case 0x2C:
		return vilAction{raw: raw, label: "SPC", output: ' '}
	case 0x2D:
		return vilAction{raw: raw, label: "-", output: '-', shifted: '_'}
	case 0x2E:
		return vilAction{raw: raw, label: "=", output: '=', shifted: '+'}
	case 0x2F:
		return vilAction{raw: raw, label: "[", output: '[', shifted: '{'}
	case 0x30:
		return vilAction{raw: raw, label: "]", output: ']', shifted: '}'}
	case 0x31:
		return vilAction{raw: raw, label: "\\", output: '\\', shifted: '|'}
	case 0x33:
		return vilAction{raw: raw, label: ";", output: ';', shifted: ':'}
	case 0x34:
		return vilAction{raw: raw, label: "'", output: '\'', shifted: '"'}
	case 0x35:
		return vilAction{raw: raw, label: "`", output: '`', shifted: '~'}
	case 0x36:
		return vilAction{raw: raw, label: ",", output: ',', shifted: '<'}
	case 0x37:
		return vilAction{raw: raw, label: ".", output: '.', shifted: '>'}
	case 0x38:
		return vilAction{raw: raw, label: "/", output: '/', shifted: '?'}
	case 0x39:
		return label("CAP")
	case 0x49:
		return label("INS")
	case 0x4A:
		return label("HOM")
	case 0x4B:
		return label("PGU")
	case 0x4C:
		return label("DEL")
	case 0x4D:
		return label("END")
	case 0x4E:
		return label("PGD")
	case 0x4F:
		return label("→")
	case 0x50:
		return label("←")
	case 0x51:
		return label("↓")
	case 0x52:
		return label("↑")
	}

	if code >= 0x3A && code <= 0x45 {
		return label(fmt.Sprintf("F%d", code-0x3A+1))
	}
	if code >= 0x68 && code <= 0x73 {
		return label(fmt.Sprintf("F%d", code-0x68+13))
	}

	switch code {
	case 0xE0, 0xE4:
		return label("CTL")
	case 0xE1, 0xE5:
		return vilAction{raw: raw, label: "SFT", isShift: true}
	case 0xE2, 0xE6:
		return label("ALT")
	case 0xE3, 0xE7:
		return label("GUI")
	}

	return label(fmt.Sprintf("%X", code))
}

func vilBasicChar(raw string) (rune, bool) {
	if !strings.HasPrefix(raw, "KC_") {
		return 0, false
	}

	name := raw[3:]
	if len(name) == 1 {
		c := name[0]
		switch {
		case c >= 'A' && c <= 'Z':
			return rune(c - 'A' + 'a'), true
		case c >= '0' && c <= '9':
			return rune(c), true
		}
	}

	if r, ok := vilCharMap[name]; ok {
		return r, true
	}
	return 0, false
}

func vilSpecialLabel(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "KC_") {
		switch {
		case strings.HasPrefix(raw, "QK_"):
			return vilAbbrev(raw[3:]), true
		case strings.HasPrefix(raw, "RGB_"):
			return "RGB", true
		case strings.HasPrefix(raw, "RESET"):
			return "RST", true
		}
		return "", false
	}

	name := raw[3:]
	if label, ok := vilSpecialMap[name]; ok {
		return label, true
	}
	if strings.HasPrefix(name, "F") {
		if n, err := strconv.Atoi(name[1:]); err == nil && n >= 1 && n <= 24 {
			return fmt.Sprintf("F%d", n), true
		}
	}

	return "", false
}

func isVilShiftKeycode(raw string) bool {
	name := strings.TrimPrefix(raw, "KC_")
	switch name {
	case "LSFT", "LSHIFT", "RSFT", "RSHIFT":
		return true
	}
	return strings.HasPrefix(name, "LSFT_T(") || strings.HasPrefix(name, "RSFT_T(")
}

func vilCharLabel(r rune) string {
	if r == ' ' {
		return "SPC"
	}
	return string(r)
}

func shiftedRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	if s, ok := vilShiftMap[r]; ok {
		return s
	}
	return 0
}

func vilAbbrev(raw string) string {
	raw = strings.TrimPrefix(raw, "KC_")
	raw = strings.TrimPrefix(raw, "QK_")

	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() == 3 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return "?"
	}
	return strings.ToUpper(b.String())
}

// setSplitMode overrides the automatic split detection. This is useful for
// Vial files whose matrix shape is ambiguous.
func (kb *vilKeyboard) setSplitMode(mode string) error {
	rows := len(kb.layers[0].matrix)

	switch mode {
	case "", "auto":
		return nil
	case "none":
		kb.splitCols = false
		kb.halfCols = 0
		kb.halfRows = 0
	case "rows":
		if rows < 2 || rows%2 != 0 {
			return fmt.Errorf("cannot split %d rows into two halves", rows)
		}
		kb.splitCols = false
		kb.halfCols = 0
		kb.halfRows = rows / 2
	case "cols":
		if kb.maxCols < 2 {
			return fmt.Errorf("cannot split %d columns into two halves", kb.maxCols)
		}
		kb.splitCols = true
		kb.halfCols = kb.maxCols / 2
		kb.halfRows = 0
	default:
		return fmt.Errorf("unknown split mode %q (want auto, rows, cols or none)", mode)
	}

	return nil
}

func (kb *vilKeyboard) actionAt(layer, row, col int) vilAction {
	if layer < 0 || layer >= len(kb.layers) {
		return vilAction{none: true}
	}

	for l := layer; l >= 0; l-- {
		a, ok := kb.matrixAction(l, row, col)
		if !ok {
			return vilAction{none: true}
		}
		if a.transparent {
			continue
		}
		return a
	}

	return vilAction{transparent: true}
}

func (kb *vilKeyboard) layerName(layer int) string {
	if layer >= 0 && layer < len(kb.layerNames) && kb.layerNames[layer] != "" {
		return kb.layerNames[layer]
	}
	if layer == 0 {
		return "base"
	}
	return fmt.Sprintf("layer %d", layer)
}

func isFnLayerLabel(label string) bool {
	if len(label) > 1 && label[0] == 'F' {
		if n, err := strconv.Atoi(label[1:]); err == nil && n >= 1 && n <= 24 {
			return true
		}
	}
	switch label {
	case "RGB", "MUT", "V+", "V-", "PLY", "NXT", "PRV", "STP", "BOO", "RST":
		return true
	}
	return false
}

func rawHasFnKey(raw string) bool {
	for {
		i := strings.Index(raw, "KC_F")
		if i < 0 {
			return false
		}
		rest := raw[i+4:]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j > 0 {
			if n, err := strconv.Atoi(rest[:j]); err == nil && n >= 1 && n <= 24 {
				return true
			}
		}
		raw = rest
	}
}

func isMouseLayerLabel(label string) bool {
	if strings.HasPrefix(label, "BT") || strings.HasPrefix(label, "MS") {
		return true
	}
	switch label {
	case "M←", "M→", "M↑", "M↓", "W←", "W→", "W↑", "W↓":
		return true
	}
	return false
}

func isNavLayerLabel(label string) bool {
	switch label {
	case "←", "→", "↑", "↓", "HOM", "END", "PGU", "PGD", "INS", "DEL":
		return true
	}
	return false
}

// inferLayerName guesses what a layer is for from the actions it defines. It
// lets guidance and Chatsubo talk about "num/sym" or "mouse" even when the
// .vil has no -layernames.
func (kb *vilKeyboard) inferLayerName(layer int) string {
	if layer < 0 || layer >= len(kb.layers) {
		return ""
	}
	if layer == 0 {
		return "base"
	}

	var digits, symbols, fn, mouse, nav, shift, ctrl, layerKeys int
	for r := range kb.layers[layer].actions {
		for c := range kb.layers[layer].actions[r] {
			a := kb.layers[layer].actions[r][c]
			if a.transparent || a.none {
				continue
			}
			if a.mods == "Shift" || a.isShift ||
				strings.HasPrefix(a.raw, "LSFT(") || strings.HasPrefix(a.raw, "RSFT(") {
				shift++
			}
			if a.mods == "Ctrl" || a.label == "CTL" ||
				strings.HasPrefix(a.raw, "LCTL(") || strings.HasPrefix(a.raw, "RCTL(") {
				ctrl++
			}
			if a.layerMode != "" {
				layerKeys++
			}
			switch {
			case a.output >= '0' && a.output <= '9':
				digits++
			case a.output != 0 && a.output != ' ' && unicode.IsPrint(a.output) && !unicode.IsLetter(a.output):
				symbols++
			}
			label := a.label
			switch {
			case isFnLayerLabel(label) || rawHasFnKey(a.raw):
				fn++
			case isMouseLayerLabel(label):
				mouse++
			case isNavLayerLabel(label):
				nav++
			}
		}
	}

	switch {
	case mouse >= 3:
		return "mouse"
	case fn > 0 && shift > 0:
		return "fn/shift"
	case fn > 0 && ctrl > 0:
		return "fn/ctrl"
	case fn >= 3:
		return "fn/media"
	case nav >= 3:
		return "nav"
	case digits > 0 && symbols > 0:
		return "num/sym"
	case digits > 0:
		return "num"
	case symbols > 0:
		return "sym"
	case shift >= 3:
		return "shift"
	case ctrl >= 3:
		return "ctrl"
	case layerKeys >= 3:
		return "layers"
	}
	return ""
}

func (kb *vilKeyboard) hintLayerName(layer int) string {
	if layer >= 0 && layer < len(kb.layerNames) && kb.layerNames[layer] != "" {
		return kb.layerNames[layer]
	}
	if inferred := kb.inferLayerName(layer); inferred != "" {
		return inferred
	}
	return kb.layerName(layer)
}

func (kb *vilKeyboard) displayRows() int {
	rows := len(kb.layers[0].matrix)
	if kb.splitCols {
		return rows
	}
	if kb.hasBoard {
		// Four alpha rows and the thumb row.
		return sofleRows + 1
	}
	if kb.halfRows > 0 {
		return kb.halfRows
	}
	return rows
}

// boardRow is the drawn board row of display row r, or -1 when r is the thumb
// row or there is no placed board.
func (kb *vilKeyboard) boardRow(r int) int {
	if !kb.hasBoard || r < 0 || r >= sofleRows {
		return -1
	}
	return r
}

func (kb *vilKeyboard) width(keyWidth, keyGap, splitGap int) int {
	if kb.hasBoard {
		return kb.halfWidth(false, keyWidth, keyGap) + splitGap + kb.halfWidth(true, keyWidth, keyGap)
	}
	if kb.splitCols {
		leftWidth := kb.halfWidthWithGap(kb.halfCols, keyWidth, keyGap)
		rightWidth := kb.halfWidthWithGap(kb.maxCols-kb.halfCols, keyWidth, keyGap)
		return leftWidth + splitGap + rightWidth
	}
	if kb.halfRows > 0 {
		return kb.halfWidth(false, keyWidth, keyGap) + splitGap + kb.halfWidth(true, keyWidth, keyGap)
	}
	if w := kb.halfWidth(false, keyWidth, keyGap); w > 0 {
		return w
	}
	if kb.maxCols == 0 {
		return 0
	}
	// A half too narrow for the home block falls back to plain columns.
	return kb.maxCols*keyWidth + (kb.maxCols-1)*keyGap
}

// vilKeyGap is the gap between Vial caps. The board is otherwise packed, so
// this is the minimum that keeps neighbouring caps apart.
const vilKeyGap = 1

// vilMaxKeyWidth bounds how wide a cap grows: past five cells a key reads as
// a square block rather than a keycap on a board.
const vilMaxKeyWidth = 5

func (kb *vilKeyboard) widestKey(screenWidth, keyGap, splitGap int) int {
	// A keyboard reads as a keyboard, not as a row of square blocks: caps
	// stay inside this band even when the window could fit wider ones. A
	// closed two-row cap has three cells of label room, so a cap of three
	// cells is tried before one of two (two is only for windows that cannot
	// hold three).
	for _, kw := range capLadder() {
		if w := kb.width(kw, keyGap, splitGap); w > 0 && w+2 <= screenWidth {
			return kw
		}
	}
	return 0
}

// capLadder is the cap widths tried, widest first: 5, 4 and 3, then the
// narrower 2 and 1 that only an otherwise-too-narrow window needs.
func capLadder() []int {
	return []int{vilMaxKeyWidth, 4, 3, 2, 1}
}

// peripheralRowGap is the blank line between the top function row and the
// first alpha row, so the unused F-row reads as separate from the typing block.
const peripheralRowGap = 1

// vilRowY returns the top of display row r, pushing the peripheral function
// row apart from the alpha rows and the thumb row below them.
func (kb *vilKeyboard) vilRowY(top, r, keyH, rowGap int) int {
	y := top + r*(keyH+rowGap)
	if !kb.hasBoard {
		// The function row (row 0 of each half) is separated from the alpha
		// rows.
		if kb.peripheralRows[0] && r >= 1 {
			y += peripheralRowGap
		}
		if kb.peripheralRows[kb.halfRows] && r >= kb.halfRows+1 {
			y += peripheralRowGap
		}
	}
	if kb.thumbRowAt(r) {
		y += thumbRowGap
	}
	return y
}

// vilGridHeight is the total height of the drawn matrix, counting the
// peripheral-row and thumb-row gaps.
func (kb *vilKeyboard) vilGridHeight(rows, keyH, rowGap int) int {
	if rows <= 0 {
		return 0
	}
	h := rows*keyH + (rows-1)*rowGap
	if kb.thumbRowAt(rows - 1) {
		h += thumbRowGap
	}
	if kb.hasBoard {
		// The placed board has no separate function row: the macro row is
		// part of the grid, as on the board itself.
		return h
	}
	// The function row of each half adds a blank line above the alpha rows.
	n := 0
	if kb.peripheralRows[0] && rows > 1 {
		n++
	}
	if kb.peripheralRows[kb.halfRows] && rows > kb.halfRows+1 {
		n++
	}
	h += n * peripheralRowGap
	return h
}

// bareRows is the thinnest the matrix draws: one-row caps, no row gaps, with
// the peripheral and thumb gaps kept.
func (kb *vilKeyboard) bareRows() int {
	rows := kb.displayRows()
	if rows == 0 {
		return builtinBareRows()
	}
	return kb.vilGridHeight(rows, 1, 0)
}

func (kb *vilKeyboard) metrics(screenWidth, availHeight, splitGap int) (kbMetrics, bool) {
	rows := kb.displayRows()
	if rows == 0 {
		return kbMetrics{}, false
	}

	// Keys are packed edge to edge with no gap between caps; the split gap is
	// the only separation.
	keyGap := 0
	keyWidth := kb.widestKey(screenWidth, keyGap, splitGap)
	if keyWidth == 0 {
		return kbMetrics{}, false
	}

	keyH, rowGap := chooseKeyMetrics(rows, availHeight, func(kh, rg int) int {
		return kb.vilGridHeight(rows, kh, rg)
	})

	return kbMetrics{
		keyWidth: keyWidth,
		keyGap:   keyGap,
		splitGap: splitGap,
		keyH:     keyH,
		rowGap:   rowGap,
		height:   kb.vilGridHeight(rows, keyH, rowGap) + boardPad,
	}, true
}

func (kb *vilKeyboard) findLayerActivator(target int) *vilActivator {
	bestScore := 1 << 30
	var best *vilActivator

	for row := range kb.layers[0].matrix {
		for col := range kb.layers[0].matrix[row] {
			a := kb.actionAt(0, row, col)
			if a.layer != target || a.layerMode == "" {
				continue
			}

			score := 0
			switch a.layerMode {
			case "MO", "LT", "LM":
				score = 0
			case "OSL", "TT":
				score = 1
			default:
				score = 2
			}
			if kb.halfRows > 0 && (row == kb.halfRows-1 || row == len(kb.layers[0].matrix)-1) {
				score -= 1
			}

			if best == nil || score < bestScore {
				bestScore = score
				best = &vilActivator{row: row, col: col, action: a}
			}
		}
	}

	return best
}

func (kb *vilKeyboard) findShiftOnSide(layer, excludeRow, excludeCol, side int, hrmOnly bool) (row, col int, ok bool) {
	for r := range kb.layers[layer].matrix {
		for c := range kb.layers[layer].matrix[r] {
			if r == excludeRow && c == excludeCol {
				continue
			}
			if side != 0 && kb.sideFor(r, c) != side {
				continue
			}
			a := kb.actionAt(layer, r, c)
			if hrmOnly {
				if a.mods == "Shift" {
					return r, c, true
				}
			} else if a.isShift {
				return r, c, true
			}
		}
	}
	return -1, -1, false
}

func (kb *vilKeyboard) findShiftKey(layer, excludeRow, excludeCol int) (row, col int, ok bool) {
	// Use the opposite half from the target key, so shifted characters are
	// typed with the other hand. Prefer an HRM shift, then a dedicated shift.
	otherSide := 0
	if side := kb.sideFor(excludeRow, excludeCol); side != 0 {
		otherSide = -side
	}
	if otherSide != 0 {
		if r, c, ok := kb.findShiftOnSide(layer, excludeRow, excludeCol, otherSide, true); ok {
			return r, c, true
		}
		if r, c, ok := kb.findShiftOnSide(layer, excludeRow, excludeCol, otherSide, false); ok {
			return r, c, true
		}
	}
	if r, c, ok := kb.findShiftOnSide(layer, excludeRow, excludeCol, 0, true); ok {
		return r, c, true
	}
	return kb.findShiftOnSide(layer, excludeRow, excludeCol, 0, false)
}

func (kb *vilKeyboard) findRune(target rune) (vilHint, bool) {
	bestScore := 1 << 30
	var best vilHint
	found := false

	for layer := 0; layer < len(kb.layers); layer++ {
		for row := range kb.layers[layer].matrix {
			for col := range kb.layers[layer].matrix[row] {
				a := kb.actionAt(layer, row, col)

				shift := false
				switch {
				case a.output != 0 && a.output == target:
				case a.shifted != 0 && a.shifted == target:
					shift = true
				default:
					continue
				}

				score := 0
				if layer != 0 {
					score += 30
				}
				if shift {
					score += 100
				}

				var activator *vilActivator
				if layer != 0 {
					activator = kb.findLayerActivator(layer)
					if activator == nil {
						score += 10000
					}
				}

				shiftRow, shiftCol := -1, -1
				if shift {
					var ok bool
					shiftRow, shiftCol, ok = kb.findShiftKey(layer, row, col)
					if !ok {
						score += 10000
					}
				}

				if !found || score < bestScore {
					found = true
					bestScore = score
					best = vilHint{
						row:       row,
						col:       col,
						action:    a,
						layer:     layer,
						shift:     shift,
						shiftRow:  shiftRow,
						shiftCol:  shiftCol,
						activator: activator,
					}
				}
			}
		}
	}

	return best, found
}

func (kb *vilKeyboard) findOnLayer(target rune, layer int) (vilKeyRef, bool) {
	if layer < 0 || layer >= len(kb.layers) {
		return vilKeyRef{}, false
	}

	for row := range kb.layers[layer].matrix {
		for col := range kb.layers[layer].matrix[row] {
			a := kb.actionAt(layer, row, col)
			if a.output == target || a.shifted == target {
				return vilKeyRef{row: row, col: col, action: a}, true
			}
		}
	}

	return vilKeyRef{}, false
}

func (kb *vilKeyboard) hintText(h vilHint, found bool) string {
	return kb.keyHintText(h, found, -1)
}

// keyHintText spells out how to press the target. When active is the layer
// currently held or previewed, the layer activator is omitted because that
// layer is already active; only the remaining key sequence is shown.
func (kb *vilKeyboard) keyHintText(h vilHint, found bool, active int) string {
	if !found {
		return ""
	}

	key := h.action.label
	if h.action.output != 0 && h.action.layerMode != "" {
		key = vilCharLabel(h.action.output)
	}
	if key == "" {
		return ""
	}

	prefix := ""
	if h.layer != 0 && h.layer != active {
		name := kb.hintLayerName(h.layer)
		if h.activator != nil {
			verb := "hold"
			switch h.activator.action.layerMode {
			case "OSL", "TG", "TO", "DF", "TT":
				verb = "tap"
			}
			if verb == "hold" {
				prefix = fmt.Sprintf("%s %s (%s) + ", verb, h.activator.action.label, name)
			} else {
				prefix = fmt.Sprintf("%s %s (%s) then ", verb, h.activator.action.label, name)
			}
		} else {
			prefix = fmt.Sprintf("%s (no layer key) + ", name)
		}
	}

	if h.shift {
		shiftLabel := "SFT"
		if h.shiftRow >= 0 {
			if a := kb.actionAt(h.layer, h.shiftRow, h.shiftCol); a.label != "" {
				shiftLabel = a.label
			}
		}
		prefix += fmt.Sprintf("hold %s (shift) + ", shiftLabel)
	}

	if prefix == "" {
		return "press " + key
	}
	return prefix + "press " + key
}

// layerHintText prefixes the target guidance with the layer currently shown,
// so holding a layer key does not hide which key to press.
func (kb *vilKeyboard) layerHintText(prefix string, layer int, h vilHint, found bool) string {
	status := prefix + ": " + kb.hintLayerName(layer)
	guide := kb.keyHintText(h, found, layer)
	if guide == "" {
		return status
	}
	return status + " — " + guide
}

// hintForLayer returns guidance for target while the given layer is shown as
// held or previewed. It finds the target on that layer (resolving transparency
// down to the base layer), so the hint names the key to press on the held layer
// rather than the base layer's key; targets that are not on the layout are
// reported explicitly.
func (kb *vilKeyboard) hintForLayer(status string, layer int, target rune) string {
	h, found := kb.findOnActiveLayer(target, layer)
	statusText := status + ": " + kb.hintLayerName(layer)
	if !found {
		if missing := noKeyHint(target); missing != "" {
			return statusText + " — " + missing
		}
		return statusText
	}
	key := h.action.label
	if h.action.output != 0 && h.action.layerMode != "" {
		key = vilCharLabel(h.action.output)
	}
	if key == "" {
		return statusText + " — press " + h.action.display
	}
	if h.shift {
		shiftLabel := "SFT"
		if h.shiftRow >= 0 {
			if a := kb.actionAt(layer, h.shiftRow, h.shiftCol); a.label != "" {
				shiftLabel = a.label
			}
		}
		return fmt.Sprintf("%s — hold %s (shift) + press %s", statusText, shiftLabel, key)
	}
	return fmt.Sprintf("%s — press %s", statusText, key)
}

// findOnActiveLayer finds target on the given layer, resolving transparent
// cells down to the base layer, and reports the (row, col) and resolved action
// plus whether a shift is needed.
func (kb *vilKeyboard) findOnActiveLayer(target rune, layer int) (vilHint, bool) {
	if layer < 0 {
		layer = 0
	}
	if layer >= len(kb.layers) {
		layer = len(kb.layers) - 1
	}
	for row := range kb.layers[layer].matrix {
		for col := range kb.layers[layer].matrix[row] {
			a := kb.actionAt(layer, row, col)
			shift := false
			switch {
			case a.output != 0 && a.output == target:
			case a.shifted != 0 && a.shifted == target:
				shift = true
			default:
				continue
			}
			return vilHint{row: row, col: col, action: a, layer: layer, shift: shift}, true
		}
	}
	return vilHint{}, false
}

// noKeyHint explains why a target has no guidance in this layout.
func noKeyHint(target rune) string {
	if target == 0 || target == '\n' {
		return ""
	}
	return fmt.Sprintf("no key for %q in this layout", string(target))
}

func drawHintLine(scr tcell.Screen, y, x0, width int, text string, def tcell.Style) {
	if width <= 0 {
		return
	}
	// Clear the whole line first; otherwise a shorter guidance string leaves
	// the tail of the previous layer's hint behind.
	for i := 0; i < width; i++ {
		scr.SetContent(x0+i, y, ' ', nil, def)
	}
	if text == "" {
		return
	}
	r := []rune(text)
	if max := width - 2; max > 0 && len(r) > max {
		r = r[:max]
	}
	drawString(scr, x0+(width-len(r))/2, y, string(r), -1, def)
}

func (t *typer) drawVilKeyboard(target rune) {
	kb := t.vil
	if kb == nil {
		return
	}

	sw, _ := t.Scr.Size()
	m, ok := t.keyboardMetrics()
	if !ok {
		return
	}

	// Let the draw light a physical key that is held, even with no action.
	kb.pressedAt = nil
	if t.layerTracker != nil {
		kb.pressedAt = t.layerTracker.Pressed
	}

	hint, found := kb.findRune(target)

	physicalLayer := -1
	if t.layerTracker != nil {
		if l := t.layerTracker.Layer(); l >= 0 && l < len(kb.layers) {
			physicalLayer = l
		}
	}
	physical := physicalLayer >= 0
	// F1/F2/F3 previews win over a physically held layer, so browsing another
	// layer with the keyboard never silently does nothing. The hint still says
	// which layer is held.
	preview := t.previewLayer >= 0 && t.previewLayer < len(kb.layers)
	layer := 0
	switch {
	case preview:
		layer = t.previewLayer
	case physical:
		layer = physicalLayer
	case found:
		layer = hint.layer
	}

	focusLeft, focusRight := true, true
	if found && !preview && !physical {
		switch {
		case kb.splitCols:
			focusLeft = hint.col < kb.halfCols
			focusRight = !focusLeft
		case kb.halfRows > 0:
			focusLeft = hint.row < kb.halfRows
			focusRight = !focusLeft
		}
	}

	showTarget := found && (!preview || hint.layer == layer)
	if physical && !preview {
		showTarget = found && hint.layer == layer
	}

	// Light the column of the finger which should reach this key. Thumbs do
	// not own a finger column, so thumb targets leave the matrix undimmed.
	activeCol := -1
	if found && !kb.isThumbRow(hint.row) {
		if _, c, ok := kb.targetHome(hint.row, hint.col); ok {
			activeCol = c
		}
	}

	width := kb.width(m.keyWidth, m.keyGap, m.splitGap)
	x0 := (sw - width) / 2
	rows := kb.displayRows()

	wrongRow, wrongCol := -1, -1
	if t.wrongKeyValid() {
		if ref, ok := kb.findOnLayer(t.wrongKey, layer); ok {
			wrongRow, wrongCol = ref.row, ref.col
		}
	}

	if t.compactBoard(m) {
		// A one-row cap is drawn as a single-line key, so a window that only
		// fits flat caps still shows the board through the compact path.
		t.drawCompactBoard(kb, layer, m, hint, found, showTarget, wrongRow, wrongCol, focusLeft, focusRight, preview || physical)
		return
	}

	// Clear the whole matrix before drawing. Layers can differ in which cells
	// are absent, and a cell that is empty in the new layer must not keep the
	// old layer's label.
	for y := m.top; y < m.top+kb.vilGridHeight(rows, m.keyH, m.rowGap); y++ {
		for x := x0; x < x0+width; x++ {
			t.Scr.SetContent(x, y, ' ', nil, t.defaultStyle)
		}
	}

	rawLayer := physical || preview
	for r := 0; r < rows; r++ {
		if kb.thumbRowAt(r) {
			t.drawVilThumbs(kb, layer, r, x0, m, hint, showTarget, wrongRow, wrongCol, focusLeft, focusRight, rawLayer)
			continue
		}
		y := kb.vilRowY(m.top, r, m.keyH, m.rowGap)

		if kb.hasBoard {
			leftWidth := kb.halfWidth(false, m.keyWidth, m.keyGap)
			kb.drawBoardRow(t.Scr, layer, r, false, x0,
				y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusLeft, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
			// A row-split matrix keeps the right half's rows halfRows below
			// the left's, so the right half reads its own keys, not a copy.
			rightRow := r + kb.halfRows
			if rightRow >= len(kb.layers[0].matrix) {
				rightRow = r
			}
			kb.drawBoardRow(t.Scr, layer, rightRow, true, x0+leftWidth+m.splitGap,
				y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusRight, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
			continue
		}

		switch {
		case kb.splitCols:
			leftWidth := kb.halfWidthWithGap(kb.halfCols, m.keyWidth, m.keyGap)
			kb.drawCols(t.Scr, layer, r, 0, kb.halfCols, x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusLeft, false, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
			kb.drawCols(t.Scr, layer, r, kb.halfCols, kb.maxCols,
				x0+leftWidth+m.splitGap, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusRight, true, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
		case kb.halfRows > 0:
			leftWidth := kb.halfWidth(false, m.keyWidth, m.keyGap)
			kb.drawHalf(t.Scr, layer, r, x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusLeft, false, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
			if r+kb.halfRows < len(kb.layers[0].matrix) {
				kb.drawHalf(t.Scr, layer, r+kb.halfRows,
					x0+leftWidth+m.splitGap,
					y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusRight, true, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
			}
		default:
			kb.drawHalf(t.Scr, layer, r, x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, true, false, activeCol, t.blinkOn, rawLayer, t.defaultStyle)
		}
	}

	// The thumb row is drawn a line below the last alpha row, so the hint
	// follows it.
	hintY := m.top + kb.vilGridHeight(rows, m.keyH, m.rowGap)
	hintText := kb.hintText(hint, found)
	switch {
	case preview:
		hintText = kb.hintForLayer("preview", layer, target)
	case physical:
		hintText = kb.hintForLayer("held", layer, target)
	default:
		if !found {
			hintText = noKeyHint(target)
		}
	}
	t.hintText = t.withFinger(hintText, target)
	if t.chatsubo == nil && !t.sprint {
		drawHintLine(t.Scr, hintY, x0, width, hintText, t.defaultStyle)
	}
}

// compactBoard reports whether the window is too narrow for the board's
// closed caps: the two halves are then drawn as single-line keys, which take
// three cells per column instead of four.
func (t *typer) compactBoard(m kbMetrics) bool {
	if t.vil == nil || !t.vil.hasBoard {
		return false
	}
	return m.keyH < 2
}

// compactCols is the drawn columns per half of the compact board: every
// column of the half (seven on the Miryoku), or the matrix's own width.
func (kb *vilKeyboard) compactCols() int {
	cols := kb.maxCols
	if kb.hasBoard && kb.boardHalfCols > 0 {
		cols = kb.boardHalfCols
	}
	if cols < 1 {
		cols = 1
	}
	return cols
}

// compactLabel is the label of a compact cap: the tap character, with the home
// row modifier's tag first when the cap is wide enough for both. It is never
// empty for a wired key.
func compactLabel(a vilAction, width int) string {
	label := a.label
	if a.display != "" && a.mods == "" {
		label = a.display
	}
	if a.transparent && label == "" {
		label = "▽"
	}
	if label == "" {
		label = "?"
	}
	// A home row modifier's tag tells the resting finger's chord, so it wins
	// over a letter the user already knows by position.
	if tag := hrmTag(a.mods); tag != "" && width >= 2 {
		label = tag
	}
	if len([]rune(label)) > width {
		label = buttonLabel(label, width)
	}
	return label
}

// drawCompactBoard draws the whole board as single-line keys on the board's
// own rows: both hands side by side with the joiner between them.
func (t *typer) drawCompactBoard(kb *vilKeyboard, layer int, m kbMetrics, hint vilHint, found, showTarget bool, wrongRow, wrongCol int, focusLeft, focusRight, raw bool) {
	sw, _ := t.Scr.Size()
	cols := kb.compactCols()
	// Each key is three cells; a blank cell keeps the columns apart. The
	// halves share the cap size the row height allows.
	keyW := 3
	keyGap := 1
	halfW := cols*keyW + (cols-1)*keyGap
	splitGap := m.splitGap
	if splitGap < 1 {
		splitGap = 1
	}
	width := 2*halfW + splitGap
	x0 := (sw - width) / 2
	if x0 < 0 {
		x0 = 0
	}
	rows := kb.displayRows()

	// Clear the board's own block first: a shorter row must not keep the
	// previous frame's cells.
	for r := 0; r < rows; r++ {
		y := kb.vilRowY(m.top, r, 1, 0)
		if y < 0 || y >= m.top+m.height {
			continue
		}
		for x := x0; x < x0+width && x < sw; x++ {
			t.Scr.SetContent(x, y, ' ', nil, t.defaultStyle)
		}
	}

	activeCol := -1
	if found && !kb.isThumbRow(hint.row) {
		if _, c, ok := kb.targetHome(hint.row, hint.col); ok {
			activeCol = c
		}
	}

	put := func(r, col int, right, focus bool) {
		if r < 0 || r > rows || r >= len(kb.layers[0].matrix) {
			return
		}
		row := kb.layers[0].matrix[r]
		var a vilAction
		if raw {
			a = kb.layers[0].actions[r][col]
		} else {
			a = kb.actionAt(layer, r, col)
		}
		present := col < len(row) && row[col] != "" && !(a.none && !strings.HasPrefix(a.raw, "TD("))
		x := x0 + col*(keyW+keyGap)
		if right {
			x = x0 + halfW + splitGap + col*(keyW+keyGap)
		}
		y := kb.vilRowY(m.top, r, 1, 0)
		if y < 0 || y >= m.top+m.height {
			// Past the rows the keys part has: the key is not on screen.
			return
		}
		if !present {
			if kb.thumbRowAt(r) {
				// Thumb rows draw only real cluster keys; empty slots are not
				// physical keys.
				return
			}
			// Unused physical position: an empty cell, lit while held.
			style := t.defaultStyle.Dim(true)
			if kb.pressedAt != nil && kb.pressedAt(r, col) {
				style = kbModStyle
			}
			drawString(t.Scr, x, y, strings.Repeat(" ", keyW), -1, style)
			return
		}
		style := t.defaultStyle
		switch {
		case kb.pressedAt != nil && kb.pressedAt(r, col):
			style = kbModStyle
		case r == wrongRow && col == wrongCol:
			style = kbErrorStyle
		case found && showTarget && r == hint.row && col == hint.col:
			style = targetStyle(t.blinkOn, t.defaultStyle)
		case found && hint.activator != nil && r == hint.activator.row && col == hint.activator.col:
			style = kbModStyle
		case found && hint.shift && r == hint.shiftRow && col == hint.shiftCol:
			style = kbModStyle
		case kb.isHomeKey(r, col):
			if col == activeCol {
				style = t.defaultStyle.Foreground(tcell.ColorGreen).Bold(true)
			} else {
				style = t.defaultStyle.Foreground(tcell.ColorGreen)
			}
		}
		if !focus {
			style = style.Dim(true)
		}
		drawString(t.Scr, x, y, compactLabel(a, keyW), -1, style)
	}

	for r := 0; r < rows; r++ {
		if kb.thumbRowAt(r) {
			// The thumbs are one keymap row per half; draw each half's own.
			for c := 0; c < cols; c++ {
				put(r, c, false, focusLeft)
				put(r+kb.halfRows, c, true, focusRight)
			}
			continue
		}
		for c := 0; c < cols; c++ {
			put(r, c, false, focusLeft)
			put(r+kb.halfRows, c, true, focusRight)
		}
	}
}

func (kb *vilKeyboard) drawHalf(scr tcell.Screen, layer, matrixRow, x, y, keyWidth, keyGap, keyH int, hint vilHint, found bool, wrongRow, wrongCol int, focused, right bool, activeCol int, blink bool, raw bool, def tcell.Style) {
	if matrixRow >= len(kb.layers[0].matrix) {
		return
	}
	kb.drawCols(scr, layer, matrixRow, 0, kb.maxCols,
		x, y, keyWidth, keyGap, keyH, hint, found, wrongRow, wrongCol, focused, right, activeCol, blink, raw, def)
}

func (kb *vilKeyboard) drawCols(scr tcell.Screen, layer, matrixRow, startCol, endCol, x, y, keyWidth, keyGap, keyH int, hint vilHint, found bool, wrongRow, wrongCol int, focused, right bool, activeCol int, blink bool, raw bool, def tcell.Style) {
	if layer < 0 || layer >= len(kb.layers) {
		return
	}
	if matrixRow < 0 || matrixRow >= len(kb.layers[layer].matrix) {
		return
	}
	// Read the keymap's own labels; the board is drawn by placing keys at
	// their offsets, never by rewriting a layer.
	row := kb.layers[layer].matrix[matrixRow]
	if startCol < 0 {
		startCol = 0
	}
	if endCol > len(row) {
		endCol = len(row)
	}

	thumbKeys, isThumbRow := kb.thumbX[matrixRow]

	// slotCol maps a drawn slot to the matrix column at that slot. For a
	// thumb row the cluster is packed by slot, so a key sits where its packed
	// x lands, not at its matrix column.
	slotCol := func(slot int) int {
		if !isThumbRow {
			return slot
		}
		maxX := 0
		for _, k := range thumbKeys {
			if k.x > maxX {
				maxX = k.x
			}
		}
		for _, k := range thumbKeys {
			x := k.x
			if right {
				x = maxX - k.x
			}
			if x == slot {
				return k.col
			}
		}
		return -1
	}

	for slot := startCol; slot < endCol; slot++ {
		col := slot
		if isThumbRow {
			col = slotCol(slot)
		}
		cx := x + kb.colXSide(matrixRow, slot, right, keyWidth, keyGap)
		if isThumbRow {
			// On a thumb row the loop variable is already the drawn slot.
			cx = x + slot*(keyWidth+keyGap)
		}

		present := col >= 0 && col < len(row) && row[col] != ""
		var a vilAction
		if present {
			if raw {
				a = kb.layers[layer].actions[matrixRow][col]
			} else {
				a = kb.actionAt(layer, matrixRow, col)
			}
			if a.none && !strings.HasPrefix(a.raw, "TD(") {
				present = false
			}
		}
		if !present {
			if isThumbRow {
				// Thumb rows draw only the real cluster keys; the filler
				// slots are not physical keys, so they are left empty.
				continue
			}
			// Every other grid position is a real physical key even with no
			// action: draw an empty cap and light it while it is held.
			style := def
			border := def
			if kb.pressedAt != nil && kb.pressedAt(matrixRow, col) {
				style = kbModStyle
				border = kbModStyle
			} else if !focused {
				style = style.Dim(true)
				border = border.Dim(true)
			}
			drawKBButton(scr, cx, y, keyWidth, keyH, "", "", style, border)
			continue
		}

		label := a.label
		top := ""
		if a.mods != "" {
			top = hrmTag(a.mods)
		}
		if a.display != "" && top == "" {
			label = a.display
		}
		if a.transparent && label == "" {
			label = "▽"
		}

		style := def
		border := style
		highlighted := false
		switch {
		case kb.pressedAt != nil && kb.pressedAt(matrixRow, col):
			// A held physical key lights, whatever its action.
			style = kbModStyle
			border = kbModStyle
			highlighted = true
		case matrixRow == wrongRow && col == wrongCol:
			style = kbErrorStyle
			border = kbErrorStyle
			highlighted = true
		case found && matrixRow == hint.row && col == hint.col:
			style = targetStyle(blink, def)
			border = style
			highlighted = true
		case found && hint.activator != nil && matrixRow == hint.activator.row && col == hint.activator.col:
			style = kbModStyle
			border = kbModStyle
			highlighted = true
		case found && hint.shift && matrixRow == hint.shiftRow && col == hint.shiftCol:
			style = kbModStyle
			border = kbModStyle
			highlighted = true
		case kb.isHomeKey(matrixRow, col):
			// The resting keys are tinted only enough to mark them: a dim
			// green on top of the dimmed inactive half would erase them.
			if col == activeCol {
				style = def.Foreground(tcell.ColorGreen).Bold(true)
			} else {
				style = def.Foreground(tcell.ColorGreen)
			}
			border = def
		}
		// Dim only the half which is not in play. Dimming every key outside
		// the active finger column makes the rest of the board unreadable.
		if !highlighted && !focused {
			style = style.Dim(true)
			border = border.Dim(true)
		}

		drawKBButton(scr, cx, y, keyWidth, keyH, label, top, style, border)
	}
}

// drawVilThumbs draws one thumb row. The keys are laid out as a cluster per
// half, right-aligned to the half's inner edge with the same gaps as the alpha
// rows; the blank line above the row is already provided by rowY. For a
// row-split matrix the two halves have their own thumb rows (row and
// row+halfRows), so each half is drawn from its own row.
func (t *typer) drawVilThumbs(kb *vilKeyboard, layer, row, x0 int, m kbMetrics, hint vilHint, showTarget bool, wrongRow, wrongCol int, focusLeft, focusRight, raw bool) {
	y := kb.vilRowY(m.top, row, m.keyH, m.rowGap)
	leftWidth := kb.halfWidth(false, m.keyWidth, m.keyGap)
	if kb.splitCols {
		leftWidth = kb.halfWidthWithGap(kb.halfCols, m.keyWidth, m.keyGap)
	}
	halfCols := kb.maxCols
	if kb.splitCols {
		halfCols = kb.halfCols
	}
	if kb.hasBoard {
		// Both halves draw their own thumb rows, clustered against the split.
		t.drawBoardThumbs(kb, layer, x0, y, m, hint, showTarget, wrongRow, wrongCol, focusLeft, focusRight, raw)
		return
	}
	kb.drawCols(t.Scr, layer, row, 0, minInt(halfCols, len(kb.layers[layer].matrix[row])),
		x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusLeft, false, -1, t.blinkOn, raw, t.defaultStyle)
	if kb.splitCols || kb.halfRows > 0 {
		rightRow := row
		rightCols := halfCols
		if kb.halfRows > 0 {
			rightRow = row + kb.halfRows
			rightCols = 0
		}
		if rightRow < len(kb.layers[layer].matrix) {
			kb.drawCols(t.Scr, layer, rightRow, rightCols, len(kb.layers[layer].matrix[rightRow]),
				x0+leftWidth+m.splitGap, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusRight, true, -1, t.blinkOn, raw, t.defaultStyle)
		}
	}
}

// matrixAction is the raw action of one matrix cell on one layer.
func (kb *vilKeyboard) matrixAction(layer, row, col int) (vilAction, bool) {
	if layer < 0 || layer >= len(kb.layers) {
		return vilAction{}, false
	}
	acts := kb.layers[layer].actions
	if row < 0 || row >= len(acts) || col < 0 || col >= len(acts[row]) {
		return vilAction{}, false
	}
	return acts[row][col], true
}

// drawBoardRow draws one drawn row r of one half of the placed board.

func (kb *vilKeyboard) drawBoardRow(scr tcell.Screen, layer, r int, right bool, x, y, keyWidth, keyGap, keyH int, hint vilHint, found bool, wrongRow, wrongCol int, focused bool, activeCol int, blink bool, raw bool, def tcell.Style) {
	kb.drawCols(scr, layer, r, 0, kb.boardHalfCols, x, y, keyWidth, keyGap, keyH,
		hint, found, wrongRow, wrongCol, focused, right, activeCol, blink, raw, def)
}

// drawBoardThumbs draws the thumb cluster of both halves on the board's
// single thumb row, each wrapped around its half's inner edge.
func (t *typer) drawBoardThumbs(kb *vilKeyboard, layer, x0, y int, m kbMetrics, hint vilHint, showTarget bool, wrongRow, wrongCol int, focusLeft, focusRight, raw bool) {
	row := sofleRows
	rightRow := row + kb.halfRows
	if rightRow >= len(kb.layers[0].matrix) {
		rightRow = row
	}
	leftWidth := kb.halfWidth(false, m.keyWidth, m.keyGap)
	kb.drawCols(t.Scr, layer, row, 0, kb.boardHalfCols, x0, y, m.keyWidth, m.keyGap, m.keyH,
		hint, showTarget, wrongRow, wrongCol, focusLeft, false, -1, t.blinkOn, raw, t.defaultStyle)
	kb.drawCols(t.Scr, layer, rightRow, 0, kb.boardHalfCols, x0+leftWidth+m.splitGap, y, m.keyWidth, m.keyGap, m.keyH,
		hint, showTarget, wrongRow, wrongCol, focusRight, true, -1, t.blinkOn, raw, t.defaultStyle)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
