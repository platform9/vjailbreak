package vmware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
)

var fingerprintKey = mustRandomKey()

func mustRandomKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("vmware: failed to generate credential fingerprint key: " + err.Error())
	}
	return key
}

// CredentialFingerprint returns a stable, keyed hash of the connection-relevant
// vCenter credential fields. It is used as part of a cache key so that any
// change to host, username, password, SSL verification, or datacenter is
// guaranteed to produce a cache miss rather than silently reusing a client
// authenticated with stale credentials. The fingerprint is process-local
// only: it is never compared across restarts, so a random per-process HMAC
// key is used instead of a fixed one.
func CredentialFingerprint(host, username, password string, insecure bool, datacenter string) string {
	mac := hmac.New(sha256.New, fingerprintKey)
	insecureByte := byte(0)
	if insecure {
		insecureByte = 1
	}
	for _, field := range []string{host, username, password, string(insecureByte), datacenter} {
		mac.Write([]byte(field))
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))
}

type cachedEntry struct {
	fingerprint string
	client      *vim25.Client
}

// ClientCache caches authenticated vim25 clients keyed by VMwareCreds UID.
//
// A cached client is only ever returned when its fingerprint matches the
// credentials just read from the Secret AND vCenter still recognizes its
// session ticket. This means a credential change (e.g. a rotated password)
// always forces a fresh login on the next Get/Store cycle instead of
// silently passing validation off a session that was authenticated before
// the change. Callers never need to remember to invalidate the cache when
// credentials change; correctness follows from the fingerprint alone.
//
// The zero value is not usable; construct with NewClientCache.
type ClientCache struct {
	entries sync.Map // uid (string) -> *cachedEntry

	// checkSession and logout are overridable in tests to avoid real
	// network calls to vCenter.
	checkSession func(ctx context.Context, client *vim25.Client) bool
	logout       func(ctx context.Context, client *vim25.Client)
}

// NewClientCache returns an empty, ready-to-use ClientCache.
func NewClientCache() *ClientCache {
	return &ClientCache{
		checkSession: defaultCheckSession,
		logout:       defaultLogout,
	}
}

// Get returns a cached, still-valid client for uid, or (nil, false) if none
// exists, the credentials have changed since it was cached, or the cached
// session has expired. A stale entry is evicted and best-effort logged out
// before returning.
func (c *ClientCache) Get(ctx context.Context, uid, fingerprint string) (*vim25.Client, bool) {
	val, ok := c.entries.Load(uid)
	if !ok {
		return nil, false
	}
	entry, _ := val.(*cachedEntry)
	if entry == nil || entry.client == nil || entry.client.Client == nil || entry.fingerprint != fingerprint {
		c.evict(ctx, uid, entry)
		return nil, false
	}
	if !c.checkSession(ctx, entry.client) {
		c.evict(ctx, uid, entry)
		return nil, false
	}
	return entry.client, true
}

// Store caches client as the authenticated client for uid under fingerprint.
// Any previously cached client for uid is best-effort logged out first.
func (c *ClientCache) Store(ctx context.Context, uid, fingerprint string, newClient *vim25.Client) {
	if val, ok := c.entries.Load(uid); ok {
		if old, _ := val.(*cachedEntry); old != nil && old.client != nil && old.client != newClient {
			c.logout(ctx, old.client)
		}
	}
	c.entries.Store(uid, &cachedEntry{fingerprint: fingerprint, client: newClient})
}

// Delete evicts and best-effort logs out the cached client for uid, if any.
func (c *ClientCache) Delete(ctx context.Context, uid string) {
	val, ok := c.entries.LoadAndDelete(uid)
	if !ok {
		return
	}
	if entry, _ := val.(*cachedEntry); entry != nil && entry.client != nil {
		c.logout(ctx, entry.client)
	}
}

func (c *ClientCache) evict(ctx context.Context, uid string, entry *cachedEntry) {
	c.entries.Delete(uid)
	if entry != nil && entry.client != nil {
		c.logout(ctx, entry.client)
	}
}

func defaultCheckSession(ctx context.Context, client *vim25.Client) bool {
	userSession, err := session.NewManager(client).UserSession(ctx)
	return err == nil && userSession != nil
}

func defaultLogout(ctx context.Context, client *vim25.Client) {
	if client == nil || client.Client == nil {
		return
	}
	_ = session.NewManager(client).Logout(ctx)
}
