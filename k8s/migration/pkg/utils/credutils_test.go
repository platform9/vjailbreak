package utils

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	netutils "github.com/platform9/vjailbreak/pkg/common/utils"
	vmwarecommon "github.com/platform9/vjailbreak/pkg/common/vmware"
	"github.com/vmware/govmomi/vim25"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	return scheme
}

func TestCreateOrUpdateVMwareMachine_CreatesWhenMissing(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "vmware", Namespace: constants.NamespaceMigrationSystem},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&vjailbreakv1alpha1.VMwareMachine{}).
		Build()

	vminfo := &vjailbreakv1alpha1.VMInfo{
		Name:        "win2k12-vjtest",
		VMID:        "vm-3539",
		ESXiName:    "10.96.11.240",
		ClusterName: "cell-1",
		VMState:     "running",
	}

	if err := CreateOrUpdateVMwareMachine(ctx, k8sClient, vmwcreds, vminfo, "prison"); err != nil {
		t.Fatalf("CreateOrUpdateVMwareMachine failed: %v", err)
	}

	vmName, err := netutils.GetVMK8sCompatibleName(vminfo.Name, vminfo.VMID, vmwcreds.Name)
	if err != nil {
		t.Fatalf("failed to compute vm name: %v", err)
	}
	got := &vjailbreakv1alpha1.VMwareMachine{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: vmName, Namespace: constants.NamespaceMigrationSystem}, got); err != nil {
		t.Fatalf("expected VMwareMachine to be created: %v", err)
	}
}

func TestShouldSkipVMwareMachineReconciliation_WhenMigrationExists(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	vmwvm := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "centos7-vm-4369-hash", Namespace: constants.NamespaceMigrationSystem},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "centos7-vm", VMID: "vm-4369"},
		},
	}
	migration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migration-centos7-vm-4369-hash",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "centos7-vm-4369",
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vmwvm, migration).
		Build()

	skip, reason, err := ShouldSkipVMwareMachineReconciliation(
		ctx, k8sClient,
		k8stypes.NamespacedName{Name: vmwvm.Name, Namespace: vmwvm.Namespace},
		vmwvm,
	)
	if err != nil {
		t.Fatalf("ShouldSkipVMwareMachineReconciliation failed: %v", err)
	}
	if !skip {
		t.Fatalf("expected skip=true, got false (reason=%q)", reason)
	}
}

func TestFindVMwareMachinesNotInVcenter_SkipsMachineWithMigration(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "vmware", Namespace: constants.NamespaceMigrationSystem},
	}

	protectedVM := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "centos7-vm-4369-hash",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.VMwareCredsLabel: vmwcreds.Name,
			},
		},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "centos7-vm", VMID: "vm-4369"},
		},
	}
	ordinaryVM := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ordinary-vm-0001",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.VMwareCredsLabel: vmwcreds.Name,
			},
		},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "ordinary-vm"},
		},
	}
	migration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migration-centos7-vm-4369-hash",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "centos7-vm-4369",
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(protectedVM, ordinaryVM, migration).
		Build()

	stale, err := FindVMwareMachinesNotInVcenter(ctx, k8sClient, vmwcreds, nil)
	if err != nil {
		t.Fatalf("FindVMwareMachinesNotInVcenter failed: %v", err)
	}
	if len(stale) != 1 || stale[0].Name != ordinaryVM.Name {
		t.Fatalf("expected only ordinary VM to be stale, got %#v", stale)
	}
}

// TestMigrationMatchesVMwareMachine_AnnotationPath verifies the new primary path: when
// OriginalVMNameAnnotation is set (post-fix CRs), it is used for matching even when the
// VM name contains spaces that would be illegal in a label value.
func TestMigrationMatchesVMwareMachine_AnnotationPath(t *testing.T) {
	rawKey := "civ6s622 - GPU Terminal Svr VM-503b0"
	vmwvm := &vjailbreakv1alpha1.VMwareMachine{
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "civ6s622 - GPU Terminal Svr VM", VMID: "vm-503b0"},
		},
	}
	migration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.OriginalVMNameAnnotation: rawKey,
			},
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "civ6s622---GPU-Terminal-Svr-VM-503b0",
			},
		},
	}
	if !migrationMatchesVMwareMachine(migration, vmwvm, rawKey) {
		t.Error("expected annotation path to match VM with spaces in name")
	}
}

