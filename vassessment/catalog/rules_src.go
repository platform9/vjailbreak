package catalog

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// applianceKeywords flags named-appliance VMs (PRD §6.1 SRC-007). Matched
// case-insensitively against config.annotation.
var applianceKeywords = []string{"fortigate", "f5", "asav", "appliance"}

// doNotTouchKeywords flags VMs a customer has marked hands-off (SRC-019).
var doNotTouchKeywords = []string{"dnd", "do-not", "dont"}

// legacyChipsetGuestIDs are guest OS short ids known to need the i440fx
// machine-type note (SRC-016). Non-exhaustive placeholder set — expand
// against the vJailbreak compatibility matrix as it's consulted.
var legacyChipsetGuestIDs = map[string]bool{
	"centos64Guest":         true,
	"centos6_64Guest":       true,
	"rhel5_64Guest":         true,
	"rhel6_64Guest":         true,
	"windows7Server64Guest": true,
}

// rfc1123Name matches an RFC-1123-style label: alphanumeric, hyphens,
// starting and ending alphanumeric.
var rfc1123Name = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$`)

func containsAny(haystack string, needles []string) (string, bool) {
	lower := strings.ToLower(haystack)
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return n, true
		}
	}
	return "", false
}

func init() {
	Register(Rule{
		ID: "SRC-001", Name: "Template VM", Category: "Inventory", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if vm.Template {
				return Result{RuleID: "SRC-001", Passed: false, Verdict: VerdictBlocked, Evidence: "config.template=true"}
			}
			return Result{RuleID: "SRC-001", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-003", Name: "Virtual HW version < 7", Category: "Compatibility", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if vm.HWVersion > 0 && vm.HWVersion < 7 {
				return Result{RuleID: "SRC-003", Passed: false, Verdict: VerdictReadyWithSteps,
					Evidence: fmt.Sprintf("config.version=vmx-%02d", vm.HWVersion)}
			}
			return Result{RuleID: "SRC-003", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-004", Name: "CBT enabled", Category: "Performance", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if vm.CBTEnabled == nil || !*vm.CBTEnabled {
				evidence := "config.changeTrackingEnabled=unknown"
				if vm.CBTEnabled != nil {
					evidence = "config.changeTrackingEnabled=false"
				}
				return Result{RuleID: "SRC-004", Passed: false, Verdict: VerdictBlocked, Evidence: evidence}
			}
			return Result{RuleID: "SRC-004", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-006", Name: "VM name rules", Category: "Hygiene", Tier: "t1",
		Evaluate: func(vm VM) Result {
			switch {
			case len(vm.Name) >= 48:
				return Result{RuleID: "SRC-006", Passed: false, Verdict: VerdictReadyWithSteps,
					Evidence: fmt.Sprintf("name length=%d (>= 48)", len(vm.Name))}
			case strings.Contains(vm.Name, " "):
				return Result{RuleID: "SRC-006", Passed: false, Verdict: VerdictReadyWithSteps, Evidence: "name contains spaces"}
			case !rfc1123Name.MatchString(vm.Name):
				return Result{RuleID: "SRC-006", Passed: false, Verdict: VerdictReadyWithSteps, Evidence: "name is not RFC-1123 compliant"}
			}
			return Result{RuleID: "SRC-006", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-007", Name: "Appliance VM keyword", Category: "Inventory", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if kw, ok := containsAny(vm.Annotation, applianceKeywords); ok {
				return Result{RuleID: "SRC-007", Passed: false, Verdict: VerdictBlocked,
					Evidence: fmt.Sprintf("config.annotation matches keyword %q", kw)}
			}
			return Result{RuleID: "SRC-007", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-013", Name: "APIPA IP on any NIC", Category: "Networking", Tier: "t1",
		Evaluate: func(vm VM) Result {
			_, apipa, _ := net.ParseCIDR("169.254.0.0/16")
			for _, raw := range vm.NICIPv4s {
				if ip := net.ParseIP(raw); ip != nil && apipa.Contains(ip) {
					return Result{RuleID: "SRC-013", Passed: false, Verdict: VerdictBlocked,
						Evidence: fmt.Sprintf("NIC IP %s is APIPA", raw)}
				}
			}
			return Result{RuleID: "SRC-013", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-014", Name: "vTPM present", Category: "Compatibility", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if vm.VTPMPresent {
				return Result{RuleID: "SRC-014", Passed: false, Verdict: VerdictReadyWithSteps, Evidence: "VirtualTPM device present"}
			}
			return Result{RuleID: "SRC-014", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-015", Name: "VM encryption flag", Category: "Compatibility", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if vm.EncryptionKeyID != "" {
				return Result{RuleID: "SRC-015", Passed: false, Verdict: VerdictNeedsAssessment, Evidence: "config.keyId set"}
			}
			return Result{RuleID: "SRC-015", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-016", Name: "Legacy chipset OS set", Category: "Compatibility", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if legacyChipsetGuestIDs[vm.GuestOSID] {
				return Result{RuleID: "SRC-016", Passed: false, Verdict: VerdictReadyWithSteps,
					Evidence: fmt.Sprintf("guestId=%s", vm.GuestOSID)}
			}
			return Result{RuleID: "SRC-016", Passed: true}
		},
	})

	Register(Rule{
		ID: "SRC-019", Name: "Do-not-touch naming", Category: "Inventory", Tier: "t1",
		Evaluate: func(vm VM) Result {
			if kw, ok := containsAny(vm.Name, doNotTouchKeywords); ok {
				return Result{RuleID: "SRC-019", Passed: false, Verdict: VerdictBlocked,
					Evidence: fmt.Sprintf("name matches do-not-touch keyword %q", kw)}
			}
			return Result{RuleID: "SRC-019", Passed: true}
		},
	})
}
