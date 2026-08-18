package server

import (
	"context"
	"fmt"
	"sync"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newTestLockManager gives each test its own lock table instead of sharing
// the package-level proxyVMLocks singleton.
func newTestLockManager() *proxyVMLockManager {
	return &proxyVMLockManager{locks: make(map[string]proxyVMLockEntry)}
}

func withK8sClient(t *testing.T, c client.Client) {
	t.Helper()
	orig := proxyVMLockK8sClient
	proxyVMLockK8sClient = c
	t.Cleanup(func() { proxyVMLockK8sClient = orig })
}

func TestProxyVMLockManager_AcquireWhenFree(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil) // lock starts free, so acquire never needs to consult staleness

	acquired, holder := m.acquire(context.Background(), "proxy-1", "migration-a")
	if !acquired {
		t.Fatalf("acquire on a free lock: got acquired=false, holder=%q, want acquired=true", holder)
	}
	if holder != "" {
		t.Errorf("acquire on a free lock: got holder=%q, want empty", holder)
	}
}

func TestProxyVMLockManager_ReacquireBySameHolderIsIdempotent(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil)

	if acquired, _ := m.acquire(context.Background(), "proxy-1", "migration-a"); !acquired {
		t.Fatal("first acquire unexpectedly failed")
	}
	acquired, holder := m.acquire(context.Background(), "proxy-1", "migration-a")
	if !acquired {
		t.Fatalf("re-acquire by the current holder: got acquired=false, holder=%q, want acquired=true", holder)
	}
}

func TestProxyVMLockManager_AcquireByDifferentMigrationBlocksWhileHeld(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil)

	if acquired, _ := m.acquire(context.Background(), "proxy-1", "migration-a"); !acquired {
		t.Fatal("first acquire unexpectedly failed")
	}

	acquired, holder := m.acquire(context.Background(), "proxy-1", "migration-b")
	if acquired {
		t.Fatal("migration-b acquired a lock still held by migration-a")
	}
	if holder != "migration-a" {
		t.Errorf("holder = %q, want %q", holder, "migration-a")
	}
}

func TestProxyVMLockManager_AcquireAfterReleaseSucceeds(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil)

	if acquired, _ := m.acquire(context.Background(), "proxy-1", "migration-a"); !acquired {
		t.Fatal("first acquire unexpectedly failed")
	}
	m.release("proxy-1", "migration-a")

	acquired, holder := m.acquire(context.Background(), "proxy-1", "migration-b")
	if !acquired {
		t.Fatalf("acquire after release: got acquired=false, holder=%q, want acquired=true", holder)
	}
}

func TestProxyVMLockManager_ReleaseByNonHolderIsNoop(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil)

	if acquired, _ := m.acquire(context.Background(), "proxy-1", "migration-a"); !acquired {
		t.Fatal("first acquire unexpectedly failed")
	}
	m.release("proxy-1", "migration-b") // migration-b never held it

	acquired, holder := m.acquire(context.Background(), "proxy-1", "migration-c")
	if acquired {
		t.Fatal("lock was cleared by a release call from a non-holder (migration-b)")
	}
	if holder != "migration-a" {
		t.Errorf("holder = %q, want %q (a release from a non-holder must be a no-op)", holder, "migration-a")
	}
}

func TestProxyVMLockManager_ReleaseTwiceIsSafe(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil)

	if acquired, _ := m.acquire(context.Background(), "proxy-1", "migration-a"); !acquired {
		t.Fatal("first acquire unexpectedly failed")
	}
	m.release("proxy-1", "migration-a")
	m.release("proxy-1", "migration-a") // must not panic on an already-released lock

	if acquired, _ := m.acquire(context.Background(), "proxy-1", "migration-b"); !acquired {
		t.Fatal("acquire after double release unexpectedly failed")
	}
}

// Regression test for the bug this lock fixes: N migrations racing to
// attach disks to the same shared Proxy VM must yield exactly one winner.
func TestProxyVMLockManager_ConcurrentAcquire_OnlyOneWins(t *testing.T) {
	m := newTestLockManager()
	withK8sClient(t, nil)

	const racers = 50
	results := make([]bool, racers)
	holders := make([]string, racers)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			acquired, holder := m.acquire(context.Background(), "proxy-shared", fmt.Sprintf("migration-%d", i))
			results[i] = acquired
			holders[i] = holder
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for _, acquired := range results {
		if acquired {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("%d of %d goroutines acquired the same Proxy VM lock concurrently, want exactly 1 (results=%v, holders=%v)",
			winners, racers, results, holders)
	}
}

func TestMigrationHolderIsStale(t *testing.T) {
	failedMigration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-failed", Namespace: migrationSystemNamespace},
		Status:     vjailbreakv1alpha1.MigrationStatus{Phase: vjailbreakv1alpha1.VMMigrationPhaseFailed},
	}
	runningMigration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{Name: "migration-running", Namespace: migrationSystemNamespace},
		Status:     vjailbreakv1alpha1.MigrationStatus{Phase: vjailbreakv1alpha1.VMMigrationPhaseCopying},
	}

	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to register vjailbreakv1alpha1 scheme: %v", err)
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(failedMigration, runningMigration).
		Build()

	tests := []struct {
		name          string
		client        client.Client
		migrationName string
		want          bool
	}{
		{
			name:          "nil k8s client defaults to not stale",
			client:        nil,
			migrationName: "migration-running",
			want:          false,
		},
		{
			name:          "migration object gone is stale",
			client:        fakeClient,
			migrationName: "migration-deleted-or-never-existed",
			want:          true,
		},
		{
			name:          "migration in Failed phase is stale",
			client:        fakeClient,
			migrationName: "migration-failed",
			want:          true,
		},
		{
			name:          "migration in a live (non-Failed) phase is not stale",
			client:        fakeClient,
			migrationName: "migration-running",
			want:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withK8sClient(t, tc.client)
			got := migrationHolderIsStale(context.Background(), tc.migrationName)
			if got != tc.want {
				t.Errorf("migrationHolderIsStale(ctx, %q) = %v, want %v", tc.migrationName, got, tc.want)
			}
		})
	}
}
