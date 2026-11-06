package main

import (
	"strings"
	"testing"
)

func TestTrimText(t *testing.T) {
	got := trimText("  hello   world ", 20, 4)
	if got != "hello world" {
		t.Fatalf("short text = %q", got)
	}

	long := strings.Repeat("alpha beta gamma delta ", 20)
	got = trimText(long, 20, 3)
	lines := strings.Split(strings.ReplaceAll(got, " \n", "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d: %q", len(lines), got)
	}
	for _, line := range lines {
		if len([]rune(line)) > 20 {
			t.Errorf("line too long (%d): %q", len([]rune(line)), line)
		}
	}
}
