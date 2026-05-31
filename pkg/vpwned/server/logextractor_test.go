package server

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractRelevantLines_ErrorWithContext(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("INFO line %d", i)
	}
	lines[25] = "ERROR disk copy failed: connection refused"

	result := ExtractRelevantLines(lines, 10, 200)

	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "ERROR disk copy failed") {
		t.Error("expected ERROR line in result")
	}
	if !strings.Contains(joined, "INFO line 15") {
		t.Errorf("expected context line 15 before error, got:\n%s", joined)
	}
	if !strings.Contains(joined, "INFO line 35") {
		t.Errorf("expected context line 35 after error, got:\n%s", joined)
	}
}

func TestExtractRelevantLines_LastNLines(t *testing.T) {
	lines := make([]string, 300)
	for i := range lines {
		lines[i] = fmt.Sprintf("INFO line %d", i)
	}
	result := ExtractRelevantLines(lines, 10, 200)
	joined := strings.Join(result, "\n")
	if strings.Contains(joined, "INFO line 0") {
		t.Error("should not include very early lines when no errors present")
	}
	if !strings.Contains(joined, "INFO line 299") {
		t.Error("should always include the last line")
	}
	if len(result) > 200 {
		t.Errorf("expected at most 200 lines from tail, got %d", len(result))
	}
}

func TestExtractRelevantLines_Empty(t *testing.T) {
	result := ExtractRelevantLines(nil, 10, 200)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d lines", len(result))
	}
}

func TestExtractRelevantLines_DeduplicatesOverlappingContext(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("INFO line %d", i)
	}
	lines[5] = "ERROR first error"
	lines[7] = "ERROR second error"

	result := ExtractRelevantLines(lines, 10, 200)

	seen := map[string]int{}
	for _, l := range result {
		seen[l]++
	}
	for l, count := range seen {
		if count > 1 {
			t.Errorf("line %q appears %d times (expected 1)", l, count)
		}
	}
}

func TestExtractRelevantLines_WarnLinesIncluded(t *testing.T) {
	lines := []string{"INFO a", "WARN disk latency high", "INFO b"}
	result := ExtractRelevantLines(lines, 2, 0)
	joined := strings.Join(result, "\n")
	if !strings.Contains(joined, "WARN disk latency high") {
		t.Error("expected WARN line in result")
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single", "hello", 1},
		{"two lines", "a\nb", 2},
		{"trailing newline", "a\nb\n", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitLines(tc.input)
			if len(got) != tc.want {
				t.Errorf("SplitLines(%q) = %d lines, want %d", tc.input, len(got), tc.want)
			}
		})
	}
}
