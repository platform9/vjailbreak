package timesettings

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestResolveZoneinfoPath_TraversalRejected(t *testing.T) {
	// These must be rejected without touching the filesystem.
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
		if _, err := resolveZoneinfoPath(tz); err == nil {
			t.Errorf("resolveZoneinfoPath(%q) accepted; want rejection", tz)
		}
	}
}

func TestResolveZoneinfoPath_ValidStaysInBase(t *testing.T) {
	// Skip on platforms without a real zoneinfo dir (CI macOS sometimes lacks it).
	if _, err := os.Stat(ZoneinfoBase); err != nil {
		t.Skipf("zoneinfo base %s not present: %v", ZoneinfoBase, err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("zoneinfo layout is unix-specific")
	}
	cases := []string{"UTC", "Etc/UTC", ""}
	for _, tz := range cases {
		got, err := resolveZoneinfoPath(tz)
		if err != nil {
			// Some distros lack a specific entry; only fail if UTC itself is missing.
			if tz == "" || tz == "UTC" {
				t.Errorf("resolveZoneinfoPath(%q) error = %v", tz, err)
			}
			continue
		}
		if !strings.HasPrefix(got, ZoneinfoBase+string(filepath.Separator)) && got != ZoneinfoBase {
			t.Errorf("resolveZoneinfoPath(%q) = %q escapes %s", tz, got, ZoneinfoBase)
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
