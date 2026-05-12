package utils

import (
	"context"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	netutils "github.com/platform9/vjailbreak/pkg/common/utils"
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
		ObjectMeta: metav1.ObjectMeta{Name: "sarika-centos7-vj-test-4369-cf786", Namespace: constants.NamespaceMigrationSystem},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "sarika-centos7-vj-test", VMID: "vm-4369"},
		},
	}
	migration := &vjailbreakv1alpha1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migration-sarika-centos7-vj-test-4369-cf786",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "sarika-centos7-vj-test-4369",
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
			Name:      "sarika-centos7-vj-test-4369-cf786",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.VMwareCredsLabel: vmwcreds.Name,
			},
		},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{Name: "sarika-centos7-vj-test", VMID: "vm-4369"},
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
			Name:      "migration-sarika-centos7-vj-test-4369-cf786",
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.MigrationVMKeyLabel: "sarika-centos7-vj-test-4369",
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
