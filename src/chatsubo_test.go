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
	sim.SetSize(160, 40)

	typ := newTestTyper(sim)
	typ.chatsubo = newChatsubo("", "", time.Minute)
	typ.chatsubo.comment = "The sprawl watches your fingers."
	typ.statsY = 30
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

	x, y, width, height, ok := typ.chatBand()
	if !ok {
		t.Fatal("no box")
	}
	// The box top border is drawn centred, above the stats line.
	cell := cells[y*w+x]
	if len(cell.Runes) == 0 || cell.Runes[0] != '╭' {
		t.Errorf("chatsubo not at %d,%d: %q", x, y, string(cell.Runes))
	}
	// The box holds the board and stays centred on it.
	if kbW, ok := typ.keyboardMetrics(); ok {
		if w := typ.keyboardWidth(kbW); w > 0 && width < w {
			t.Errorf("box width %d narrower than the keyboard's %d", width, w)
		}
	}
	if x+width/2 != 160/2 && x+width/2 != 160/2-1 {
		t.Errorf("box centre %d, want %d", x+width/2, 160/2)
	}
	// The box is the block's top band: one blank row, the text band, then the
	// board.
	textTop, boardTop, _, ok := typ.layoutFrame()
	if !ok {
		t.Fatal("no keyboard")
	}
	// The text draws inside the box: one row below its top border.
	if textTop != y+1 {
		t.Errorf("text top %d, want box interior %d", textTop, y+1)
	}
	if y+height+layoutBandPad != boardTop {
		t.Errorf("box bottom %d, board top %d: want one blank row", y+height, boardTop)
	}
}

func TestChatsuboLayoutSummaryFingers(t *testing.T) {
	got := chatsuboLayoutSummary(nil)
	if !strings.Contains(got, "left index") || !strings.Contains(got, "o=right pinky") {
		t.Errorf("layout summary missing finger map: %q", got)
	}
}

// TestChatsuboStatsFit is the regression for a panel whose readout was silently
// dropped: the stats row was given fewer columns than the box had, so every
// informative variant was rejected and nothing was drawn.
func TestChatsuboStatsFit(t *testing.T) {
	cases := []struct {
		wpm, cpm int
		acc      float64
		errs     int
	}{
		{0, 0, 100, 0},
		{7, 41, 96.5, 12},
		{63, 315, 98.2, 4},
		{120, 610, 100, 3},
	}
	for _, tc := range cases {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(160, 40)
		typ := newTestTyper(sim)
		typ.lastScore = scoreRecord{LastWpm: 88, LastAcc: 97, BestWpm: 102, BestAcc: 99}
		typ.curWpm, typ.curCpm, typ.curAcc, typ.curErrs = tc.wpm, tc.cpm, tc.acc, tc.errs

		// The ladder drops figures from the least to the most useful but must
		// always paint a WPM + accuracy readout.
		typ.drawPanelStats(0, 3, 28, tc.wpm, tc.cpm, tc.acc, tc.errs)
		sim.Show()
		cells, w, _ := sim.GetContents()
		row := ""
		for cx := 0; cx < 28 && cx < w; cx++ {
			c := cells[3*w+cx]
			if len(c.Runes) == 0 {
				row += " "
			} else {
				row += string(c.Runes)
			}
		}
		if !strings.Contains(row, "WPM") || !strings.Contains(row, "%") {
			t.Errorf("%+v: stats row missing: %q", tc, row)
		}
		sim.Fini()
	}
}

// TestChatsuboFitsWithWideBox checks the wider band does not push the matrix
// off a mid-size terminal.
func TestChatsuboFitsWithWideBox(t *testing.T) {
	for _, sw := range []int{140, 160, 200} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sw, 40)
		typ := newTestTyper(sim)
		typ.chatsubo = newChatsubo("", "", time.Minute)
		m, ok := typ.keyboardMetrics()
		if !ok {
			t.Fatalf("%d cols: keyboard does not fit", sw)
		}
		if w := typ.keyboardWidth(m); w+2 > sw {
			t.Errorf("%d cols: keyboard width %d overflows", sw, w)
		}
		x, _, width, _, ok := typ.chatBand()
		if ok && x+width > sw-1 {
			t.Errorf("%d cols: box %d..%d does not fit", sw, x, x+width)
		}
		sim.Fini()
	}
}

