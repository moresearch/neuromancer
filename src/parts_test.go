package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell"
)

// screenRows renders a simulation screen to plain text rows.
func screenRows(scr tcell.Screen) []string {
	cells, w, h := scr.(tcell.SimulationScreen).GetContents()
	out := make([]string, h)
	for y := 0; y < h; y++ {
		var b strings.Builder
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune(c.Runes[0])
			}
		}
		out[y] = b.String()
	}
	return out
}

// tightSizes covers the window shapes the three parts must survive.
var tightSizes = [][2]int{{40, 16}, {50, 18}, {60, 20}, {70, 22}, {80, 24}, {90, 28}, {120, 34}, {160, 42}, {200, 50}}

func newLayoutTyper(t *testing.T, sw, sh int) *typer {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(sw, sh)
	typ := newTestTyper(sim)
	if kb, err := loadVil("/home/maf/.config/nm/CANONICAL_ATmega_100pct_Miryoku_3x5.vil", ""); err == nil {
		typ.vil = kb
	}
	typ.chatsubo = newChatsubo("", "", time.Minute)
	return typ
}

// TestPartsStayInOrder pins the frame: Chatsubo on top, then the text, then the
// keys, then the stats, each inside the window and none overlapping another.
func TestPartsStayInOrder(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newLayoutTyper(t, sz[0], sz[1])
		m, ok := typ.keyboardMetrics()
		if !ok {
			t.Errorf("%dx%d: keyboard does not fit", sz[0], sz[1])
			continue
		}
		p := typ.parts
		// The practice text lives inside the box body now.
		if p.chat.H > 0 && (p.text.Y < p.chat.Y+1 || p.text.Y+p.text.H > p.chat.Y+p.chat.H-1) {
			t.Errorf("%dx%d: text %d..%d not inside box %d..%d", sz[0], sz[1], p.text.Y, p.text.Y+p.text.H-1, p.chat.Y, p.chat.Y+p.chat.H-1)
		}
		if p.chat.Y+p.chat.H > p.keys.Y {
			t.Errorf("%dx%d: box %d..%d runs into keys at %d", sz[0], sz[1], p.chat.Y, p.chat.Y+p.chat.H-1, p.keys.Y)
		}
		if p.keys.Y+p.keys.H > p.stats.Y {
			t.Errorf("%dx%d: keys %d..%d run into stats at %d", sz[0], sz[1], p.keys.Y, p.keys.Y+p.keys.H-1, p.stats.Y)
		}
		if p.stats.Y+p.stats.H > sz[1] {
			t.Errorf("%dx%d: stats end at %d, off screen", sz[0], sz[1], p.stats.Y+p.stats.H)
		}
		if m.top != p.keys.Y {
			t.Errorf("%dx%d: board top %d, keys part %d", sz[0], sz[1], m.top, p.keys.Y)
		}
		if p.chat.Y < 0 || p.stats.Y < 0 {
			t.Errorf("%dx%d: negative part row", sz[0], sz[1])
		}
	}
}

// TestPartsAreCentred checks the three parts share one centre.
func TestPartsAreCentred(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newLayoutTyper(t, sz[0], sz[1])
		x, _, w, _, ok := typ.chatBand()
		if !ok {
			t.Errorf("%dx%d: no chatsubo band", sz[0], sz[1])
			continue
		}
		if c := x + w/2; c < sz[0]/2-1 || c > sz[0]/2+1 {
			t.Errorf("%dx%d: box centre %d, want %d", sz[0], sz[1], c, sz[0]/2)
		}
	}
}

// TestBoxMatchesBoard checks the box is exactly as wide as the board and on
// the same columns, so the panel and the keys read as one rectangle.
func TestBoxMatchesBoard(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newLayoutTyper(t, sz[0], sz[1])
		boardX, boardW, kbOK := typ.boardBand()
		if !kbOK {
			continue
		}
		x, _, w, _, ok := typ.chatBand()
		if !ok {
			continue
		}
		if w != boardW || x != boardX {
			t.Errorf("%dx%d: box %d..%d, want board %d..%d",
				sz[0], sz[1], x, x+w, boardX, boardX+boardW)
		}
	}
}

