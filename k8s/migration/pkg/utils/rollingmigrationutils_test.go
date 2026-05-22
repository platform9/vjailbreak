package utils

import (
	"context"
	"strings"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func makePlanScope(t *testing.T, credsName, templateName, ns string,
	clusters []vjailbreakv1alpha1.ClusterMigrationInfo,
	vms []*vjailbreakv1alpha1.VMwareMachine,
) *scope.RollingMigrationPlanScope {
	t.Helper()
	scheme := testScheme(t)

	plan := &vjailbreakv1alpha1.RollingMigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan", Namespace: ns},
		Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
			ClusterSequence: clusters,
			MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
				MigrationTemplate: templateName,
			},
		},
	}
	template := &vjailbreakv1alpha1.MigrationTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: templateName, Namespace: ns},
		Spec: vjailbreakv1alpha1.MigrationTemplateSpec{
			Source: vjailbreakv1alpha1.MigrationTemplateSource{VMwareRef: credsName},
		},
	}
	creds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: credsName, Namespace: ns},
	}

	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(plan, template, creds)
	for _, vm := range vms {
		builder = builder.WithObjects(vm)
	}

	s, err := scope.NewRollingMigrationPlanScope(scope.RollingMigrationPlanScopeParams{
		Client:               builder.Build(),
		RollingMigrationPlan: plan,
	})
	if err != nil {
		t.Fatalf("failed to create scope: %v", err)
	}
	return s
}

func TestUpdateESXiNamesInRollingMigrationPlan(t *testing.T) {
	ns := constants.NamespaceMigrationSystem

	// Helper: build a VMwareMachine whose k8s name doesn't match what
	// GetK8sCompatibleVMWareObjectName would generate — this proves the fix
	// no longer relies on name reconstruction.
	makeMachine := func(displayName, esxi string) *vjailbreakv1alpha1.VMwareMachine {
		return &vjailbreakv1alpha1.VMwareMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      displayName + "-moidhash", // deliberately different from old lookup
				Namespace: ns,
				Labels:    map[string]string{constants.VMwareCredsLabel: "vmware-creds"},
			},
			Spec: vjailbreakv1alpha1.VMwareMachineSpec{
				VMInfo: vjailbreakv1alpha1.VMInfo{Name: displayName, ESXiName: esxi},
			},
		}
	}

	tests := []struct {
		name        string
		clusters    []vjailbreakv1alpha1.ClusterMigrationInfo
		vms         []*vjailbreakv1alpha1.VMwareMachine
		wantESXi    map[string]string // vmName -> expected ESXiName
		wantErr     bool
		errContains string
	}{
		{
			name: "populates ESXi names from VMwareMachine objects",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "ubuntu-1"},
					{VMName: "ubuntu-2"},
				}},
			},
			vms: []*vjailbreakv1alpha1.VMwareMachine{
				makeMachine("ubuntu-1", "esxi-host-1"),
				makeMachine("ubuntu-2", "esxi-host-2"),
			},
			wantESXi: map[string]string{"ubuntu-1": "esxi-host-1", "ubuntu-2": "esxi-host-2"},
		},
		{
			name: "returns error when VMwareMachine not found for a VM",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "missing-vm"},
				}},
			},
			wantErr:     true,
			errContains: "missing-vm",
		},
		{
			name: "handles multiple clusters",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{{VMName: "vm-a"}}},
				{ClusterName: "cluster-b", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{{VMName: "vm-b"}}},
			},
			vms: []*vjailbreakv1alpha1.VMwareMachine{
				makeMachine("vm-a", "esxi-1"),
				makeMachine("vm-b", "esxi-2"),
			},
			wantESXi: map[string]string{"vm-a": "esxi-1", "vm-b": "esxi-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := makePlanScope(t, "vmware-creds", "test-template", ns, tt.clusters, tt.vms)

			err := UpdateESXiNamesInRollingMigrationPlan(context.Background(), s)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for i, cluster := range s.RollingMigrationPlan.Spec.ClusterSequence {
				for j, vm := range cluster.VMSequence {
					if want, ok := tt.wantESXi[vm.VMName]; ok && vm.ESXiName != want {
						t.Errorf("cluster[%d].VMSequence[%d] (%s): ESXiName = %q, want %q",
							i, j, vm.VMName, vm.ESXiName, want)
					}
				}
			}
		})
	}
}
