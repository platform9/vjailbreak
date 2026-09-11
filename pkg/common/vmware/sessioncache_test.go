package vmware

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

// newFakeClient returns a distinguishable, non-nil *vim25.Client without
// making any network calls, so cache tests never talk to a real vCenter.
func newFakeClient() *vim25.Client {
	return &vim25.Client{Client: &soap.Client{}}
}

func TestCredentialFingerprint(t *testing.T) {
	base := CredentialFingerprint("host", "user", "pass", false, "dc1")

	tests := []struct {
		name string
		got  string
	}{
		{"same inputs", CredentialFingerprint("host", "user", "pass", false, "dc1")},
		{"different password", CredentialFingerprint("host", "user", "newpass", false, "dc1")},
		{"different username", CredentialFingerprint("host", "newuser", "pass", false, "dc1")},
		{"different host", CredentialFingerprint("newhost", "user", "pass", false, "dc1")},
		{"different insecure flag", CredentialFingerprint("host", "user", "pass", true, "dc1")},
		{"different datacenter", CredentialFingerprint("host", "user", "pass", false, "dc2")},
	}

	if tests[0].got != base {
		t.Fatalf("identical inputs must produce identical fingerprints")
	}
	for _, tt := range tests[1:] {
		if tt.got == base {
			t.Errorf("%s: expected fingerprint to differ from base, got same value", tt.name)
		}
	}
}

func TestClientCache_GetMissOnEmptyCache(t *testing.T) {
	c := NewClientCache()
	if _, ok := c.Get(context.Background(), "uid-1", "fp-1"); ok {
		t.Fatal("expected miss on empty cache")
	}
}

func TestClientCache_HitWhenFingerprintMatchesAndSessionAlive(t *testing.T) {
	c := NewClientCache()
	c.checkSession = func(context.Context, *vim25.Client) bool { return true }
	logoutCalls := 0
	c.logout = func(context.Context, *vim25.Client) { logoutCalls++ }

	client := newFakeClient()
	ctx := context.Background()
	c.Store(ctx, "uid-1", "fp-1", client)

	got, ok := c.Get(ctx, "uid-1", "fp-1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != client {
		t.Fatal("expected cached client to be returned")
	}
	if logoutCalls != 0 {
		t.Fatalf("expected no logout on a fresh store+hit, got %d", logoutCalls)
	}
}

func TestClientCache_MissAndEvictOnFingerprintMismatch(t *testing.T) {
	// This is the regression case for the "revalidate succeeds after password
	// change" bug: a cached session that vCenter still considers alive must
	// NOT be returned once the credential fingerprint (e.g. password) changes.
	c := NewClientCache()
	c.checkSession = func(context.Context, *vim25.Client) bool { return true } // old session still "alive" server-side
	logoutCalls := 0
	c.logout = func(context.Context, *vim25.Client) { logoutCalls++ }

	ctx := context.Background()
	oldClient := newFakeClient()
	c.Store(ctx, "uid-1", "fp-old-password", oldClient)

	got, ok := c.Get(ctx, "uid-1", "fp-new-password")
	if ok {
		t.Fatal("expected miss when fingerprint no longer matches (password changed)")
	}
	if got != nil {
		t.Fatal("expected nil client on miss")
	}
	if logoutCalls != 1 {
		t.Fatalf("expected stale client to be logged out exactly once, got %d", logoutCalls)
	}

	// Entry must actually be evicted, not just skipped.
	if _, ok := c.entries.Load("uid-1"); ok {
		t.Fatal("expected stale entry to be removed from the cache")
	}
}

func TestClientCache_MissAndEvictWhenSessionExpired(t *testing.T) {
	c := NewClientCache()
	c.checkSession = func(context.Context, *vim25.Client) bool { return false }
	logoutCalls := 0
	c.logout = func(context.Context, *vim25.Client) { logoutCalls++ }

	ctx := context.Background()
	c.Store(ctx, "uid-1", "fp-1", newFakeClient())

	if _, ok := c.Get(ctx, "uid-1", "fp-1"); ok {
		t.Fatal("expected miss when the cached session has expired")
	}
	if logoutCalls != 1 {
		t.Fatalf("expected expired session to be logged out, got %d calls", logoutCalls)
	}
}

func TestClientCache_StoreLogsOutPreviousClientForSameUID(t *testing.T) {
	c := NewClientCache()
	loggedOut := []*vim25.Client{}
	c.logout = func(_ context.Context, client *vim25.Client) { loggedOut = append(loggedOut, client) }

	ctx := context.Background()
	first := newFakeClient()
	second := newFakeClient()

	c.Store(ctx, "uid-1", "fp-1", first)
	c.Store(ctx, "uid-1", "fp-2", second)

	if len(loggedOut) != 1 || loggedOut[0] != first {
		t.Fatalf("expected the first client to be logged out exactly once when replaced, got %v", loggedOut)
	}
}

func TestClientCache_Delete(t *testing.T) {
	c := NewClientCache()
	logoutCalls := 0
	c.logout = func(context.Context, *vim25.Client) { logoutCalls++ }

	ctx := context.Background()
	c.Store(ctx, "uid-1", "fp-1", newFakeClient())
	c.Delete(ctx, "uid-1")

	if _, ok := c.entries.Load("uid-1"); ok {
		t.Fatal("expected entry to be removed after Delete")
	}
	if logoutCalls != 1 {
		t.Fatalf("expected Delete to log out the cached client, got %d calls", logoutCalls)
	}

	// Deleting an already-absent uid must be a safe no-op.
	c.Delete(ctx, "uid-1")
	if logoutCalls != 1 {
		t.Fatalf("expected Delete on a missing uid not to call logout again, got %d calls", logoutCalls)
	}
}

func TestClientCache_IndependentUIDs(t *testing.T) {
	c := NewClientCache()
	c.checkSession = func(context.Context, *vim25.Client) bool { return true }
	c.logout = func(context.Context, *vim25.Client) {}

	ctx := context.Background()
	clientA := newFakeClient()
	clientB := newFakeClient()
	c.Store(ctx, "uid-a", "fp-a", clientA)
	c.Store(ctx, "uid-b", "fp-b", clientB)

	gotA, ok := c.Get(ctx, "uid-a", "fp-a")
	if !ok || gotA != clientA {
		t.Fatal("expected uid-a to resolve to clientA")
	}
	gotB, ok := c.Get(ctx, "uid-b", "fp-b")
	if !ok || gotB != clientB {
		t.Fatal("expected uid-b to resolve to clientB")
	}
}
