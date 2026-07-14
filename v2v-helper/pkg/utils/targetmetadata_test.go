package utils

import (
	"reflect"
	"strings"
	"testing"
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
