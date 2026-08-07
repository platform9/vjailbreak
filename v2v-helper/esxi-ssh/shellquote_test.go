// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain path needs no escaping but is still quoted",
			input: "/vmfs/volumes/ds-prod-01/web01/web01.vmdk",
			want:  "'/vmfs/volumes/ds-prod-01/web01/web01.vmdk'",
		},
		{
			name:  "space in VM folder and file name",
			input: "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
			want:  "'/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk'",
		},
		{
			name:  "apostrophe in VM name",
			input: "/vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
			want:  `'/vmfs/volumes/ds/suhas'\''s VM/suhas'\''s VM.vmdk'`,
		},
		{
			name:  "leading apostrophe",
			input: "'quoted",
			want:  `''\''quoted'`,
		},
		{
			name:  "only an apostrophe",
			input: "'",
			want:  `''\'''`,
		},
		{
			name:  "parentheses and ampersand",
			input: "/vmfs/volumes/ds/VM (copy) & backup/disk.vmdk",
			want:  "'/vmfs/volumes/ds/VM (copy) & backup/disk.vmdk'",
		},
		{
			name:  "command substitution is neutralised",
			input: "/vmfs/volumes/ds/$(rm -rf /)/disk.vmdk",
			want:  "'/vmfs/volumes/ds/$(rm -rf /)/disk.vmdk'",
		},
		{
			name:  "semicolon does not terminate the command",
			input: "/vmfs/volumes/ds/a;reboot/disk.vmdk",
			want:  "'/vmfs/volumes/ds/a;reboot/disk.vmdk'",
		},
		{
			name:  "empty string",
			input: "",
			want:  "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellQuote(tt.input); got != tt.want {
				t.Errorf("shellQuote(%q)\n got: %s\nwant: %s", tt.input, got, tt.want)
			}
		})
	}
}

// TestShellQuoteRoundTripsThroughShell asserts the quoted form is decoded back
// into the original string by a real POSIX shell — the property that actually
// matters. Skipped when /bin/sh is unavailable.
func TestShellQuoteRoundTripsThroughShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	inputs := []string{
		"/vmfs/volumes/ds-prod-01/web01/web01.vmdk",
		"/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
		"/vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
		"/vmfs/volumes/ds/VM (copy) & backup/disk.vmdk",
		"/vmfs/volumes/ds/a;reboot/disk.vmdk",
		"/vmfs/volumes/ds/$(echo pwned)/disk.vmdk",
		"/vmfs/volumes/ds/tab\tseparated/disk.vmdk",
		`/vmfs/volumes/ds/back\slash/disk.vmdk`,
		`/vmfs/volumes/ds/"double quoted"/disk.vmdk`,
		"'",
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			// printf '%s' <quoted> must echo the original bytes back verbatim.
			out, err := exec.Command("sh", "-c", "printf '%s' "+shellQuote(in)).Output()
			if err != nil {
				t.Fatalf("shell rejected quoted form of %q: %v", in, err)
			}
			if string(out) != in {
				t.Errorf("round trip mismatch\n got: %q\nwant: %q", string(out), in)
			}
		})
	}
}

// TestShellQuoteProducesSingleArgument guards the specific failure mode from the
// bug: a path with spaces must arrive as one argument, not several.
func TestShellQuoteProducesSingleArgument(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	path := "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk"

	// Unquoted: demonstrates the bug — the shell splits into 3 arguments.
	out, err := exec.Command("sh", "-c", "set -- "+path+"; echo $#").Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "3" {
		t.Fatalf("expected unquoted path to split into 3 args, got %s", strings.TrimSpace(string(out)))
	}

	// Quoted: exactly one argument.
	out, err = exec.Command("sh", "-c", "set -- "+shellQuote(path)+"; echo $#").Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Errorf("expected quoted path to be 1 arg, got %s", strings.TrimSpace(string(out)))
	}
}

func TestShellQuotePrefixed(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		path   string
		want   string
	}{
		{
			name:   "rdm device path",
			prefix: "rdm:",
			path:   "/vmfs/devices/disks/naa.624a9370ae0bbb05e168463d00016e91",
			want:   "rdm:'/vmfs/devices/disks/naa.624a9370ae0bbb05e168463d00016e91'",
		},
		{
			name:   "prefix stays outside the quotes when path has a space",
			prefix: "rdm:",
			path:   "/vmfs/devices/disks/odd name",
			want:   "rdm:'/vmfs/devices/disks/odd name'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellQuotePrefixed(tt.prefix, tt.path); got != tt.want {
				t.Errorf("shellQuotePrefixed(%q, %q)\n got: %s\nwant: %s", tt.prefix, tt.path, got, tt.want)
			}
		})
	}
}

// TestShellQuoteGlobStillExpands verifies the directory is protected while the
// glob remains active — quoting the whole thing would break disk discovery.
func TestShellQuoteGlobStillExpands(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		pattern string
		want    string
	}{
		{
			name:    "plain directory",
			dir:     "/vmfs/volumes/ds/web01",
			pattern: "/*.vmdk",
			want:    "'/vmfs/volumes/ds/web01'/*.vmdk",
		},
		{
			name:    "directory with space keeps glob outside quotes",
			dir:     "/vmfs/volumes/ds/My VM",
			pattern: "/*.vmdk",
			want:    "'/vmfs/volumes/ds/My VM'/*.vmdk",
		},
		{
			name:    "directory with apostrophe",
			dir:     "/vmfs/volumes/ds/suhas's VM",
			pattern: "/*.vmdk",
			want:    `'/vmfs/volumes/ds/suhas'\''s VM'/*.vmdk`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuoteGlob(tt.dir, tt.pattern)
			if got != tt.want {
				t.Errorf("shellQuoteGlob(%q, %q)\n got: %s\nwant: %s", tt.dir, tt.pattern, got, tt.want)
			}
			if strings.HasSuffix(got, "'") {
				t.Errorf("glob pattern must remain outside the quotes, got %s", got)
			}
		})
	}
}

func TestParseLsFilename(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "path without spaces",
			line: "-rw-------    1 root     root     107374182400 Dec  6 10:30 /vmfs/volumes/ds/web01/web01.vmdk",
			want: "/vmfs/volumes/ds/web01/web01.vmdk",
		},
		{
			name: "path with spaces is not truncated",
			line: "-rw-------    1 root     root           609 Jul 31 08:18 /vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
			want: "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
		},
		{
			name: "path with apostrophe and spaces",
			line: "-rw-------    1 root     root           609 Jul 31 08:18 /vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
			want: "/vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
		},
		{
			name: "short line is rejected",
			line: "total 4",
			want: "",
		},
		{
			name: "empty line is rejected",
			line: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLsFilename(tt.line); got != tt.want {
				t.Errorf("parseLsFilename(%q)\n got: %q\nwant: %q", tt.line, got, tt.want)
			}
		})
	}
}
