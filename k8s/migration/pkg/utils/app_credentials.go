package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/applicationcredentials"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
)

// AppCredentialDetails holds the metadata vjailbreak needs from a fetched
// Keystone Application Credential. ExpiresAt may be nil for credentials
// without an expiration.
type AppCredentialDetails struct {
	ID        string
	ExpiresAt *time.Time
}

// ExpirationWindow days thresholds for the Expiring Condition reasons.
const (
	ExpirationWindow30Days = 30 * 24 * time.Hour
	ExpirationWindow7Days  = 7 * 24 * time.Hour
)

// EvaluateAppCredExpiration classifies an Application Credential's expiration
// state relative to now. Returns the boolean flags used to drive the Expiring
// and Expired Conditions on the OpenstackCreds status. A nil expiresAt (the
// credential never expires) returns all-false.
func EvaluateAppCredExpiration(expiresAt *time.Time, now time.Time) (within30, within7, expired bool) {
	if expiresAt == nil {
		return false, false, false
	}
	if !now.Before(*expiresAt) {
		return false, false, true
	}
	remaining := expiresAt.Sub(now)
	if remaining <= ExpirationWindow7Days {
		return true, true, false
	}
	if remaining <= ExpirationWindow30Days {
		return true, false, false
	}
	return false, false, false
}

// FetchAppCredentialDetails queries Keystone for the Application Credential
// referenced by the authenticated session and returns its metadata. The caller
// supplies an already-authenticated ProviderClient and the credential ID from
// the clouds.yaml auth block.
//
// Returns nil, nil when the AppCred metadata cannot be retrieved for benign
// reasons (e.g., insufficient permission to read application credentials); the
// caller treats this as "details unavailable, skip Expiring/Expired conditions".
// Returns nil, error for unexpected failures the caller may want to log.
func FetchAppCredentialDetails(ctx context.Context, providerClient *gophercloud.ProviderClient, credentialID string) (*AppCredentialDetails, error) {
	if providerClient == nil {
		return nil, fmt.Errorf("provider client is nil")
	}
	if credentialID == "" {
		return nil, fmt.Errorf("application credential id is empty")
	}

	authResult := providerClient.GetAuthResult()
	if authResult == nil {
		return nil, fmt.Errorf("provider client has no auth result; was Authenticate called?")
	}
	createResult, ok := authResult.(tokens.CreateResult)
	if !ok {
		// Token-create auth result not available (e.g., admin auth path); skip.
		return nil, nil //nolint:nilnil // intentional: details unavailable, not an error
	}
	user, err := createResult.ExtractUser()
	if err != nil {
		return nil, fmt.Errorf("extract user from auth result: %w", err)
	}

	identityClient, err := openstack.NewIdentityV3(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, fmt.Errorf("create identity v3 client: %w", err)
	}

	appCred, err := applicationcredentials.Get(ctx, identityClient, user.ID, credentialID).Extract()
	if err != nil {
		// 403 here typically means the credential lacks identity:get_application_credential
		// scope. Authentication itself succeeded, so we treat this as "details
		// unavailable" rather than a hard failure.
		return nil, fmt.Errorf("fetch application credential %s for user %s: %w", credentialID, user.ID, err)
	}

	details := &AppCredentialDetails{ID: appCred.ID}
	if appCred.ExpiresAt != (time.Time{}) {
		expires := appCred.ExpiresAt
		details.ExpiresAt = &expires
	}
	return details, nil
}

// MapKeystoneError maps a gophercloud authentication error to one of the
// Condition Reason codes defined in this package. The mapping is heuristic —
// gophercloud surfaces HTTP status via typed errors when available and via
// string matching as a fallback.
func MapKeystoneError(err error) string {
	if err == nil {
		return ReasonAuthSucceeded
	}

	var errDefault gophercloud.ErrUnexpectedResponseCode
	if errors.As(err, &errDefault) {
		switch errDefault.Actual {
		case http.StatusUnauthorized:
			return ReasonCredentialInvalidOrRevoked
		case http.StatusForbidden:
			return ReasonInsufficientRoles
		case http.StatusNotFound:
			return ReasonKeystoneUnreachable
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"), strings.Contains(msg, "Unauthorized"), strings.Contains(msg, "revoked"):
		return ReasonCredentialInvalidOrRevoked
	case strings.Contains(msg, "403"), strings.Contains(msg, "Forbidden"):
		return ReasonInsufficientRoles
	case strings.Contains(msg, "404"), strings.Contains(msg, "auth_url"), strings.Contains(msg, "Auth URL"):
		return ReasonKeystoneUnreachable
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "no such host"), strings.Contains(msg, "connection refused"):
		return ReasonKeystoneUnreachable
	case strings.Contains(msg, "x509"), strings.Contains(msg, "certificate"), strings.Contains(msg, "TLS"):
		return ReasonTLSVerificationFailed
	}
	return ReasonCredentialInvalidOrRevoked
}
