// Package utils — conditions helpers for the OpenstackCreds CRD.
//
// Defines the Condition Type and Reason constants that the controller writes
// into OpenstackCreds.status.conditions per the contract documented at
// specs/003-clouds-yaml-credentials/contracts/conditions.md, and provides
// thin helpers around k8s.io/apimachinery/pkg/api/meta.SetStatusCondition for
// the common patterns.
package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition Types reported on OpenstackCreds.status.conditions.
const (
	// ConditionCredentialsParsed indicates that the credential data was parsed
	// successfully from either clouds.yaml or the legacy OS_* keys.
	ConditionCredentialsParsed = "CredentialsParsed"

	// ConditionCredentialsValidated indicates that authentication against the
	// destination Keystone endpoint succeeded.
	ConditionCredentialsValidated = "CredentialsValidated"

	// ConditionRolesSufficient indicates that the authenticated principal
	// carries the role set vjailbreak requires for destination operations.
	ConditionRolesSufficient = "RolesSufficient"

	// ConditionExpiring indicates that an Application Credential is approaching
	// its expires_at (within 30 days, or within 7 days as a stronger warning).
	ConditionExpiring = "Expiring"

	// ConditionExpired indicates that an Application Credential has passed its
	// expires_at and authentication will fail on the next attempt.
	ConditionExpired = "Expired"
)

// Reason codes for ConditionCredentialsParsed.
const (
	ReasonParsed                 = "Parsed"
	ReasonInvalidYAML            = "InvalidYAML"
	ReasonMissingRequiredField   = "MissingRequiredField"
	ReasonAmbiguousCloudName     = "AmbiguousCloudName"
	ReasonUnknownAuthType        = "UnknownAuthType"
	ReasonCacertPathUnresolvable = "CacertPathUnresolvable"
)

// Reason codes for ConditionCredentialsValidated.
const (
	ReasonAuthSucceeded = "AuthSucceeded"
	// ReasonCredentialInvalidOrRevoked is the Reason for an authentication
	// failure where the supplied credential is rejected or revoked.
	//nolint:gosec // G101: the literal is a Reason code, not a credential value.
	ReasonCredentialInvalidOrRevoked = "CredentialInvalidOrRevoked"
	ReasonKeystoneUnreachable        = "KeystoneUnreachable"
	ReasonTLSVerificationFailed      = "TLSVerificationFailed"
)

// Reason codes for ConditionRolesSufficient.
const (
	ReasonRolesPresent     = "RolesPresent"
	ReasonInsufficientRoles = "InsufficientRoles"
)

// Reason codes for ConditionExpiring.
const (
	ReasonWithin30Days  = "Within30Days"
	ReasonWithin7Days   = "Within7Days"
	ReasonNotApplicable = "NotApplicable"
)

// Reason codes for ConditionExpired.
const (
	ReasonExpired = "Expired"
	ReasonActive  = "Active"
)

// SetCondition is a stub. It is intended to wrap meta.SetStatusCondition for
// the common case where the caller supplies Type, Status, Reason, and Message.
// Implementation lives in a follow-up commit; this stub exists only to allow
// the test file to compile during the TDD red phase.
func SetCondition(conditions *[]metav1.Condition, conditionType string, status metav1.ConditionStatus, reason, message string) {
	_ = conditions
	_ = conditionType
	_ = status
	_ = reason
	_ = message
}
