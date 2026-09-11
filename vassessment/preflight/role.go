package preflight

import (
	"encoding/json"
	"fmt"
	"os"
)

// RoleDefinition is the shape of the "Role definition (JSON)" file vAssessment
// ships for a customer to apply, and of the file they'd export back to
// confirm what was actually granted:
//
//	{"name": "vassessment-readonly", "privileges": ["System.View", ...]}
//
// The authoritative minimal-privilege vSphere privilege list is intentionally
// NOT hardcoded here: it depends on exactly which govmomi calls the
// discovery engine ends up making (#2387), so fixing it now would risk
// shipping a list that's wrong for what discovery actually needs. Once
// #2387's collectors land, add that list as a checked-in required-privileges
// JSON file and validate it with ValidatePrivileges below.
type RoleDefinition struct {
	Name       string   `json:"name"`
	Privileges []string `json:"privileges"`
}

// LoadRoleDefinition reads and parses a role-definition JSON file. It does
// not contact vCenter — the file is expected to have been produced by the
// "Role definition (JSON)" download or exported from vCenter by the customer.
func LoadRoleDefinition(path string) (RoleDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RoleDefinition{}, fmt.Errorf("reading role definition %s: %w", path, err)
	}
	var rd RoleDefinition
	if err := json.Unmarshal(data, &rd); err != nil {
		return RoleDefinition{}, fmt.Errorf("parsing role definition %s: %w", path, err)
	}
	if rd.Name == "" {
		return RoleDefinition{}, fmt.Errorf("role definition %s: missing \"name\"", path)
	}
	return rd, nil
}

// ValidatePrivileges compares a granted privilege set against a required
// one. missing lists required privileges granted lacks; extra lists granted
// privileges beyond what's required (informational — not a failure).
func ValidatePrivileges(required, granted []string) (missing, extra []string, ok bool) {
	grantedSet := make(map[string]bool, len(granted))
	for _, p := range granted {
		grantedSet[p] = true
	}
	requiredSet := make(map[string]bool, len(required))
	for _, p := range required {
		requiredSet[p] = true
		if !grantedSet[p] {
			missing = append(missing, p)
		}
	}
	for _, p := range granted {
		if !requiredSet[p] {
			extra = append(extra, p)
		}
	}
	return missing, extra, len(missing) == 0
}
