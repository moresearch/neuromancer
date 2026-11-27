package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// testVil is a small split layout: the first three rows are the left half and
// the last three rows are the right half. Each top-level array is a layer.
const testVil = `{
  "layout": [
    [
      ["KC_Q", "KC_W"],
      ["KC_A", "MO(1)"],
      ["KC_SPC", "KC_LSFT"],
      ["KC_1", "KC_2"],
      ["KC_3", "KC_4"],
      ["KC_5", "KC_6"]
    ],
    [
      ["KC_EXLM", "KC_AT"],
      ["KC_TRNS", "KC_TRNS"],
      ["KC_TRNS", "KC_TRNS"],
      ["KC_HASH", "KC_DLR"],
      ["KC_PERC", "KC_CIRC"],
      ["KC_AMPR", "KC_ASTR"]
    ]
  ],
  "tap_dance": []
}`

func loadTestVil(t *testing.T, data string) *vilKeyboard {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.vil")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	kb, err := loadVil(path, "base,lower")
	if err != nil {
		t.Fatal(err)
	}
	return kb
}

func TestVilLayersAndHints(t *testing.T) {
	kb := loadTestVil(t, testVil)

	if len(kb.layers) != 2 {
		t.Fatalf("layers=%d", len(kb.layers))
	}
	if kb.layerName(0) != "base" || kb.layerName(1) != "lower" {
		t.Fatalf("layer names: %q %q", kb.layerName(0), kb.layerName(1))
	}

	h, ok := kb.findRune('!')
	if !ok {
		t.Fatal("! not found")
	}
	if h.layer != 1 || h.row != 0 || h.col != 0 {
		t.Fatalf("! hint = layer %d, %d,%d", h.layer, h.row, h.col)
	}
	if h.activator == nil || h.activator.row != 1 || h.activator.col != 1 {
		t.Fatalf("! activator = %+v", h.activator)
	}
	if h.activator.action.label != "MO1" {
		t.Fatalf("activator label = %q", h.activator.action.label)
	}
	if got := kb.hintText(h, true); !strings.Contains(got, "hold MO1") || !strings.Contains(got, "press !") {
		t.Fatalf("hint = %q", got)
	}

	h, ok = kb.findRune('a')
	if !ok || h.layer != 0 || h.row != 1 || h.col != 0 || h.shift {
		t.Fatalf("a hint = %+v ok=%v", h, ok)
	}

	h, ok = kb.findRune('A')
	if !ok || h.layer != 0 || !h.shift || h.shiftRow != 2 || h.shiftCol != 1 {
		t.Fatalf("A hint = %+v ok=%v", h, ok)
	}

	h, ok = kb.findRune(' ')
	if !ok || h.layer != 0 || h.row != 2 || h.col != 0 {
		t.Fatalf("space hint = %+v ok=%v", h, ok)
	}

	if _, ok := kb.findRune('7'); ok {
		t.Fatal("7 should not be found")
	}
}

