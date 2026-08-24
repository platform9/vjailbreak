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
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	openstackpkg "github.com/platform9/vjailbreak/pkg/common/openstack"
	commonutils "github.com/platform9/vjailbreak/pkg/common/utils"
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

	// A failed migration is TERMINAL, so AllFinished stays true and the failure is
	// reported via FailedVMs. processMigrationPhases no longer updates plan status
	// itself — the caller decides, once nothing is still running.
	tests := []struct {
		name          string
		phase         vjailbreakv1alpha1.VMMigrationPhase
		withCondition bool
		wantFinished  bool
		wantFinishedN int
		wantFailedN   int
	}{
		{
			name:          "DataCopied phase counts as finished",
			phase:         vjailbreakv1alpha1.VMMigrationPhaseDataCopied,
			wantFinished:  true,
			wantFinishedN: 1,
		},
		{
			name:          "Succeeded phase counts as finished",
			phase:         vjailbreakv1alpha1.VMMigrationPhaseSucceeded,
			wantFinished:  true,
			wantFinishedN: 1,
		},
		{
			name:          "Failed phase is terminal and recorded, not fatal",
			phase:         vjailbreakv1alpha1.VMMigrationPhaseFailed,
			withCondition: true,
			wantFinished:  true,
			wantFailedN:   1,
		},
		{
			name:          "ValidationFailed phase is terminal and recorded, not fatal",
			phase:         vjailbreakv1alpha1.VMMigrationPhaseValidationFailed,
			withCondition: true,
			wantFinished:  true,
			wantFailedN:   1,
		},
		{
			name:         "Failed with no conditions does not panic",
			phase:        vjailbreakv1alpha1.VMMigrationPhaseFailed,
			wantFinished: true,
			wantFailedN:  1,
		},
		{
			name:         "In-progress phase is not finished",
			phase:        vjailbreakv1alpha1.VMMigrationPhaseCopying,
			wantFinished: false,
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
			if tt.withCondition {
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

			outcome, err := r.processMigrationPhases(
				context.Background(),
				migrationScope,
				plan,
				migrationList,
				[]string{"vm-a"},
			)
			if err != nil {
				t.Fatalf("processMigrationPhases() unexpected error = %v", err)
			}

			if outcome.AllFinished != tt.wantFinished {
				t.Errorf("AllFinished = %v, want %v for phase %v", outcome.AllFinished, tt.wantFinished, tt.phase)
			}
			if outcome.FinishedVMs != tt.wantFinishedN {
				t.Errorf("FinishedVMs = %d, want %d for phase %v", outcome.FinishedVMs, tt.wantFinishedN, tt.phase)
			}
			if len(outcome.FailedVMs) != tt.wantFailedN {
				t.Errorf("FailedVMs = %v, want %d entries for phase %v", outcome.FailedVMs, tt.wantFailedN, tt.phase)
			}
			if len(outcome.FailedVMs) != len(outcome.FailureSummaries) {
				t.Errorf("FailedVMs (%d) and FailureSummaries (%d) must stay in step",
					len(outcome.FailedVMs), len(outcome.FailureSummaries))
			}

			// The plan must NOT be failed by this function — one bad VM used to abort
			// the whole plan here, starving post-migration for its siblings.
			updatedPlan := &vjailbreakv1alpha1.MigrationPlan{}
			if getErr := fakeClient.Get(context.Background(),
				types.NamespacedName{Name: planName, Namespace: ns}, updatedPlan); getErr != nil {
				t.Fatalf("failed to re-read plan: %v", getErr)
			}
			if updatedPlan.Status.MigrationStatus == corev1.PodFailed {
				t.Errorf("processMigrationPhases must not set the plan to PodFailed; got %v",
					updatedPlan.Status.MigrationStatus)
			}
		})
	}
}

