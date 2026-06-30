package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestConditionTypeConstants pins the user-facing Type strings that downstream
// tooling (dashboards, alerts, kubectl jsonpath) may rely on. Renaming any of
// these is a breaking change to the CRD's status API.
func TestConditionTypeConstants(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"CredentialsParsed", ConditionCredentialsParsed, "CredentialsParsed"},
		{"CredentialsValidated", ConditionCredentialsValidated, "CredentialsValidated"},
		{"RolesSufficient", ConditionRolesSufficient, "RolesSufficient"},
		{"Expiring", ConditionExpiring, "Expiring"},
		{"Expired", ConditionExpired, "Expired"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("Condition Type constant = %q; want %q", c.got, c.want)
			}
		})
	}
}

// TestReasonConstants pins the user-facing Reason strings. See conditions.md
// in the design contracts directory for the contract definition.
func TestReasonConstants(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Parsed", ReasonParsed, "Parsed"},
		{"InvalidYAML", ReasonInvalidYAML, "InvalidYAML"},
		{"MissingRequiredField", ReasonMissingRequiredField, "MissingRequiredField"},
		{"AmbiguousCloudName", ReasonAmbiguousCloudName, "AmbiguousCloudName"},
		{"UnknownAuthType", ReasonUnknownAuthType, "UnknownAuthType"},
		{"CacertPathUnresolvable", ReasonCacertPathUnresolvable, "CacertPathUnresolvable"},

		{"AuthSucceeded", ReasonAuthSucceeded, "AuthSucceeded"},
		{"CredentialInvalidOrRevoked", ReasonCredentialInvalidOrRevoked, "CredentialInvalidOrRevoked"},
		{"KeystoneUnreachable", ReasonKeystoneUnreachable, "KeystoneUnreachable"},
		{"TLSVerificationFailed", ReasonTLSVerificationFailed, "TLSVerificationFailed"},

		{"RolesPresent", ReasonRolesPresent, "RolesPresent"},
		{"InsufficientRoles", ReasonInsufficientRoles, "InsufficientRoles"},

		{"Within30Days", ReasonWithin30Days, "Within30Days"},
		{"Within7Days", ReasonWithin7Days, "Within7Days"},
		{"NotApplicable", ReasonNotApplicable, "NotApplicable"},

		{"Expired", ReasonExpired, "Expired"},
		{"Active", ReasonActive, "Active"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("Reason constant = %q; want %q", c.got, c.want)
			}
		})
	}
}

// TestSetCondition_AddsNewEntry verifies that SetCondition appends a new
// Condition with the supplied Type/Status/Reason/Message when no existing
// Condition of that Type is present.
func TestSetCondition_AddsNewEntry(t *testing.T) {
	var conds []metav1.Condition
	SetCondition(&conds, ConditionCredentialsParsed, metav1.ConditionTrue, ReasonParsed, "parsed clouds.yaml")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition after SetCondition, got %d", len(conds))
	}
	c := conds[0]
	if c.Type != ConditionCredentialsParsed {
		t.Errorf("Type = %q; want %q", c.Type, ConditionCredentialsParsed)
	}
	if c.Status != metav1.ConditionTrue {
		t.Errorf("Status = %q; want True", c.Status)
	}
	if c.Reason != ReasonParsed {
		t.Errorf("Reason = %q; want %q", c.Reason, ReasonParsed)
	}
	if c.Message != "parsed clouds.yaml" {
		t.Errorf("Message = %q; want %q", c.Message, "parsed clouds.yaml")
	}
	if c.LastTransitionTime.IsZero() {
		t.Errorf("LastTransitionTime must be set when adding a new Condition")
	}
}

// TestSetCondition_UpdatesExistingEntry verifies that SetCondition replaces
// the existing Condition of the same Type rather than appending a duplicate,
// and updates LastTransitionTime only when Status changes.
func TestSetCondition_UpdatesExistingEntry(t *testing.T) {
	var conds []metav1.Condition
	SetCondition(&conds, ConditionCredentialsValidated, metav1.ConditionTrue, ReasonAuthSucceeded, "auth ok")
	if len(conds) != 1 {
		t.Fatalf("setup: expected 1 condition, got %d", len(conds))
	}
	firstTransition := conds[0].LastTransitionTime

	// Same Status: LastTransitionTime should NOT change.
	SetCondition(&conds, ConditionCredentialsValidated, metav1.ConditionTrue, ReasonAuthSucceeded, "auth still ok")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition after second SetCondition with same Status, got %d", len(conds))
	}
	if !conds[0].LastTransitionTime.Equal(&firstTransition) {
		t.Errorf("LastTransitionTime changed when Status did not change")
	}

	// Status flip: LastTransitionTime should advance.
	SetCondition(&conds, ConditionCredentialsValidated, metav1.ConditionFalse, ReasonCredentialInvalidOrRevoked, "revoked")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition after Status flip, got %d", len(conds))
	}
	if conds[0].Status != metav1.ConditionFalse {
		t.Errorf("Status = %q; want False", conds[0].Status)
	}
	if conds[0].LastTransitionTime.Equal(&firstTransition) {
		t.Errorf("LastTransitionTime did not advance on Status change")
	}
}
