package fcutil_test

import (
	"strings"
	"testing"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/fcutil"
)

// ── ParseFCUID ────────────────────────────────────────────────────────────────

func TestParseFCUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantWWNN    string
		wantWWPN    string
		wantErrFrag string // non-empty → expect an error containing this substring
	}{
		// ── valid inputs ───────────────────────────────────────────────────────
		{
			name:     "standard lowercase",
			input:    "fc.2000000000000001:2100000000000001",
			wantWWNN: "2000000000000001",
			wantWWPN: "2100000000000001",
		},
		{
			name:     "standard uppercase prefix",
			input:    "fc.AABBCCDDEEFF0011:1122334455667788",
			wantWWNN: "AABBCCDDEEFF0011",
			wantWWPN: "1122334455667788",
		},
		{
			name:     "mixed case hex digits are uppercased",
			input:    "fc.20000090fa6e67a8:21000090fa6e67a8",
			wantWWNN: "20000090FA6E67A8",
			wantWWPN: "21000090FA6E67A8",
		},
		{
			name:  "short but even-length wwn (4 chars each)",
			input: "fc.aabb:ccdd",
			// short WWNs are unusual but the parser should accept even-length values
			wantWWNN: "AABB",
			wantWWPN: "CCDD",
		},
		{
			name:     "all-zeros wwn",
			input:    "fc.0000000000000000:0000000000000000",
			wantWWNN: "0000000000000000",
			wantWWPN: "0000000000000000",
		},
		{
			name:     "all-f wwn",
			input:    "fc.ffffffffffffffff:ffffffffffffffff",
			wantWWNN: "FFFFFFFFFFFFFFFF",
			wantWWPN: "FFFFFFFFFFFFFFFF",
		},

		// ── invalid inputs ─────────────────────────────────────────────────────
		{
			name:        "missing fc. prefix",
			input:       "2000000000000001:2100000000000001",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "wrong prefix (naa.)",
			input:       "naa.624a93702000000000000001",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "empty string",
			input:       "",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "no colon separator between wwnn and wwpn",
			input:       "fc.20000000000000012100000000000001",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "non-hex character in wwnn",
			input:       "fc.2000000000000XYZ:2100000000000001",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "non-hex character in wwpn",
			input:       "fc.2000000000000001:210000000000000G",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "odd-length wwnn",
			input:       "fc.200000000000001:2100000000000001",
			wantErrFrag: "odd hex length",
		},
		{
			name:        "odd-length wwpn",
			input:       "fc.2000000000000001:210000000000001",
			wantErrFrag: "odd hex length",
		},
		{
			name:        "empty wwnn",
			input:       "fc.:2100000000000001",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "empty wwpn",
			input:       "fc.2000000000000001:",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "three colon-delimited segments",
			input:       "fc.200000:000000:000001",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "fc. prefix only",
			input:       "fc.",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "colon-formatted wwnn (not contiguous hex)",
			input:       "fc.20:00:00:00:00:00:00:01:21:00:00:00:00:00:00:01",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotWWNN, gotWWPN, err := fcutil.ParseFCUID(tc.input)

			if tc.wantErrFrag != "" {
				if err == nil {
					t.Fatalf("ParseFCUID(%q) expected error containing %q, got nil (wwnn=%q wwpn=%q)",
						tc.input, tc.wantErrFrag, gotWWNN, gotWWPN)
				}
				if !strings.Contains(err.Error(), tc.wantErrFrag) {
					t.Fatalf("ParseFCUID(%q) error = %q, want it to contain %q", tc.input, err.Error(), tc.wantErrFrag)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseFCUID(%q) unexpected error: %v", tc.input, err)
			}
			if gotWWNN != tc.wantWWNN {
				t.Errorf("ParseFCUID(%q) WWNN = %q, want %q", tc.input, gotWWNN, tc.wantWWNN)
			}
			if gotWWPN != tc.wantWWPN {
				t.Errorf("ParseFCUID(%q) WWPN = %q, want %q", tc.input, gotWWPN, tc.wantWWPN)
			}
		})
	}
}

