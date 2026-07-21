package debugbundle

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testMigrationGraph() []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		cr("Migration", "migration-testvm", map[string]interface{}{
			"spec": map[string]interface{}{
				"migrationPlan": "plan1",
				"podRef":        "v2v-helper-testvm",
				"vmName":        "testvm",
			},
			"status": map[string]interface{}{
				"agentName": "vjailbreak-node-1",
			},
		}),
		cr("MigrationPlan", "plan1", map[string]interface{}{
			"spec": map[string]interface{}{
				"migrationTemplate": "tmpl1",
			},
		}),
		cr("MigrationTemplate", "tmpl1", map[string]interface{}{
			"spec": map[string]interface{}{
				"networkMapping":    "nm1",
				"storageMapping":    "sm1",
				"storageCopyMethod": "HotAdd",
				"proxyVMRef": map[string]interface{}{
					"name": "proxy1",
				},
				"destination": map[string]interface{}{
					"openstackRef": "osc1",
				},
				"source": map[string]interface{}{
					"vmwareRef": "vmc1",
				},
			},
		}),
		cr("ProxyVM", "proxy1", map[string]interface{}{
			"spec": map[string]interface{}{
				"vmName": "proxy-vm-1",
				"vmwareCredsRef": map[string]interface{}{
					"name": "vmc1",
				},
			},
		}),
		cr("ProxyVM", "unrelated-proxy", nil),
		cr("NetworkMapping", "nm1", nil),
		cr("StorageMapping", "sm1", nil),
		cr("OpenstackCreds", "osc1", nil),
		cr("VMwareCreds", "vmc1", nil),
		cr("VjailbreakNode", "vjailbreak-node-1", nil),
		cr("VMwareMachine", "testvm-machine", map[string]interface{}{
			"spec": map[string]interface{}{
				"vms": map[string]interface{}{
					"name":        "testvm",
					"clusterName": "vw-cluster-1",
					"esxiName":    "esxi-1",
				},
			},
		}),
		cr("VMwareCluster", "vw-cluster-1", map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "vw-cluster-1",
			},
		}),
		cr("VMwareHost", "esxi-1", map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "esxi-1",
			},
		}),
		// Unrelated resources that must NOT be picked up.
		cr("MigrationPlan", "unrelated-plan", nil),
		cr("OpenstackCreds", "unrelated-creds", nil),
		coreObj("ConfigMap", "migration-config-testvm", map[string]interface{}{
			"data": map[string]interface{}{
				"SOURCE_VM_NAME": "testvm",
			},
		}),
		coreObj("ConfigMap", "unrelated-configmap", nil),
		coreObj("ConfigMap", "vjailbreak-settings", map[string]interface{}{
			"data": map[string]interface{}{
				"CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD": "20",
			},
		}),
		coreObj("ConfigMap", "version-config", map[string]interface{}{
			"data": map[string]interface{}{
				"version":          "v0.4.7",
				"upgradeAvailable": "false",
			},
		}),
		coreObj("Pod", "v2v-helper-testvm", nil),
	}
}

func TestCollectResourcesByMigrationName(t *testing.T) {
	c := newFakeClient(t, testMigrationGraph()...)

	entries, warnings := CollectResources(context.Background(), c, testNamespace, "migration-testvm", "v2v-helper-testvm")

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	paths := entryPaths(entries)
	expected := []string{
		"kubernetes/migrations/migration-testvm.yaml",
		"kubernetes/migrationplans/plan1.yaml",
		"kubernetes/migrationtemplates/tmpl1.yaml",
		"kubernetes/networkmappings/nm1.yaml",
		"kubernetes/storagemappings/sm1.yaml",
		"kubernetes/openstackcreds/osc1.yaml",
		"kubernetes/vmwarecreds/vmc1.yaml",
		"kubernetes/vjailbreaknodes/vjailbreak-node-1.yaml",
		"kubernetes/vmwaremachines/testvm-machine.yaml",
		"kubernetes/vmwareclusters/vw-cluster-1.yaml",
		"kubernetes/vmwarehosts/esxi-1.yaml",
		"kubernetes/configmaps/migration-config-testvm.yaml",
		"kubernetes/configmaps/vjailbreak-settings.yaml",
		"kubernetes/configmaps/version-config.yaml",
		"kubernetes/proxyvms/proxy1.yaml",
		"kubernetes/pods/v2v-helper-testvm.yaml",
	}
	for _, path := range expected {
		if !paths[path] {
			t.Errorf("expected bundle to include %s, got paths %v", path, paths)
		}
	}

	unexpected := []string{
		"kubernetes/migrationplans/unrelated-plan.yaml",
		"kubernetes/openstackcreds/unrelated-creds.yaml",
		"kubernetes/configmaps/unrelated-configmap.yaml",
		"kubernetes/proxyvms/unrelated-proxy.yaml",
	}
	for _, path := range unexpected {
		if paths[path] {
			t.Errorf("bundle must not include %s", path)
		}
	}
}

func TestCollectResourcesFindsMigrationByPodRef(t *testing.T) {
	c := newFakeClient(t, testMigrationGraph()...)

	entries, _ := CollectResources(context.Background(), c, testNamespace, "", "v2v-helper-testvm")

	paths := entryPaths(entries)
	if !paths["kubernetes/migrations/migration-testvm.yaml"] {
		t.Fatalf("expected migration located via spec.podRef, got paths %v", paths)
	}
}

func TestCollectResourcesMigrationNotFound(t *testing.T) {
	c := newFakeClient(t)

	entries, warnings := CollectResources(context.Background(), c, testNamespace, "missing-migration", "")

	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %v", entryPaths(entries))
	}
	found := false
	for _, warning := range warnings {
		if strings.Contains(warning, "Migration resource not found for migrationName=missing-migration") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected not-found warning, got %v", warnings)
	}
}

func TestCollectResourcesConfigMapDiscoversVMAndNode(t *testing.T) {
	objs := []*unstructured.Unstructured{
		cr("Migration", "migration-othervm", map[string]interface{}{
			"spec": map[string]interface{}{},
		}),
		coreObj("ConfigMap", "migration-config-othervm", map[string]interface{}{
			"data": map[string]interface{}{
				"VMWARE_MACHINE_OBJECT_NAME": "machine-from-cm",
				"VJAILBREAK_NODE":            "node-from-cm",
			},
		}),
		cr("VMwareMachine", "machine-from-cm", nil),
		cr("VjailbreakNode", "node-from-cm", nil),
	}
	c := newFakeClient(t, objs...)

	entries, _ := CollectResources(context.Background(), c, testNamespace, "migration-othervm", "")

	paths := entryPaths(entries)
	for _, path := range []string{
		"kubernetes/configmaps/migration-config-othervm.yaml",
		"kubernetes/vmwaremachines/machine-from-cm.yaml",
		"kubernetes/vjailbreaknodes/node-from-cm.yaml",
	} {
		if !paths[path] {
			t.Errorf("expected bundle to include %s, got paths %v", path, paths)
		}
	}
}