// TestMigrationMatchesVMwareMachine_AnnotationTakesPrecedenceOverLabel verifies that when
// both annotation and label are present, the annotation (raw key) is used for comparison.
func TestMigrationMatchesVMwareMachine_AnnotationTakesPrecedenceOverLabel(t *testing.T) {
	rawKey := "my vm-abc12"
	vmwvm := &vjailbreakv1alpha1.VMwareMachine{
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "my vm", VMID: "vm-abc12"},
		},
	}
	// annotation holds raw key; label holds sanitized value — they differ
	migration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.OriginalVMNameAnnotation: rawKey,
			},
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "my-vm-abc12",
			},
		},
	}
	// matching against raw key must succeed
	if !migrationMatchesVMwareMachine(migration, vmwvm, rawKey) {
		t.Error("expected annotation to take precedence and match raw key")
	}
	// matching against sanitized label value must NOT succeed (raw key != sanitized)
	if migrationMatchesVMwareMachine(migration, vmwvm, "my-vm-abc12") {
		t.Error("expected sanitized label value to not match when annotation holds different raw key")
	}
}

// TestShouldSkipVMwareMachineReconciliation_WhenMigrationExistsWithAnnotation verifies that
// ShouldSkipVMwareMachineReconciliation returns skip=true when the matching migration uses
// the new annotation-based path (VM name with spaces).
func TestShouldSkipVMwareMachineReconciliation_WhenMigrationExistsWithAnnotation(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	vmwvm := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "civ6s622---gpu-terminal-svr-vm-503b0-cf786", Namespace: constants.NamespaceMigrationSystem},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "civ6s622 - GPU Terminal Svr VM", VMID: "vm-503b0"},
		},
	}
	rawKey := "civ6s622 - GPU Terminal Svr VM-503b0"
	migration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migration-civ6s622---gpu-terminal-svr-vm-503b0-cf786",
			Namespace: constants.NamespaceMigrationSystem,
			Annotations: map[string]string{
				constants.OriginalVMNameAnnotation: rawKey,
			},
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "civ6s622---GPU-Terminal-Svr-VM-503b0",
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vmwvm, migration).
		Build()

	skip, reason, err := ShouldSkipVMwareMachineReconciliation(
		ctx, k8sClient,
		k8stypes.NamespacedName{Name: vmwvm.Name, Namespace: vmwvm.Namespace},
		vmwvm,
	)
	if err != nil {
		t.Fatalf("ShouldSkipVMwareMachineReconciliation failed: %v", err)
	}
	if !skip {
		t.Fatalf("expected skip=true for VM with spaces in name via annotation path, got false (reason=%q)", reason)
	}
}

func TestCleanupCachedVMwareClient_NoEntry_NoPanic(t *testing.T) {
	ctx := context.Background()
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vmware-cleanup-noop",
			Namespace: constants.NamespaceMigrationSystem,
			UID:       k8stypes.UID("uid-cleanup-noop"),
		},
	}

	// Must be a safe no-op when nothing was ever cached for this UID.
	CleanupCachedVMwareClient(ctx, vmwcreds)
}

func TestCleanupCachedVMwareClient_EvictsCachedEntry(t *testing.T) {
	ctx := context.Background()
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vmware-cleanup",
			Namespace: constants.NamespaceMigrationSystem,
			UID:       k8stypes.UID("uid-cleanup-test"),
		},
	}
	uid := string(vmwcreds.UID)
	vmwareClients.Store(ctx, uid, "fp-1", &vim25.Client{})

	CleanupCachedVMwareClient(ctx, vmwcreds)

	if _, ok := vmwareClients.Get(ctx, uid, "fp-1"); ok {
		t.Fatal("expected CleanupCachedVMwareClient to evict the cached entry for this UID")
	}
}

// TestValidateVMwareCreds_CacheMissesOnPasswordChange is a regression test for
// the bug where revalidating VMwareCreds after a password change still
// reported success: the cache was keyed only by the CR's (immutable) UID, so
// a still-alive cached vCenter session masked the fact that the new password
// was never actually checked. The cache key must change whenever the
// credentials do, so a rotated password always forces a fresh login instead
// of reusing a session authenticated with the old password.
func TestValidateVMwareCreds_CacheMissesOnPasswordChange(t *testing.T) {
	ctx := context.Background()
	uid := "uid-password-rotation-test"
	oldFingerprint := vmwarecommon.CredentialFingerprint("host", "user", "old-password", false, "dc1")
	newFingerprint := vmwarecommon.CredentialFingerprint("host", "user", "new-password", false, "dc1")

	if oldFingerprint == newFingerprint {
		t.Fatal("expected fingerprint to change when the password changes")
	}

	vmwareClients.Store(ctx, uid, oldFingerprint, &vim25.Client{})

	if _, ok := vmwareClients.Get(ctx, uid, newFingerprint); ok {
		t.Fatal("expected cache miss when the fingerprint no longer matches (password changed) - " +
			"a hit here would reproduce the revalidate-succeeds-after-password-change bug")
	}
}
