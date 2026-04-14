// Package fcutil provides helpers for working with Fibre Channel World Wide
// Names (WWNs) as reported by ESXi hosts and Pure FlashArray storage arrays.
package fcutil

import (
	"fmt"
	"regexp"
	"strings"
)

// fcAdapterRE matches ESXi FC adapter UIDs: fc.<hex16>:<hex16>
// Group 1 = WWNN, Group 2 = WWPN (both as contiguous lowercase hex).
var fcAdapterRE = regexp.MustCompile(`(?i)^fc\.([0-9a-f]+):([0-9a-f]+)$`)

// wwnCharsRE validates that a string contains only hex digits.
var wwnCharsRE = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// ParseFCUID splits an ESXi FC adapter UID (format: "fc.WWNN:WWPN") into its
// two components. Both values are returned as uppercase hex strings without
// separators. Returns an error if the input does not match the expected format
// or contains non-hex characters.
func ParseFCUID(uid string) (wwnn, wwpn string, err error) {
	m := fcAdapterRE.FindStringSubmatch(uid)
	if m == nil {
		return "", "", fmt.Errorf("fcutil: %q is not a valid ESXi FC adapter UID (expected fc.WWNN:WWPN)", uid)
	}
	wwnn, wwpn = strings.ToUpper(m[1]), strings.ToUpper(m[2])
	if len(wwnn)%2 != 0 {
		return "", "", fmt.Errorf("fcutil: WWNN %q in %q has odd hex length", wwnn, uid)
	}
	if len(wwpn)%2 != 0 {
		return "", "", fmt.Errorf("fcutil: WWPN %q in %q has odd hex length", wwpn, uid)
	}
	return wwnn, wwpn, nil
}

// StripWWNFormatting removes colons, dashes, and spaces from a WWN string and
// returns it in uppercase. This produces a canonical form suitable for
// equality comparison regardless of the source's formatting convention.
//
// Pure FlashArray returns WWNs as "20:00:00:90:fa:6e:67:a8".
// ESXi reports them as contiguous hex "20000090fa6e67a8".
// Both normalise to "20000090FA6E67A8".
func StripWWNFormatting(wwn string) string {
	r := strings.NewReplacer(":", "", "-", "", " ", "")
	return strings.ToUpper(r.Replace(wwn))
}

// ColonSeparated inserts a colon between every pair of hex characters in wwn,
// producing the standard storage-array display format (e.g. "21:00:00:00:00:00:00:01").
func ColonSeparated(wwn string) string {
	wwn = strings.ToUpper(wwn)
	if len(wwn) < 2 {
		return wwn
	}
	pairs := make([]string, 0, len(wwn)/2)
	for i := 0; i+1 < len(wwn); i += 2 {
		pairs = append(pairs, wwn[i:i+2])
	}
	// Handle trailing odd character (non-standard but defensive)
	if len(wwn)%2 != 0 {
		pairs = append(pairs, wwn[len(wwn)-1:])
	}
	return strings.Join(pairs, ":")
}

// WWPNFromFCUID extracts the port name (WWPN) from an ESXi FC adapter UID.
// The WWPN is returned as an uppercase hex string without separators.
//
//	"fc.2000000000000001:2100000000000001" → "2100000000000001", nil
func WWPNFromFCUID(uid string) (string, error) {
	_, wwpn, err := ParseFCUID(uid)
	return wwpn, err
}

// FormattedWWPNFromFCUID extracts the WWPN and returns it colon-separated,
// matching the format used by storage array APIs such as Pure FlashArray.
//
//	"fc.2000000000000001:2100000000000001" → "21:00:00:00:00:00:00:01", nil
func FormattedWWPNFromFCUID(uid string) (string, error) {
	wwpn, err := WWPNFromFCUID(uid)
	if err != nil {
		return "", err
	}
	return ColonSeparated(wwpn), nil
}

// EqualWWNs reports whether two WWN strings refer to the same port address.
// Comparison is case-insensitive and ignores colon, dash, and space separators
// so that "21:00:00:00:00:00:00:01" and "2100000000000001" are considered equal.
func EqualWWNs(a, b string) bool {
	return StripWWNFormatting(a) == StripWWNFormatting(b)
}
