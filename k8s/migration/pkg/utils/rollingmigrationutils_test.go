package utils

import (
	"context"
	"reflect"
	"strings"
	"testing"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
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

// makeClusterScope builds a minimal ClusterMigrationScope for use in unit tests.
// It only populates the fields required by the functions under test.
func makeClusterScope(ns string, plan *vjailbreakv1alpha1.RollingMigrationPlan, c fake.ClientBuilder) *scope.ClusterMigrationScope {
	s, _ := scope.NewClusterMigrationScope(scope.ClusterMigrationScopeParams{
		Client:               c.Build(),
		ClusterMigration:     &vjailbreakv1alpha1.ClusterMigration{ObjectMeta: metav1.ObjectMeta{Namespace: ns}},
		RollingMigrationPlan: plan,
	})
	return s
}

func TestConvertVMSequenceToBatches(t *testing.T) {
	tests := []struct {
		name               string
		clusters           []vjailbreakv1alpha1.ClusterMigrationInfo
		vmKeyByDisplayName map[string]string
		batchSize          int
		wantBatches        [][]string
	}{
		{
			name: "uses vmid-keyed names from map",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "ubuntu-1"},
					{VMName: "ubuntu-2"},
				}},
			},
			vmKeyByDisplayName: map[string]string{
				"ubuntu-1": "ubuntu-1-180",
				"ubuntu-2": "ubuntu-2-182",
			},
			batchSize:   10,
			wantBatches: [][]string{{"ubuntu-1-180", "ubuntu-2-182"}},
		},
		{
			name: "falls back to display name when not in map",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "unknown-vm"},
				}},
			},
			vmKeyByDisplayName: map[string]string{},
			batchSize:          10,
			wantBatches:        [][]string{{"unknown-vm"}},
		},
		{
			name: "respects batch size",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "vm-a"},
					{VMName: "vm-b"},
					{VMName: "vm-c"},
				}},
			},
			vmKeyByDisplayName: map[string]string{
				"vm-a": "vm-a-100",
				"vm-b": "vm-b-101",
				"vm-c": "vm-c-102",
			},
			batchSize:   2,
			wantBatches: [][]string{{"vm-a-100", "vm-b-101"}, {"vm-c-102"}},
		},
		{
			name: "multiple clusters are batched independently",
			clusters: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{ClusterName: "cluster-a", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "vm-a"},
				}},
				{ClusterName: "cluster-b", VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "vm-b"},
				}},
			},
			vmKeyByDisplayName: map[string]string{
				"vm-a": "vm-a-100",
				"vm-b": "vm-b-101",
			},
			batchSize:   10,
			wantBatches: [][]string{{"vm-a-100"}, {"vm-b-101"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &vjailbreakv1alpha1.RollingMigrationPlan{
				Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
					ClusterSequence: tt.clusters,
				},
			}
			s := &scope.ClusterMigrationScope{RollingMigrationPlan: plan}

			got := convertVMSequenceToBatches(s, tt.batchSize, tt.vmKeyByDisplayName)

			if !reflect.DeepEqual(got, tt.wantBatches) {
				t.Errorf("convertVMSequenceToBatches() = %v, want %v", got, tt.wantBatches)
			}
		})
	}
}

