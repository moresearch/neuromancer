package main

import (
	"strings"
	"testing"
)

func TestTypableRunes(t *testing.T) {
	kb := loadTestVil(t, testVil)
	chars := typableRunes(kb)
	joined := strings.Join(chars, "")
	for _, want := range []string{"a", "A", "!", "1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("typable runes missing %q: %q", want, joined)
		}
	}
}

func TestSanitizeGenLesson(t *testing.T) {
	kb := loadTestVil(t, testVil)

	lesson, err := sanitizeGenLesson(kb, genLesson{Chars: []string{"a", "!", "a"}, Count: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(lesson.Chars) != 2 || lesson.Count != 50 {
		t.Fatalf("lesson = %+v", lesson)
	}
	if _, err := sanitizeGenLesson(kb, genLesson{Chars: []string{"~"}}); err == nil {
		t.Fatal("unsupported character should fail")
	}
}

func TestGenerateLLMDrill(t *testing.T) {
	gen := generateLLMDrill(genLesson{Chars: []string{"a", "!"}, Count: 3}, 10, 1)
	segs := gen()
	if len(segs) != 1 || strings.Count(segs[0].Text, " ") != 2 {
		t.Fatalf("chars drill = %+v", segs)
	}

	gen = generateLLMDrill(genLesson{Text: "hello world"}, 10, 1)
	segs = gen()
	if len(segs) != 1 || segs[0].Text != "hello world" {
		t.Fatalf("text drill = %+v", segs)
	}
}
