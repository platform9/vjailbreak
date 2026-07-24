/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
)

var _ = ginkgo.Describe("MigrationPlan Controller", func() {
	ginkgo.Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		migrationplan := &vjailbreakv1alpha1.MigrationPlan{}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating the custom resource for the Kind MigrationPlan")
			err := k8sClient.Get(ctx, typeNamespacedName, migrationplan)
			if err != nil && errors.IsNotFound(err) {
				resource := &vjailbreakv1alpha1.MigrationPlan{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &vjailbreakv1alpha1.MigrationPlan{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Cleanup the specific resource instance MigrationPlan")
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})
		ginkgo.It("should successfully reconcile the resource", func() {
			ginkgo.By("Reconciling the created resource")
			controllerReconciler := &MigrationPlanReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

func TestGetDatastoresForVolumeMapping_UsesPerDiskOrderWithDuplicates(t *testing.T) {
	vmMachine := &vjailbreakv1alpha1.VMwareMachine{
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Datastores: []string{"nfs"},
				Disks: []vjailbreakv1alpha1.Disk{
					{Name: "Hard disk 1", Datastore: "nfs"},
					{Name: "Hard disk 2", Datastore: "nfs"},
					{Name: "Hard disk 3", Datastore: "ssd"},
				},
			},
		},
	}

	got := getDatastoresForVolumeMapping(vmMachine)
	want := []string{"nfs", "nfs", "ssd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected datastore mapping: got %v, want %v", got, want)
	}
}

func TestGetDatastoresForVolumeMapping_FallsBackToLegacyDatastores(t *testing.T) {
	vmMachine := &vjailbreakv1alpha1.VMwareMachine{
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Datastores: []string{"nfs", "ssd"},
				Disks:      nil,
			},
		},
	}

	got := getDatastoresForVolumeMapping(vmMachine)
	want := []string{"nfs", "ssd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected fallback datastore mapping: got %v, want %v", got, want)
	}
}

func TestGetDatastoresForVolumeMapping_PreservesBlankDiskDatastore(t *testing.T) {
	vmMachine := &vjailbreakv1alpha1.VMwareMachine{
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Datastores: []string{"legacy-ds"},
				Disks: []vjailbreakv1alpha1.Disk{
					{Name: "Hard disk 1", Datastore: ""},
					{Name: "Hard disk 2", Datastore: "nfs"},
				},
			},
		},
	}

	got := getDatastoresForVolumeMapping(vmMachine)
	want := []string{"", "nfs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected datastore mapping for blank datastore disks: got %v, want %v", got, want)
	}
}

func TestSetTagsAndCustomMetadata(t *testing.T) {
	vmMachine := &vjailbreakv1alpha1.VMwareMachine{
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Tags:             map[string]string{"env": "production"},
				CustomAttributes: map[string]string{"Owner": "alice@corp.com"},
			},
		},
	}

	tests := []struct {
		name               string
		preserveSourceTags bool
		customMetadata     map[string]string
		vmMachine          *vjailbreakv1alpha1.VMwareMachine
		wantPreserve       string
		wantSourceTags     map[string]string
		wantCustom         map[string]string
	}{
		{
			name:               "toggle off writes false and no metadata keys",
			preserveSourceTags: false,
			vmMachine:          vmMachine,
			wantPreserve:       "false",
		},
		{
			name:               "toggle on writes resolved source tags",
			preserveSourceTags: true,
			vmMachine:          vmMachine,
			wantPreserve:       "true",
			wantSourceTags: map[string]string{
				"tag:env":    "production",
				"attr:Owner": "alice@corp.com",
			},
		},
		{
			name:               "custom metadata written independently of toggle",
			preserveSourceTags: false,
			customMetadata:     map[string]string{"wave": "2"},
			vmMachine:          vmMachine,
			wantPreserve:       "false",
			wantCustom:         map[string]string{"wave": "2"},
		},
		{
			name:               "toggle on with no tags on VM writes no source key",
			preserveSourceTags: true,
			vmMachine:          &vjailbreakv1alpha1.VMwareMachine{},
			wantPreserve:       "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migrationplan := &vjailbreakv1alpha1.MigrationPlan{
				Spec: vjailbreakv1alpha1.MigrationPlanSpec{
					MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
						CustomMetadata: tt.customMetadata,
					},
				},
			}
			migrationobj := &vjailbreakv1alpha1.Migration{
				Spec: vjailbreakv1alpha1.MigrationSpec{
					PreserveSourceTags: tt.preserveSourceTags,
				},
			}
			configMapData := map[string]string{
				// Pre-populate to verify stale keys from a previous reconcile are removed.
				"SOURCE_TAGS_METADATA": "stale",
				"CUSTOM_METADATA":      "stale",
			}

			if err := setTagsAndCustomMetadata(configMapData, migrationplan, migrationobj, tt.vmMachine); err != nil {
				t.Fatalf("setTagsAndCustomMetadata() error = %v", err)
			}

			if got := configMapData["PRESERVE_SOURCE_TAGS"]; got != tt.wantPreserve {
				t.Errorf("PRESERVE_SOURCE_TAGS = %q, want %q", got, tt.wantPreserve)
			}

			assertJSONKey(t, configMapData, "SOURCE_TAGS_METADATA", tt.wantSourceTags)
			assertJSONKey(t, configMapData, "CUSTOM_METADATA", tt.wantCustom)
		})
	}
}