func TestVilRendering(t *testing.T) {
	kb := loadTestVil(t, testVil)

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)

	typ := newTestTyper(sim)
	typ.vil = kb
	typ.drawKeyboard('!')

	// The board is placed by the typer's centred block, not by the raw
	// metrics, so read the drawn position back from it.
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	width := kb.width(m.keyWidth, m.keyGap, m.splitGap)
	x0 := (80 - width) / 2
	keyY := func(row int) int {
		return kb.vilRowY(m.top, row, m.keyH, m.rowGap)
	}

	_, _, targetStyleCell, _ := sim.GetContent(x0+m.keyWidth/2, keyY(0)+1)
	if got := bgOf(targetStyleCell); got != bgOf(typ.defaultStyle) {
		t.Errorf("target interior bg=%v, want %v", got, bgOf(typ.defaultStyle))
	}
	// The activator (MO1) is highlighted; the home-column gap shifts it by
	// one key cell, so use the drawing's own column offset.
	actX := x0 + kb.colX(1, 1, m.keyWidth, m.keyGap) + m.keyWidth/2
	_, _, actStyle, _ := sim.GetContent(actX, keyY(1))
	if _, _, actAttrs := actStyle.Decompose(); actAttrs&tcell.AttrBold == 0 {
		t.Error("activator MO1 is not highlighted")
	}

	// The target (!, the lower layer's top-left key) is on the left half, so
	// the right half is dimmed. Which glyph an inactive cell holds depends on
	// the cap size (border or label row), so compare the halves' cell
	// attributes rather than one cell.
	halfCap := func(half int) (dim, bright int) {
		for _, row := range []int{0, 1} {
			for col := 0; col < 2; col++ {
				x := x0 + col*(m.keyWidth+m.keyGap) + m.keyWidth/2
				if half == 1 {
					x += kb.halfWidth(false, m.keyWidth, m.keyGap) + m.splitGap
				}
				_, _, st, _ := sim.GetContent(x, keyY(row))
				if _, _, attrs := st.Decompose(); attrs&tcell.AttrDim != 0 {
					dim++
				} else {
					bright++
				}
			}
		}
		return dim, bright
	}
	leftDim, leftBright := halfCap(0)
	rightDim, rightBright := halfCap(1)
	if leftBright == 0 || leftDim != 0 {
		t.Errorf("focused left half: %d dim, %d bright", leftDim, leftBright)
	}
	if rightDim == 0 {
		t.Errorf("inactive right half: %d dim, %d bright", rightDim, rightBright)
	}

	typ.wrongKey = '@'
	typ.wrongUntil = time.Now().Add(time.Second)
	typ.drawKeyboard('!')
	// The wrong key (@, column 1 of the top row) is not dimmed by the finger
	// highlight; it is drawn with the error style.
	_, _, errStyle, _ := sim.GetContent(x0+kb.colX(1, 1, m.keyWidth, m.keyGap)+m.keyWidth/2, keyY(0))
	if _, _, errAttrs := errStyle.Decompose(); errAttrs&tcell.AttrDim != 0 {
		t.Error("wrong key is dimmed")
	}
}

func TestVilBlankKeysDrawn(t *testing.T) {
	// A row with an empty cell (""): the physical position must still draw a
	// blank keycap so the board is a solid rectangle, not a ragged gap.
	const vil = `{
  "layout": [
    [
      ["KC_Q", "KC_W", ""],
      ["KC_A", "KC_S", "KC_D"],
      ["KC_Z", "KC_X", "KC_C"],
      ["KC_1", "KC_2", "KC_3"],
      ["KC_4", "KC_5", "KC_6"]
    ]
  ],
  "tap_dance": []
}`
	kb := loadTestVil(t, vil)

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)

	typ := newTestTyper(sim)
	typ.vil = kb
	typ.drawKeyboard('q')
	sim.Show()

	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	width := kb.width(m.keyWidth, m.keyGap, m.splitGap)
	x0 := (80 - width) / 2
	// The empty cell is column 2 of row 0. Its cap must be bordered (a corner
	// glyph on the cap's top row).
	x := x0 + kb.colXSide(0, 2, false, m.keyWidth, m.keyGap)
	y := kb.vilRowY(m.top, 0, m.keyH, m.rowGap)
	mainc, _, _, _ := sim.GetContent(x, y)
	if mainc != '╭' && mainc != '┌' {
		t.Errorf("empty cell cap corner = %q, want a border glyph", string(mainc))
	}
}