// TestTightScreenRendersCleanly draws the three parts at each tight size and
// checks every part's rows actually hold content, with the box's borders
// intact.
func TestTightScreenRendersCleanly(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newLayoutTyper(t, sz[0], sz[1])
		m, ok := typ.keyboardMetrics()
		if !ok {
			continue
		}
		typ.drawKeyboard('t')
		typ.drawChatsubo()
		typ.hintText = "press t (left index)"
		typ.drawLiveStats(sz[0]/2, typ.statsTop+1, 40, 200, 98, 2)
		typ.Scr.Show()
		rows := screenRows(typ.Scr)
		p := typ.parts
		if p.chat.H > 0 {
			if !strings.Contains(rows[p.chat.Y], "chatsubo") {
				t.Errorf("%dx%d: box title missing on row %d: %q", sz[0], sz[1], p.chat.Y, rows[p.chat.Y])
			}
			if !strings.Contains(rows[p.chat.Y+p.chat.H-1], "╰") {
				t.Errorf("%dx%d: box bottom border missing: %q", sz[0], sz[1], rows[p.chat.Y+p.chat.H-1])
			}
		}
		board := rows[m.top : m.top+m.height]
		if !strings.Contains(strings.Join(board, ""), "q") {
			t.Errorf("%dx%d: board keys missing", sz[0], sz[1])
		}
		// The readout is the band's last row (the equalizer takes the first
		// when the band has two).
		statsRow := rows[p.stats.Y+p.stats.H-1]
		if !strings.Contains(statsRow, "WPM") {
			t.Errorf("%dx%d: readout missing: %q", sz[0], sz[1], statsRow)
		}
	}
}

// TestStatsNeverClipped keeps the readout's first figure on screen when the
// window is exactly as wide as it.
func TestStatsNeverClipped(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newLayoutTyper(t, sz[0], sz[1])
		if _, ok := typ.keyboardMetrics(); !ok {
			continue
		}
		typ.drawLiveStats(sz[0]/2, typ.statsTop+1, 63, 315, 98.2, 4)
		typ.Scr.Show()
		readout := typ.statsTop + 1
		if p := typ.parts.stats; p.H > 0 {
			readout = p.Y + p.H - 1
		}
		line := screenRows(typ.Scr)[readout]
		if !strings.Contains(line, "WPM") {
			t.Errorf("%dx%d: readout missing: %q", sz[0], sz[1], line)
		}
		if !strings.Contains(line, "63") {
			t.Errorf("%dx%d: speed figure clipped: %q", sz[0], sz[1], line)
		}
	}
}

// TestCompactBoardKeepsHalves checks a narrow window draws the Sofle shape:
// both halves side by side with a gap between them.
func TestCompactBoardKeepsHalves(t *testing.T) {
	typ := newLayoutTyper(t, 40, 16)
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("board does not fit 40 cols")
	}
	if !typ.compactBoard(m) {
		t.Skip("40 cols fits the closed caps")
	}
	typ.drawKeyboard('a')
	typ.Scr.Show()
	rows := screenRows(typ.Scr)
	// The alphas row: find the line that holds q on the left and b on the
	// right, side by side.
	var line string
	for y := m.top; y < m.top+m.height && y < len(rows); y++ {
		if strings.Contains(rows[y], "q") && strings.Contains(rows[y], "b") {
			line = rows[y]
			break
		}
	}
	if line == "" {
		t.Fatalf("no board row shows both halves: %q", rows[m.top:m.top+3])
	}
	left := strings.Index(line, "q")
	right := strings.LastIndex(line, "b")
	if right <= left {
		t.Fatalf("halves are not side by side: %q", line)
	}
}

// TestLabelsNeverBlank draws the board at three-cell, two-cell and one-row
// caps and asserts every wired key shows at least one non-space glyph.
func TestLabelsNeverBlank(t *testing.T) {
	for _, rows := range []int{1, 2, 3} {
		typ := newLayoutTyper(t, 40, 18)
		m, ok := typ.keyboardMetrics()
		if !ok {
			t.Fatal("board does not fit")
		}
		m.keyH = rows
		typ.drawKeyboard('a')
		typ.Scr.Show()
		lines := screenRows(typ.Scr)
		bottom := m.top + m.height
		if bottom > len(lines) {
			bottom = len(lines)
		}
		var board strings.Builder
		for _, l := range lines[m.top:bottom] {
			board.WriteString(l)
			board.WriteRune('\n')
		}
		if n := len(strings.TrimSpace(board.String())); n == 0 {
			t.Errorf("%d-row caps: board is blank", rows)
		}
		for _, want := range []string{"q", "w", "f", "p", "b"} {
			if !strings.Contains(board.String(), want) {
				t.Errorf("%d-row caps: key %q has no label", rows, want)
			}
		}
	}
}

