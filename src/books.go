package main

import (
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

func generateBookTest(dir string) func() []segment {
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
		return []segment{{Text: p, Attribution: filepath.Base(b.path)}}
	}
}