// TestVilEmptyKeyLightsWhenPressed checks that an unbound physical cell (empty
// cell, no action) still lights up when its matrix position is held.
func TestVilEmptyKeyLightsWhenPressed(t *testing.T) {
	rows := make([][]interface{}, 5)
	for r := range rows {
		rows[r] = make([]interface{}, 12)
		for c := range rows[r] {
			rows[r][c] = "KC_Q"
		}
	}
	rows[0][2] = "" // an empty, unbound physical key
	data, err := json.Marshal(map[string]interface{}{"layout": []interface{}{rows}})
	if err != nil {
		t.Fatal(err)
	}
	kb := loadTestVil(t, string(data))

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(120, 30)

	typ := newTestTyper(sim)
	typ.vil = kb
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}

	typ.layerTracker = &layerTracker{prevPressed: map[[2]int]bool{{0, 2}: true}}
	typ.drawKeyboard('q')
	width := kb.width(m.keyWidth, m.keyGap, m.splitGap)
	x0 := (120 - width) / 2
	x := x0 + kb.colXSide(0, 2, false, m.keyWidth, m.keyGap)
	y := kb.vilRowY(m.top, 0, m.keyH, m.rowGap) + m.keyH/2
	_, _, pressStyle, _ := sim.GetContent(x, y)

	typ.layerTracker = nil
	typ.drawKeyboard('q')
	_, _, idleStyle, _ := sim.GetContent(x, y)
	if pressStyle == idleStyle {
		t.Error("empty key does not change style when pressed")
	}
}

func TestVilTransparentAndMissingLayers(t *testing.T) {
	kb := loadTestVil(t, testVil)

	a := kb.actionAt(0, 2, 1)
	if !a.isShift || a.label != "SFT" {
		t.Fatalf("shift action = %+v", a)
	}

	if a := kb.actionAt(1, 1, 0); a.output != 'a' {
		t.Fatalf("transparent fallback = %+v", a)
	}

	if a := kb.actionAt(5, 0, 0); !a.none {
		t.Fatalf("missing layer action = %+v", a)
	}
}

func TestVilLayerDrill(t *testing.T) {
	kb := loadTestVil(t, testVil)

	chars := layerDrillRunes(kb)
	if len(chars) != 8 {
		t.Fatalf("drill chars = %v", chars)
	}
	for _, c := range chars {
		if c == "a" {
			t.Fatal("base character leaked into drill")
		}
	}

	gen := generateVilDrill(kb, 3, 1)
	if gen == nil {
		t.Fatal("no drill generator")
	}
	segs := gen()
	if len(segs) != 1 {
		t.Fatalf("segments = %d", len(segs))
	}
	if strings.Count(segs[0].Text, " ") != 2 {
		t.Fatalf("drill text = %q", segs[0].Text)
	}
}

func TestVilSplitOverride(t *testing.T) {
	kb := loadTestVil(t, testVil)
	if kb.splitCols || kb.halfRows != 3 {
		t.Fatalf("auto split: cols=%v rows=%d", kb.splitCols, kb.halfRows)
	}

	if err := kb.setSplitMode("cols"); err != nil {
		t.Fatal(err)
	}
	if !kb.splitCols || kb.halfCols != 1 || kb.halfRows != 0 {
		t.Fatalf("cols split: %v %d %d", kb.splitCols, kb.halfCols, kb.halfRows)
	}

	if err := kb.setSplitMode("none"); err != nil {
		t.Fatal(err)
	}
	if kb.splitCols || kb.halfRows != 0 {
		t.Fatalf("none split: %v %d", kb.splitCols, kb.halfRows)
	}

	if err := kb.setSplitMode("rows"); err != nil {
		t.Fatal(err)
	}
	if kb.halfRows != 3 {
		t.Fatalf("rows split: %d", kb.halfRows)
	}

	if err := kb.setSplitMode("bogus"); err == nil {
		t.Fatal("bogus split mode should fail")
	}
}

func loadWideTestVil(t *testing.T) *vilKeyboard {
	t.Helper()

	rows := make([][]interface{}, 5)
	for r := range rows {
		rows[r] = make([]interface{}, 12)
		for c := range rows[r] {
			rows[r][c] = "KC_Q"
		}
	}

	data, err := json.Marshal(map[string]interface{}{"layout": []interface{}{rows}})
	if err != nil {
		t.Fatal(err)
	}
	return loadTestVil(t, string(data))
}

func TestVilAutoColumnSplit(t *testing.T) {
	kb := loadWideTestVil(t)
	if !kb.splitCols || kb.halfCols != 6 || kb.halfRows != 0 {
		t.Fatalf("auto cols: split=%v halfCols=%d halfRows=%d", kb.splitCols, kb.halfCols, kb.halfRows)
	}
	if rows := kb.displayRows(); rows != 5 {
		t.Fatalf("display rows = %d", rows)
	}
	if m, ok := kb.metrics(80, 24, boardHalfGap); !ok || m.keyWidth < 3 || m.keyGap < 0 {
		t.Fatalf("metrics = %+v %v", m, ok)
	}
}