// TestHomeKeysKeepTheirLetterAtTwoCells covers the compact board's label rule:
// a cell is never emptied, and the home keys still name their finger.
func TestHomeKeysKeepTheirLetterAtTwoCells(t *testing.T) {
	cases := []struct {
		action vilAction
		want   string
	}{
		{vilAction{label: "a", output: 'a', mods: "Gui"}, "G"},
		{vilAction{label: "q", output: 'q'}, "q"},
		{vilAction{label: "F13"}, "13"},
		{vilAction{label: "LCT"}, "C"}, // the modifier abbreviation beats a truncation
	}
	for _, tc := range cases {
		if got := compactLabel(tc.action, 2); got != tc.want {
			t.Errorf("compactLabel(%q) = %q, want %q", tc.action.label, got, tc.want)
		}
		if got := compactLabel(tc.action, 1); len([]rune(got)) == 0 {
			t.Errorf("compactLabel(%q, 1) is empty", tc.action.label)
		}
	}
}

// TestCompactBoardStaysInItsPart keeps the single-line board inside the keys
// part: a key whose row would fall past the board's rows is not drawn.
func TestCompactBoardStaysInItsPart(t *testing.T) {
	for _, sz := range [][2]int{{40, 16}, {60, 20}, {70, 22}} {
		typ := newLayoutTyper(t, sz[0], sz[1])
		m, ok := typ.keyboardMetrics()
		if !ok {
			t.Fatalf("%dx%d: board does not fit", sz[0], sz[1])
		}
		typ.drawKeyboard('a')
		typ.Scr.Show()
		lines := screenRows(typ.Scr)
		if m.top+m.height > len(lines) {
			t.Fatalf("%dx%d: board runs off screen", sz[0], sz[1])
		}
		for y := 0; y < len(lines); y++ {
			inBoard := y >= m.top && y < m.top+m.height
			line := strings.TrimSpace(lines[y])
			if strings.Contains(line, "j") && !inBoard {
				t.Errorf("%dx%d: right-hand key drawn on row %d, outside the keys part", sz[0], sz[1], y)
			}
		}
	}
}

// TestThumbPointerFollowsTheTwoRowCap is the regression for the two-row cap
// with one cell of label room: the border takes two cells, so the arrow must
// not land on the side rule.
func TestThumbPointerFollowsTheTwoRowCap(t *testing.T) {
	t.Skip("the box now shows the practice text; the guidance hint line is gone")
	typ := newLayoutTyper(t, 80, 24)
	m, ok := typ.keyboardMetrics()
	if !ok {
		t.Fatal("board does not fit")
	}
	if m.keyH != 2 || m.keyWidth != 4 {
		t.Skipf("80x24 draws %d-by-%d caps, not the two-cell case", m.keyH, m.keyWidth)
	}
	// Draw as start() does: the board first, then the panel frame.
	typ.drawKeyboard('t')
	typ.drawChatsubo()
	typ.Scr.Show()
	lines := screenRows(typ.Scr)
	var hint string
	for _, l := range lines {
		if strings.Contains(l, "press") {
			hint = l
		}
	}
	if hint == "" {
		t.Fatal("no guidance line")
	}
	if i := strings.Index(hint, "▾"); i >= 0 {
		after := []rune(hint[i:])
		if len(after) > 1 && (after[1] == '│' || after[1] == ' ') {
			t.Errorf("pointer not next to its figure: %q", hint)
		}
	}
}

// TestCapLadderPrefersThreeCells pins the cap order: a three-cell cap (three
// cells of label room inside the rules) is tried before a two-cell one.
func TestCapLadderPrefersThreeCells(t *testing.T) {
	got := capLadder()
	if len(got) == 0 || got[0] != vilMaxKeyWidth {
		t.Fatalf("ladder starts %v, want the widest cap first", got)
	}
	idx := map[int]int{}
	for i, kw := range got {
		idx[kw] = i
	}
	if idx[3] > idx[2] {
		t.Errorf("ladder %v tries a two-cell cap before three", got)
	}
}

