package catalog

import (
	"strings"
	"testing"
)

func findRule(t *testing.T, id string) Rule {
	t.Helper()
	for _, r := range All() {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("rule %s not registered", id)
	return Rule{}
}

func TestSRC001_TemplateVM(t *testing.T) {
	rule := findRule(t, "SRC-001")
	cases := []struct {
		name   string
		vm     VM
		passed bool
	}{
		{"non-template passes", VM{Template: false}, true},
		{"template fails", VM{Template: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rule.Evaluate(c.vm)
			if got.Passed != c.passed {
				t.Errorf("Passed = %v, want %v", got.Passed, c.passed)
			}
			if !c.passed && got.Verdict != VerdictBlocked {
				t.Errorf("Verdict = %v, want %v", got.Verdict, VerdictBlocked)
			}
		})
	}
}

func TestSRC003_HWVersion(t *testing.T) {
	rule := findRule(t, "SRC-003")
	cases := []struct {
		name   string
		hw     int
		passed bool
	}{
		{"unset (0) passes", 0, true},
		{"hw 15 passes", 15, true},
		{"hw 4 fails", 4, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rule.Evaluate(VM{HWVersion: c.hw})
			if got.Passed != c.passed {
				t.Errorf("Passed = %v, want %v", got.Passed, c.passed)
			}
		})
	}
}

func TestSRC004_CBTEnabled(t *testing.T) {
	rule := findRule(t, "SRC-004")
	tru, fls := true, false
	cases := []struct {
		name   string
		cbt    *bool
		passed bool
	}{
		{"enabled passes", &tru, true},
		{"disabled fails", &fls, false},
		{"unknown (nil) fails", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rule.Evaluate(VM{CBTEnabled: c.cbt})
			if got.Passed != c.passed {
				t.Errorf("Passed = %v, want %v", got.Passed, c.passed)
			}
			if !c.passed && got.Verdict != VerdictBlocked {
				t.Errorf("Verdict = %v, want %v", got.Verdict, VerdictBlocked)
			}
		})
	}
}

func TestSRC006_NameRules(t *testing.T) {
	rule := findRule(t, "SRC-006")
	cases := []struct {
		name   string
		vmName string
		passed bool
	}{
		{"short compliant name passes", "erp-app-01", true},
		{"name with space fails", "erp app 01", false},
		{"name >= 48 chars fails", strings.Repeat("a", 48), false},
		{"non RFC-1123 char fails", "erp_app_01!", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rule.Evaluate(VM{Name: c.vmName})
			if got.Passed != c.passed {
				t.Errorf("Passed = %v, want %v (evidence: %s)", got.Passed, c.passed, got.Evidence)
			}
		})
	}
}

func TestSRC007_ApplianceKeyword(t *testing.T) {
	rule := findRule(t, "SRC-007")
	cases := []struct {
		name       string
		annotation string
		passed     bool
	}{
		{"empty annotation passes", "", true},
		{"FortiGate annotation fails", "FortiGate appliance", false},
		{"case-insensitive match fails", "internal ASAV box", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rule.Evaluate(VM{Annotation: c.annotation})
			if got.Passed != c.passed {
				t.Errorf("Passed = %v, want %v", got.Passed, c.passed)
			}
		})
	}
}

func TestSRC013_APIPA(t *testing.T) {
	rule := findRule(t, "SRC-013")
	cases := []struct {
		name   string
		ips    []string
		passed bool
	}{
		{"normal IP passes", []string{"10.20.4.11"}, true},
		{"APIPA IP fails", []string{"169.254.1.5"}, false},
		{"mixed, one APIPA fails", []string{"10.0.0.1", "169.254.0.1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rule.Evaluate(VM{NICIPv4s: c.ips})
			if got.Passed != c.passed {
				t.Errorf("Passed = %v, want %v", got.Passed, c.passed)
			}
		})
	}
}

func TestSRC014_VTPM(t *testing.T) {
	rule := findRule(t, "SRC-014")
	if got := rule.Evaluate(VM{VTPMPresent: false}); !got.Passed {
		t.Errorf("expected pass without vTPM, got %+v", got)
	}
	if got := rule.Evaluate(VM{VTPMPresent: true}); got.Passed {
		t.Errorf("expected fail with vTPM present, got %+v", got)
	}
}

func TestSRC015_Encryption(t *testing.T) {
	rule := findRule(t, "SRC-015")
	if got := rule.Evaluate(VM{EncryptionKeyID: ""}); !got.Passed {
		t.Errorf("expected pass without key id, got %+v", got)
	}
	got := rule.Evaluate(VM{EncryptionKeyID: "key-123"})
	if got.Passed || got.Verdict != VerdictNeedsAssessment {
		t.Errorf("expected needs-assessment fail, got %+v", got)
	}
}

func TestSRC016_LegacyChipset(t *testing.T) {
	rule := findRule(t, "SRC-016")
	if got := rule.Evaluate(VM{GuestOSID: "rhel8_64Guest"}); !got.Passed {
		t.Errorf("expected pass for rhel8_64Guest, got %+v", got)
	}
	if got := rule.Evaluate(VM{GuestOSID: "rhel6_64Guest"}); got.Passed {
		t.Errorf("expected fail for legacy rhel6_64Guest, got %+v", got)
	}
}

func TestSRC019_DoNotTouchNaming(t *testing.T) {
	rule := findRule(t, "SRC-019")
	if got := rule.Evaluate(VM{Name: "erp-app-01"}); !got.Passed {
		t.Errorf("expected pass, got %+v", got)
	}
	if got := rule.Evaluate(VM{Name: "dnd-oracle-rac1"}); got.Passed {
		t.Errorf("expected fail for dnd-prefixed name, got %+v", got)
	}
}