// ── StripWWNFormatting ────────────────────────────────────────────────────────

func TestStripWWNFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "colon-separated Pure FlashArray format",
			input: "21:00:00:90:fa:6e:67:a8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "contiguous hex ESXi format",
			input: "21000090fa6e67a8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "dash-separated",
			input: "21-00-00-90-fa-6e-67-a8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "space-separated",
			input: "21 00 00 90 fa 6e 67 a8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "mixed separators",
			input: "21:00-00 90:fa:6e:67:a8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "already uppercase and stripped",
			input: "21000090FA6E67A8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "all-zeros",
			input: "00:00:00:00:00:00:00:00",
			want:  "0000000000000000",
		},
		{
			name:  "uppercase colon-separated",
			input: "21:00:00:90:FA:6E:67:A8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single byte",
			input: "ff",
			want:  "FF",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fcutil.StripWWNFormatting(tc.input)
			if got != tc.want {
				t.Errorf("StripWWNFormatting(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── ColonSeparated ────────────────────────────────────────────────────────────

func TestColonSeparated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard 16-char wwn",
			input: "2100000000000001",
			want:  "21:00:00:00:00:00:00:01",
		},
		{
			name:  "lowercase input is uppercased",
			input: "21000090fa6e67a8",
			want:  "21:00:00:90:FA:6E:67:A8",
		},
		{
			name:  "all-zeros",
			input: "0000000000000000",
			want:  "00:00:00:00:00:00:00:00",
		},
		{
			name:  "all-f",
			input: "ffffffffffffffff",
			want:  "FF:FF:FF:FF:FF:FF:FF:FF",
		},
		{
			name:  "4-char wwn",
			input: "aabb",
			want:  "AA:BB",
		},
		{
			name:  "2-char wwn",
			input: "ab",
			want:  "AB",
		},
		{
			name:  "single character (less than 2)",
			input: "a",
			want:  "A",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "odd-length input (defensive handling)",
			input: "abc",
			want:  "AB:C",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fcutil.ColonSeparated(tc.input)
			if got != tc.want {
				t.Errorf("ColonSeparated(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── WWPNFromFCUID ─────────────────────────────────────────────────────────────

func TestWWPNFromFCUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		want        string
		wantErrFrag string
	}{
		{
			name:  "returns wwpn portion uppercase",
			input: "fc.2000000000000001:2100000000000001",
			want:  "2100000000000001",
		},
		{
			name:  "lowercase hex uppercased",
			input: "fc.20000090fa6e67a8:21000090fa6e67a8",
			want:  "21000090FA6E67A8",
		},
		{
			name:  "all-zeros wwpn",
			input: "fc.ffffffffffffffff:0000000000000000",
			want:  "0000000000000000",
		},
		{
			name:        "invalid uid returns error",
			input:       "not-an-fc-uid",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "odd wwpn length returns error",
			input:       "fc.2000000000000001:210000000000001",
			wantErrFrag: "odd hex length",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fcutil.WWPNFromFCUID(tc.input)

			if tc.wantErrFrag != "" {
				if err == nil {
					t.Fatalf("WWPNFromFCUID(%q) expected error, got nil (result=%q)", tc.input, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrFrag) {
					t.Fatalf("WWPNFromFCUID(%q) error = %q, want substring %q", tc.input, err.Error(), tc.wantErrFrag)
				}
				return
			}
			if err != nil {
				t.Fatalf("WWPNFromFCUID(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("WWPNFromFCUID(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── FormattedWWPNFromFCUID ────────────────────────────────────────────────────

func TestFormattedWWPNFromFCUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		want        string
		wantErrFrag string
	}{
		{
			name:  "typical real-world fc uid",
			input: "fc.20000090fa6e67a8:21000090fa6e67a8",
			want:  "21:00:00:90:FA:6E:67:A8",
		},
		{
			name:  "all-zeros wwpn",
			input: "fc.2000000000000001:0000000000000000",
			want:  "00:00:00:00:00:00:00:00",
		},
		{
			name:  "all-f wwpn",
			input: "fc.2000000000000001:ffffffffffffffff",
			want:  "FF:FF:FF:FF:FF:FF:FF:FF",
		},
		{
			name:        "invalid uid returns error",
			input:       "iqn.1998-01.com.vmware:host-123",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
		{
			name:        "empty string returns error",
			input:       "",
			wantErrFrag: "not a valid ESXi FC adapter UID",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fcutil.FormattedWWPNFromFCUID(tc.input)

			if tc.wantErrFrag != "" {
				if err == nil {
					t.Fatalf("FormattedWWPNFromFCUID(%q) expected error, got nil (result=%q)", tc.input, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrFrag) {
					t.Fatalf("FormattedWWPNFromFCUID(%q) error = %q, want substring %q",
						tc.input, err.Error(), tc.wantErrFrag)
				}
				return
			}
			if err != nil {
				t.Fatalf("FormattedWWPNFromFCUID(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("FormattedWWPNFromFCUID(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── EqualWWNs ─────────────────────────────────────────────────────────────────

func TestEqualWWNs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		// ── should be equal ────────────────────────────────────────────────────
		{
			name: "colon-separated vs contiguous hex",
			a:    "21:00:00:90:fa:6e:67:a8",
			b:    "21000090fa6e67a8",
			want: true,
		},
		{
			name: "dash-separated vs colon-separated",
			a:    "21-00-00-90-fa-6e-67-a8",
			b:    "21:00:00:90:fa:6e:67:a8",
			want: true,
		},
		{
			name: "space-separated vs contiguous",
			a:    "21 00 00 90 fa 6e 67 a8",
			b:    "21000090fa6e67a8",
			want: true,
		},
		{
			name: "uppercase vs lowercase",
			a:    "21000090FA6E67A8",
			b:    "21000090fa6e67a8",
			want: true,
		},
		{
			name: "Pure array format vs ESXi contiguous",
			a:    "20:00:00:90:FA:6E:67:A8",
			b:    "20000090fa6e67a8",
			want: true,
		},
		{
			name: "identical stripped strings",
			a:    "2100000000000001",
			b:    "2100000000000001",
			want: true,
		},
		{
			name: "mixed separator formats",
			a:    "21:00-00 90:fa:6e:67:a8",
			b:    "21000090FA6E67A8",
			want: true,
		},
		{
			name: "all-zeros equal",
			a:    "00:00:00:00:00:00:00:00",
			b:    "0000000000000000",
			want: true,
		},
		{
			name: "both empty",
			a:    "",
			b:    "",
			want: true,
		},

		// ── should not be equal ────────────────────────────────────────────────
		{
			name: "different wwns",
			a:    "21000090fa6e67a8",
			b:    "21000090fa6e67a9",
			want: false,
		},
		{
			name: "wwnn vs wwpn of same hba (different values)",
			a:    "20000090fa6e67a8",
			b:    "21000090fa6e67a8",
			want: false,
		},
		{
			name: "completely different wwns",
			a:    "2100000000000001",
			b:    "2100000000000002",
			want: false,
		},
		{
			name: "all-zeros vs all-ones",
			a:    "00:00:00:00:00:00:00:00",
			b:    "1111111111111111",
			want: false,
		},
		{
			name: "empty vs non-empty",
			a:    "",
			b:    "2100000000000001",
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fcutil.EqualWWNs(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("EqualWWNs(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			// Commutativity: EqualWWNs(a, b) == EqualWWNs(b, a)
			if rev := fcutil.EqualWWNs(tc.b, tc.a); rev != tc.want {
				t.Errorf("EqualWWNs(%q, %q) [reversed] = %v, want %v", tc.b, tc.a, rev, tc.want)
			}
		})
	}
}

// ── Integration: full fc.WWNN:WWPN → Pure array WWN comparison ───────────────
// This is the real-world path: ESXi reports an adapter UID, Pure returns
// a host WWN list.  We need them to match despite different formatting.

func TestFCUIDToPureWWNComparison(t *testing.T) {
	t.Parallel()

	// Simulate: ESXi adapter "fc.20000090fa6e67a8:21000090fa6e67a8"
	// Pure host record stores WWPN as "21:00:00:90:fa:6e:67:a8"
	esxiUID := "fc.20000090fa6e67a8:21000090fa6e67a8"
	pureHostWWN := "21:00:00:90:fa:6e:67:a8"

	wwpn, err := fcutil.WWPNFromFCUID(esxiUID)
	if err != nil {
		t.Fatalf("WWPNFromFCUID(%q): %v", esxiUID, err)
	}
	if !fcutil.EqualWWNs(wwpn, pureHostWWN) {
		t.Errorf("ESXi adapter WWPN %q does not match Pure host WWN %q (EqualWWNs returned false)", wwpn, pureHostWWN)
	}

	// Also verify the formatted form matches the pure format directly
	formatted, err := fcutil.FormattedWWPNFromFCUID(esxiUID)
	if err != nil {
		t.Fatalf("FormattedWWPNFromFCUID(%q): %v", esxiUID, err)
	}
	if !fcutil.EqualWWNs(formatted, pureHostWWN) {
		t.Errorf("formatted WWPN %q does not match Pure host WWN %q", formatted, pureHostWWN)
	}
}

// TestParseFCUIDReturnsBothComponents verifies that ParseFCUID returns
// both WWNN and WWPN correctly and that WWPNFromFCUID returns only the WWPN.
func TestParseFCUIDReturnsBothComponents(t *testing.T) {
	t.Parallel()

	uid := "fc.20000090fa6e67a8:21000090fa6e67a8"
	wantWWNN := "20000090FA6E67A8"
	wantWWPN := "21000090FA6E67A8"

	wwnn, wwpn, err := fcutil.ParseFCUID(uid)
	if err != nil {
		t.Fatalf("ParseFCUID(%q): %v", uid, err)
	}
	if wwnn != wantWWNN {
		t.Errorf("WWNN = %q, want %q", wwnn, wantWWNN)
	}
	if wwpn != wantWWPN {
		t.Errorf("WWPN = %q, want %q", wwpn, wantWWPN)
	}

	// WWPNFromFCUID must return the same WWPN
	onlyWWPN, err := fcutil.WWPNFromFCUID(uid)
	if err != nil {
		t.Fatalf("WWPNFromFCUID(%q): %v", uid, err)
	}
	if onlyWWPN != wantWWPN {
		t.Errorf("WWPNFromFCUID = %q, want %q", onlyWWPN, wantWWPN)
	}
}

// TestColonSeparatedRoundTrip verifies that StripWWNFormatting(ColonSeparated(x)) == ToUpper(x)
// for any even-length hex string — i.e. the two operations are inverse to each other.
func TestColonSeparatedRoundTrip(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"2100000000000001",
		"21000090fa6e67a8",
		"0000000000000000",
		"ffffffffffffffff",
		"aabbccddeeff0011",
	}

	for _, in := range inputs {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			colonFmt := fcutil.ColonSeparated(in)
			stripped := fcutil.StripWWNFormatting(colonFmt)
			wantUpper := strings.ToUpper(in)
			if stripped != wantUpper {
				t.Errorf("round-trip of %q: ColonSeparated=%q, StripWWNFormatting=%q, want %q",
					in, colonFmt, stripped, wantUpper)
			}
		})
	}
}
