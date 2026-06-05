package server

import (
	"strings"
)

// SplitLines splits a log string into non-empty lines.
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// TruncateTailChars keeps the last maxChars bytes of s, prepending a marker if truncated.
func TruncateTailChars(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}
	return "... [truncated] ...\n" + s[len(s)-maxChars:]
}

// ExtractRelevantLines returns ERROR/WARN/FAILED lines with +/-contextWindow
// surrounding lines, plus the last tailLines lines. Deduplicates.
func ExtractRelevantLines(lines []string, contextWindow, tailLines int) []string {
	if len(lines) == 0 {
		return nil
	}

	include := make([]bool, len(lines))

	for i, line := range lines {
		up := strings.ToUpper(line)
		if strings.Contains(up, "ERROR") || strings.Contains(up, "FAILED") || strings.Contains(up, "WARN") {
			start := i - contextWindow
			if start < 0 {
				start = 0
			}
			end := i + contextWindow
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for j := start; j <= end; j++ {
				include[j] = true
			}
		}
	}

	tailStart := len(lines) - tailLines
	if tailStart < 0 {
		tailStart = 0
	}
	for i := tailStart; i < len(lines); i++ {
		include[i] = true
	}

	result := make([]string, 0, len(lines))
	for i, line := range lines {
		if include[i] {
			result = append(result, line)
		}
	}
	return result
}
