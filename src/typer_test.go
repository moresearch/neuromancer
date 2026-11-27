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

func TestWordAheadStyle(t *testing.T) {
	tp := &typer{
		currentWordStyle: tcell.StyleDefault.Foreground(tcell.ColorGreen),
		nextWordStyle:    tcell.StyleDefault.Foreground(tcell.ColorYellow),
		defaultStyle:     tcell.StyleDefault,
	}

	// Within the current word every non-space char is the current style.
	for _, wordByWord := range []bool{false, true} {
		tp.WordByWord = wordByWord
		var st tcell.Style
		if got := tp.wordAheadStyle('a', 0, &st); got != 0 || st != tp.currentWordStyle {
			t.Errorf("wordByWord=%v current word: inword=%d style=%v", wordByWord, got, st)
		}
		// After the space the next word is highlighted only when not word-by-word.
		if got := tp.wordAheadStyle(' ', 0, &st); got != 1 {
			t.Errorf("space should advance inword, got %d", got)
		}
		want := tp.nextWordStyle
		if wordByWord {
			want = tp.defaultStyle
		}
		if got := tp.wordAheadStyle('b', 1, &st); got != 1 || st != want {
			t.Errorf("wordByWord=%v next word: inword=%d style=%v want %v", wordByWord, got, st, want)
		}
		// Words beyond the next are never highlighted.
		if got := tp.wordAheadStyle('c', 2, &st); got != 2 || st != tp.defaultStyle {
			t.Errorf("third word should be default: inword=%d style=%v", got, st)
		}
	}
}