func TestConvertVMSequenceToMigrationPlans_UsesVMIDKeyedNames(t *testing.T) {
	ns := constants.NamespaceMigrationSystem
	credsName := "vmware-creds"
	templateName := "test-template"

	makeMachineWithVMID := func(displayName, vmid, esxi string) *vjailbreakv1alpha1.VMwareMachine {
		return &vjailbreakv1alpha1.VMwareMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      displayName + "-" + strings.TrimPrefix(vmid, "vm-") + "-hash",
				Namespace: ns,
				Labels:    map[string]string{constants.VMwareCredsLabel: credsName},
			},
			Spec: vjailbreakv1alpha1.VMwareMachineSpec{
				VMInfo: vjailbreakv1alpha1.VMInfo{
					Name:     displayName,
					VMID:     vmid,
					ESXiName: esxi,
				},
			},
		}
	}

	plan := &vjailbreakv1alpha1.RollingMigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rmp", Namespace: ns},
		Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
			MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
				MigrationTemplate: templateName,
			},
			ClusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{
					ClusterName:          "cluster-01",
					VMMigrationBatchSize: 10,
					VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
						{VMName: "ubuntu-1"},
						{VMName: "ubuntu-2"},
					},
				},
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
	vm1 := makeMachineWithVMID("ubuntu-1", "vm-180", "10.9.26.32")
	vm2 := makeMachineWithVMID("ubuntu-2", "vm-182", "10.9.26.31")

	scheme := testScheme(t)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(plan, template, creds, vm1, vm2).
		Build()

	s, err := scope.NewClusterMigrationScope(scope.ClusterMigrationScopeParams{
		Client:               fakeClient,
		ClusterMigration:     &vjailbreakv1alpha1.ClusterMigration{ObjectMeta: metav1.ObjectMeta{Namespace: ns}},
		RollingMigrationPlan: plan,
	})
	if err != nil {
		t.Fatalf("failed to create scope: %v", err)
	}

	if err := ConvertVMSequenceToMigrationPlans(context.Background(), s, 10); err != nil {
		t.Fatalf("ConvertVMSequenceToMigrationPlans() error: %v", err)
	}

	// Verify the created MigrationPlan has vmid-keyed names, not display names.
	mpList := &vjailbreakv1alpha1.MigrationPlanList{}
	if err := fakeClient.List(context.Background(), mpList); err != nil {
		t.Fatalf("failed to list MigrationPlans: %v", err)
	}
	if len(mpList.Items) == 0 {
		t.Fatal("expected at least one MigrationPlan to be created")
	}

	mp := mpList.Items[0]
	if len(mp.Spec.VirtualMachines) == 0 || len(mp.Spec.VirtualMachines[0]) == 0 {
		t.Fatal("MigrationPlan has empty VirtualMachines")
	}

	batch := mp.Spec.VirtualMachines[0]
	wantKeys := map[string]bool{"ubuntu-1-180": true, "ubuntu-2-182": true}
	for _, vmKey := range batch {
		if !wantKeys[vmKey] {
			t.Errorf("MigrationPlan VirtualMachines contains %q; want vmid-keyed names %v", vmKey, wantKeys)
		}
	}
	// Verify display names are NOT present.
	displayNames := map[string]bool{"ubuntu-1": true, "ubuntu-2": true}
	for _, vmKey := range batch {
		if displayNames[vmKey] {
			t.Errorf("MigrationPlan VirtualMachines contains display name %q instead of vmid-keyed name", vmKey)
		}
	}

	// Verify VMMigrationPlans is persisted in the API server, not just in-memory.
	// This is the regression test for the bug where the list was appended in-memory
	// but never persisted, causing the rolling migration plan controller to see an
	// empty list and log "No MigrationPlans found".
	persistedPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
	if err := fakeClient.Get(context.Background(), k8stypes.NamespacedName{Name: plan.Name, Namespace: ns}, persistedPlan); err != nil {
		t.Fatalf("failed to get persisted RollingMigrationPlan: %v", err)
	}
	if len(persistedPlan.Spec.VMMigrationPlans) == 0 {
		t.Error("VMMigrationPlans not persisted: spec.vMMigrationPlans is empty in API server")
	}
	if persistedPlan.Spec.VMMigrationPlans[0] != mp.Name {
		t.Errorf("VMMigrationPlans[0] = %q, want %q", persistedPlan.Spec.VMMigrationPlans[0], mp.Name)
	}
}