func TestVilNumericKeycodes(t *testing.T) {
	if a := parseVilKey("\x004", nil, 0); a.output != 'a' || a.label != "a" {
		t.Fatalf("numeric 0x04 = %+v", a)
	}
	if a := parseVilKey("\x0044", nil, 0); a.output != ' ' {
		t.Fatalf("numeric space = %+v", a)
	}
	if a := parseVilKey("\x0021025", nil, 0); a.layer != 1 || a.layerMode != "MO" || a.label != "MO1" {
		t.Fatalf("numeric MO1 = %+v", a)
	}
	if a := parseVilKey("\x0016644", nil, 0); a.layer != 1 || a.layerMode != "LT" || a.output != 'a' {
		t.Fatalf("numeric LT1 = %+v", a)
	}
	if a := parseVilKey("\x008708", nil, 0); !a.isShift || a.output != 'a' {
		t.Fatalf("numeric shift-tap = %+v", a)
	}
}

func TestVilFlatLayout(t *testing.T) {
	cells := make([]interface{}, 58)
	for i := range cells {
		cells[i] = 0
	}
	cells[0] = 4 // KC_A
	cells[1] = 5 // KC_B

	data, err := json.Marshal(map[string]interface{}{"layout": []interface{}{cells}})
	if err != nil {
		t.Fatal(err)
	}
	kb := loadTestVil(t, string(data))

	if len(kb.layers) != 1 || len(kb.layers[0].matrix) != 5 {
		t.Fatalf("flat layers: %d %d", len(kb.layers), len(kb.layers[0].matrix))
	}
	if !kb.splitCols || kb.halfCols != 6 {
		t.Fatalf("flat split: cols=%v halfCols=%d", kb.splitCols, kb.halfCols)
	}
	for _, r := range kb.layers[0].matrix {
		if len(r) != 12 {
			t.Fatalf("flat row width = %d", len(r))
		}
	}
	if a := kb.actionAt(0, 0, 0); a.output != 'a' {
		t.Fatalf("flat a = %+v", a)
	}
	if a := kb.actionAt(0, 0, 1); a.output != 'b' {
		t.Fatalf("flat b = %+v", a)
	}
}

func TestVilTenHomeKeys(t *testing.T) {
	kb := loadWideTestVil(t)

	count := 0
	for r := range kb.layers[0].matrix {
		for c := range kb.layers[0].matrix[r] {
			if kb.isHomeKey(r, c) {
				count++
			}
		}
	}
	if count != 10 {
		t.Fatalf("home keys = %d, want 10", count)
	}

	if !kb.isHomeKey(kb.homeRow, 4) {
		t.Error("left index home (t) is not marked")
	}
	if kb.isHomeKey(kb.homeRow, 5) {
		t.Error("left inner index key (g) should not be a home key")
	}

	if r, c, ok := kb.targetHome(0, 5); !ok || r != kb.homeRow || c != 4 {
		t.Fatalf("g maps to %d,%d,%v; want %d,4", r, c, ok, kb.homeRow)
	}
	if r, c, ok := kb.targetHome(0, 6); !ok || r != kb.homeRow || c != 7 {
		t.Fatalf("m maps to %d,%d,%v; want %d,7", r, c, ok, kb.homeRow)
	}
}

