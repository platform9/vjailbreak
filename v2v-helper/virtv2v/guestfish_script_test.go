// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// buildScriptLines
// ---------------------------------------------------------------------------

func TestBuildScriptLines(t *testing.T) {
	tests := []struct {
		name  string
		steps []guestfishStep
		want  string
	}{
		{
			name:  "no steps",
			steps: nil,
			want:  "",
		},
		{
			name: "fail-fast step has no marker and no tolerance prefix",
			steps: []guestfishStep{
				{Command: "upload", Args: []string{"/local/path", "/guest/path"}},
			},
			want: "upload \"/local/path\" \"/guest/path\"\n",
		},
		{
			name: "tolerant step is wrapped in echo and the - prefix",
			steps: []guestfishStep{
				{Command: "stat", Args: []string{"/sbin/mkinitrd"}, Marker: "---VJB-STAT-1---"},
			},
			want: "echo ---VJB-STAT-1---\n- stat \"/sbin/mkinitrd\"\n",
		},
		{
			name: "fail-fast chain runs in order with no markers at all",
			steps: []guestfishStep{
				{Command: "upload", Args: []string{"/tmp/script.sh", "/tmp/script.sh"}},
				{Command: "chmod", Args: []string{"0755", "/tmp/script.sh"}},
				{Command: "sh", Args: []string{"/tmp/script.sh"}},
			},
			want: "upload \"/tmp/script.sh\" \"/tmp/script.sh\"\n" +
				"chmod \"0755\" \"/tmp/script.sh\"\n" +
				"sh \"/tmp/script.sh\"\n",
		},
		{
			name: "a mix of tolerant and fail-fast steps in one script",
			steps: []guestfishStep{
				{Command: "stat", Args: []string{"/sbin/mkinitrd"}, Marker: "M1"},
				{Command: "stat", Args: []string{"/usr/bin/dracut"}, Marker: "M2"},
				{Command: "upload", Args: []string{"/local/wrapper.sh", "/sbin/mkinitrd"}},
			},
			want: "echo M1\n- stat \"/sbin/mkinitrd\"\n" +
				"echo M2\n- stat \"/usr/bin/dracut\"\n" +
				"upload \"/local/wrapper.sh\" \"/sbin/mkinitrd\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildScriptLines(tt.steps))
		})
	}
}

// ---------------------------------------------------------------------------
// splitByMarker
// ---------------------------------------------------------------------------

func TestSplitByMarker(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		markers []string
		want    map[string]string
	}{
		{
			name:    "empty output",
			out:     "",
			markers: []string{"M1"},
			want:    map[string]string{},
		},
		{
			name:    "single marker with output",
			out:     "M1\n/sbin/mkinitrd: cannot stat\n",
			markers: []string{"M1"},
			want: map[string]string{
				"M1": "/sbin/mkinitrd: cannot stat",
			},
		},
		{
			name:    "multiple markers, each keeps only its own lines",
			out:     "M1\nfound\nM2\nnot found\nline two\n",
			markers: []string{"M1", "M2"},
			want: map[string]string{
				"M1": "found",
				"M2": "not found\nline two",
			},
		},
		{
			name:    "a marker with no output before the next one maps to empty string, not omitted",
			out:     "M1\nM2\nsomething\n",
			markers: []string{"M1", "M2"},
			want: map[string]string{
				"M1": "",
				"M2": "something",
			},
		},
		{
			name:    "blank lines are ignored, not treated as content or markers",
			out:     "M1\n\n  \nvalue\n\n",
			markers: []string{"M1"},
			want: map[string]string{
				"M1": "value",
			},
		},
		{
			name:    "text that looks like a marker but isn't in the known set is treated as content",
			out:     "M1\nUNKNOWN-MARKER\nvalue\n",
			markers: []string{"M1"},
			want: map[string]string{
				"M1": "UNKNOWN-MARKER\nvalue",
			},
		},
		{
			name:    "output before the first known marker is discarded",
			out:     "stray guestfish banner\nM1\nvalue\n",
			markers: []string{"M1"},
			want: map[string]string{
				"M1": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, splitByMarker(tt.out, tt.markers))
		})
	}
}

// ---------------------------------------------------------------------------
// countBoot
// ---------------------------------------------------------------------------

func TestCountBoot(t *testing.T) {
	before := atomic.LoadInt64(&guestfishBootCount)

	countBoot("test boot %d", 1)
	countBoot("test boot %d", 2)

	after := atomic.LoadInt64(&guestfishBootCount)
	assert.Equal(t, before+2, after)
}
