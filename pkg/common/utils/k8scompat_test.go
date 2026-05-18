package utils

import (
	"testing"
)

func TestSanitizeLabelValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no spaces no change",
			input: "civ6s622-gpu-terminal-svr-vm-503b0",
			want:  "civ6s622-gpu-terminal-svr-vm-503b0",
		},
		{
			name:  "spaces replaced with hyphens",
			input: "civ6s622 - GPU Terminal Svr VM-503b0",
			want:  "civ6s622---GPU-Terminal-Svr-VM-503b0",
		},
		{
			name:  "VM name from bug report",
			input: "civ6s622 - GPU Terminal Svr VM-10023",
			want:  "civ6s622---GPU-Terminal-Svr-VM-10023",
		},
		{
			name:  "leading trailing invalid chars trimmed",
			input: "-foo-bar-",
			want:  "foo-bar",
		},
		{
			name:  "special chars removed",
			input: "vm@name!test",
			want:  "vmnametest",
		},
		{
			name:  "dots and underscores preserved",
			input: "vm_name.test",
			want:  "vm_name.test",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "truncated to 63 chars",
			input: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnop",
			want:  "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeLabelValue(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeLabelValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
