package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

func TestRecentTyped(t *testing.T) {
	typed := []rune{'t', 'h', 0, 'e', '\n', 's', 'p', 'r', 'a', 'w', 'l'}
	if got := recentTyped(typed, 6); got != "sprawl" {
		t.Fatalf("recentTyped = %q", got)
	}
}

func TestNewChatsuboInterval(t *testing.T) {
	c := newChatsubo("", "", time.Second)
	if c.interval < 5*time.Second {
		t.Fatalf("interval = %v", c.interval)
	}
}

func TestDrawChatsubo(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 40)

	typ := newTestTyper(sim)
	typ.chatsubo = newChatsubo("", "", time.Minute)
	typ.chatsubo.comment = "The sprawl watches your fingers."
	typ.drawChatsubo()
	sim.Show()

	cells, w, h := sim.GetContents()
	var text strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				text.WriteRune(' ')
			} else {
				text.WriteRune(c.Runes[0])
			}
		}
		text.WriteRune('\n')
	}

	if !strings.Contains(text.String(), "chatsubo") {
		t.Error("chatsubo title missing")
	}
	if !strings.Contains(text.String(), "The sprawl watches") {
		t.Error("chatsubo comment missing")
	}
}