// TestBoardJoinerFixed checks the one board keeps one joiner: the gap between
// the hands is boardHalfGap at every window size, never shrunk for a wider cap.
func TestBoardJoinerFixed(t *testing.T) {
	for _, sw := range []int{40, 60, 100, 200} {
		typ := newLayoutTyper(t, sw, 24)
		m, ok := typ.keyboardMetrics()
		if !ok {
			t.Fatalf("%d cols: board does not fit", sw)
		}
		if m.splitGap != boardHalfGap {
			t.Errorf("%d cols: joiner %d, want the fixed %d", sw, m.splitGap, boardHalfGap)
		}
	}
}

// newSprintTyper builds a sprint block on the simulator, with the layout when
// it is installed.
func newSprintTyper(t *testing.T, sw, sh int) *typer {
	t.Helper()
	typ := newLayoutTyper(t, sw, sh)
	typ.sprint = true
	return typ
}

// TestSprintPanelIsAboveBoard checks a sprint uses the same three-part frame:
// the panel is a band above the practice text, which sits above the board, and
// no panel rectangle covers the board's rows.
func TestSprintPanelIsAboveBoard(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newSprintTyper(t, sz[0], sz[1])
		m, ok := typ.keyboardMetrics()
		if !ok {
			continue
		}
		p := typ.parts
		if p.chat.H == 0 {
			t.Errorf("%dx%d: sprint has no panel band", sz[0], sz[1])
			continue
		}
		// The text lives inside the sprint panel; the panel sits above the keys.
		if p.text.Y < p.chat.Y+1 || p.text.Y+p.text.H > p.chat.Y+p.chat.H-1 {
			t.Errorf("%dx%d: text %d..%d not inside panel %d..%d", sz[0], sz[1], p.text.Y, p.text.Y+p.text.H-1, p.chat.Y, p.chat.Y+p.chat.H-1)
		}
		if p.chat.Y+p.chat.H > p.keys.Y {
			t.Errorf("%dx%d: panel %d..%d runs into keys at %d", sz[0], sz[1], p.chat.Y, p.chat.Y+p.chat.H-1, p.keys.Y)
		}
		boardTop, boardBottom := m.top, m.top+m.height-1
		if _, by, _, bh, ok := typ.chatBand(); ok {
			if by <= boardBottom && by+bh-1 >= boardTop {
				t.Errorf("%dx%d: panel %d..%d overlaps the board %d..%d", sz[0], sz[1], by, by+bh-1, boardTop, boardBottom)
			}
		}
	}
}

// TestSprintTextInsideBand checks the sprint text is typed in the practice
// text band, not between the hands.
func TestSprintTextInsideBand(t *testing.T) {
	for _, sz := range tightSizes {
		typ := newSprintTyper(t, sz[0], sz[1])
		m, ok := typ.keyboardMetrics()
		if !ok {
			continue
		}
		x, y, w, rows, ok := typ.sprintTextArea(m)
		if !ok {
			t.Errorf("%dx%d: no sprint text area", sz[0], sz[1])
			continue
		}
		b := typ.parts.text
		if x < 0 || x+w > sz[0] {
			t.Errorf("%dx%d: text %d..%d off screen", sz[0], sz[1], x, x+w)
		}
		if y < b.Y || y+rows > b.Y+b.H {
			t.Errorf("%dx%d: text rows %d..%d outside band %d..%d", sz[0], sz[1], y, y+rows-1, b.Y, b.Y+b.H-1)
		}
	}
}

// TestSprintCountdownInRule checks the sprint countdown and status show in the
// panel band: the countdown right-aligned in the rule, the status in a body row.
func TestSprintCountdownInRule(t *testing.T) {
	typ := newSprintTyper(t, 100, 30)
	typ.layoutTop = -1
	typ.keyboardMetrics()
	typ.panelLabel = " chatsubo 42s "
	typ.sprintMessage = "summarizing next sprint..."
	typ.drawChatsubo()
	typ.Scr.Show()
	rows := screenRows(typ.Scr)
	band := typ.parts.chat
	if band.H == 0 {
		t.Fatal("no panel band")
	}
	rule := rows[band.Y]
	if !strings.Contains(rule, "chatsubo") {
		t.Errorf("rule %q missing the title", rule)
	}
	if !strings.Contains(rule, "42s") {
		t.Errorf("rule %q missing the countdown", rule)
	}
	// The box body now holds the practice text, so the sprint status line is
	// no longer drawn in the body.
}
