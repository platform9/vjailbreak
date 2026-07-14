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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
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
					CustomMetadata: tt.customMetadata,
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
