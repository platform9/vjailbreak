package utils

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2"
)

func TestEvaluateAppCredExpiration(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		expiresAt  *time.Time
		want30     bool
		want7      bool
		wantExpired bool
	}{
		{
			name:      "never expires (nil)",
			expiresAt: nil,
		},
		{
			name:        "already expired (1 hour ago)",
			expiresAt:   timePtr(now.Add(-1 * time.Hour)),
			wantExpired: true,
		},
		{
			name:        "exactly now -> expired",
			expiresAt:   timePtr(now),
			wantExpired: true,
		},
		{
			name:      "1 day from now -> within7 + within30",
			expiresAt: timePtr(now.Add(24 * time.Hour)),
			want30:    true,
			want7:     true,
		},
		{
			name:      "7 days exactly -> within7 + within30 (boundary inclusive)",
			expiresAt: timePtr(now.Add(7 * 24 * time.Hour)),
			want30:    true,
			want7:     true,
		},
		{
			name:      "8 days from now -> within30 only",
			expiresAt: timePtr(now.Add(8 * 24 * time.Hour)),
			want30:    true,
		},
		{
			name:      "30 days exactly -> within30 (boundary inclusive)",
			expiresAt: timePtr(now.Add(30 * 24 * time.Hour)),
			want30:    true,
		},
		{
			name:      "31 days from now -> neither flag",
			expiresAt: timePtr(now.Add(31 * 24 * time.Hour)),
		},
		{
			name:      "1 year from now -> neither flag",
			expiresAt: timePtr(now.Add(365 * 24 * time.Hour)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got30, got7, gotExpired := EvaluateAppCredExpiration(tc.expiresAt, now)
			if got30 != tc.want30 || got7 != tc.want7 || gotExpired != tc.wantExpired {
				t.Errorf("EvaluateAppCredExpiration(%v, %v) = (within30=%v, within7=%v, expired=%v); want (%v, %v, %v)",
					tc.expiresAt, now, got30, got7, gotExpired, tc.want30, tc.want7, tc.wantExpired)
			}
		})
	}
}

func TestMapKeystoneError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil -> AuthSucceeded", nil, ReasonAuthSucceeded},
		{"typed 401 -> CredentialInvalidOrRevoked", &gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusUnauthorized}, ReasonCredentialInvalidOrRevoked},
		{"typed 403 -> InsufficientRoles", &gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusForbidden}, ReasonInsufficientRoles},
		{"typed 404 -> KeystoneUnreachable", &gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}, ReasonKeystoneUnreachable},
		{"string 401 -> CredentialInvalidOrRevoked", errors.New("auth failed: 401 Unauthorized"), ReasonCredentialInvalidOrRevoked},
		{"string Unauthorized -> CredentialInvalidOrRevoked", errors.New("got Unauthorized response"), ReasonCredentialInvalidOrRevoked},
		{"revoked -> CredentialInvalidOrRevoked", errors.New("application credential is revoked"), ReasonCredentialInvalidOrRevoked},
		{"string 403 -> InsufficientRoles", errors.New("403 Forbidden, missing role"), ReasonInsufficientRoles},
		{"string Forbidden -> InsufficientRoles", errors.New("Forbidden"), ReasonInsufficientRoles},
		{"string 404 -> KeystoneUnreachable", errors.New("got 404 from keystone"), ReasonKeystoneUnreachable},
		{"timeout -> KeystoneUnreachable", errors.New("connection timeout"), ReasonKeystoneUnreachable},
		{"no such host -> KeystoneUnreachable", errors.New("dial tcp: no such host"), ReasonKeystoneUnreachable},
		{"connection refused -> KeystoneUnreachable", errors.New("connection refused"), ReasonKeystoneUnreachable},
		{"x509 cert -> TLSVerificationFailed", errors.New("x509: certificate signed by unknown authority"), ReasonTLSVerificationFailed},
		{"TLS handshake -> TLSVerificationFailed", errors.New("TLS handshake failure"), ReasonTLSVerificationFailed},
		{"unknown -> CredentialInvalidOrRevoked default", errors.New("totally unexpected error"), ReasonCredentialInvalidOrRevoked},
		{"wrapped 401 -> CredentialInvalidOrRevoked", fmt.Errorf("authenticate: %w", &gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusUnauthorized}), ReasonCredentialInvalidOrRevoked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MapKeystoneError(tc.err); got != tc.want {
				t.Errorf("MapKeystoneError(%v) = %q; want %q", tc.err, got, tc.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
