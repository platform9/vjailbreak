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
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// copyOnlyTestScheme returns a scheme with the vjailbreak types registered.
func copyOnlyTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := vjailbreakv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1 AddToScheme() error = %v", err)
	}
	return scheme
}

// newCopyOnlyPlan builds a minimal MigrationPlan with the given CopyOnly setting.
func newCopyOnlyPlan(ns string, copyOnly bool) *vjailbreakv1alpha1.MigrationPlan {
	return &vjailbreakv1alpha1.MigrationPlan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan-copyonly", Namespace: ns},
		Spec: vjailbreakv1alpha1.MigrationPlanSpec{
			MigrationPlanSpecPerVM: vjailbreakv1alpha1.MigrationPlanSpecPerVM{
				MigrationTemplate: "test-template",
				MigrationStrategy: vjailbreakv1alpha1.MigrationPlanStrategy{
					Type:     "cold",
					CopyOnly: copyOnly,
				},
			},
			VirtualMachines: [][]string{{"vm-a"}},
		},
	}
}

// TestBuildBaseConfigMapData_ConvertReflectsCopyOnly verifies that the CONVERT key handed to
// v2v-helper is the inverse of the plan's CopyOnly setting. CONVERT is the single signal the
// helper uses to decide whether to run virt-v2v and any other in-guest modification, so getting
// this inverted would either convert a guest the operator asked to leave alone, or silently ship
// an unconverted guest for a normal migration.
func TestBuildBaseConfigMapData_ConvertReflectsCopyOnly(t *testing.T) {
	const ns = "migration-system"

	tests := []struct {
		name        string
		copyOnly    bool
		wantConvert string
	}{
		{
			name:        "copyOnly=false requests conversion",
			copyOnly:    false,
			wantConvert: "true",
		},
		{
			name:        "copyOnly=true suppresses conversion",
			copyOnly:    true,
			wantConvert: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &MigrationPlanReconciler{ctxlog: logr.Discard()}

			plan := newCopyOnlyPlan(ns, tt.copyOnly)
			migrationobj := &vjailbreakv1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{Name: "test-migration", Namespace: ns},
			}
			vmMachine := &vjailbreakv1alpha1.VMwareMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "vm-a-moid123", Namespace: ns},
				Spec: vjailbreakv1alpha1.VMwareMachineSpec{
					VMInfo: vjailbreakv1alpha1.VMInfo{Name: "vm-a", VMID: "moid123"},
				},
			}

			data := r.buildBaseConfigMapData(plan, migrationobj, vmMachine, "vm-a", "virtio-win", nil, nil, nil)

			if got := data["CONVERT"]; got != tt.wantConvert {
				t.Errorf("CONVERT = %q, want %q", got, tt.wantConvert)
			}
		})
	}
}

// TestCopyOnlyPropagation verifies CopyOnly on the plan's strategy reaches the Migration CR, which
// is what surfaces the mode on the migration detail view and in `kubectl get migration -o yaml`.
func TestCopyOnlyPropagation(t *testing.T) {
	const ns = "migration-system"
	const vmwarecredsName = "test-vmwcreds"

	tests := []struct {
		name         string
		copyOnly     bool
		wantCopyOnly bool
	}{
		{name: "CopyOnly=true propagates to Migration", copyOnly: true, wantCopyOnly: true},
		{name: "CopyOnly=false propagates to Migration", copyOnly: false, wantCopyOnly: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := copyOnlyTestScheme(t)

			vmMachine := &vjailbreakv1alpha1.VMwareMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "vm-a-moid123", Namespace: ns},
				Spec: vjailbreakv1alpha1.VMwareMachineSpec{
					VMInfo: vjailbreakv1alpha1.VMInfo{Name: "vm-a", VMID: "moid123"},
				},
			}
			vmwarecreds := &vjailbreakv1alpha1.VMwareCreds{
				ObjectMeta: metav1.ObjectMeta{Name: vmwarecredsName, Namespace: ns},
			}
			migrationTemplate := &vjailbreakv1alpha1.MigrationTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "test-template", Namespace: ns},
				Spec: vjailbreakv1alpha1.MigrationTemplateSpec{
					Source: vjailbreakv1alpha1.MigrationTemplateSource{VMwareRef: vmwarecredsName},
				},
			}
			plan := newCopyOnlyPlan(ns, tt.copyOnly)

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

			if _, err := r.CreateMigration(context.Background(), plan, "vm-a", vmMachine); err != nil {
				t.Fatalf("CreateMigration() error = %v", err)
			}

			migrationList := &vjailbreakv1alpha1.MigrationList{}
			if err := fakeClient.List(context.Background(), migrationList); err != nil {
				t.Fatalf("List migrations error = %v", err)
			}
			if len(migrationList.Items) != 1 {
				t.Fatalf("expected 1 Migration, got %d", len(migrationList.Items))
			}
			if got := migrationList.Items[0].Spec.CopyOnly; got != tt.wantCopyOnly {
				t.Errorf("Migration.Spec.CopyOnly = %v, want %v", got, tt.wantCopyOnly)
			}
		})
	}
}

// TestUpdateMigrationConfigMap_RefreshesConvert verifies CONVERT is rewritten when an existing
// migration ConfigMap is reconciled again. Without this, flipping CopyOnly and retrying would
// leave the stale value in place and the retry would silently use the old conversion behaviour.
func TestUpdateMigrationConfigMap_RefreshesConvert(t *testing.T) {
	const ns = "migration-system"
	const cmName = "migration-config-vm-a-moid123"

	tests := []struct {
		name           string
		existing       string
		copyOnly       bool
		wantConvert    string
		describeIntent string
	}{
		{
			name:           "stale CONVERT=true is corrected when copyOnly is enabled",
			existing:       "true",
			copyOnly:       true,
			wantConvert:    "false",
			describeIntent: "operator turned copy-only on and retried",
		},
		{
			name:           "stale CONVERT=false is corrected when copyOnly is disabled",
			existing:       "false",
			copyOnly:       false,
			wantConvert:    "true",
			describeIntent: "operator turned copy-only off and retried",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := copyOnlyTestScheme(t)

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: ns},
				Data:       map[string]string{"CONVERT": tt.existing},
			}
			plan := newCopyOnlyPlan(ns, tt.copyOnly)
			migrationobj := &vjailbreakv1alpha1.Migration{
				ObjectMeta: metav1.ObjectMeta{Name: "test-migration", Namespace: ns},
			}
			vmMachine := &vjailbreakv1alpha1.VMwareMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "vm-a-moid123", Namespace: ns},
				Spec: vjailbreakv1alpha1.VMwareMachineSpec{
					VMInfo: vjailbreakv1alpha1.VMInfo{Name: "vm-a", VMID: "moid123"},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(configMap).
				Build()

			r := &MigrationPlanReconciler{
				Client: fakeClient,
				Scheme: scheme,
				ctxlog: logr.Discard(),
			}

			if err := r.updateMigrationConfigMap(context.Background(), configMap, plan, migrationobj, vmMachine, cmName); err != nil {
				t.Fatalf("updateMigrationConfigMap() error = %v (%s)", err, tt.describeIntent)
			}

			if got := configMap.Data["CONVERT"]; got != tt.wantConvert {
				t.Errorf("CONVERT = %q, want %q (%s)", got, tt.wantConvert, tt.describeIntent)
			}
		})
	}
}
