package debugbundle

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Credentials referenced by ClusterMigration/ESXIMigration (cluster
// conversion flow) and by the VjailbreakNode are discovered after the main
// credential passes; the final resolution pass must still bundle them.
func TestCollectResourcesRollingChainCreds(t *testing.T) {
	objs := []*unstructured.Unstructured{
		cr("Migration", "migration-rollvm", map[string]interface{}{
			"spec": map[string]interface{}{
				"migrationPlan": "plan-roll",
				"vmName":        "rollvm",
			},
			"status": map[string]interface{}{
				"agentName": "node-roll",
			},
		}),
		cr("MigrationPlan", "plan-roll", nil),
		cr("VjailbreakNode", "node-roll", map[string]interface{}{
			"spec": map[string]interface{}{
				"openstackCreds": map[string]interface{}{
					"name": "osc-from-node",
				},
			},
		}),
		cr("RollingMigrationPlan", "rmp1", map[string]interface{}{
			"spec": map[string]interface{}{
				"vmMigrationPlans": []interface{}{"plan-roll"},
			},
		}),
		cr("ClusterMigration", "cm1", map[string]interface{}{
			"spec": map[string]interface{}{
				"rollingMigrationPlanRef": map[string]interface{}{"name": "rmp1"},
				"openstackCredsRef":       map[string]interface{}{"name": "osc-from-cm"},
				"vmwareCredsRef":          map[string]interface{}{"name": "vmc-from-cm"},
			},
		}),
		cr("ESXIMigration", "em1", map[string]interface{}{
			"spec": map[string]interface{}{
				"rollingMigrationPlanRef": map[string]interface{}{"name": "rmp1"},
				"openstackCredsRef":       map[string]interface{}{"name": "osc-from-em"},
				"vmwareCredsRef":          map[string]interface{}{"name": "vmc-from-em"},
			},
		}),
		cr("OpenstackCreds", "osc-from-node", nil),
		cr("OpenstackCreds", "osc-from-cm", nil),
		cr("OpenstackCreds", "osc-from-em", nil),
		cr("VMwareCreds", "vmc-from-cm", nil),
		cr("VMwareCreds", "vmc-from-em", nil),
		cr("OpenstackCreds", "unrelated-osc", nil),
	}
	c := newFakeClient(t, objs...)

	entries, _ := CollectResources(context.Background(), c, testNamespace, "migration-rollvm", "")

	paths := entryPaths(entries)
	for _, path := range []string{
		"kubernetes/rollingmigrationplans/rmp1.yaml",
		"kubernetes/clustermigrations/cm1.yaml",
		"kubernetes/esximigrations/em1.yaml",
		"kubernetes/vjailbreaknodes/node-roll.yaml",
		"kubernetes/openstackcreds/osc-from-node.yaml",
		"kubernetes/openstackcreds/osc-from-cm.yaml",
		"kubernetes/openstackcreds/osc-from-em.yaml",
		"kubernetes/vmwarecreds/vmc-from-cm.yaml",
		"kubernetes/vmwarecreds/vmc-from-em.yaml",
	} {
		if !paths[path] {
			t.Errorf("expected bundle to include %s, got paths %v", path, paths)
		}
	}
	if paths["kubernetes/openstackcreds/unrelated-osc.yaml"] {
		t.Errorf("bundle must not include unrelated OpenstackCreds")
	}
}
