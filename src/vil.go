package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell"
)

// Vial stores a keymap as a stack of 2D matrices. Each cell is a QMK keycode
// string such as "KC_Q", "MO(2)", "LT(1,KC_SPC)" or -1 for a missing key.
type vilFile struct {
	Layout   []interface{}   `json:"layout"`
	TapDance [][]interface{} `json:"tap_dance"`
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
	tapDances  []string
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
	shift    bool
}

const (
	vilSplitGap    = 4
	vilBottomPad   = 3
	vilMinTextRows = 4
)

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

	for li, lay := range kb.layers {
		fmt.Fprintf(&b, "\nlayer %d (%s):\n", li, kb.layerName(li))
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

func (kb *vilKeyboard) targetHome(row, col int) (int, int, bool) {
	if kb.isThumbRow(row) {
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

func (kb *vilKeyboard) isThumbRow(row int) bool {
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

func parseVilTapDances(raw [][]interface{}) []string {
	out := make([]string, len(raw))
	for i, td := range raw {
		if len(td) > 0 {
			if s, ok := td[0].(string); ok {
				out[i] = s
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

func parseVilKey(raw string, tapDances []string, depth int) vilAction {
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
				if n, err := strconv.Atoi(args[0]); err == nil && n >= 0 && n < len(tapDances) && tapDances[n] != "" {
					td := parseVilKey(tapDances[n], tapDances, depth+1)
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
		lay := kb.layers[l]
		if row < 0 || row >= len(lay.actions) || col < 0 || col >= len(lay.actions[row]) {
			return vilAction{none: true}
		}

		a := lay.actions[row][col]
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

func (kb *vilKeyboard) displayRows() int {
	rows := len(kb.layers[0].matrix)
	if kb.splitCols {
		return rows
	}
	if kb.halfRows > 0 {
		return kb.halfRows
	}
	return rows
}

func (kb *vilKeyboard) halfWidth(keyWidth, keyGap int) int {
	if kb.maxCols == 0 {
		return 0
	}
	return kb.maxCols*keyWidth + (kb.maxCols-1)*keyGap
}

func (kb *vilKeyboard) width(keyWidth, keyGap int) int {
	if kb.splitCols {
		leftWidth := kb.halfCols*keyWidth + (kb.halfCols-1)*keyGap
		rightCols := kb.maxCols - kb.halfCols
		rightWidth := rightCols*keyWidth + (rightCols-1)*keyGap
		return leftWidth + vilSplitGap + rightWidth
	}

	half := kb.halfWidth(keyWidth, keyGap)
	if kb.halfRows > 0 {
		return 2*half + vilSplitGap
	}
	return half
}

func (kb *vilKeyboard) metrics(screenWidth, screenHeight int) (kbMetrics, bool) {
	rows := kb.displayRows()
	if rows == 0 {
		return kbMetrics{}, false
	}

	keyWidth, keyGap := 0, 0
	for kw := 8; kw >= 1; kw-- {
		if w := kb.width(kw, 0); w > 0 && w+2 <= screenWidth {
			keyWidth = kw
			break
		}
	}
	if keyWidth == 0 {
		return kbMetrics{}, false
	}

	keyH, rowGap := chooseKeyMetrics(rows, screenHeight, vilBottomPad, vilMinTextRows)
	keyArea := rows*keyH + (rows-1)*rowGap
	height := keyArea + 1 + vilBottomPad
	top := screenHeight - height
	if top < 2 {
		return kbMetrics{}, false
	}

	return kbMetrics{
		keyWidth: keyWidth,
		keyGap:   keyGap,
		keyH:     keyH,
		rowGap:   rowGap,
		top:      top,
		height:   height,
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

func (kb *vilKeyboard) findShiftKey(layer, excludeRow, excludeCol int) (row, col int, ok bool) {
	// Prefer a home-row-mod shift key over a dedicated shift key.
	for r := range kb.layers[layer].matrix {
		for c := range kb.layers[layer].matrix[r] {
			if r == excludeRow && c == excludeCol {
				continue
			}
			if a := kb.actionAt(layer, r, c); a.mods == "Shift" {
				return r, c, true
			}
		}
	}
	for r := range kb.layers[layer].matrix {
		for c := range kb.layers[layer].matrix[r] {
			if r == excludeRow && c == excludeCol {
				continue
			}
			if a := kb.actionAt(layer, r, c); a.isShift {
				return r, c, true
			}
		}
	}
	return -1, -1, false
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
			if a.output == target {
				return vilKeyRef{row: row, col: col, action: a}, true
			}
			if a.shifted == target {
				return vilKeyRef{row: row, col: col, action: a, shift: true}, true
			}
		}
	}

	return vilKeyRef{}, false
}

func (kb *vilKeyboard) hintText(h vilHint, found bool) string {
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
	if h.layer != 0 {
		name := kb.layerName(h.layer)
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
		return "base layer"
	}
	return prefix + "press " + key
}

func drawHintLine(scr tcell.Screen, y, x0, width int, text string, def tcell.Style) {
	if width <= 0 || text == "" {
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

	sw, sh := t.Scr.Size()
	m, ok := kb.metrics(sw, sh)
	if !ok {
		return
	}

	hint, found := kb.findRune(target)
	preview := t.previewLayer >= 0 && t.previewLayer < len(kb.layers)
	layer := 0
	if preview {
		layer = t.previewLayer
	} else if found {
		layer = hint.layer
	}

	focusLeft, focusRight := true, true
	if found && !preview {
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

	width := kb.width(m.keyWidth, m.keyGap)
	x0 := (sw - width) / 2

	wrongRow, wrongCol := -1, -1
	if t.wrongKeyValid() {
		if ref, ok := kb.findOnLayer(t.wrongKey, layer); ok {
			wrongRow, wrongCol = ref.row, ref.col
		}
	}

	rows := kb.displayRows()
	for r := 0; r < rows; r++ {
		y := m.top + r*(m.keyH+m.rowGap)

		switch {
		case kb.splitCols:
			leftWidth := kb.halfCols*m.keyWidth + (kb.halfCols-1)*m.keyGap
			kb.drawCols(t.Scr, layer, r, 0, kb.halfCols, x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusLeft, t.blinkOn, t.defaultStyle)
			kb.drawCols(t.Scr, layer, r, kb.halfCols, kb.maxCols,
				x0+leftWidth+vilSplitGap, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusRight, t.blinkOn, t.defaultStyle)
		case kb.halfRows > 0:
			leftWidth := kb.halfWidth(m.keyWidth, m.keyGap)
			kb.drawHalf(t.Scr, layer, r, x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusLeft, t.blinkOn, t.defaultStyle)
			if r+kb.halfRows < len(kb.layers[0].matrix) {
				kb.drawHalf(t.Scr, layer, r+kb.halfRows,
					x0+leftWidth+vilSplitGap,
					y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, focusRight, t.blinkOn, t.defaultStyle)
			}
		default:
			kb.drawHalf(t.Scr, layer, r, x0, y, m.keyWidth, m.keyGap, m.keyH, hint, showTarget, wrongRow, wrongCol, true, t.blinkOn, t.defaultStyle)
		}
	}

	hintY := m.top + rows*m.keyH + (rows-1)*m.rowGap
	hintText := kb.hintText(hint, found)
	if preview {
		hintText = "preview: " + kb.layerName(layer)
	}
	t.hintText = hintText
	if t.chatsubo == nil {
		drawHintLine(t.Scr, hintY, x0, width, hintText, t.defaultStyle)
	}
}

func (kb *vilKeyboard) drawHalf(scr tcell.Screen, layer, matrixRow, x, y, keyWidth, keyGap, keyH int, hint vilHint, found bool, wrongRow, wrongCol int, focused, blink bool, def tcell.Style) {
	if layer < 0 || layer >= len(kb.layers) {
		return
	}
	if matrixRow < 0 || matrixRow >= len(kb.layers[layer].matrix) {
		return
	}

	kb.drawCols(scr, layer, matrixRow, 0, len(kb.layers[layer].matrix[matrixRow]),
		x, y, keyWidth, keyGap, keyH, hint, found, wrongRow, wrongCol, focused, blink, def)
}

func (kb *vilKeyboard) drawCols(scr tcell.Screen, layer, matrixRow, startCol, endCol, x, y, keyWidth, keyGap, keyH int, hint vilHint, found bool, wrongRow, wrongCol int, focused, blink bool, def tcell.Style) {
	if layer < 0 || layer >= len(kb.layers) {
		return
	}
	if matrixRow < 0 || matrixRow >= len(kb.layers[layer].matrix) {
		return
	}

	row := kb.layers[layer].matrix[matrixRow]
	if startCol < 0 {
		startCol = 0
	}
	if endCol > len(row) {
		endCol = len(row)
	}

	for col := startCol; col < endCol; col++ {
		if row[col] == "" {
			continue
		}

		a := kb.actionAt(layer, matrixRow, col)
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
		case matrixRow == wrongRow && col == wrongCol:
			style = kbErrorStyle
			border = kbErrorStyle
			highlighted = true
		case found && matrixRow == hint.row && col == hint.col:
			style = def
			border = targetStyle(blink, def)
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
			style = def.Foreground(tcell.ColorGreen).Dim(true)
			border = def
		}
		if !highlighted && !focused {
			style = style.Dim(true)
			border = border.Dim(true)
		}

		drawKBButton(scr, x+(col-startCol)*(keyWidth+keyGap), y, keyWidth, keyH, label, top, style, border)
	}
}

func drawCenteredLine(scr tcell.Screen, y int, s string, style tcell.Style) {
	sw, _ := scr.Size()
	r := []rune(s)
	if sw <= 1 {
		return
	}
	if len(r) > sw-1 {
		r = r[:sw-1]
	}
	drawString(scr, (sw-len(r))/2, y, string(r), -1, style)
}
