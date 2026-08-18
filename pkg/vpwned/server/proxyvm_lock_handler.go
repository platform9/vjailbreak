package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// vpwned-sdk runs as a single-replica Deployment, so this in-memory map *is*
// the Proxy VM attach lock -- there's no CRD, no resourceVersion, no
// distributed compare-and-swap. One mutex in one process is genuinely
// exclusive. See v2v-helper/pkg/utils/proxyvm_lock.go for the caller side.

// proxyVMLockEntry records which migration currently holds a Proxy VM's
// attach lock.
type proxyVMLockEntry struct {
	holder     string
	acquiredAt time.Time
}

// proxyVMLockManager guards proxyVMLocks. Every method takes the mutex for
// its entire check-and-set so two concurrent Acquire calls can't both read a
// stale holder and both overwrite it.
type proxyVMLockManager struct {
	mu    sync.Mutex
	locks map[string]proxyVMLockEntry
}

var proxyVMLocks = &proxyVMLockManager{locks: make(map[string]proxyVMLockEntry)}

// proxyVMLockK8sClient is used only to check whether a lock's current holder
// is stale (see migrationHolderIsStale). Set once at startup by InitProxyVMLock.
var proxyVMLockK8sClient client.Client

// InitProxyVMLock creates the k8s client used to detect stale lock holders.
// Mirrors InitK8sProxy/InitDebugBundle's non-cluster-safe init pattern.
func InitProxyVMLock() error {
	k8sClient, err := CreateInClusterClient()
	if err != nil {
		return err
	}
	proxyVMLockK8sClient = k8sClient
	return nil
}

// acquire grants the lock if it's free, already held by this same migration
// (idempotent retry), or held by a migration whose lock is stale. Otherwise
// it reports who's currently holding it.
func (m *proxyVMLockManager) acquire(ctx context.Context, proxyVMName, migrationName string) (acquired bool, holder string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, held := m.locks[proxyVMName]
	if held && entry.holder != migrationName && !migrationHolderIsStale(ctx, entry.holder) {
		return false, entry.holder
	}
	m.locks[proxyVMName] = proxyVMLockEntry{holder: migrationName, acquiredAt: time.Now()}
	return true, ""
}

// release drops the lock only if migrationName is the current holder, so a
// delayed or duplicate release from a migration that already lost the lock
// can't clear someone else's in-progress attach.
func (m *proxyVMLockManager) release(proxyVMName, migrationName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, held := m.locks[proxyVMName]; held && entry.holder == migrationName {
		delete(m.locks, proxyVMName)
	}
}

// migrationHolderIsStale reports whether the migration holding a lock will
// never release it itself: either its Migration object is gone, or it has
// already failed. Any other phase -- including one that hasn't caught up yet
// due to the usual Event -> Reconcile lag -- is treated as still valid, so a
// holder that's genuinely mid-attach is never mistakenly preempted.
//
// On any error checking this (no k8s client, API server hiccup), we default
// to "not stale": it's safer to make a waiter poll a little longer than to
// preempt a lock that's still legitimately held.
func migrationHolderIsStale(ctx context.Context, migrationName string) bool {
	if proxyVMLockK8sClient == nil {
		logrus.Warn("proxyvm-lock: k8s client not initialised, treating lock holder as still valid")
		return false
	}

	migration := &vjailbreakv1alpha1.Migration{}
	key := k8stypes.NamespacedName{Name: migrationName, Namespace: migrationSystemNamespace}
	if err := proxyVMLockK8sClient.Get(ctx, key, migration); err != nil {
		if apierrors.IsNotFound(err) {
			return true
		}
		logrus.Warnf("proxyvm-lock: failed to get Migration %q, treating lock holder as still valid: %v", migrationName, err)
		return false
	}
	return migration.Status.Phase == vjailbreakv1alpha1.VMMigrationPhaseFailed
}

// proxyVMLockRequest is the JSON body both endpoints below expect.
type proxyVMLockRequest struct {
	ProxyVMName   string `json:"proxyVMName"`
	MigrationName string `json:"migrationName"`
}

func decodeProxyVMLockRequest(r *http.Request) (proxyVMLockRequest, error) {
	var req proxyVMLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, fmt.Errorf("invalid request body: %w", err)
	}
	if req.ProxyVMName == "" || req.MigrationName == "" {
		return req, fmt.Errorf("proxyVMName and migrationName are required")
	}
	return req, nil
}

// HandleAcquireProxyVMLock is POST /vpw/v1/proxyvm-lock/acquire.
func HandleAcquireProxyVMLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := decodeProxyVMLockRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	acquired, holder := proxyVMLocks.acquire(r.Context(), req.ProxyVMName, req.MigrationName)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Acquired bool   `json:"acquired"`
		Holder   string `json:"holder,omitempty"`
	}{Acquired: acquired, Holder: holder})
}

// HandleReleaseProxyVMLock is POST /vpw/v1/proxyvm-lock/release. Always
// returns released:true -- releasing a lock you don't hold is a no-op, not
// an error, since a caller may legitimately race a stale-lock takeover.
func HandleReleaseProxyVMLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, err := decodeProxyVMLockRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proxyVMLocks.release(req.ProxyVMName, req.MigrationName)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Released bool `json:"released"`
	}{Released: true})
}
