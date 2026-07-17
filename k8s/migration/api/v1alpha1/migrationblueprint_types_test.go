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

package v1alpha1

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sampleBlueprint() *MigrationBlueprint {
	renameVM := true
	return &MigrationBlueprint{
		ObjectMeta: metav1.ObjectMeta{Name: "bp-1", Namespace: "migration-system"},
		Spec: MigrationBlueprintSpec{
			DisplayName:          "Production RHEL East",
			Description:          "Standard hot migration for east-region RHEL tiers",
			VMwareRef:            "vcenter-east",
			PCDRef:               "pcd-east-1",
			TargetPCDClusterName: "cluster-prod-a",
			NetworkMappings: []Network{
				{Source: "vmnet-prod", Target: "net-prod-east-a"},
				{Source: "vmnet-data", Target: "net-data-east"},
			},
			StorageMappings: []Storage{
				{Source: "ds-nvme-01", Target: "ceph-nvme"},
			},
			ArrayCredsMappings: []DatastoreArrayCredsMapping{
				{Source: "ds-nvme-01", Target: "pure-array-1"},
			},
			ProxyVMRef: &corev1.LocalObjectReference{Name: "proxy-vm-1"},
			MigrationStrategy: &MigrationPlanStrategy{
				Type:                  "hot",
				AdminInitiatedCutOver: true,
				PerformHealthChecks:   true,
				HealthCheckPort:       "443",
			},
			AdvancedOptions: AdvancedOptions{
				GranularVolumeTypes: []string{"ceph-nvme"},
				PeriodicSyncEnabled: true,
			},
			PostMigrationAction: &PostMigrationAction{
				RenameVM: &renameVM,
				Suffix:   "-migrated",
			},
			FirstBootScript:    "echo migrated",
			SecurityGroups:     []string{"default", "web"},
			ServerGroup:        "sg-east",
			FallbackToDHCP:     true,
			PreserveSourceTags: true,
			CustomMetadata:     map[string]string{"owner": "platform-ops"},
			UseGPUFlavor:       false,
			StorageCopyMethod:  "normal",
			OSFamily:           "linuxGuest",
			VirtioWinDriver:    "0.1.240",
		},
	}
}

func TestMigrationBlueprintDeepCopyRoundTrip(t *testing.T) {
	orig := sampleBlueprint()
	copied := orig.DeepCopy()

	if !reflect.DeepEqual(orig, copied) {
		t.Fatalf("DeepCopy mismatch:\noriginal: %+v\ncopy: %+v", orig, copied)
	}
}

func TestMigrationBlueprintDeepCopyIsIndependent(t *testing.T) {
	orig := sampleBlueprint()
	copied := orig.DeepCopy()

	// Mutate every reference-typed field on the copy; original must not change.
	copied.Spec.NetworkMappings[0].Target = "changed"
	copied.Spec.StorageMappings[0].Source = "changed"
	copied.Spec.ArrayCredsMappings[0].Target = "changed"
	copied.Spec.ProxyVMRef.Name = "changed"
	copied.Spec.MigrationStrategy.Type = "cold"
	copied.Spec.CustomMetadata["owner"] = "changed"
	copied.Spec.SecurityGroups[0] = "changed"
	copied.Spec.AdvancedOptions.GranularVolumeTypes[0] = "changed"
	*copied.Spec.PostMigrationAction.RenameVM = false
	copied.Spec.PostMigrationAction.Suffix = "changed"

	if orig.Spec.NetworkMappings[0].Target == "changed" {
		t.Error("NetworkMappings not deep-copied: slice shared with original")
	}
	if orig.Spec.StorageMappings[0].Source == "changed" {
		t.Error("StorageMappings not deep-copied: slice shared with original")
	}
	if orig.Spec.ArrayCredsMappings[0].Target == "changed" {
		t.Error("ArrayCredsMappings not deep-copied: slice shared with original")
	}
	if orig.Spec.ProxyVMRef.Name == "changed" {
		t.Error("ProxyVMRef not deep-copied: pointer shared with original")
	}
	if orig.Spec.MigrationStrategy.Type == "cold" {
		t.Error("MigrationStrategy not deep-copied: pointer shared with original")
	}
	if orig.Spec.CustomMetadata["owner"] == "changed" {
		t.Error("CustomMetadata not deep-copied: map shared with original")
	}
	if orig.Spec.SecurityGroups[0] == "changed" {
		t.Error("SecurityGroups not deep-copied: slice shared with original")
	}
	if orig.Spec.AdvancedOptions.GranularVolumeTypes[0] == "changed" {
		t.Error("AdvancedOptions.GranularVolumeTypes not deep-copied")
	}
	if !*orig.Spec.PostMigrationAction.RenameVM {
		t.Error("PostMigrationAction.RenameVM pointer shared with original")
	}
	if orig.Spec.PostMigrationAction.Suffix == "changed" {
		t.Error("PostMigrationAction not deep-copied: pointer shared with original")
	}
}

func TestMigrationBlueprintDeepCopyObject(t *testing.T) {
	bp := sampleBlueprint()
	obj := bp.DeepCopyObject()
	copied, ok := obj.(*MigrationBlueprint)
	if !ok {
		t.Fatalf("DeepCopyObject returned %T, want *MigrationBlueprint", obj)
	}
	if !reflect.DeepEqual(bp, copied) {
		t.Fatal("DeepCopyObject mismatch with original")
	}

	list := &MigrationBlueprintList{Items: []MigrationBlueprint{*bp}}
	objList := list.DeepCopyObject()
	copiedList, ok := objList.(*MigrationBlueprintList)
	if !ok {
		t.Fatalf("DeepCopyObject returned %T, want *MigrationBlueprintList", objList)
	}
	if len(copiedList.Items) != 1 || !reflect.DeepEqual(&copiedList.Items[0], bp) {
		t.Fatal("MigrationBlueprintList DeepCopyObject mismatch")
	}
}

func TestMigrationBlueprintRegisteredInScheme(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	if err != nil {
		t.Fatalf("building scheme: %v", err)
	}
	for _, kind := range []string{"MigrationBlueprint", "MigrationBlueprintList"} {
		if !scheme.Recognizes(GroupVersion.WithKind(kind)) {
			t.Errorf("scheme does not recognize %s", kind)
		}
	}
}