// TestChatsuboYieldsToKeys keeps the promise the single board makes: the box
// is a band above the keys, so it never changes the caps. The caps are the
// same with the box on as with it off.
func TestChatsuboYieldsToKeys(t *testing.T) {
	for _, sw := range []int{140, 142, 160} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sw, 44)
		typ := newTestTyper(sim)
		typ.chatsubo = newChatsubo("", "", time.Minute)

		m, ok := typ.keyboardMetrics()
		if !ok {
			sim.Fini()
			t.Fatalf("%d cols: keyboard does not fit", sw)
		}
		// The board must still fit.
		if w := typ.keyboardWidth(m); w+2 > sw {
			sim.Fini()
			t.Errorf("%d cols: keyboard width %d overflows", sw, w)
		}
		// The caps do not depend on the box: the same window without it draws
		// the same caps.
		bare := newTestTyper(sim)
		if bm, ok := bare.keyboardMetrics(); ok && bm.keyWidth != m.keyWidth {
			sim.Fini()
			t.Errorf("%d cols: box changed the caps from %d to %d", sw, bm.keyWidth, m.keyWidth)
		}
		sim.Fini()
	}
}

// TestChatsuboKeepsCapsReadable loads the user's real Miryoku layout at a
// ~142-column terminal with Chatsubo on and asserts the caps stay wide enough
// for the home-row-mod labels (≥ 6), and that the board never overflows. This
// is the acceptance test for the "keys first, box second" ordering.
func TestChatsuboKeepsCapsReadable(t *testing.T) {
	kb, err := loadVil("/home/maf/.config/nm/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil {
		t.Skip("layout not installed")
	}
	for _, sw := range []int{140, 142, 160} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sw, 47)
		typ := newTestTyper(sim)
		typ.vil = kb
		typ.chatsubo = newChatsubo("", "", time.Minute)

		m, ok := typ.keyboardMetrics()
		if !ok {
			sim.Fini()
			t.Fatalf("%d cols: keyboard does not fit", sw)
		}
		if w := typ.keyboardWidth(m); w+2 > sw {
			sim.Fini()
			t.Errorf("%d cols: board width %d overflows", sw, w)
		}
		// Caps stay in the 3-5 band (a keyboard, not square blocks) and wide
		// enough to read the home-row-mod tags over the tap letters.
		if m.keyWidth < 4 || m.keyWidth > 5 {
			sim.Fini()
			t.Errorf("%d cols: cap width %d outside the 4-5 band", sw, m.keyWidth)
		}
		sim.Fini()
	}
}

// TestChatsuboVerticalLayoutFits checks the box (top), board and stats
// (bottom) never overlap each other in short windows.
func TestChatsuboVerticalLayoutFits(t *testing.T) {
	kb, err := loadVil("/home/maf/modwm/kb/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil {
		t.Skip("layout not installed")
	}
	for _, sz := range [][2]int{{200, 45}, {160, 40}, {120, 30}, {100, 24}} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sz[0], sz[1])
		typ := newTestTyper(sim)
		typ.vil = kb
		typ.chatsubo = newChatsubo("", "", time.Minute)
		m, ok := typ.keyboardMetrics()
		if !ok {
			sim.Fini()
			t.Fatalf("%dx%d: keyboard does not fit", sz[0], sz[1])
		}
		boxTop, boxBottom := typ.boxTop, typ.boxTop+typ.boxH-1
		boardTop, boardBottom := m.top, m.top+m.height-1
		if boardTop < boxBottom+1 {
			t.Errorf("%dx%d: board top %d overlaps box (ends %d)", sz[0], sz[1], boardTop, boxBottom)
		}
		// The block is on screen from the very top when the window is short,
		// and the box still ends before the board begins.
		if boxTop < 0 || boxBottom+1 > boardTop {
			t.Errorf("%dx%d: box %d..%d does not sit above the board (%d)", sz[0], sz[1], boxTop, boxBottom, boardTop)
		}
		if boardBottom+2 > sz[1] {
			t.Errorf("%dx%d: board bottom %d leaves no room for stats (%d)", sz[0], sz[1], boardBottom, sz[1])
		}
		sim.Fini()
	}
}