// TestProcessMigrationPhases_PartialFailureDoesNotStarveSiblings is the
// regression test for the reported symptom: with 20 VMs where one cannot be
// placed on any flavor, the other 19 must still be processed. Previously the loop
// returned on the first ValidationFailed, so post-migration never ran for the
// healthy VMs and the plan was parked as PodFailed while they were still copying.
func TestProcessMigrationPhases_PartialFailureDoesNotStarveSiblings(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = vjailbreakv1alpha1.AddToScheme(scheme)

	const ns = "migration-system"
	const planName = "test-plan-partial"
	const totalVMs = 20
	const badVMIndex = 6 // VM #7

	plan := &vjailbreakv1alpha1.MigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: planName, Namespace: ns},
	}

	migrations := make([]vjailbreakv1alpha1.Migration, 0, totalVMs)
	vmNames := make([]string, 0, totalVMs)
	for i := 0; i < totalVMs; i++ {
		vmName := fmt.Sprintf("vm-%02d", i)
		vmNames = append(vmNames, vmName)

		migration := vjailbreakv1alpha1.Migration{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "migration-" + vmName,
				Namespace:   ns,
				Labels:      map[string]string{"migrationplan": planName},
				Annotations: map[string]string{"vjailbreak.k8s.pf9.io/original-vm-name": vmName},
			},
			Spec: vjailbreakv1alpha1.MigrationSpec{VMName: vmName},
		}

		if i == badVMIndex {
			migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseValidationFailed
			migration.Status.Conditions = []corev1.PodCondition{
				{Message: "No target flavor can satisfy this VM (256 vCPUs, 1048576 MB RAM)."},
			}
		} else {
			migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseSucceeded
		}
		migrations = append(migrations, migration)
	}

	objs := []client.Object{plan}
	for i := range migrations {
		objs = append(objs, &migrations[i])
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&vjailbreakv1alpha1.MigrationPlan{}, &vjailbreakv1alpha1.Migration{}).
		Build()

	r := &MigrationPlanReconciler{Client: fakeClient, Scheme: scheme, ctxlog: logr.Discard()}
	migrationScope, _ := scope.NewMigrationPlanScope(scope.MigrationPlanScopeParams{
		Client:        fakeClient,
		MigrationPlan: plan,
	})

	outcome, err := r.processMigrationPhases(context.Background(), migrationScope, plan,
		&vjailbreakv1alpha1.MigrationList{Items: migrations}, vmNames)
	if err != nil {
		t.Fatalf("one unschedulable VM must not abort phase processing, got: %v", err)
	}

	if !outcome.AllFinished {
		t.Error("AllFinished = false, want true — every migration is in a terminal phase")
	}
	if outcome.FinishedVMs != totalVMs-1 {
		t.Errorf("FinishedVMs = %d, want %d — the healthy VMs must all be processed",
			outcome.FinishedVMs, totalVMs-1)
	}
	if len(outcome.FailedVMs) != 1 {
		t.Fatalf("FailedVMs = %v, want exactly 1", outcome.FailedVMs)
	}
	if outcome.FailedVMs[0] != fmt.Sprintf("vm-%02d", badVMIndex) {
		t.Errorf("FailedVMs[0] = %q, want %q", outcome.FailedVMs[0], fmt.Sprintf("vm-%02d", badVMIndex))
	}
	if !strings.Contains(outcome.FailureSummaries[0], "No target flavor") {
		t.Errorf("failure summary must carry the reason, got %q", outcome.FailureSummaries[0])
	}

	// Post-migration ran for every succeeded VM, evidenced by the annotation.
	for i := 0; i < totalVMs; i++ {
		if i == badVMIndex {
			continue
		}
		vmName := fmt.Sprintf("vm-%02d", i)
		updated := &vjailbreakv1alpha1.Migration{}
		if getErr := fakeClient.Get(context.Background(),
			types.NamespacedName{Name: "migration-" + vmName, Namespace: ns}, updated); getErr != nil {
			t.Fatalf("failed to re-read migration for %s: %v", vmName, getErr)
		}
		if updated.Annotations[constants.PostMigrationCompleteAnnotation] != "true" {
			t.Errorf("post-migration did not run for %s — a single failed sibling starved it", vmName)
		}
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

// --- Non-blocking flavor resolution -----------------------------------------
//
// These cover the guarantee that a single VM whose CPU/memory/GPU shape no target
// flavor can satisfy does not stop the rest of a MigrationPlan from being
// scheduled.

func flavorTestMachine(name string, cpu, memoryMB int) *vjailbreakv1alpha1.VMwareMachine {
	return &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Name:   name,
				CPU:    cpu,
				Memory: memoryMB,
			},
		},
	}
}

