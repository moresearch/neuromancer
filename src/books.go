package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type book struct {
	path       string
	paragraphs []string
}

var bookWhitespace = regexp.MustCompile("\\s+")

func bookParagraphs(text string) []string {
	var out []string
	for _, p := range getParagraphs(text) {
		p = strings.TrimSpace(bookWhitespace.ReplaceAllString(p, " "))
		if len([]rune(p)) >= 80 {
			out = append(out, p)
		}
	}
	return out
}

func loadBooks(dir string) ([]book, error) {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".txt", ".md", ".text", ".org":
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)

	var books []book
	for _, path := range paths {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}
		paragraphs := bookParagraphs(string(b))
		if len(paragraphs) > 0 {
			books = append(books, book{path: path, paragraphs: paragraphs})
		}
	}
	return books, nil
}

// Book summaries are capped by words, with a generous rune cap as a safety
// net for very long words.
const (
	bookSummaryMaxWords = 20
	bookSummaryMaxRunes = 160
)

func finishTypingSentence(s string) string {
	s = strings.TrimRight(s, " ,;:-")
	if s == "" {
		return ""
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
	default:
		s += "."
	}
	return s
}

func clipTypingText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > max {
		s = string(r[:max])
		if i := strings.LastIndexByte(s, ' '); i > 0 {
			s = s[:i]
		}
	}
	return finishTypingSentence(s)
}

func clipTypingWords(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	words := strings.Fields(s)
	if len(words) > max {
		words = words[:max]
	}
	if len(words) == 0 {
		return ""
	}
	return finishTypingSentence(strings.Join(words, " "))
}

func generateBookTest(dir string) func() []segment {
	return generateBookTestWith(dir, nil)
}

func generateBookSummaryTest(dir string, summarize func(string) (string, error)) func() []segment {
	return generateBookTestWith(dir, summarize)
}

func generateBookTestWith(dir string, summarize func(string) (string, error)) func() []segment {
	books, err := loadBooks(dir)
	if err != nil {
		die("Failed to read books directory %s: %v", dir, err)
	}
	if len(books) == 0 {
		die("No usable books found in %s (need .txt/.md/.text/.org files with paragraphs of 80+ characters).", dir)
	}

	return func() []segment {
		b := books[rand.Intn(len(books))]
		p := b.paragraphs[rand.Intn(len(b.paragraphs))]
		if summarize != nil {
			if s, err := summarize(p); err == nil && s != "" {
				p = s
			} else {
				if err != nil {
					warnf("book summary failed: %v", err)
				}
				p = clipTypingWords(p, bookSummaryMaxWords)
				p = clipTypingText(p, bookSummaryMaxRunes)
			}
		}
		return []segment{{Text: p, Attribution: filepath.Base(b.path)}}
	}
}

const bookSummaryQueueSize = 3

// bookSummaryQueue keeps a small buffer of short summaries ready so sprints
// start without waiting for Ollama. A single worker refills the queue while
// the typist is reading or typing the current sprint.
type bookSummaryQueue struct {
	books     []book
	summarize func(string) (string, error)
	ch        chan segment
}

func newBookSummaryQueue(dir string, size int, summarize func(string) (string, error)) (*bookSummaryQueue, error) {
	books, err := loadBooks(dir)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, fmt.Errorf("no usable books found in %s", dir)
	}
	if size < 1 {
		size = 1
	}
	return &bookSummaryQueue{
		books:     books,
		summarize: summarize,
		ch:        make(chan segment, size),
	}, nil
}

func (q *bookSummaryQueue) makeSegment() (segment, error) {
	b := q.books[rand.Intn(len(q.books))]
	p := b.paragraphs[rand.Intn(len(b.paragraphs))]
	s, err := q.summarize(p)
	if err != nil || s == "" {
		s = clipTypingWords(p, bookSummaryMaxWords)
		s = clipTypingText(s, bookSummaryMaxRunes)
	}
	return segment{Text: s, Attribution: filepath.Base(b.path)}, err
}

func (q *bookSummaryQueue) prefill(progress func(done, total int)) {
	total := cap(q.ch)
	var firstErr error
	for i := 0; i < total; i++ {
		seg, err := q.makeSegment()
		if err != nil && firstErr == nil {
			firstErr = err
		}
		q.ch <- seg
		if progress != nil {
			progress(i+1, total)
		}
	}
	if firstErr != nil {
		warnf("book summary failed: %v", firstErr)
	}
	go func() {
		for {
			seg, _ := q.makeSegment()
			q.ch <- seg
		}
	}()
}

func (q *bookSummaryQueue) ready() bool {
	return len(q.ch) > 0
}

func (q *bookSummaryQueue) next() []segment {
	return []segment{<-q.ch}
}
