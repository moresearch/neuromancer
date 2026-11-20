package main

import (
	"testing"

	"github.com/gdamore/tcell"
)

func TestEnsureReadable(t *testing.T) {
	bg := tcell.NewHexColor(0x282828) // gruvbox-dark background

	// The gruvbox-dark hicol2 is 2.7:1 on its own background: too dim for the
	// word being typed, so it falls back to the normal text colour.
	if got := ensureReadable(tcell.NewHexColor(0xbb3f30), bg); got != tcell.ColorDefault {
		t.Errorf("dim highlight = %v, want the default text colour", got)
	}

	// A colour with plenty of contrast is kept as-is.
	bright := tcell.NewHexColor(0x00F0FF)
	if got := ensureReadable(bright, bg); got != bright {
		t.Errorf("bright highlight = %v, want %v", got, bright)
	}

	// Unknown backgrounds (terminal default) are left alone.
	if got := ensureReadable(tcell.ColorGreen, tcell.ColorDefault); got != tcell.ColorGreen {
		t.Errorf("default bg = %v, want %v", got, tcell.ColorGreen)
	}
}