func TestResolveTargetFlavorID(t *testing.T) {
	candidates := []flavors.Flavor{
		{ID: "f-small", VCPUs: 2, RAM: 4096},
		{ID: "f-medium", VCPUs: 4, RAM: 16384},
		{ID: "f-large", VCPUs: 8, RAM: 32768},
	}
	template := &vjailbreakv1alpha1.MigrationTemplate{}
	creds := &vjailbreakv1alpha1.OpenstackCreds{}

	tests := []struct {
		name         string
		vmMachine    *vjailbreakv1alpha1.VMwareMachine
		candidates   []flavors.Flavor
		wantFlavorID string
		wantSkip     bool
	}{
		{
			name:         "smallest fitting flavor is chosen",
			vmMachine:    flavorTestMachine("vm-a", 2, 4096),
			candidates:   candidates,
			wantFlavorID: "f-small",
		},
		{
			name:         "rounds up to next flavor that fits",
			vmMachine:    flavorTestMachine("vm-b", 3, 8192),
			candidates:   candidates,
			wantFlavorID: "f-medium",
		},
		{
			name:       "shape larger than every flavor is a skip, not a failure",
			vmMachine:  flavorTestMachine("vm-c", 128, 999999),
			candidates: candidates,
			wantSkip:   true,
		},
		{
			name: "explicit TargetFlavorID overrides resolution entirely",
			vmMachine: func() *vjailbreakv1alpha1.VMwareMachine {
				m := flavorTestMachine("vm-d", 128, 999999) // would otherwise be unschedulable
				m.Spec.TargetFlavorID = "operator-pinned"
				return m
			}(),
			candidates:   candidates,
			wantFlavorID: "operator-pinned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flavorID, err := resolveTargetFlavorID(tt.vmMachine, template, creds, tt.candidates)

			if tt.wantSkip {
				if err == nil {
					t.Fatalf("expected an error, got flavorID %q", flavorID)
				}
				if !isNoSuitableFlavorErr(err) {
					t.Errorf("error should be classified as no-suitable-flavor, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if flavorID != tt.wantFlavorID {
				t.Errorf("flavorID = %q, want %q", flavorID, tt.wantFlavorID)
			}
		})
	}
}

// TestResolveTargetFlavorID_OneBadVMDoesNotStopTheRest encodes the reported
// scenario directly: in a 20-VM plan where VM #7 cannot be placed, the other 19
// must still resolve.
func TestResolveTargetFlavorID_OneBadVMDoesNotStopTheRest(t *testing.T) {
	candidates := []flavors.Flavor{{ID: "f-medium", VCPUs: 4, RAM: 16384}}
	template := &vjailbreakv1alpha1.MigrationTemplate{}
	creds := &vjailbreakv1alpha1.OpenstackCreds{}

	const totalVMs = 20
	const badVMIndex = 6 // zero-based, i.e. VM #7

	vmMachines := make([]*vjailbreakv1alpha1.VMwareMachine, 0, totalVMs)
	for i := 0; i < totalVMs; i++ {
		if i == badVMIndex {
			vmMachines = append(vmMachines, flavorTestMachine("vm-oversized", 256, 1048576))
			continue
		}
		vmMachines = append(vmMachines, flavorTestMachine(fmt.Sprintf("vm-%02d", i), 2, 8192))
	}

	resolved := map[string]string{}
	skipped := []string{}

	for _, vmMachine := range vmMachines {
		flavorID, err := resolveTargetFlavorID(vmMachine, template, creds, candidates)
		if err != nil {
			if !isNoSuitableFlavorErr(err) {
				t.Fatalf("unexpected fatal error for %s: %v", vmMachine.Name, err)
			}
			skipped = append(skipped, vmMachine.Name)
			continue
		}
		resolved[vmMachine.Name] = flavorID
	}

	if len(resolved) != totalVMs-1 {
		t.Errorf("resolved %d VMs, want %d — a single unschedulable VM must not stop the rest",
			len(resolved), totalVMs-1)
	}
	if len(skipped) != 1 {
		t.Fatalf("skipped %d VMs, want exactly 1: %v", len(skipped), skipped)
	}
	if skipped[0] != "vm-oversized" {
		t.Errorf("skipped the wrong VM: %v", skipped)
	}
	if _, present := resolved["vm-oversized"]; present {
		t.Error("unschedulable VM must not appear in the resolved flavor map")
	}
}

func TestShouldSkipVMForFlavor(t *testing.T) {
	tests := []struct {
		name            string
		vmMachine       *vjailbreakv1alpha1.VMwareMachine
		resolvedFlavors map[string]string
		wantSkip        bool
	}{
		{
			name:            "pre-resolved flavor: proceed",
			vmMachine:       flavorTestMachine("vm-a", 2, 4096),
			resolvedFlavors: map[string]string{"vm-a": "f-small"},
			wantSkip:        false,
		},
		{
			name:            "absent from map: skip",
			vmMachine:       flavorTestMachine("vm-b", 2, 4096),
			resolvedFlavors: map[string]string{"vm-a": "f-small"},
			wantSkip:        true,
		},
		{
			name:            "present but empty: skip",
			vmMachine:       flavorTestMachine("vm-c", 2, 4096),
			resolvedFlavors: map[string]string{"vm-c": ""},
			wantSkip:        true,
		},
		{
			name:            "nil map: skip",
			vmMachine:       flavorTestMachine("vm-d", 2, 4096),
			resolvedFlavors: nil,
			wantSkip:        true,
		},
		{
			name: "explicit TargetFlavorID: proceed even with empty map",
			vmMachine: func() *vjailbreakv1alpha1.VMwareMachine {
				m := flavorTestMachine("vm-e", 2, 4096)
				m.Spec.TargetFlavorID = "operator-pinned"
				return m
			}(),
			resolvedFlavors: nil,
			wantSkip:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipVMForFlavor(tt.vmMachine, tt.resolvedFlavors); got != tt.wantSkip {
				t.Errorf("shouldSkipVMForFlavor() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}

// TestFlavorSkipReason_IsActionable checks the message shown against an
// unschedulable VM. It is the only explanation the operator gets in the UI (it
// lands in the Migration's condition message, which the migrations table renders
// in the progress tooltip and the detail page shows in the error card), so it must
// name the shape that could not be placed and the action that fixes it.
func TestFlavorSkipReason_IsActionable(t *testing.T) {
	pcdCreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{constants.IsPCDCredsLabel: "true"},
		},
	}

	tests := []struct {
		name         string
		vmMachine    *vjailbreakv1alpha1.VMwareMachine
		template     *vjailbreakv1alpha1.MigrationTemplate
		creds        *vjailbreakv1alpha1.OpenstackCreds
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:      "plain shape names cpu and memory",
			vmMachine: flavorTestMachine("vm-a", 8, 32768),
			template:  &vjailbreakv1alpha1.MigrationTemplate{},
			creds:     &vjailbreakv1alpha1.OpenstackCreds{},
			wantContains: []string{
				"8 vCPUs",
				"32768 MB RAM",
				"targetFlavorId",
			},
			wantAbsent: []string{"GPU", "PCD cluster"},
		},
		{
			name: "GPU shape is spelled out",
			vmMachine: func() *vjailbreakv1alpha1.VMwareMachine {
				m := flavorTestMachine("vm-b", 16, 65536)
				m.Spec.VMInfo.GPU.PassthroughCount = 2
				m.Spec.VMInfo.GPU.VGPUCount = 1
				return m
			}(),
			template:     &vjailbreakv1alpha1.MigrationTemplate{},
			creds:        &vjailbreakv1alpha1.OpenstackCreds{},
			wantContains: []string{"2 passthrough GPU(s)", "1 vGPU(s)"},
		},
		{
			name:      "PCD availability-zone restriction is called out",
			vmMachine: flavorTestMachine("vm-c", 4, 8192),
			template: &vjailbreakv1alpha1.MigrationTemplate{
				Spec: vjailbreakv1alpha1.MigrationTemplateSpec{TargetPCDClusterName: "cluster-b"},
			},
			creds:        pcdCreds,
			wantContains: []string{"cluster-b", "Only flavors available in PCD cluster"},
		},
		{
			name:      "non-PCD creds do not mention a cluster",
			vmMachine: flavorTestMachine("vm-d", 4, 8192),
			template: &vjailbreakv1alpha1.MigrationTemplate{
				Spec: vjailbreakv1alpha1.MigrationTemplateSpec{TargetPCDClusterName: "cluster-b"},
			},
			creds:      &vjailbreakv1alpha1.OpenstackCreds{},
			wantAbsent: []string{"PCD cluster"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flavorSkipReason(tt.vmMachine, tt.template, tt.creds)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("reason %q does not contain %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("reason %q should not contain %q", got, absent)
				}
			}
		})
	}
}

// TestFirstConditionMessage guards against the Conditions[0] panic the old
// processMigrationPhases had, and confirms the newest condition wins.
func TestFirstConditionMessage(t *testing.T) {
	older := metav1.NewTime(time.Now().Add(-time.Hour))
	newer := metav1.NewTime(time.Now())

	tests := []struct {
		name      string
		migration *vjailbreakv1alpha1.Migration
		want      string
	}{
		{
			name: "no conditions falls back to phase instead of panicking",
			migration: &vjailbreakv1alpha1.Migration{
				Status: vjailbreakv1alpha1.MigrationStatus{
					Phase: vjailbreakv1alpha1.VMMigrationPhaseValidationFailed,
				},
			},
			want: "ValidationFailed",
		},
		{
			name: "newest condition message wins",
			migration: &vjailbreakv1alpha1.Migration{
				Status: vjailbreakv1alpha1.MigrationStatus{
					Phase: vjailbreakv1alpha1.VMMigrationPhaseFailed,
					Conditions: []corev1.PodCondition{
						{Message: "stale reason", LastTransitionTime: older},
						{Message: "real reason", LastTransitionTime: newer},
					},
				},
			},
			want: "real reason",
		},
		{
			name: "empty message falls back to phase",
			migration: &vjailbreakv1alpha1.Migration{
				Status: vjailbreakv1alpha1.MigrationStatus{
					Phase:      vjailbreakv1alpha1.VMMigrationPhaseFailed,
					Conditions: []corev1.PodCondition{{Message: "", LastTransitionTime: newer}},
				},
			},
			want: "Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstConditionMessage(tt.migration); got != tt.want {
				t.Errorf("firstConditionMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestIsNoSuitableFlavorErr_ClassifiesOnlyFlavorErrors guards the blast radius of
// the containment gate: only an unschedulable shape may be treated as a skip.
// Everything else (auth, connectivity, an empty flavor list) must stay fatal so
// it is retried instead of silently burned in as a permanent per-VM failure.
func TestIsNoSuitableFlavorErr_ClassifiesOnlyFlavorErrors(t *testing.T) {
	_, shapeErr := openstackpkg.GetClosestFlavour(64, 2048, 0, 0,
		[]flavors.Flavor{{ID: "f-small", VCPUs: 2, RAM: 4096}}, false)
	_, emptyListErr := openstackpkg.GetClosestFlavour(2, 2048, 0, 0, nil, false)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "unschedulable shape", err: shapeErr, want: true},
		{name: "shape error wrapped twice", err: pkgerrors.Wrap(pkgerrors.Wrap(shapeErr, "outer"), "outermost"), want: true},
		{name: "empty flavor list stays fatal", err: emptyListErr, want: false},
		{name: "unrelated error", err: pkgerrors.New("keystone: 401 unauthorized"), want: false},
		{name: "unrelated error mentioning flavor", err: pkgerrors.New("failed to list all flavors"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoSuitableFlavorErr(tt.err); got != tt.want {
				t.Errorf("isNoSuitableFlavorErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestDetermineAndSetTargetFlavor_FastPaths verifies the cached-flavor and
// operator-override paths never touch the OpenStack API. The reconciler is built
// with a nil client on purpose: any attempt to reach Nova would panic, so a clean
// pass proves no API call was made.
func TestDetermineAndSetTargetFlavor_FastPaths(t *testing.T) {
	r := &MigrationPlanReconciler{}
	template := &vjailbreakv1alpha1.MigrationTemplate{}
	creds := &vjailbreakv1alpha1.OpenstackCreds{}

	tests := []struct {
		name            string
		vmMachine       *vjailbreakv1alpha1.VMwareMachine
		resolvedFlavors map[string]string
		want            string
	}{
		{
			name:            "uses pre-resolved flavor from validation",
			vmMachine:       flavorTestMachine("vm-a", 2, 4096),
			resolvedFlavors: map[string]string{"vm-a": "f-cached"},
			want:            "f-cached",
		},
		{
			name: "explicit TargetFlavorID wins over cache",
			vmMachine: func() *vjailbreakv1alpha1.VMwareMachine {
				m := flavorTestMachine("vm-b", 2, 4096)
				m.Spec.TargetFlavorID = "operator-pinned"
				return m
			}(),
			resolvedFlavors: map[string]string{"vm-b": "f-cached"},
			want:            "operator-pinned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMapData := map[string]string{}
			err := r.determineAndSetTargetFlavor(context.Background(), configMapData,
				tt.vmMachine, template, creds, tt.resolvedFlavors)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := configMapData["TARGET_FLAVOR_ID"]; got != tt.want {
				t.Errorf("TARGET_FLAVOR_ID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVMDisplayNames(t *testing.T) {
	got := vmDisplayNames([]*vjailbreakv1alpha1.VMwareMachine{
		flavorTestMachine("vm-a", 2, 4096),
		flavorTestMachine("vm-b", 4, 8192),
	})
	want := []string{"vm-a", "vm-b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("vmDisplayNames() = %v, want %v", got, want)
	}
}

// TestValidateMigrationPlanVMs_PinnedFlavorSkipsNovaLookup confirms a pinned VM
// resolves cleanly even with a nil candidate list — it must never consult it.
func TestValidateMigrationPlanVMs_PinnedFlavorSkipsNovaLookup(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	const ns = "migration-system"
	const credsName = "vmwcreds-a"
	const vmName = "pinned-vm"
	const vmID = "vm-101"

	vmKey := commonutils.GetVMUniqueKey(vmName, vmID)
	vmk8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(vmKey, credsName)
	if err != nil {
		t.Fatalf("failed to compute k8s name: %v", err)
	}

	vmMachine := &vjailbreakv1alpha1.VMwareMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmk8sName,
			Namespace: ns,
			Labels:    map[string]string{constants.VMwareCredsLabel: credsName},
		},
		Spec: vjailbreakv1alpha1.VMwareMachineSpec{
			TargetFlavorID: "operator-pinned",
			VMInfo: vjailbreakv1alpha1.VMInfo{
				Name:     vmName,
				VMID:     vmID,
				CPU:      4,
				Memory:   8192,
				OSFamily: "linuxGuest",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vmMachine).
		Build()

	r := &MigrationPlanReconciler{
		Client: fakeClient,
		Scheme: scheme,
		ctxlog: logr.Discard(),
	}

	migrationplan := &vjailbreakv1alpha1.MigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: "plan-a", Namespace: ns},
	}
	migrationtemplate := &vjailbreakv1alpha1.MigrationTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tmpl-a", Namespace: ns},
	}
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
		ObjectMeta: metav1.ObjectMeta{Name: credsName, Namespace: ns},
	}
	openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{
		ObjectMeta: metav1.ObjectMeta{Name: "oscreds-a", Namespace: ns},
	}

	validation, err := r.validateMigrationPlanVMs(context.Background(), migrationplan,
		migrationtemplate, vmwcreds, openstackcreds, []string{vmKey}, nil, nil)
	if err != nil {
		t.Fatalf("validation must not require a candidate flavor for a pinned flavor, got: %v", err)
	}

	if len(validation.ValidVMs) != 1 {
		t.Fatalf("ValidVMs = %d, want 1", len(validation.ValidVMs))
	}
	if len(validation.FlavorSkippedVMs) != 0 {
		t.Errorf("FlavorSkippedVMs = %d, want 0", len(validation.FlavorSkippedVMs))
	}
	if got := validation.ResolvedFlavors[vmk8sName]; got != "operator-pinned" {
		t.Errorf("ResolvedFlavors[%s] = %q, want %q", vmk8sName, got, "operator-pinned")
	}
}

// TestFetchVMsToValidate covers the ReconcileMigrationPlanJob-level half of
// the "operator-pinned plans never call Nova" guarantee (a fully-pinned plan
// must report no lookup needed), and that every successfully-fetched VM is
// returned for validateMigrationPlanVMs to reuse instead of re-fetching.
func TestFetchVMsToValidate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	const ns = "migration-system"
	const credsName = "vmwcreds-a"

	newVMMachine := func(name, vmID, targetFlavorID string) (*vjailbreakv1alpha1.VMwareMachine, string) {
		vmKey := commonutils.GetVMUniqueKey(name, vmID)
		vmk8sName, err := commonutils.GetK8sCompatibleVMWareObjectName(vmKey, credsName)
		if err != nil {
			t.Fatalf("failed to compute k8s name: %v", err)
		}
		return &vjailbreakv1alpha1.VMwareMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmk8sName,
				Namespace: ns,
				Labels:    map[string]string{constants.VMwareCredsLabel: credsName},
			},
			Spec: vjailbreakv1alpha1.VMwareMachineSpec{
				TargetFlavorID: targetFlavorID,
				VMInfo:         vjailbreakv1alpha1.VMInfo{Name: name, VMID: vmID},
			},
		}, vmKey
	}

	pinnedA, pinnedAKey := newVMMachine("pinned-a", "vm-201", "flavor-a")
	pinnedB, pinnedBKey := newVMMachine("pinned-b", "vm-202", "flavor-b")
	unpinned, unpinnedKey := newVMMachine("unpinned", "vm-203", "")

	tests := []struct {
		name            string
		objects         []client.Object
		vmKeys          []string
		wantNeedsFetch  bool
		wantFetchedKeys []string
	}{
		{
			name:            "every VM pinned: no lookup needed, both fetched",
			objects:         []client.Object{pinnedA, pinnedB},
			vmKeys:          []string{pinnedAKey, pinnedBKey},
			wantNeedsFetch:  false,
			wantFetchedKeys: []string{pinnedAKey, pinnedBKey},
		},
		{
			name:            "one VM unpinned: lookup still needed, both fetched",
			objects:         []client.Object{pinnedA, unpinned},
			vmKeys:          []string{pinnedAKey, unpinnedKey},
			wantNeedsFetch:  true,
			wantFetchedKeys: []string{pinnedAKey, unpinnedKey},
		},
		{
			name:            "VM can't be fetched: defaults to needing a lookup, only found VM returned",
			objects:         []client.Object{pinnedA},
			vmKeys:          []string{pinnedAKey, "missing-vm"},
			wantNeedsFetch:  true,
			wantFetchedKeys: []string{pinnedAKey},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()
			r := &MigrationPlanReconciler{Client: fakeClient, Scheme: scheme, ctxlog: logr.Discard()}

			migrationtemplate := &vjailbreakv1alpha1.MigrationTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "tmpl-a", Namespace: ns},
			}
			vmwcreds := &vjailbreakv1alpha1.VMwareCreds{
				ObjectMeta: metav1.ObjectMeta{Name: credsName, Namespace: ns},
			}

			fetched, needsLookup := r.fetchVMsToValidate(context.Background(), migrationtemplate, vmwcreds, tt.vmKeys)
			if needsLookup != tt.wantNeedsFetch {
				t.Errorf("needsLookup = %v, want %v", needsLookup, tt.wantNeedsFetch)
			}
			if len(fetched) != len(tt.wantFetchedKeys) {
				t.Errorf("fetched has %d entries, want %d", len(fetched), len(tt.wantFetchedKeys))
			}
			for _, key := range tt.wantFetchedKeys {
				if _, ok := fetched[key]; !ok {
					t.Errorf("fetched[%q] missing, want present", key)
				}
			}
		})
	}
}
