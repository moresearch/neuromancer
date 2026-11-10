package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell"
)

var CONFIG_DIRS []string

func init() {
	home, _ := os.LookupEnv("HOME")

	CONFIG_DIRS = []string{
		filepath.Join(home, ".nm"),
		"/etc/nm",
	}
}

func getParagraphs(s string) []string {
	s = strings.Replace(s, "\r", "", -1)
	s = regexp.MustCompile("\n\n+").ReplaceAllString(s, "\n\n")
	return strings.Split(strings.Trim(s, "\n"), "\n\n")
}

// wordWrap breaks s into lines of at most n characters. It prefers to break at
// spaces, but long tokens without spaces are hard-wrapped so a line can never
// exceed n. Newlines in the input are treated as spaces.
func wordWrap(s string, n int) string {
	if n < 1 {
		n = 1
	}

	var b strings.Builder
	line := make([]rune, 0, n)
	flush := func() {
		b.WriteString(string(line))
		b.WriteByte('\n')
		line = line[:0]
	}

	for _, r := range strings.ReplaceAll(s, "\n", " ") {
		if len(line) < n {
			line = append(line, r)
			continue
		}

		// The line is full. A space at this point is simply replaced by the
		// line break.
		if r == ' ' {
			flush()
			continue
		}
		if line[len(line)-1] == ' ' {
			b.WriteString(string(line[:len(line)-1]))
			b.WriteByte('\n')
			line = line[:0]
			line = append(line, r)
			continue
		}

		lastSpace := -1
		for i := len(line) - 1; i >= 0; i-- {
			if line[i] == ' ' {
				lastSpace = i
				break
			}
		}
		if lastSpace > 0 {
			b.WriteString(string(line[:lastSpace]))
			b.WriteByte('\n')
			rest := append([]rune(nil), line[lastSpace+1:]...)
			line = append(rest, r)
		} else {
			// A single token is longer than the line: hard-wrap it.
			flush()
			line = append(line, r)
		}
	}

	b.WriteString(string(line))
	return b.String()
}

var whitespaceRegexp = regexp.MustCompile("\\s+")

// trimText wraps s to width and keeps at most maxLines lines. The final line is
// cut at a word boundary produced by wordWrap. A space is inserted before each
// line break so the typer treats wrapped lines as part of the same text.
func trimText(s string, width, maxLines int) string {
	if width < 1 {
		width = 1
	}
	if maxLines < 1 {
		maxLines = 1
	}

	s = whitespaceRegexp.ReplaceAllString(s, " ")
	wrapped := wordWrap(strings.TrimSpace(s), width)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Replace(strings.Join(lines, "\n"), "\n", " \n", -1)
}

func init() {
	rand.Seed(time.Now().Unix())
}

func randomText(n int, words []string) string {
	if len(words) == 0 {
		return ""
	}
	if len(words) == 1 {
		r := strings.TrimSpace(strings.Repeat(words[0]+" ", n))
		return strings.Replace(r, "\n", " \n", -1)
	}

	r := ""

	var last string
	for i := 0; i < n; i++ {
		w := words[rand.Int()%len(words)]
		for last == w {
			w = words[rand.Int()%len(words)]
		}

		r += w
		if i != n-1 {
			r += " "
		}

		last = w
	}

	return strings.Replace(r, "\n", " \n", -1)
}

func drawString(scr tcell.Screen, x, y int, s string, cursorIdx int, style tcell.Style) {
	sx := x

	for i, c := range s {
		if c == '\n' {
			y++
			x = sx
		} else {
			scr.SetContent(x, y, c, nil, style)
			if i == cursorIdx {
				scr.ShowCursor(x, y)
			}

			x++
		}
	}

	if cursorIdx == len(s) {
		scr.ShowCursor(x, y)
	}
}

func drawStringAtCenter(scr tcell.Screen, s string, style tcell.Style) {
	nc, nr := calcStringDimensions(s)
	sw, sh := scr.Size()

	x := (sw - nc) / 2
	y := (sh - nr) / 2

	drawString(scr, x, y, s, -1, style)
}

func calcStringDimensions(s string) (nc, nr int) {
	if s == "" {
		return 0, 0
	}

	c := 0

	for _, x := range s {
		if x == '\n' {
			nr++
			if c > nc {
				nc = c
			}
			c = 0
		} else {
			c++
		}
	}

	nr++
	if c > nc {
		nc = c
	}

	return
}

func newTcellColor(s string) (tcell.Color, error) {
	if len(s) != 7 || s[0] != '#' {
		return 0, fmt.Errorf("%s is not a valid hex color", s)
	}

	tonum := func(c byte) int32 {
		if c > '9' {
			if c >= 'a' {
				return (int32)(c - 'a' + 10)
			} else {
				return (int32)(c - 'A' + 10)
			}
		} else {
			return (int32)(c - '0')
		}
	}

	r := tonum(s[1])<<4 | tonum(s[2])
	g := tonum(s[3])<<4 | tonum(s[4])
	b := tonum(s[5])<<4 | tonum(s[6])

	return tcell.NewRGBColor(r, g, b), nil
}

func readResource(typ, name string) []byte {
	if name == "-" {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}

		return b
	}

	if b, err := ioutil.ReadFile(name); err == nil {
		return b
	}

	for _, d := range CONFIG_DIRS {
		if b, err := ioutil.ReadFile(filepath.Join(d, typ, name)); err == nil {
			return b
		}
	}

	return readPackedFile(filepath.Join(typ, name))
}
