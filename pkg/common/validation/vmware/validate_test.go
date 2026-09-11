package vmware

import (
	"context"
	"fmt"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// stubLoginAndLogout overrides loginToVCenter/logoutOfVCenter for the
// duration of a test and restores the real implementations afterward.
func stubLoginAndLogout(t *testing.T, login func(ctx context.Context, s *cache.Session, c *vim25.Client) error) *int {
	t.Helper()
	origLogin, origLogout := loginToVCenter, logoutOfVCenter
	logoutCalls := 0
	loginToVCenter = login
	logoutOfVCenter = func(context.Context, *vim25.Client) { logoutCalls++ }
	t.Cleanup(func() {
		loginToVCenter = origLogin
		logoutOfVCenter = origLogout
	})
	return &logoutCalls
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add vjailbreak scheme: %v", err)
	}
	return scheme
}

// newTestVMwareCreds returns a VMwareCreds pointing at a Secret seeded with
// the given password (no datacenter, so the finder.Datacenter lookup - which
// needs a real vCenter connection - is never exercised).
func newTestVMwareCreds(t *testing.T, password string) (*vjailbreakv1alpha1.VMwareCreds, client.Client) {
	t.Helper()
	scheme := newTestScheme(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "vmware-secret", Namespace: namespaceMigrationSystem},
		Data: map[string][]byte{
			"VCENTER_HOST":     []byte("vcenter.example.com"),
			"VCENTER_USERNAME": []byte("admin"),
			"VCENTER_PASSWORD": []byte(password),
			"VCENTER_INSECURE": []byte("true"),
		},
	}
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "vmware", Namespace: namespaceMigrationSystem},
		Spec:       vjailbreakv1alpha1.VMwareCredsSpec{SecretRef: corev1.ObjectReference{Name: "vmware-secret"}},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, vmwcreds).Build()
	return vmwcreds, k8sClient
}

func TestValidate_Success(t *testing.T) {
	vmwcreds, k8sClient := newTestVMwareCreds(t, "correct-password")
	logoutCalls := stubLoginAndLogout(t, func(context.Context, *cache.Session, *vim25.Client) error {
		return nil
	})

	result := Validate(context.Background(), k8sClient, vmwcreds, 1)

	if !result.Valid {
		t.Fatalf("expected Valid=true, got %+v", result)
	}
	if *logoutCalls != 1 {
		t.Fatalf("expected exactly one logout after a successful login, got %d", *logoutCalls)
	}
}

// TestValidate_InvalidCredentials is the regression test for the reported
// bug: a real login attempt with the wrong password must be reported as
// Invalid, not masked by a previously cached session.
func TestValidate_InvalidCredentials(t *testing.T) {
	vmwcreds, k8sClient := newTestVMwareCreds(t, "wrong-password")
	logoutCalls := stubLoginAndLogout(t, func(context.Context, *cache.Session, *vim25.Client) error {
		return fmt.Errorf("ServerFaultCode: Cannot complete login due to an incorrect user name or password")
	})

	result := Validate(context.Background(), k8sClient, vmwcreds, 3)

	if result.Valid {
		t.Fatalf("expected Valid=false for a wrong password, got %+v", result)
	}
	if result.Message != "Authentication failed: invalid username or password. Please verify your credentials" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if *logoutCalls != 0 {
		t.Fatalf("expected no logout when login never succeeded, got %d calls", *logoutCalls)
	}
}

// TestValidate_NoCrossCallCaching is the core regression test: back-to-back
// calls to Validate() must each reflect the CURRENT login outcome. Before
// the fix, a cached/still-alive vCenter session from a prior successful
// call could make a later call report Valid=true even though the password
// had since changed (a live session ticket survives a password change
// server-side). Validate() no longer caches anything, so the second call
// here - simulating "password changed since the last check" - must fail
// even though the first call just succeeded.
func TestValidate_NoCrossCallCaching(t *testing.T) {
	vmwcreds, k8sClient := newTestVMwareCreds(t, "original-password")

	stubLoginAndLogout(t, func(context.Context, *cache.Session, *vim25.Client) error {
		return nil
	})
	first := Validate(context.Background(), k8sClient, vmwcreds, 1)
	if !first.Valid {
		t.Fatalf("expected first call to succeed, got %+v", first)
	}

	stubLoginAndLogout(t, func(context.Context, *cache.Session, *vim25.Client) error {
		return fmt.Errorf("ServerFaultCode: Cannot complete login due to an incorrect user name or password")
	})
	second := Validate(context.Background(), k8sClient, vmwcreds, 1)
	if second.Valid {
		t.Fatal("expected second call to fail after the password changed - " +
			"a Valid=true here would reproduce the stale-cached-session bug")
	}
}

func TestValidate_RetriesTransientErrorThenSucceeds(t *testing.T) {
	vmwcreds, k8sClient := newTestVMwareCreds(t, "correct-password")
	attempts := 0
	stubLoginAndLogout(t, func(context.Context, *cache.Session, *vim25.Client) error {
		attempts++
		if attempts == 1 {
			return fmt.Errorf("connection reset by peer")
		}
		return nil
	})

	result := Validate(context.Background(), k8sClient, vmwcreds, 3)

	if !result.Valid {
		t.Fatalf("expected eventual success after a transient error, got %+v", result)
	}
	if attempts != 2 {
		t.Fatalf("expected exactly 2 login attempts, got %d", attempts)
	}
}

func TestValidate_FailsAfterExhaustingRetries(t *testing.T) {
	vmwcreds, k8sClient := newTestVMwareCreds(t, "correct-password")
	attempts := 0
	stubLoginAndLogout(t, func(context.Context, *cache.Session, *vim25.Client) error {
		attempts++
		return fmt.Errorf("connection reset by peer")
	})

	// maxRetries=1 keeps this test instant: the backoff sleep is only
	// reached when attempt < retryLimit, which never holds for a limit of 1.
	result := Validate(context.Background(), k8sClient, vmwcreds, 1)

	if result.Valid {
		t.Fatalf("expected failure once retries are exhausted, got %+v", result)
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 login attempt for maxRetries=1, got %d", attempts)
	}
}

func TestValidate_MissingSecret(t *testing.T) {
	scheme := newTestScheme(t)
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "vmware", Namespace: namespaceMigrationSystem},
		Spec:       vjailbreakv1alpha1.VMwareCredsSpec{SecretRef: corev1.ObjectReference{Name: "does-not-exist"}},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vmwcreds).Build()

	result := Validate(context.Background(), k8sClient, vmwcreds, 1)

	if result.Valid {
		t.Fatalf("expected failure for a missing secret, got %+v", result)
	}
}
