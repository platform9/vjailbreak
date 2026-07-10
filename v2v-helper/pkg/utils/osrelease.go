package utils

import "strings"

const OsReleaseProbeMarker = "===vjailbreak_osrel_probe_sep==="

func ParseOsReleaseProbe(rawOutput string, releaseFiles []string) (content string, matchedFile string, found bool) {
	segments := strings.Split(rawOutput, OsReleaseProbeMarker)

	for i, file := range releaseFiles {
		if i >= len(segments) {
			break
		}
		seg := strings.TrimSpace(segments[i])
		if seg == "" {
			continue
		}
		return seg, file, true
	}

	return "", "", false
}
