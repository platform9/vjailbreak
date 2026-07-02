package debugbundle

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mirrors the StorageAcceleratedCopy lookup chain in
// migrationplan_controller.go: template.spec.arrayCredsMapping →
// ArrayCredsMapping.spec.mappings[].target → ArrayCreds, plus RDM disks
// owned by the VM with their OpenStack credential reference.
func TestCollectResourcesStorageAcceleratedCopy(t *testing.T) {
	objs := []*unstructured.Unstructured{
		cr("Migration", "migration-storagevm", map[string]interface{}{
			"spec": map[string]interface{}{
				"migrationPlan": "plan-sac",
				"vmName":        "storagevm",
			},
		}),
		cr("MigrationPlan", "plan-sac", map[string]interface{}{
			"spec": map[string]interface{}{
				"migrationTemplate": "tmpl-sac",
			},
		}),
		cr("MigrationTemplate", "tmpl-sac", map[string]interface{}{
			"spec": map[string]interface{}{
				"storageCopyMethod": "StorageAcceleratedCopy",
				"arrayCredsMapping": "acm1",
			},
		}),
		cr("ArrayCredsMapping", "acm1", map[string]interface{}{
			"spec": map[string]interface{}{
				"mappings": []interface{}{
					map[string]interface{}{"source": "datastore-1", "target": "arraycreds-1"},
				},
			},
		}),
		cr("ArrayCreds", "arraycreds-1", nil),
		cr("ArrayCreds", "unrelated-arraycreds", nil),
		cr("RDMDisk", "rdm-1", map[string]interface{}{
			"spec": map[string]interface{}{
				"diskName": "rdm-disk-1",
				"ownerVMs": []interface{}{"storagevm"},
				"openstackVolumeRef": map[string]interface{}{
					"openstackCreds": "osc-from-rdm",
				},
			},
		}),
		cr("OpenstackCreds", "osc-from-rdm", nil),
	}
	c := newFakeClient(t, objs...)

	entries, _ := CollectResources(context.Background(), c, testNamespace, "migration-storagevm", "")

	paths := entryPaths(entries)
	for _, path := range []string{
		"kubernetes/migrationtemplates/tmpl-sac.yaml",
		"kubernetes/arraycredsmappings/acm1.yaml",
		"kubernetes/arraycreds/arraycreds-1.yaml",
		"kubernetes/rdmdisks/rdm-1.yaml",
		"kubernetes/openstackcreds/osc-from-rdm.yaml",
	} {
		if !paths[path] {
			t.Errorf("expected bundle to include %s, got paths %v", path, paths)
		}
	}
	if paths["kubernetes/arraycreds/unrelated-arraycreds.yaml"] {
		t.Errorf("bundle must not include unrelated ArrayCreds")
	}
}
