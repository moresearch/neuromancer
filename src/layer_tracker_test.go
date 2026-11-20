package main

import (
	"encoding/json"
	"testing"
	"time"
)

// layerTestVil is a 4-row split matrix (rows 0-1 left, rows 2-3 right) with
// one key per layer behaviour on the base layer:
//
//	(0,0) MO(1)   (0,1) LT(2,KC_A)   (0,2) TG(3)   (0,3) TO(4)
//	(1,0) DF(0)   (1,1) DF(5)        (1,2) TD(0)   (1,3) MO(6)
//
// Layer 5 is a real layer, so DF(5) selects it; TD(0) is a tap-dance whose
// hold is MO(2). Layer 6 does not exist and must be ignored. Layers 1-5 are
// transparent so actionAt resolves against the base layer.
func layerTestVil(t *testing.T) *vilKeyboard {
	t.Helper()

	base := [][]interface{}{
		{"MO(1)", "LT(2,KC_A)", "TG(3)", "TO(4)"},
		{"DF(0)", "DF(5)", "TD(0)", "MO(6)"},
		{"KC_TRNS", "KC_TRNS", "KC_TRNS", "KC_TRNS"},
		{"KC_TRNS", "KC_TRNS", "KC_TRNS", "KC_TRNS"},
	}

	layout := []interface{}{base}
	for i := 0; i < 5; i++ {
		layer := make([][]interface{}, len(base))
		for r := range base {
			layer[r] = make([]interface{}, len(base[r]))
			for c := range layer[r] {
				layer[r][c] = "KC_TRNS"
			}
		}
		layout = append(layout, layer)
	}

	data, err := json.Marshal(map[string]interface{}{
		"layout":    layout,
		"tap_dance": []interface{}{map[string]interface{}{"tap": "KC_A", "hold": "MO(2)"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return loadTestVil(t, string(data))
}

func newTestTracker(kb *vilKeyboard) *layerTracker {
	return &layerTracker{
		kb:           kb,
		layer:        -1,
		prevPressed:  map[[2]int]bool{},
		pressedSince: map[[2]int]time.Time{},
	}
}

// press feeds the tracker one matrix sample and returns the resulting layer.
func (tr *layerTracker) press(keys ...[2]int) int {
	pressed := map[[2]int]bool{}
	for _, k := range keys {
		pressed[k] = true
	}
	tr.updateActive(pressed)
	return tr.Layer()
}

func TestLayerTrackerMomentary(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	if got := tr.press([2]int{0, 0}); got != 1 {
		t.Fatalf("MO(1) held: layer=%d want 1", got)
	}
	if got := tr.press(); got != -1 {
		t.Fatalf("MO(1) released: layer=%d want -1 (target layer)", got)
	}
}

func TestLayerTrackerLayerTap(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	// A quick tap keeps the layer down so a normal tap never flashes it.
	if got := tr.press([2]int{0, 1}); got != -1 {
		t.Fatalf("LT(2) just pressed: layer=%d want -1", got)
	}
	if got := tr.press(); got != -1 {
		t.Fatalf("LT(2) tapped: layer=%d want -1", got)
	}

	// Held past the delay it activates.
	tr.press([2]int{0, 1})
	tr.pressedSince[[2]int{0, 1}] = time.Now().Add(-layerTapDelay - time.Millisecond)
	if got := tr.press([2]int{0, 1}); got != 2 {
		t.Fatalf("LT(2) held: layer=%d want 2", got)
	}

	// Another key pressed while it is down activates it immediately.
	tr = newTestTracker(layerTestVil(t))
	if got := tr.press([2]int{0, 1}, [2]int{2, 0}); got != 2 {
		t.Fatalf("LT(2) + other key: layer=%d want 2", got)
	}
	if got := tr.press([2]int{2, 0}); got != -1 {
		t.Fatalf("LT(2) released: layer=%d want -1", got)
	}
}

func TestLayerTrackerToggle(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	if got := tr.press([2]int{0, 2}); got != 3 {
		t.Fatalf("TG(3) pressed: layer=%d want 3", got)
	}
	if got := tr.press(); got != 3 {
		t.Fatalf("TG(3) released: layer=%d want 3 (still toggled)", got)
	}
	if got := tr.press([2]int{0, 2}); got != -1 {
		t.Fatalf("TG(3) second press: layer=%d want -1", got)
	}
}

func TestLayerTrackerTo(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	if got := tr.press([2]int{0, 3}); got != 4 {
		t.Fatalf("TO(4): layer=%d want 4", got)
	}
	if got := tr.press(); got != 4 {
		t.Fatalf("TO(4) released: layer=%d want 4", got)
	}

	// A momentary layer on top of a TO is released back to the TO layer.
	if got := tr.press([2]int{0, 0}); got != 4 {
		t.Fatalf("MO(1) over TO(4): layer=%d want 4 (highest wins)", got)
	}
	if got := tr.press(); got != 4 {
		t.Fatalf("MO(1) released over TO(4): layer=%d want 4", got)
	}
}

func TestLayerTrackerDefault(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	if got := tr.press([2]int{0, 3}); got != 4 {
		t.Fatalf("TO(4): layer=%d want 4", got)
	}
	if got := tr.press([2]int{1, 1}); got != 5 {
		t.Fatalf("DF(5): layer=%d want 5", got)
	}
	if got := tr.press(); got != 5 {
		t.Fatalf("DF(5) released: layer=%d want 5", got)
	}
	if got := tr.press([2]int{1, 0}); got != -1 {
		t.Fatalf("DF(0): layer=%d want -1 (base)", got)
	}
}

func TestLayerTrackerTapDanceLayer(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	kb := tr.kb
	if a := kb.actionAt(0, 1, 2); a.layerMode != "MO" || a.layer != 2 {
		t.Fatalf("TD hold not parsed: mode=%q layer=%d", a.layerMode, a.layer)
	}
	if got := tr.press([2]int{1, 2}); got != 2 {
		t.Fatalf("TD(0) whose hold is MO(2): layer=%d want 2", got)
	}
}

func TestLayerTrackerIgnoresUnknownLayer(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	// MO(6) has no layer 6; it must not set an out-of-range bit.
	if got := tr.press([2]int{1, 3}); got != -1 {
		t.Fatalf("MO(6): layer=%d want -1", got)
	}
	if tr.toggled != 0 {
		t.Fatalf("toggled=%b want 0", tr.toggled)
	}
}

func TestLayerTrackerHighestWins(t *testing.T) {
	tr := newTestTracker(layerTestVil(t))

	// Two momentary keys at once: MO(1) and LT(2) held past the delay.
	tr.press([2]int{0, 0})
	tr.press([2]int{0, 0}, [2]int{0, 1})
	tr.pressedSince[[2]int{0, 1}] = time.Now().Add(-layerTapDelay - time.Millisecond)
	if got := tr.press([2]int{0, 0}, [2]int{0, 1}); got != 2 {
		t.Fatalf("MO(1)+LT(2): layer=%d want 2 (highest wins)", got)
	}
	// Releasing the higher one leaves the other still held.
	if got := tr.press([2]int{0, 0}); got != 1 {
		t.Fatalf("after releasing LT(2): layer=%d want 1 (MO still held)", got)
	}
	if got := tr.press(); got != -1 {
		t.Fatalf("all released: layer=%d want -1", got)
	}

	// A toggle stacked on a held momentary key: the toggle wins, and
	// releasing it returns to the momentary key.
	tr = newTestTracker(layerTestVil(t))
	if got := tr.press([2]int{0, 2}); got != 3 {
		t.Fatalf("TG(3): layer=%d want 3", got)
	}
	if got := tr.press([2]int{0, 0}, [2]int{0, 2}); got != 3 {
		t.Fatalf("MO(1) over TG(3): layer=%d want 3", got)
	}
	if got := tr.press([2]int{0, 0}); got != 3 {
		t.Fatalf("after TG release: layer=%d want 3 (toggled)", got)
	}
}

func TestParseVilTapDanceHold(t *testing.T) {
	kb := layerTestVil(t)
	if len(kb.tapDances) != 1 || kb.tapDances[0].tap != "KC_A" || kb.tapDances[0].hold != "MO(2)" {
		t.Fatalf("tap dances = %+v", kb.tapDances)
	}

	a := parseVilKey("TD(0)", kb.tapDances, 0)
	if a.layerMode != "MO" || a.layer != 2 || a.output != 'a' {
		t.Fatalf("TD(0) = mode %q layer %d output %q", a.layerMode, a.layer, a.output)
	}

	// Plain string tap-dances keep working and never carry a layer.
	plain := parseVilTapDances([]interface{}{"KC_B", "LT(1,KC_SPC)"})
	if plain[0].tap != "KC_B" || plain[0].hold != "" {
		t.Fatalf("plain tap dance = %+v", plain[0])
	}
	if a := parseVilKey("TD(1)", plain, 0); a.layerMode != "LT" || a.layer != 1 {
		t.Fatalf("LT tap dance = mode %q layer %d", a.layerMode, a.layer)
	}
}

func TestLayerTrackerPressFromState(t *testing.T) {
	tr := newTestTracker(nil)

	// Four rows of four columns: one byte per row, low bit of a byte is the
	// last column (VIA packs each row high byte first).
	tr.kb = &vilKeyboard{maxCols: 4, layers: []*vilLayer{{matrix: make([][]string, 4)}}}
	pressed := tr.pressedFromState([]byte{2, 0, 8, 0})
	if len(pressed) != 2 || !pressed[[2]int{0, 1}] || !pressed[[2]int{2, 3}] {
		t.Fatalf("pressed = %v", pressed)
	}
	if short := tr.pressedFromState([]byte{2}); len(short) != 0 {
		t.Fatalf("short state decoded as %v", short)
	}

	// Nine columns need two bytes per row; columns 0-6 live in the second byte.
	tr.kb = &vilKeyboard{maxCols: 9, layers: []*vilLayer{{matrix: make([][]string, 1)}}}
	pressed = tr.pressedFromState([]byte{0, 0b10000010})
	if len(pressed) != 2 || !pressed[[2]int{0, 1}] || !pressed[[2]int{0, 7}] {
		t.Fatalf("wide pressed = %v", pressed)
	}
}
