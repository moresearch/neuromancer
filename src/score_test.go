package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell"
)

func TestRecordScore(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".scores")

	s := recordScore(path, "time:60s", 80, 97)
	if s.LastWpm != 80 || s.BestWpm != 80 || s.BestAcc != 97 {
		t.Fatalf("first run: got %+v", s)
	}

	s = recordScore(path, "time:60s", 60, 99)
	if s.LastWpm != 60 || s.BestWpm != 80 {
		t.Fatalf("slower run must update last, not best: got %+v", s)
	}

	s = recordScore(path, "time:60s", 90, 90)
	if s.LastWpm != 90 || s.BestWpm != 90 || s.BestAcc != 90 {
		t.Fatalf("faster run must become best: got %+v", s)
	}

	// A tie on WPM is decided by accuracy.
	s = recordScore(path, "time:60s", 90, 99)
	if s.BestAcc != 99 {
		t.Fatalf("accuracy tie-break: got %+v", s)
	}

	// A different mode keeps its own record.
	s = recordScore(path, "words:50", 40, 95)
	if s.LastWpm != 40 || s.BestWpm != 40 || s.BestAcc != 95 {
		t.Fatalf("new mode must not inherit other mode's best: got %+v", s)
	}

	s = loadScore(path, "time:60s")
	if s.BestWpm != 90 || s.BestAcc != 99 || s.LastWpm != 90 {
		t.Fatalf("time mode record lost: got %+v", s)
	}
}

func screenText(scr tcell.SimulationScreen) string {
	scr.Show()
	cells, w, h := scr.GetContents()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		line := make([]rune, w)
		for x := 0; x < w; x++ {
			mainc := cells[y*w+x].Runes
			c := ' '
			if len(mainc) > 0 {
				c = mainc[0]
			}
			line[x] = c
		}
		rows[y] = string(line)
	}
	return strings.Join(rows, "\n")
}

func TestStatsShowLastAndBest(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(120, 35)

	ty := newTestTyper(scr)
	ty.lastScore = scoreRecord{LastWpm: 42, LastAcc: 96, BestWpm: 88, BestAcc: 98}
	ty.bestWpm, ty.bestAcc = 88, 98
	ty.curWpm = 50

	ty.drawLiveStats(60, 3, 50, 250, 97, 2)
	if got := screenText(scr); !strings.Contains(got, "LAST 42%96") || !strings.Contains(got, "BEST 88%98") {
		t.Fatalf("HUD missing last/best:\n%s", got)
	}

	scr.Clear()
	// Narrow panel: errors must give way so LAST/BEST still show.
	ty.drawPanelStats(2, 5, 50, 50, 250, 97, 2)
	if got := screenText(scr); !strings.Contains(got, "LAST 42%96") || !strings.Contains(got, "BEST 88%98") {
		t.Fatalf("panel missing last/best:\n%s", got)
	}
}
