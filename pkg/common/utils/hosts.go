package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// HostEntry maps a single IP to one or more hostnames (mirrors /etc/hosts format).
type HostEntry struct {
	IP        string   `json:"ip"`
	Hostnames []string `json:"hostnames"`
}

var hostnameRegexp = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`)

// isValidHostname returns true if s matches the hostname character rules.
// Single-character names (e.g. "a") are allowed.
func isValidHostname(s string) bool {
	if len(s) == 1 {
		return regexp.MustCompile(`^[a-zA-Z0-9]$`).MatchString(s)
	}
	return hostnameRegexp.MatchString(s)
}

// buildHostsLines returns the formatted /etc/hosts lines for each entry.
func buildHostsLines(entries []HostEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, e.IP+" "+strings.Join(e.Hostnames, " "))
	}
	return lines
}

// ValidateHostEntry returns a descriptive error if IP or any hostname is invalid.
// IP must be non-empty and parseable by net.ParseIP. At least one hostname required;
// each must match ^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$
func ValidateHostEntry(entry HostEntry) error {
	if entry.IP == "" {
		return fmt.Errorf("IP is required")
	}
	if net.ParseIP(entry.IP) == nil {
		return fmt.Errorf("invalid IP address %q", entry.IP)
	}
	if len(entry.Hostnames) == 0 {
		return fmt.Errorf("at least one hostname is required")
	}
	for _, h := range entry.Hostnames {
		if !isValidHostname(h) {
			return fmt.Errorf("invalid hostname %q: must match ^[a-zA-Z0-9]([a-zA-Z0-9\\-.]*[a-zA-Z0-9])?$", h)
		}
	}
	return nil
}

// ParseHostEntries deserializes a JSON string from the ConfigMap.
// Empty string or "[]" returns ([]HostEntry{}, nil). Invalid JSON returns a wrapped error.
func ParseHostEntries(jsonStr string) ([]HostEntry, error) {
	if jsonStr == "" {
		return []HostEntry{}, nil
	}
	var entries []HostEntry
	if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
		return nil, fmt.Errorf("failed to parse host entries JSON: %w", err)
	}
	return entries, nil
}

// SerializeHostEntries serializes entries to a JSON string for ConfigMap storage.
func SerializeHostEntries(entries []HostEntry) (string, error) {
	b, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("failed to serialize host entries: %w", err)
	}
	return string(b), nil
}

// BuildUserData produces the cloud-init UserData YAML for a worker agent node.
// When entries is nil/empty, output is identical to
// fmt.Sprintf(constants.K3sCloudInitScript, envFilePath, "false", masterIP, token).
func BuildUserData(envFilePath, masterIP, token string, entries []HostEntry) string {
	base := fmt.Sprintf(constants.K3sCloudInitScript, envFilePath, "false", masterIP, token)
	if len(entries) == 0 {
		return base
	}
	var sb strings.Builder
	sb.WriteString(base)
	for _, line := range buildHostsLines(entries) {
		sb.WriteString(fmt.Sprintf("  - echo %q >> /etc/hosts\n", line))
	}
	return sb.String()
}