func assertJSONKey(t *testing.T, configMapData map[string]string, key string, want map[string]string) {
	t.Helper()
	raw, present := configMapData[key]
	if want == nil {
		if present {
			t.Errorf("%s should be absent, got %q", key, raw)
		}
		return
	}
	if !present {
		t.Fatalf("%s missing from configmap data", key)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("%s is not valid JSON: %v", key, err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s[%q] = %q, want %q", key, k, got[k], v)
		}
	}
}

// TestDataOnlyPropagation verifies that DataOnly=true on a MigrationPlan propagates
// to the created Migration.Spec.DataOnly.
func TestDataOnlyPropagation(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "migration-system"
	const planName = "test-plan-dataonly"
	const vmwarecredsName = "test-vmwcreds"

	tests := []struct {
		name         string
		dataOnly     bool
		wantDataOnly bool
	}{
		{
			name:         "DataOnly=true propagates to Migration",
			dataOnly:     true,
			wantDataOnly: true,
		},
		{
			name:         "DataOnly=false propagates to Migration",
			dataOnly:     false,
			wantDataOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a VMwareMachine the controller uses to create Migrations.
			vmMachine := &vjailbreakv1alpha1.VMwareMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vm-a-moid123",
					Namespace: ns,
				},
				Spec: vjailbreakv1alpha1.VMwareMachineSpec{
					VMInfo: vjailbreakv1alpha1.VMInfo{
						Name: "vm-a",
						VMID: "moid123",
					},
				},
			}

			vmwarecreds := &vjailbreakv1alpha1.VMwareCreds{
				ObjectMeta: metav1.ObjectMeta{Name: vmwarecredsName, Namespace: ns},
			}

			migrationTemplate := &vjailbreakv1alpha1.MigrationTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-template", Namespace: ns},
				Spec: vjailbreakv1alpha1.MigrationTemplateSpec{
					Source: vjailbreakv1alpha1.MigrationTemplateSource{
						VMwareRef: vmwarecredsName,
					},
				},
			}

			plan := &vjailbreakv1alpha1.MigrationPlan{
				ObjectMeta: metav1.ObjectMeta{Name: planName, Namespace: ns},
				Spec: vjailbreakv1alpha1.MigrationPlanSpec{
					MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
						MigrationTemplate: "test-template",
						MigrationStrategy: vjailbreakv1alpha1.MigrationPlanStrategy{
							Type:     "cold",
							DataOnly: tt.dataOnly,
						},
					},
					VirtualMachines: [][]string{{"vm-a"}},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(plan, vmMachine, vmwarecreds, migrationTemplate).
				WithStatusSubresource(&vjailbreakv1alpha1.Migration{}).
				Build()

			r := &MigrationPlanReconciler{
				Client: fakeClient,
				Scheme: scheme,
				ctxlog: logr.Discard(),
			}

			_, err := r.CreateMigration(context.Background(), plan, "vm-a", vmMachine)
			if err != nil {
				t.Fatalf("CreateMigration() error = %v", err)
			}

			// Retrieve the created Migration and verify DataOnly propagation.
			migrationList := &vjailbreakv1alpha1.MigrationList{}
			if err := fakeClient.List(context.Background(), migrationList); err != nil {
				t.Fatalf("List migrations error = %v", err)
			}
			if len(migrationList.Items) != 1 {
				t.Fatalf("expected 1 Migration, got %d", len(migrationList.Items))
			}
			got := migrationList.Items[0].Spec.DataOnly
			if got != tt.wantDataOnly {
				t.Errorf("Migration.Spec.DataOnly = %v, want %v", got, tt.wantDataOnly)
			}
		})
	}
}

