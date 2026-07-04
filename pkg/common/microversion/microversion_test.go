package microversion

import "testing"

func TestFloor(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		hard     string
		expected string
	}{
		{
			name:     "config empty, hardcoded set -> hardcoded wins",
			config:   "",
			hard:     "2.60",
			expected: "2.60",
		},
		{
			name:     "config lower than hardcoded -> hardcoded wins",
			config:   "2.50",
			hard:     "2.60",
			expected: "2.60",
		},
		{
			name:     "config higher than hardcoded -> config wins",
			config:   "2.95",
			hard:     "2.60",
			expected: "2.95",
		},
		{
			name:     "config equal to hardcoded -> either is fine",
			config:   "2.60",
			hard:     "2.60",
			expected: "2.60",
		},
		{
			name:     "config set, hardcoded empty -> config wins",
			config:   "2.60",
			hard:     "",
			expected: "2.60",
		},
		{
			name:     "both empty -> empty",
			config:   "",
			hard:     "",
			expected: "",
		},
		{
			name:     "latest config beats any hardcoded",
			config:   "latest",
			hard:     "2.95",
			expected: "latest",
		},
		{
			name:     "hardcoded latest, config empty -> latest",
			config:   "",
			hard:     "latest",
			expected: "latest",
		},
		{
			name:     "config 2.100 vs hard 2.60 -> config wins (numeric compare, not lexical)",
			config:   "2.100",
			hard:     "2.60",
			expected: "2.100",
		},
		{
			name:     "different majors -> higher major wins",
			config:   "3.0",
			hard:     "2.95",
			expected: "3.0",
		},
		{
			name:     "config integer-only -> normalize and compare",
			config:   "3",
			hard:     "2.95",
			expected: "3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Floor(tc.config, tc.hard)
			if got != tc.expected {
				t.Errorf("Floor(%q, %q) = %q; want %q", tc.config, tc.hard, got, tc.expected)
			}
		})
	}
}

func TestFloor_InvalidInputs(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		hard     string
		expected string
	}{
		{
			name:     "garbage config -> fall back to hardcoded",
			config:   "not-a-version",
			hard:     "2.60",
			expected: "2.60",
		},
		{
			name:     "garbage hard, valid config -> use config",
			config:   "2.60",
			hard:     "garbage",
			expected: "2.60",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Floor(tc.config, tc.hard)
			if got != tc.expected {
				t.Errorf("Floor(%q, %q) = %q; want %q", tc.config, tc.hard, got, tc.expected)
			}
		})
	}
}
