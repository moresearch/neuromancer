package main

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestBookParagraphsSkipsHeadings(t *testing.T) {
	text := "1\n\nA short line.\n\n" + strings.Repeat("the sky above the port was the color of television ", 3)
	ps := bookParagraphs(text)
	if len(ps) != 1 {
		t.Fatalf("paragraphs = %d, want 1: %v", len(ps), ps)
	}
	if len([]rune(ps[0])) < 80 {
		t.Fatalf("paragraph too short: %d", len([]rune(ps[0])))
	}
}

func TestGenerateBookTest(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("the sky above the port was the color of television ", 4)
	if err := ioutil.WriteFile(filepath.Join(dir, "one.txt"), []byte("heading\n\n"+long), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "two.md"), []byte("other\n\n"+long), 0644); err != nil {
		t.Fatal(err)
	}

	gen := generateBookTest(dir)
	segs := gen()
	if len(segs) != 1 {
		t.Fatalf("segments = %d", len(segs))
	}
	if len([]rune(segs[0].Text)) < 80 {
		t.Fatalf("book text too short: %q", segs[0].Text)
	}
}