func loadHomeTestVil(t *testing.T) *vilKeyboard {
	t.Helper()

	rows := make([][]interface{}, 5)
	for r := range rows {
		rows[r] = make([]interface{}, 12)
		for c := range rows[r] {
			rows[r][c] = "KC_Q"
		}
	}

	// Extra column on the inner side, like the user's Sofle .vil.
	rows[2][0] = "KC_A"
	rows[2][1] = "KC_R"
	rows[2][2] = "KC_S"
	rows[2][3] = "KC_T"
	rows[2][4] = "KC_G"
	rows[2][5] = "KC_NO"
	rows[2][6] = "KC_NO"
	rows[2][7] = "KC_M"
	rows[2][8] = "KC_N"
	rows[2][9] = "KC_E"
	rows[2][10] = "KC_I"
	rows[2][11] = "KC_O"

	rows[4][4] = "KC_SPC"
	rows[4][8] = "KC_SPC"

	data, err := json.Marshal(map[string]interface{}{"layout": []interface{}{rows}})
	if err != nil {
		t.Fatal(err)
	}
	return loadTestVil(t, string(data))
}

func TestVilHomeKeysByCharacter(t *testing.T) {
	kb := loadHomeTestVil(t)

	for _, c := range []int{0, 1, 2, 3, 8, 9, 10, 11} {
		if !kb.isHomeKey(kb.homeRow, c) {
			t.Errorf("home key missing at col %d", c)
		}
	}
	for _, c := range []int{4, 7} {
		if kb.isHomeKey(kb.homeRow, c) {
			t.Errorf("non-home key marked at col %d", c)
		}
	}

	count := 0
	for r := range kb.layers[0].matrix {
		for c := range kb.layers[0].matrix[r] {
			if kb.isHomeKey(r, c) {
				count++
			}
		}
	}
	if count != 10 {
		t.Fatalf("home keys = %d, want 10", count)
	}

	if r, c, ok := kb.targetHome(0, 4); !ok || r != kb.homeRow || c != 3 {
		t.Fatalf("g home = %d,%d,%v; want %d,3", r, c, ok, kb.homeRow)
	}
	if r, c, ok := kb.targetHome(0, 7); !ok || r != kb.homeRow || c != 8 {
		t.Fatalf("m home = %d,%d,%v; want %d,8", r, c, ok, kb.homeRow)
	}
}

func TestVilHrmParsing(t *testing.T) {
	tests := []struct {
		raw     string
		mod     string
		ch      rune
		display string
	}{
		{"LCTL_T(KC_A)", "Ctrl", 'a', "Ca"},
		{"LALT_T(KC_R)", "Alt", 'r', "Ar"},
		{"LGUI_T(KC_S)", "Gui", 's', "Gs"},
		{"LSFT_T(KC_T)", "Shift", 't', "St"},
		{"MT(MOD_LCTL, KC_A)", "Ctrl", 'a', "Ca"},
		{"MT(MOD_LSFT, KC_T)", "Shift", 't', "St"},
	}

	for _, tc := range tests {
		a := parseVilKey(tc.raw, nil, 0)
		if a.mods != tc.mod || a.output != tc.ch || a.display != tc.display {
			t.Errorf("%s = mods %q output %q display %q; want %q %q %q", tc.raw, a.mods, a.output, a.display, tc.mod, tc.ch, tc.display)
		}
		if (tc.mod == "Shift") != a.isShift {
			t.Errorf("%s isShift = %v", tc.raw, a.isShift)
		}
	}
}

const hrmTestVil = `{
  "layout": [
    [
      ["KC_A", "KC_B"],
      ["KC_LSFT", "LSFT_T(KC_T)"],
      ["KC_SPC", "KC_NO"],
      ["KC_C", "KC_D"],
      ["KC_E", "KC_F"],
      ["KC_G", "KC_H"]
    ]
  ]
}`

func TestFindShiftPrefersHrm(t *testing.T) {
	kb := loadTestVil(t, hrmTestVil)

	h, ok := kb.findRune('A')
	if !ok || !h.shift {
		t.Fatalf("A hint = %+v %v", h, ok)
	}
	if h.shiftRow != 1 || h.shiftCol != 1 {
		t.Fatalf("A shift key = %d,%d; want HRM at 1,1", h.shiftRow, h.shiftCol)
	}

	chars := hrmShiftRunes(kb)
	if len(chars) == 0 {
		t.Fatal("no HRM shift runes found")
	}
	if gen := generateHrmDrill(kb, 2, 1); gen == nil {
		t.Fatal("no HRM drill generator")
	}
}