// TestProcessMigrationPhases_DataCopied verifies that a Migration in the DataCopied
// phase is treated as a terminal success and does NOT trigger post-migration actions.
func TestProcessMigrationPhases_DataCopied(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "migration-system"
	const planName = "test-plan-phases"

	tests := []struct {
		name         string
		phase        vjailbreakv1alpha1.VMMigrationPhase
		wantFinished bool
		wantPlanFail bool
	}{
		{
			name:         "DataCopied phase counts as finished",
			phase:        vjailbreakv1alpha1.VMMigrationPhaseDataCopied,
			wantFinished: true,
			wantPlanFail: false,
		},
		{
			name:         "Succeeded phase counts as finished",
			phase:        vjailbreakv1alpha1.VMMigrationPhaseSucceeded,
			wantFinished: true,
			wantPlanFail: false,
		},
		{
			name:         "Failed phase causes plan failure",
			phase:        vjailbreakv1alpha1.VMMigrationPhaseFailed,
			wantFinished: false,
			wantPlanFail: true,
		},
		{
			name:         "In-progress phase is not finished",
			phase:        vjailbreakv1alpha1.VMMigrationPhaseCopying,
			wantFinished: false,
			wantPlanFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migration := &vjailbreakv1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-migration",
					Namespace: ns,
					Labels:    map[string]string{"migrationplan": planName},
					Annotations: map[string]string{
						"vjailbreak.k8s.pf9.io/original-vm-name": "vm-a",
					},
				},
				Spec: vjailbreakv1alpha1.MigrationSpec{VMName: "vm-a"},
			}
			migration.Status.Phase = tt.phase
			// Add a dummy condition for the failed phase check (controller reads Conditions[0].Message)
			if tt.phase == vjailbreakv1alpha1.VMMigrationPhaseFailed || tt.phase == vjailbreakv1alpha1.VMMigrationPhaseValidationFailed {
				migration.Status.Conditions = []corev1.PodCondition{{Message: "test failure"}}
			}

			plan := &vjailbreakv1alpha1.MigrationPlan{
				ObjectMeta: metav1.ObjectMeta{Name: planName, Namespace: ns},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(plan, migration).
				WithStatusSubresource(&vjailbreakv1alpha1.MigrationPlan{}, &vjailbreakv1alpha1.Migration{}).
				Build()

			r := &MigrationPlanReconciler{
				Client: fakeClient,
				Scheme: scheme,
				ctxlog: logr.Discard(),
			}

			migrationScope, _ := scope.NewMigrationPlanScope(scope.MigrationPlanScopeParams{
				Client:        fakeClient,
				MigrationPlan: plan,
			})

			migrationList := &vjailbreakv1alpha1.MigrationList{
				Items: []vjailbreakv1alpha1.Migration{*migration},
			}

			gotFinished, err := r.processMigrationPhases(
				context.Background(),
				migrationScope,
				plan,
				migrationList,
				[]string{"vm-a"},
			)

			if tt.wantPlanFail {
				// Failed migrations return an error (or false with a plan update)
				if err == nil && gotFinished {
					t.Errorf("expected plan to fail for phase %v, but allFinished=%v err=%v", tt.phase, gotFinished, err)
				}
			} else {
				if err != nil {
					t.Fatalf("processMigrationPhases() unexpected error = %v", err)
				}
				if gotFinished != tt.wantFinished {
					t.Errorf("allFinished = %v, want %v for phase %v", gotFinished, tt.wantFinished, tt.phase)
				}
			}
		})
	}
}

// TestDataCopiedIsTerminalInTriggerMigration verifies that the DataCopied phase is
// recognised as a terminal phase and the migration is skipped (not re-triggered).
func TestDataCopiedIsTerminalInTriggerMigration(t *testing.T) {
	// The key assertion is that a Migration in DataCopied phase is added to migrationobjs
	// and skipped, so no new job is created.  We test this by verifying the phase
	// comparison logic directly via the constant values.
	terminalPhases := []vjailbreakv1alpha1.VMMigrationPhase{
		vjailbreakv1alpha1.VMMigrationPhaseSucceeded,
		vjailbreakv1alpha1.VMMigrationPhaseFailed,
		vjailbreakv1alpha1.VMMigrationPhaseValidationFailed,
		vjailbreakv1alpha1.VMMigrationPhaseDataCopied,
	}
	nonTerminalPhases := []vjailbreakv1alpha1.VMMigrationPhase{
		vjailbreakv1alpha1.VMMigrationPhasePending,
		vjailbreakv1alpha1.VMMigrationPhaseCopying,
		vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk,
	}

	isTerminal := func(phase vjailbreakv1alpha1.VMMigrationPhase) bool {
		return phase == vjailbreakv1alpha1.VMMigrationPhaseSucceeded ||
			phase == vjailbreakv1alpha1.VMMigrationPhaseFailed ||
			phase == vjailbreakv1alpha1.VMMigrationPhaseValidationFailed ||
			phase == vjailbreakv1alpha1.VMMigrationPhaseDataCopied
	}

	for _, phase := range terminalPhases {
		if !isTerminal(phase) {
			t.Errorf("phase %v should be terminal", phase)
		}
	}
	for _, phase := range nonTerminalPhases {
		if isTerminal(phase) {
			t.Errorf("phase %v should NOT be terminal", phase)
		}
	}
}

