package utils

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildTargetMetadata(t *testing.T) {
	longValue := strings.Repeat("x", 300)

	tests := []struct {
		name               string
		sourceTagsMetadata map[string]string
		customMetadata     map[string]string
		expected           map[string]string
	}{
		{
			name:     "both empty returns nil",
			expected: nil,
		},
		{
			name:               "source only",
			sourceTagsMetadata: map[string]string{"tag:env": "production"},
			expected:           map[string]string{"tag:env": "production"},
		},
		{
			name:           "custom only",
			customMetadata: map[string]string{"wave": "2"},
			expected:       map[string]string{"wave": "2"},
		},
		{
			name:               "custom wins on collision",
			sourceTagsMetadata: map[string]string{"tag:env": "production", "attr:Owner": "bob@corp.com"},
			customMetadata:     map[string]string{"attr:Owner": "alice@corp.com"},
			expected: map[string]string{
				"tag:env":    "production",
				"attr:Owner": "alice@corp.com",
			},
		},
		{
			name:               "values truncated to nova 255 limit",
			sourceTagsMetadata: map[string]string{"tag:notes": longValue},
			expected:           map[string]string{"tag:notes": longValue[:255]},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTargetMetadata(tt.sourceTagsMetadata, tt.customMetadata)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("BuildTargetMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTruncateMetadata_UTF8Boundary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short string unchanged",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "exactly 255 bytes unchanged",
			input: strings.Repeat("x", 255),
			want:  strings.Repeat("x", 255),
		},
		{
			name:  "ascii over limit cut at 255",
			input: strings.Repeat("x", 300),
			want:  strings.Repeat("x", 255),
		},
		{
			name: "multi-byte char straddling the boundary is dropped, not split",
			// 254 ASCII bytes + "€" (3 bytes): a byte-position cut at 255 would
			// keep 1 of the euro sign's 3 bytes, producing invalid UTF-8.
			input: strings.Repeat("x", 254) + "€" + strings.Repeat("y", 50),
			want:  strings.Repeat("x", 254),
		},
		{
			name: "4-byte emoji straddling the boundary is dropped, not split",
			// 253 ASCII bytes + "😀" (4 bytes): cut at 255 keeps 2 of 4 bytes.
			input: strings.Repeat("x", 253) + "😀" + strings.Repeat("y", 50),
			want:  strings.Repeat("x", 253),
		},
		{
			name: "multi-byte char ending exactly at the boundary is kept",
			// 252 ASCII bytes + "€" (3 bytes) = 255 bytes exactly, then overflow.
			input: strings.Repeat("x", 252) + "€" + strings.Repeat("y", 50),
			want:  strings.Repeat("x", 252) + "€",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateMetadata(tt.input)
			if got != tt.want {
				t.Errorf("truncateMetadata() = %q (len %d), want %q (len %d)", got, len(got), tt.want, len(tt.want))
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateMetadata() produced invalid UTF-8: %q", got)
			}
			if len(got) > novaMetadataMaxLength {
				t.Errorf("truncateMetadata() result is %d bytes, exceeds nova limit %d", len(got), novaMetadataMaxLength)
			}
		})
	}
}