func TestVilLayerHintKeepsKeyGuidance(t *testing.T) {
	kb := loadTestVil(t, testVil)

	h, ok := kb.findRune('!')
	if !ok {
		t.Fatal("! not found")
	}

	// While the target layer is held, the activator is already down; the hint
	// must still say which key to press.
	got := kb.layerHintText("held", h.layer, h, true)
	if !strings.Contains(got, "held: lower") || !strings.Contains(got, "press !") {
		t.Fatalf("held hint = %q", got)
	}
	if strings.Contains(got, "hold MO1") {
		t.Fatalf("held hint repeats the activator: %q", got)
	}

	// While another layer is held, the hint keeps the full access sequence.
	got = kb.layerHintText("held", 0, h, true)
	if !strings.Contains(got, "held: base") || !strings.Contains(got, "hold MO1") || !strings.Contains(got, "press !") {
		t.Fatalf("other-layer hint = %q", got)
	}

	got = kb.layerHintText("preview", h.layer, h, true)
	if !strings.Contains(got, "preview: lower") || !strings.Contains(got, "press !") {
		t.Fatalf("preview hint = %q", got)
	}

	got = kb.hintForLayer("held", h.layer, '7')
	if !strings.Contains(got, "held: lower") || !strings.Contains(got, "no key") {
		t.Fatalf("missing-key layer hint = %q", got)
	}

	if got := noKeyHint(';'); !strings.Contains(got, "no key") || !strings.Contains(got, ";") {
		t.Fatalf("missing-key hint = %q", got)
	}
}

// The home columns are the ones the fingers rest on: a r s t on the left half
// and n e i o on the right. The neighbouring g and m are *not* home columns.
func TestVilHomeColumns(t *testing.T) {
	// A Miryoku-shaped matrix if one is installed, otherwise the test fixture.
	kb, err := loadVil("/home/maf/.config/nm/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil || kb.halfRows == 0 {
		kb = loadWideTestVil(t)
	}

	home := func(row, col int) bool {
		b := kb.homeBlockAt(row, col)
		return col >= b.start && col < b.start+b.cols
	}

	leftRow := kb.homeRow
	for col := 0; col < kb.maxCols; col++ {
		got := kb.actionAt(0, leftRow, col).output
		want := strings.ContainsRune("arst", got)
		if home(leftRow, col) != want {
			t.Errorf("left row: %q home=%v, want %v", string(got), home(leftRow, col), want)
		}
	}
	if kb.halfRows == 0 {
		return
	}
	rightRow := kb.halfRows + kb.homeRow
	for col := 0; col < kb.maxCols; col++ {
		got := kb.actionAt(0, rightRow, col).output
		want := strings.ContainsRune("neio", got)
		if home(rightRow, col) != want {
			t.Errorf("right row: %q home=%v, want %v", string(got), home(rightRow, col), want)
		}
	}
}

// Thumb keys belong to their half: their offset must be inside it, not spill
// into the other half or the gap between the halves.
// The Chatsubo box is a band centred on the board, on the board's columns.
func TestChatsuboCentredOnBoard(t *testing.T) {
	for _, vil := range []string{"", "/home/maf/.config/nm/CANONICAL_ATmega_100pct_Miryoku_3x5.vil"} {
		for _, sw := range []int{160, 170, 199} {
			sim := tcell.NewSimulationScreen("UTF-8")
			if err := sim.Init(); err != nil {
				t.Fatal(err)
			}
			sim.SetSize(sw, 40)
			typ := newTestTyper(sim)
			if vil != "" {
				kb, err := loadVil(vil, "")
				if err != nil {
					sim.Fini()
					t.Skip("layout not installed")
				}
				typ.vil = kb
			}
			typ.chatsubo = newChatsubo("", "", 0)
			typ.keyboardMetrics()
			px, _, pw, _, ok := typ.chatBand()
			if !ok {
				sim.Fini()
				continue
			}
			bx, bw, ok := typ.boardBand()
			if !ok {
				sim.Fini()
				continue
			}
			// The box may be wider than the board, but it is centred on it
			// and must fully contain it.
			if pw < bw || px > bx || px+pw < bx+bw {
				t.Errorf("sw=%d vil=%v: box %d..%d does not hold board %d..%d", sw, vil != "", px, px+pw, bx, bx+bw)
			}
			sim.Fini()
		}
	}
}