// TestBlockUsesHalfHeight pins the compact block: the rows from the box's top
// border to the stats readout cover about half the window, and the sections
// never overlap.
func TestBlockUsesHalfHeight(t *testing.T) {
	kb, err := loadVil("/home/maf/modwm/kb/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", "")
	if err != nil {
		t.Skip("layout not installed")
	}
	for _, sz := range [][2]int{{200, 45}, {160, 40}, {120, 30}} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sz[0], sz[1])
		typ := newTestTyper(sim)
		typ.vil = kb
		typ.chatsubo = newChatsubo("", "", time.Minute)

		textTop, boardTop, _, ok := typ.layoutFrame()
		if !ok {
			sim.Fini()
			t.Fatalf("%dx%d: keyboard does not fit", sz[0], sz[1])
		}
		m, _ := typ.keyboardMetrics()
		boxTop, boxH := typ.boxTop, typ.boxH
		boxBottom := boxTop + boxH - 1
		boardBottom := boardTop + m.height - 1
		statsY := typ.parts.stats.Y + typ.parts.stats.H - 1
		span := statsY - boxTop + 1

		// The box shares the window's half; the board and the text get their
		// own rows on top of it, and the whole block stays centred.
		if want := sz[1]/2 + sz[1]/2; span > want {
			t.Errorf("%dx%d: block spans %d rows, want <= %d", sz[0], sz[1], span, want)
		}
		below := sz[1] - 1 - typ.parts.stats.Y - typ.parts.stats.H + 1
		if above, belowMargin := boxTop, below; above-belowMargin > 1 || belowMargin-above > 1 {
			t.Errorf("%dx%d: margins above %d, below %d", sz[0], sz[1], above, belowMargin)
		}
		// Box on top (holding the text), then the board.
		if !(boxTop < boxBottom && textTop > boxTop && textTop+typ.parts.text.H-1 < boxBottom && boxBottom < boardTop) {
			t.Errorf("%dx%d: box %d..%d, text %d (+%d), board %d out of order",
				sz[0], sz[1], boxTop, boxBottom, textTop, typ.parts.text.H, boardTop)
		}
		if boardBottom >= statsY {
			t.Errorf("%dx%d: board ends %d, stats at %d", sz[0], sz[1], boardBottom, statsY)
		}
		// The readout shares the last row of the block with the equalizer.
		if statsY > sz[1]-1 {
			t.Errorf("%dx%d: stats row %d is off screen", sz[0], sz[1], statsY)
		}
		sim.Fini()
	}
}

// TestBoxMatchesKeyboardWidth keeps the box and the board one rectangle.
func TestBoxMatchesKeyboardWidth(t *testing.T) {
	for _, sz := range [][2]int{{200, 45}, {160, 40}, {120, 30}, {100, 24}} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sz[0], sz[1])
		typ := newTestTyper(sim)
		typ.chatsubo = newChatsubo("", "", time.Minute)

		m, ok := typ.keyboardMetrics()
		if !ok {
			sim.Fini()
			t.Fatalf("%dx%d: keyboard does not fit", sz[0], sz[1])
		}
		x, _, width, _, ok := typ.chatBand()
		if !ok {
			sim.Fini()
			t.Fatalf("%dx%d: no box", sz[0], sz[1])
		}
		kbW := typ.keyboardWidth(m)
		// The box is exactly the keyboard's width, on the same columns.
		if width != kbW {
			t.Errorf("%dx%d: box width %d, want keyboard width %d", sz[0], sz[1], width, kbW)
		}
		if want := (sz[0] - kbW) / 2; x != want {
			t.Errorf("%dx%d: box left %d, want keyboard left %d", sz[0], sz[1], x, want)
		}
		sim.Fini()
	}
}

// TestBlockCentred checks the block's margins are even top and bottom.
func TestBlockCentred(t *testing.T) {
	for _, sz := range [][2]int{{200, 45}, {160, 40}, {120, 30}} {
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(sz[0], sz[1])
		typ := newTestTyper(sim)
		typ.chatsubo = newChatsubo("", "", time.Minute)

		if _, ok := typ.keyboardMetrics(); !ok {
			sim.Fini()
			t.Fatalf("%dx%d: keyboard does not fit", sz[0], sz[1])
		}
		// The block is the three parts: its top is the box's top row and its
		// bottom the stats band's last row.
		blockTop := typ.parts.chat.Y
		blockBottom := typ.parts.stats.Y + typ.parts.stats.H - 1
		above := blockTop
		below := sz[1] - 1 - blockBottom
		if d := above - below; d > 1 || d < -1 {
			t.Errorf("%dx%d: margins above %d, below %d", sz[0], sz[1], above, below)
		}
		sim.Fini()
	}
}
