// Package preflight validates discovery prerequisites that can be checked
// without ever contacting vCenter: connection-input shape, and an exported
// role definition against a required-privileges list. Each check is its own
// file so the connection, role-file, and privilege-diff pieces can change
// independently.
package preflight

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// hostnameRe matches an RFC-1123 hostname/FQDN label sequence.
var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// ConnectionInput is what the "discover" command needs before it can even
// attempt to reach vCenter.
type ConnectionInput struct {
	Host     string
	Username string
	Password string
}

// ValidateConnectionInput checks that host/username/password are present and
// that host is a syntactically valid hostname, FQDN, IP, or https URL. It
// never opens a network connection.
func ValidateConnectionInput(in ConnectionInput) error {
	var problems []string

	if strings.TrimSpace(in.Host) == "" {
		problems = append(problems, "host is required")
	} else if err := validateHost(in.Host); err != nil {
		problems = append(problems, err.Error())
	}

	if strings.TrimSpace(in.Username) == "" {
		problems = append(problems, "username is required")
	}

	if in.Password == "" {
		problems = append(problems, "password is required")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid connection input: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateHost(host string) error {
	h := host
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		h = u.Hostname()
	}
	if h == "" {
		return fmt.Errorf("host %q is not a valid hostname, FQDN, IP, or URL", host)
	}
	if net.ParseIP(h) != nil {
		return nil
	}
	if !hostnameRe.MatchString(h) {
		return fmt.Errorf("host %q is not a valid hostname, FQDN, IP, or URL", host)
	}
	return nil
}
