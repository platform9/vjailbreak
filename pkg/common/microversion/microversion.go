// Package microversion provides an operator-configurable floor over hardcoded
// OpenStack service microversion values.
//
// vjailbreak hardcodes microversion values for specific operations (for example,
// 2.60 on the Nova compute attach call to support multi-attach volumes). When
// the operator configures a per-service API version through clouds.yaml
// (compute_api_version, volume_api_version, image_api_version,
// network_api_version, identity_api_version), the value is consumed as a floor:
// the higher of the operator-configured version and the internal hardcoded
// version wins, so a misconfigured low version cannot lower the microversion
// the operation requires.
package microversion

import (
	"math"
	"strconv"
	"strings"
)

const latestSentinel = "latest"

// Floor returns the higher of two OpenStack microversion strings of the form
// "MAJOR.MINOR" or "MAJOR". The literal value "latest" is treated as greater
// than any specific version. An empty or unparseable value is treated as "no
// override" and loses to any valid version.
//
// The returned string is the original (un-normalized) form of whichever input
// wins.
func Floor(configValue, hardcodedValue string) string {
	if compareVersions(configValue, hardcodedValue) >= 0 {
		return configValue
	}
	return hardcodedValue
}

// compareVersions returns -1, 0, or 1 if a is less than, equal to, or greater
// than b. Unparseable values are treated as "no version" and rank below any
// valid version (and equal to each other).
func compareVersions(a, b string) int {
	aMajor, aMinor, aOK := parseVersion(a)
	bMajor, bMinor, bOK := parseVersion(b)

	switch {
	case !aOK && !bOK:
		return 0
	case !aOK:
		return -1
	case !bOK:
		return 1
	}

	if aMajor != bMajor {
		return signInt(aMajor - bMajor)
	}
	return signInt(aMinor - bMinor)
}

func signInt(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// parseVersion returns (major, minor, true) for valid "MAJOR.MINOR", "MAJOR",
// or the "latest" sentinel. For "latest" it returns max int values so any
// numeric comparison ranks it highest.
func parseVersion(s string) (major, minor int, ok bool) {
	if s == "" {
		return 0, 0, false
	}
	if s == latestSentinel {
		return math.MaxInt32, math.MaxInt32, true
	}

	parts := strings.SplitN(s, ".", 2)
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	if len(parts) == 1 {
		return major, 0, true
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}
