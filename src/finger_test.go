package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell"
)

func TestBuiltinFingers(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()

	typ := newTestTyper(scr)
	cases := map[rune]string{
		'a':  "l4", // left pinky home
		'r':  "l3",
		's':  "l2",
		'f':  "l2", // above s
		't':  "l1",
		'g':  "l1", // inner index column
		'b':  "l1", // outer index column
		'n':  "r1",
		'm':  "r1", // inner index column
		'e':  "r2",
		'i':  "r3",
		'o':  "r4",
		'\'': "r4", // outer pinky column
	}
	for r, want := range cases {
		if got := typ.fingerForTarget(r); got != want {
			t.Errorf("fingerForTarget(%q) = %q, want %q", r, got, want)
		}
	}
	if got := typ.fingerForTarget(' '); got != "lt/rt" {
		t.Errorf("space finger = %q, want lt/rt", got)
	}
	if got := typ.hintFor('f'); !strings.Contains(got, "(left middle finger)") {
		t.Errorf("hintFor(f) = %q, want l2 finger", got)
	}
}

func TestVilFingers(t *testing.T) {
	kb := loadTestVil(t, testVil)
	if got := kb.fingerForHome(1, 0); got != "l4" {
		t.Errorf("home a finger = %q, want l4", got)
	}

	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	typ := newTestTyper(scr)
	typ.vil = kb
	if got := typ.fingerForTarget('a'); got != "l4" {
		t.Errorf("target a finger = %q, want l4", got)
	}
	if got := typ.hintFor('a'); !strings.Contains(got, "(left pinky)") {
		t.Errorf("hintFor(a) = %q, want l4 finger", got)
	}
}

func TestFingerLabel(t *testing.T) {
	if got := fingerLabel("l2"); got != "left middle finger" {
		t.Errorf("fingerLabel = %q", got)
	}
	if got := fingerLabel("rt"); got != "right thumb" {
		t.Errorf("fingerLabel = %q", got)
	}
}
