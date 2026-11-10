package main

import (
	"fmt"
	"sort"
)

// Finger codes used by Miryoku-style layouts: the number runs from the index
// finger outward, so l1 is the left index, l4 the left pinky (and mirrored on
// the right). Thumbs are lt/rt.
var fingerNames = map[string]string{
	"l1":    "left index finger",
	"l2":    "left middle finger",
	"l3":    "left ring finger",
	"l4":    "left pinky",
	"r1":    "right index finger",
	"r2":    "right middle finger",
	"r3":    "right ring finger",
	"r4":    "right pinky",
	"lt":    "left thumb",
	"rt":    "right thumb",
	"lt/rt": "either thumb",
}

const fingerLegend = "Use plain Miryoku/Colemak-DH finger language: left pinky, left ring, left middle, left index, left thumb; right thumb, right index, right middle, right ring, right pinky. Home columns: a=left pinky, r=left ring, s=left middle, t=left index; n=right index, e=right middle, i=right ring, o=right pinky. Talk about the home row, inner and outer column reaches, home-row mods, thumb keys, and layers."

// Colemak-DH/Miryoku names the finger by the home key it rests on.
var homeFinger = map[rune]string{
	'a': "l4",
	'r': "l3",
	's': "l2",
	't': "l1",
	'n': "r1",
	'e': "r2",
	'i': "r3",
	'o': "r4",
}

func fingerLabel(code string) string {
	if code == "" {
		return ""
	}
	if name, ok := fingerNames[code]; ok {
		return name
	}
	return code
}

func kbBuiltinFinger(homeCol int) string {
	switch homeCol {
	case 0:
		return "l4"
	case 1:
		return "l3"
	case 2:
		return "l2"
	case 3:
		return "l1"
	case 6:
		return "r1"
	case 7:
		return "r2"
	case 8:
		return "r3"
	case 9:
		return "r4"
	}
	return ""
}

// fingerForHome names the finger which rests on a home key. On the left the
// homes run pinky->index left to right, so the leftmost column is l4; on the
// right they run index->pinky, so the leftmost column is r1.
func (kb *vilKeyboard) fingerForHome(row, col int) string {
	if kb.thumbLeftOK && row == kb.thumbLeftRow && col == kb.thumbLeftCol {
		return "lt"
	}
	if kb.thumbRightOK && row == kb.thumbRightRow && col == kb.thumbRightCol {
		return "rt"
	}

	if a := kb.actionAt(0, row, col); homeFinger[a.output] != "" {
		return homeFinger[a.output]
	}

	side := kb.sideFor(row, col)
	var homes []homePos
	switch side {
	case -1:
		homes = append(homes, kb.homeLeft...)
	case 1:
		homes = append(homes, kb.homeRight...)
	default:
		return ""
	}
	if len(homes) == 0 {
		return ""
	}
	sort.Slice(homes, func(i, j int) bool {
		if homes[i].col == homes[j].col {
			return homes[i].row < homes[j].row
		}
		return homes[i].col < homes[j].col
	})

	idx := -1
	for i, h := range homes {
		if h.row == row && h.col == col {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ""
	}

	n := idx + 1
	if side == -1 {
		n = len(homes) - idx
	}
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	if side == -1 {
		return fmt.Sprintf("l%d", n)
	}
	return fmt.Sprintf("r%d", n)
}

// fingerForTarget returns the finger which should press target.
func (t *typer) fingerForTarget(target rune) string {
	if target == 0 || target == '\n' {
		return ""
	}
	if t.vil != nil {
		hint, found := t.vil.findRune(target)
		if !found {
			return ""
		}
		row, col, ok := t.vil.targetHome(hint.row, hint.col)
		if !ok {
			return ""
		}
		return t.vil.fingerForHome(row, col)
	}

	if target == ' ' {
		return "lt/rt"
	}
	row, col, ok := kbFind(target)
	if !ok {
		return ""
	}
	_, homeCol, ok := kbBuiltinTargetHome(row, col)
	if !ok {
		return ""
	}
	return kbBuiltinFinger(homeCol)
}

// withFinger appends the ergonomic finger name to a guidance string.
func (t *typer) withFinger(hint string, target rune) string {
	if hint == "" {
		return ""
	}
	finger := t.fingerForTarget(target)
	if finger == "" {
		return hint
	}
	return hint + " (" + fingerLabel(finger) + ")"
}