func TestConvertVMSequenceToMigrationPlans_PropagatesTagsAndCustomMetadata(t *testing.T) {
	ns := constants.NamespaceMigrationSystem
	credsName := "vmware-creds"
	templateName := "test-template"

	plan := &vjailbreakv1alpha1.RollingMigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-rmp-tags", Namespace: ns},
		Spec: vjailbreakv1alpha1.RollingMigrationPlanSpec{
			MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
				MigrationTemplate:  templateName,
				PreserveSourceTags: true,
				CustomMetadata:     map[string]string{"wave": "2", "migrated_by": "vjailbreak"},
			},
			ClusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{
					ClusterName:          "cluster-01",
					VMMigrationBatchSize: 10,
					VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
						{VMName: "ubuntu-1"},
					},
				},
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
	vm1 := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ubuntu-1-180-hash",
			Namespace: ns,
			Labels:    map[string]string{constants.VMwareCredsLabel: credsName},
		},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Name:     "ubuntu-1",
				VMID:     "vm-180",
				ESXiName: "10.9.26.32",
			},
		},
	}

	scheme := testScheme(t)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(plan, template, creds, vm1).
		Build()

	s, err := scope.NewClusterMigrationScope(scope.ClusterMigrationScopeParams{
		Client:               fakeClient,
		ClusterMigration:     &vjailbreakv1alpha1.ClusterMigration{ObjectMeta: metav1.ObjectMeta{Namespace: ns}},
		RollingMigrationPlan: plan,
	})
	if err != nil {
		t.Fatalf("failed to create scope: %v", err)
	}

	if err := ConvertVMSequenceToMigrationPlans(context.Background(), s, 10); err != nil {
		t.Fatalf("ConvertVMSequenceToMigrationPlans() error: %v", err)
	}

	mpList := &vjailbreakv1alpha1.MigrationPlanList{}
	if err := fakeClient.List(context.Background(), mpList); err != nil {
		t.Fatalf("failed to list MigrationPlans: %v", err)
	}
	if len(mpList.Items) == 0 {
		t.Fatal("expected at least one MigrationPlan to be created")
	}

	mp := mpList.Items[0]
	if !mp.Spec.PreserveSourceTags {
		t.Error("PreserveSourceTags not propagated from RollingMigrationPlan to batch MigrationPlan")
	}
	wantMetadata := map[string]string{"wave": "2", "migrated_by": "vjailbreak"}
	if len(mp.Spec.CustomMetadata) != len(wantMetadata) {
		t.Fatalf("CustomMetadata = %v, want %v", mp.Spec.CustomMetadata, wantMetadata)
	}
	for k, v := range wantMetadata {
		if mp.Spec.CustomMetadata[k] != v {
			t.Errorf("CustomMetadata[%q] = %q, want %q", k, mp.Spec.CustomMetadata[k], v)
		}
	}
}

func TestFilterHostsForMigrationPlan(t *testing.T) {
	makeHost := func(name string) vjailbreakv1alpha1.VMwareHost {
		return vjailbreakv1alpha1.VMwareHost{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       vjailbreakv1alpha1.VMwareHostSpec{Name: name},
		}
	}
	makeMachine := func(displayName, esxi string) vjailbreakv1alpha1.VMwareMachine {
		return vjailbreakv1alpha1.VMwareMachine{
			Spec: vjailbreakv1alpha1.VMwareMachineSpec{
				VMInfo: vjailbreakv1alpha1.VMInfo{Name: displayName, ESXiName: esxi},
			},
		}
	}

	allHosts := []vjailbreakv1alpha1.VMwareHost{
		makeHost("10.9.26.29"),
		makeHost("10.9.26.31"),
		makeHost("10.9.26.32"),
	}
	allVMs := []vjailbreakv1alpha1.VMwareMachine{
		makeMachine("ubuntu-1", "10.9.26.32"),
		makeMachine("ubuntu-2", "10.9.26.31"),
		makeMachine("other-vm", "10.9.26.29"),
	}

	tests := []struct {
		name            string
		clusterSequence []vjailbreakv1alpha1.ClusterMigrationInfo
		wantHostNames   []string
	}{
		{
			name: "returns only hosts with migration VMs",
			clusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "ubuntu-1"},
					{VMName: "ubuntu-2"},
				}},
			},
			wantHostNames: []string{"10.9.26.32", "10.9.26.31"},
		},
		{
			name: "excludes hosts not in migration plan",
			clusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "ubuntu-1"},
				}},
			},
			wantHostNames: []string{"10.9.26.32"},
		},
		{
			name: "deduplicates hosts when multiple VMs on same esxi",
			clusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "ubuntu-1"},
					{VMName: "ubuntu-1"}, // duplicate
				}},
			},
			wantHostNames: []string{"10.9.26.32"},
		},
		{
			name: "ignores VMs not found in VMwareMachine list",
			clusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{
				{VMSequence: []vjailbreakv1alpha1.VMSequenceInfo{
					{VMName: "missing-vm"},
				}},
			},
			wantHostNames: []string{},
		},
		{
			name:            "returns empty for empty cluster sequence",
			clusterSequence: []vjailbreakv1alpha1.ClusterMigrationInfo{},
			wantHostNames:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterHostsForMigrationPlan(allHosts, allVMs, tt.clusterSequence)

			gotNames := make(map[string]bool, len(got))
			for _, h := range got {
				gotNames[h.Spec.Name] = true
			}
			wantNames := make(map[string]bool, len(tt.wantHostNames))
			for _, n := range tt.wantHostNames {
				wantNames[n] = true
			}
			if !reflect.DeepEqual(gotNames, wantNames) {
				t.Errorf("filterHostsForMigrationPlan() hosts = %v, want %v", gotNames, wantNames)
			}
		})
	}
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
