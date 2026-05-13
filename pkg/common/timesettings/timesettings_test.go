package timesettings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsValidNTPServer(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Valid IPv4
		{"1.2.3.4", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"192.168.1.1", true},
		// Invalid IPv4 (octet > 255 — must reject as both IP and hostname-of-digits is fine)
		{"256.1.1.1", false},
		// Note: "1.2.3", "1.2.3.4.5", "1.2.3.a" pass the hostname regex (digits
		// are valid label chars) — same behavior as the install.sh validator.
		// timesyncd would fail to resolve them at runtime; we accept syntactically.
		{"1.2.3", true},
		{"1.2.3.4.5", true},
		{"1.2.3.a", true},

		// Valid hostnames
		{"pool.ntp.org", true},
		{"0.pool.ntp.org", true},
		{"time-a.example.com", true},
		{"a", true},
		{"a-b.c-d.e", true},

		// Invalid hostnames
		{"", false},
		{".", false},
		{".foo.com", false},
		{"foo..com", false},
		{"-foo.com", false},
		{"foo-.com", false},
		{"foo_bar.com", false},
		{"foo.com/path", false},
		{"http://foo.com", false},
		{"https://foo.com", false},
		{strings.Repeat("a", 64) + ".com", false}, // label > 63
	}
	for _, c := range cases {
		got := IsValidNTPServer(c.in)
		if got != c.want {
			t.Errorf("IsValidNTPServer(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFilterValidNTPServers(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"pool.ntp.org", "pool.ntp.org"},
		{"a.com,b.com", "a.com b.com"},
		{"a.com\nb.com", "a.com b.com"},
		{"a.com, b.com , c.com", "a.com b.com c.com"},
		{"a.com,bad..host,1.2.3.4", "a.com 1.2.3.4"},
		{"http://a.com,1.2.3.4", "1.2.3.4"},
		{"256.1.1.1,1.2.3.4", "1.2.3.4"},
	}
	for _, c := range cases {
		got := FilterValidNTPServers(c.in)
		if got != c.want {
			t.Errorf("FilterValidNTPServers(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeTimezone_TraversalRejected(t *testing.T) {
	bad := []string{
		"../../etc/shadow",
		"..",
		"../UTC",
		"foo/../bar",
		"/etc/shadow",
		"/UTC",
		"UTC\x00",
	}
	for _, tz := range bad {
		if _, err := sanitizeTimezone(tz); err == nil {
			t.Errorf("sanitizeTimezone(%q) accepted; want rejection", tz)
		}
	}
}

func TestSanitizeTimezone_ValidPassesThrough(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"UTC", "UTC"},
		{"Etc/UTC", "Etc/UTC"},
		// Backward-compat aliases must NOT be rejected — D-Bus validates.
		{"Asia/Calcutta", "Asia/Calcutta"},
		{"Asia/Kolkata", "Asia/Kolkata"},
		{"America/New_York", "America/New_York"},
	}
	for _, c := range cases {
		got, err := sanitizeTimezone(c.in)
		if err != nil {
			t.Errorf("sanitizeTimezone(%q) error = %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("sanitizeTimezone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWriteTimesyncdConf_ContentAndClear(t *testing.T) {
	// Redirect to a tempdir for hermetic test.
	tmp := t.TempDir()
	origDir := timesyncdConfDirOverride
	origFile := timesyncdConfFileOverride
	timesyncdConfDirOverride = tmp
	timesyncdConfFileOverride = filepath.Join(tmp, "99-vjailbreak.conf")
	t.Cleanup(func() {
		timesyncdConfDirOverride = origDir
		timesyncdConfFileOverride = origFile
	})

	// Write servers.
	if err := writeTimesyncdConf("a.com b.com"); err != nil {
		t.Fatalf("write: %v", err)
	}
	body, err := os.ReadFile(timesyncdConfFileOverride)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "[Time]\nNTP=a.com b.com\n"
	if string(body) != want {
		t.Errorf("file body = %q, want %q", string(body), want)
	}

	// Clear when servers empty.
	if err := writeTimesyncdConf(""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := os.Stat(timesyncdConfFileOverride); !os.IsNotExist(err) {
		t.Errorf("file still exists after clear; err=%v", err)
	}

	// Idempotent clear.
	if err := writeTimesyncdConf(""); err != nil {
		t.Errorf("idempotent clear: %v", err)
	}
}
