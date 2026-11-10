package main

import (
	"fmt"
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

func TestClipTypingText(t *testing.T) {
	if got := clipTypingText("the sky above the port", 120); got != "the sky above the port." {
		t.Errorf("clipTypingText = %q", got)
	}
	got := clipTypingText("the sky above the port was the color of television tuned to a dead channel", 30)
	if len([]rune(got)) > 30 || !strings.HasSuffix(got, ".") {
		t.Errorf("clipped text = %q (%d runes)", got, len([]rune(got)))
	}
}

func TestClipTypingWords(t *testing.T) {
	got := clipTypingWords("one two three four five six", 4)
	if got != "one two three four." {
		t.Errorf("clipTypingWords = %q", got)
	}
	if words := len(strings.Fields(got)); words != 4 {
		t.Errorf("clipTypingWords words = %d, want 4", words)
	}
}

func TestGenerateBookSummaryTest(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("the sky above the port was the color of television ", 4)
	if err := ioutil.WriteFile(filepath.Join(dir, "one.txt"), []byte(long), 0644); err != nil {
		t.Fatal(err)
	}

	gen := generateBookSummaryTest(dir, func(string) (string, error) {
		return "Case watched the sky above the port.", nil
	})
	segs := gen()
	if len(segs) != 1 || segs[0].Text != "Case watched the sky above the port." {
		t.Fatalf("summary segments = %+v", segs)
	}
}

func TestGenerateBookSummaryFallback(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("the sky above the port was the color of television ", 4)
	if err := ioutil.WriteFile(filepath.Join(dir, "one.txt"), []byte(long), 0644); err != nil {
		t.Fatal(err)
	}

	gen := generateBookSummaryTest(dir, func(string) (string, error) {
		return "", fmt.Errorf("ollama down")
	})
	segs := gen()
	if len(segs) != 1 {
		t.Fatalf("segments = %d", len(segs))
	}
	if got := len([]rune(segs[0].Text)); got > bookSummaryMaxRunes+1 {
		t.Errorf("fallback text too long: %d runes", got)
	}
	if got := len(strings.Fields(segs[0].Text)); got > bookSummaryMaxWords {
		t.Errorf("fallback text too many words: %d", got)
	}
}

func TestBookSummaryQueue(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("the sky above the port was the color of television ", 4)
	if err := ioutil.WriteFile(filepath.Join(dir, "one.txt"), []byte(long), 0644); err != nil {
		t.Fatal(err)
	}

	q, err := newBookSummaryQueue(dir, 2, func(string) (string, error) {
		return "Case watched the sky.", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	q.prefill(nil)
	if !q.ready() {
		t.Fatal("queue is empty after prefill")
	}
	segs := q.next()
	if len(segs) != 1 || segs[0].Text != "Case watched the sky." {
		t.Fatalf("queue segment = %+v", segs)
	}
}

func TestBookSummaryQueueFallback(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("the sky above the port was the color of television ", 4)
	if err := ioutil.WriteFile(filepath.Join(dir, "one.txt"), []byte(long), 0644); err != nil {
		t.Fatal(err)
	}

	q, err := newBookSummaryQueue(dir, 1, func(string) (string, error) {
		return "", fmt.Errorf("ollama down")
	})
	if err != nil {
		t.Fatal(err)
	}
	q.prefill(nil)
	segs := q.next()
	if len(segs) != 1 {
		t.Fatalf("segments = %d", len(segs))
	}
	if got := len(strings.Fields(segs[0].Text)); got > bookSummaryMaxWords {
		t.Errorf("fallback text too many words: %d", got)
	}
}
