package main

import (
	"sync"

	"github.com/gdamore/tcell"
)

// layerTracker follows the physical layer keys by polling the Vial raw HID
// matrix. Layer is -1 when no layer key is held, so the display falls back to
// the layer required by the next character.
type layerTracker struct {
	mu        sync.Mutex
	layer     int
	scr       tcell.Screen
	kb        *vilKeyboard
	fd        int
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once

	prevPressed map[[2]int]bool
	toggled     uint32
}

func (t *layerTracker) Layer() int {
	t.mu.Lock()
	layer := t.layer
	t.mu.Unlock()
	return layer
}

func (t *layerTracker) setLayer(layer int) {
	t.mu.Lock()
	changed := t.layer != layer
	t.layer = layer
	t.mu.Unlock()
	if changed && t.scr != nil {
		_ = t.scr.PostEvent(tcell.NewEventInterrupt(nil))
	}
}

// pressedFromState decodes the VIA switch-matrix-state response. VIA packs
// each matrix row high byte first, using the same order as the .vil layout.
func (t *layerTracker) pressedFromState(state []byte) map[[2]int]bool {
	pressed := map[[2]int]bool{}
	if t.kb == nil || len(t.kb.layers) == 0 {
		return pressed
	}

	rows := len(t.kb.layers[0].matrix)
	cols := t.kb.maxCols
	bytesPerRow := (cols + 7) / 8
	if bytesPerRow < 1 {
		bytesPerRow = 1
	}
	if len(state) < rows*bytesPerRow {
		return pressed
	}

	for r := 0; r < rows; r++ {
		var bits uint32
		for b := 0; b < bytesPerRow; b++ {
			bits = bits<<8 | uint32(state[r*bytesPerRow+b])
		}
		for c := 0; c < cols; c++ {
			if bits&(1<<uint(c)) != 0 {
				pressed[[2]int{r, c}] = true
			}
		}
	}
	return pressed
}

// updateActive turns the pressed matrix cells into the highest active layer.
// Momentary layer keys (MO/LT/OSL/TT/LM) count while held; TG/TO/DF are tracked
// as toggles.
func (t *layerTracker) updateActive(pressed map[[2]int]bool) {
	if t.kb == nil || len(t.kb.layers) == 0 {
		t.setLayer(-1)
		return
	}

	for pos := range pressed {
		if t.prevPressed[pos] {
			continue
		}
		a := t.kb.actionAt(0, pos[0], pos[1])
		switch a.layerMode {
		case "TG":
			if a.layer > 0 {
				t.toggled ^= 1 << uint(a.layer)
			}
		case "TO":
			if a.layer > 0 {
				t.toggled = 1 << uint(a.layer)
			}
		case "DF":
			t.toggled = 0
		}
	}

	active := t.toggled
	for pos := range pressed {
		a := t.kb.actionAt(0, pos[0], pos[1])
		switch a.layerMode {
		case "MO", "LT", "OSL", "TT", "LM":
			if a.layer > 0 {
				active |= 1 << uint(a.layer)
			}
		}
	}

	layer := -1
	for l := len(t.kb.layers) - 1; l >= 1; l-- {
		if active&(1<<uint(l)) != 0 {
			layer = l
			break
		}
	}
	t.prevPressed = pressed
	t.setLayer(layer)
}