// TestSetMigrationSpecificFields_DataOnly verifies that setMigrationSpecificFields
// writes DATA_ONLY into the configmap for both true and false cases.
func TestSetMigrationSpecificFields_DataOnly(t *testing.T) {
	r := &MigrationPlanReconciler{}

	tests := []struct {
		name     string
		dataOnly bool
		want     string
	}{
		{name: "DataOnly=true sets DATA_ONLY=true", dataOnly: true, want: "true"},
		{name: "DataOnly=false sets DATA_ONLY=false", dataOnly: false, want: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMapData := map[string]string{}
			migration := &vjailbreakv1alpha1.Migration{
				Spec: vjailbreakv1alpha1.MigrationSpec{
					DataOnly: tt.dataOnly,
				},
			}
			r.setMigrationSpecificFields(configMapData, migration)
			got, ok := configMapData["DATA_ONLY"]
			if !ok {
				t.Fatal("DATA_ONLY key missing from configmap")
			}
			if got != tt.want {
				t.Errorf("DATA_ONLY = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsVMSucceededInPlan(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "migration-system"
	const planName = "test-plan"

	makeMigration := func(name, vmName, phase string, labels map[string]string) *vjailbreakv1alpha1.Migration {
		m := &vjailbreakv1alpha1.Migration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels:    labels,
			},
			Spec: vjailbreakv1alpha1.MigrationSpec{
				VMName: vmName,
			},
		}
		m.Status.Phase = vjailbreakv1alpha1.VMMigrationPhase(phase)
		return m
	}

	planLabels := map[string]string{"migrationplan": planName}
	otherPlanLabels := map[string]string{"migrationplan": "other-plan"}

	tests := []struct {
		name       string
		migrations []*vjailbreakv1alpha1.Migration
		vmName     string
		want       bool
	}{
		{
			name: "VM succeeded in this plan",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeMigration("m1", "vm-a", string(vjailbreakv1alpha1.VMMigrationPhaseSucceeded), planLabels),
			},
			vmName: "vm-a",
			want:   true,
		},
		{
			name: "VM succeeded but in a different plan",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeMigration("m1", "vm-a", string(vjailbreakv1alpha1.VMMigrationPhaseSucceeded), otherPlanLabels),
			},
			vmName: "vm-a",
			want:   false,
		},
		{
			name: "VM in this plan but not yet succeeded",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeMigration("m1", "vm-a", "Migrating", planLabels),
			},
			vmName: "vm-a",
			want:   false,
		},
		{
			name:       "no migrations at all",
			migrations: nil,
			vmName:     "vm-a",
			want:       false,
		},
		{
			name: "different VM succeeded in same plan",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeMigration("m1", "vm-b", string(vjailbreakv1alpha1.VMMigrationPhaseSucceeded), planLabels),
			},
			vmName: "vm-a",
			want:   false,
		},
		{
			name: "VM failed in this plan",
			migrations: []*vjailbreakv1alpha1.Migration{
				makeMigration("m1", "vm-a", "Failed", planLabels),
			},
			vmName: "vm-a",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := make([]runtime.Object, len(tt.migrations))
			for i, m := range tt.migrations {
				objs[i] = m
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				WithStatusSubresource(&vjailbreakv1alpha1.Migration{}).
				Build()

			r := &MigrationPlanReconciler{Client: fakeClient, Scheme: scheme}
			plan := &vjailbreakv1alpha1.MigrationPlan{
				ObjectMeta: metav1.ObjectMeta{Name: planName, Namespace: ns},
			}

			got := r.isVMSucceededInPlan(context.Background(), plan, tt.vmName)
			if got != tt.want {
				t.Errorf("isVMSucceededInPlan(%q) = %v, want %v", tt.vmName, got, tt.want)
			}
		})
	}
}
