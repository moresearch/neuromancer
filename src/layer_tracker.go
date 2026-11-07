package main

import (
	"sync"
	"time"

	"github.com/gdamore/tcell"
)

// layerTapDelay keeps a layer-tap key (for example space on LT1) from flashing
// its layer for the duration of a normal tap. A held layer key still activates
// immediately when another key is pressed.
const layerTapDelay = 180 * time.Millisecond

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

	prevPressed  map[[2]int]bool
	pressedSince map[[2]int]time.Time
	toggled      uint32
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
	if t.prevPressed == nil {
		t.prevPressed = map[[2]int]bool{}
	}
	if t.pressedSince == nil {
		t.pressedSince = map[[2]int]time.Time{}
	}
	now := time.Now()

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

	for pos := range t.pressedSince {
		if !pressed[pos] {
			delete(t.pressedSince, pos)
		}
	}

	hasOtherKey := false
	for pos := range pressed {
		a := t.kb.actionAt(0, pos[0], pos[1])
		if a.layerMode == "" {
			hasOtherKey = true
		}
		if a.layerMode == "LT" || a.layerMode == "TT" {
			if _, ok := t.pressedSince[pos]; !ok {
				t.pressedSince[pos] = now
			}
		}
	}

	active := t.toggled
	for pos := range pressed {
		a := t.kb.actionAt(0, pos[0], pos[1])
		switch a.layerMode {
		case "MO", "OSL", "LM":
			if a.layer > 0 {
				active |= 1 << uint(a.layer)
			}
		case "LT", "TT":
			if a.layer > 0 {
				since := t.pressedSince[pos]
				if hasOtherKey || now.Sub(since) >= layerTapDelay {
					active |= 1 << uint(a.layer)
				}
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
