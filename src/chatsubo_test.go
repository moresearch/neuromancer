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
	sim.SetSize(120, 40)

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

	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("keyboard does not fit")
	}
	gapX, split := typ.chatPanelGap(m)
	if !split || m.splitGap < chatsuboBoxWidth+2 {
		t.Fatalf("chatsubo gap: split=%v gap=%d", split, m.splitGap)
	}
	maxHeight := chatsuboMaxKeyRows*m.keyH + (chatsuboMaxKeyRows-1)*m.rowGap + chatsuboExtraRows
	height := maxHeight
	if height < chatsuboCompactHeight {
		height = chatsuboCompactHeight
	}
	if height > m.height {
		height = m.height
	}
	if height > maxHeight {
		t.Errorf("chatsubo height %d exceeds %d key rows", height, chatsuboMaxKeyRows)
	}
	x := gapX + (m.splitGap-chatsuboBoxWidth)/2
	y := m.top + (m.height-height)/2
	cell := cells[y*w+x]
	if len(cell.Runes) == 0 || cell.Runes[0] != '╭' {
		t.Errorf("chatsubo not in the split gap at %d,%d: %q", x, y, string(cell.Runes))
	}
}

func TestChatsuboLayoutSummaryFingers(t *testing.T) {
	got := chatsuboLayoutSummary(nil)
	if !strings.Contains(got, "left index") || !strings.Contains(got, "o=right pinky") {
		t.Errorf("layout summary missing finger map: %q", got)
	}
}