// The halves of a row-split matrix are drawn from their own inner edges, so
// the same column number lands in a different place on each half. The right
// half's home block must therefore be measured against the right half.

// A literal KC_NO cell is not drawn (no placeholder keycap), but a tap-dance
// whose tap is KC_NO is still a wired key (it has a double-tap action) and
// must still show. The draw loop skips only cells whose raw string is KC_NO,
// so a TD(n) key keeps its raw prefix and renders.
func TestVilNoiselessNoCell(t *testing.T) {
	kb, err := loadVil("/home/maf/modwm/kb/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil || kb.halfRows == 0 {
		t.Skip("layout not installed")
	}
	// TD(0) lives at row 6 col 0 (the first alpha row of the right half); its
	// tap is KC_NO but it has a double-tap, so it must not be skipped.
	a := kb.actionAt(0, kb.halfRows+1, 0)
	if a.none && !strings.HasPrefix(a.raw, "TD(") {
		t.Errorf("none key at (%d,0) has raw %q, but it is a wired tap-dance", kb.halfRows+1, a.raw)
	}
}

// TestVilBoardThumbMirror checks that a board's thumb cluster is mirrored on
// the right half so it hugs that half's inner edge, instead of the same x on
// both halves (which leaves the right cluster out at the outer edge).
func TestVilBoardThumbMirror(t *testing.T) {
	kb, err := loadVil(os.Getenv("HOME")+"/.config/nm/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil || !kb.hasBoard {
		t.Skip("board layout not installed")
	}
	row := -1
	for r := range kb.thumbX {
		row = r
	}
	if row < 0 {
		t.Skip("no thumb row")
	}
	const kw, kg = 5, 1
	step := kw // contiguous: no inter-key gap
	maxX := 0
	for _, k := range kb.thumbX[row] {
		if k.x > maxX {
			maxX = k.x
		}
	}
	for _, k := range kb.thumbX[row] {
		left := kb.colXSide(row, k.col, false, kw, kg)
		right := kb.colXSide(row, k.col, true, kw, kg)
		if left != k.x*step {
			t.Errorf("col %d: left thumb x %d, want %d", k.col, left, k.x*step)
		}
		if right != (maxX-k.x)*step {
			t.Errorf("col %d: right thumb x %d, want mirrored %d", k.col, right, (maxX-k.x)*step)
		}
	}
}

// TestVilBoardHomeLettersGreen checks the resting home keys (a r s t / n e i o)
// draw in green so they stand out from the rest of the board.
func TestVilBoardHomeLettersGreen(t *testing.T) {
	kb, err := loadVil(os.Getenv("HOME")+"/.config/nm/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil || !kb.hasBoard || kb.halfRows == 0 {
		t.Skip("board layout not installed")
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(120, 40)

	typ := newTestTyper(sim)
	typ.vil = kb
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	typ.drawVilKeyboard('q')
	sim.Show()

	width := kb.width(m.keyWidth, m.keyGap, m.splitGap)
	x0 := (120 - width) / 2
	rightX := x0 + kb.halfWidth(false, m.keyWidth, m.keyGap) + m.splitGap
	homeY := kb.vilRowY(m.top, 2, m.keyH, 0) + m.keyH/2
	for _, half := range []struct {
		ox   int
		cols [2]int
		side bool
	}{{x0, [2]int{1, 4}, false}, {rightX, [2]int{2, 5}, true}} {
		for c := half.cols[0]; c <= half.cols[1]; c++ {
			x := half.ox + kb.colXSide(2, c, half.side, m.keyWidth, m.keyGap) + m.keyWidth/2
			_, _, st, _ := sim.GetContent(x, homeY)
			fg, _, _ := st.Decompose()
			if fg != tcell.ColorGreen {
				t.Errorf("side right=%v col %d home letter fg=%v, want green", half.side, c, fg)
			}
		}
	}
}
