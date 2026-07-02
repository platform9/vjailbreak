package debugbundle

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	vmKeyLabel          = "vjailbreak.k8s.pf9.io/vm-key"
	originalVMNameAnn   = "vjailbreak.k8s.pf9.io/original-vm-name"
	migrationPlanLabel  = "migrationplan"
	openstackCredsLabel = "vjailbreak.k8s.pf9.io/openstackcreds"
	migrationNamePrefix = "migration-"
)

// CollectResources gathers the migration object and every related resource,
// mirroring the traversal implemented by the UI in
// ui/src/api/kubernetes/migrationResourceBundle/buildMigrationResourceBundle.ts.
// The migration is located by name, falling back to matching spec.podRef
// against podName. Errors on individual reads are reported as warnings so a
// partial bundle is still produced.
func CollectResources(ctx context.Context, c client.Client, namespace, migrationName, podName string) ([]BundleEntry, []string) {
	warnings := []string{}
	resourceLists := map[string][]unstructured.Unstructured{}

	for plural, kind := range relatedCRDKinds {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(VjailbreakGroupVersion.WithKind(kind + "List"))
		if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to list %s: %v", plural, err))
			continue
		}
		resourceLists[plural] = list.Items
	}

	var migration *unstructured.Unstructured
	for i := range resourceLists["migrations"] {
		item := &resourceLists["migrations"][i]
		if item.GetName() == migrationName {
			migration = item
			break
		}
	}
	if migration == nil && podName != "" {
		for i := range resourceLists["migrations"] {
			item := &resourceLists["migrations"][i]
			if nestedString(item.Object, "spec", "podRef") == podName {
				migration = item
				break
			}
		}
	}
	if migration == nil {
		name := migrationName
		if name == "" {
			name = "<empty>"
		}
		warnings = append(warnings, fmt.Sprintf("Migration resource not found for migrationName=%s", name))
		return nil, warnings
	}

	entries := map[string]BundleEntry{}
	addEntry := func(plural string, obj *unstructured.Unstructured) {
		name := obj.GetName()
		if name == "" {
			return
		}
		entries[plural+"/"+name] = BundleEntry{
			Path:   fmt.Sprintf("kubernetes/%s/%s.yaml", plural, name),
			Object: obj,
		}
	}

	addEntry("migrations", migration)

	migrationUID := string(migration.GetUID())
	migrationOwnerKeys := newStringSet()
	migrationOwnerKeys.add(migration.GetName())
	migrationOwnerKeys.add(migrationUID)

	vmK8sName := strings.TrimPrefix(migration.GetName(), migrationNamePrefix)
	vmNames := newStringSet()
	vmNames.add(vmK8sName)
	vmNames.add(nestedString(migration.Object, "spec", "vmName"))
	vmNames.add(migration.GetAnnotations()[originalVMNameAnn])
	vmNames.add(migration.GetLabels()[vmKeyLabel])

	planNames := newStringSet()
	planNames.add(nestedString(migration.Object, "spec", "migrationPlan"))
	planNames.add(migration.GetLabels()[migrationPlanLabel])

	nodeNames := newStringSet()
	nodeNames.add(nestedString(migration.Object, "status", "agentName"))

	configMapNames := newStringSet()
	if vmK8sName != "" {
		configMapNames.add("migration-config-" + vmK8sName)
		configMapNames.add("firstboot-config-" + vmK8sName)
	}
	// Global settings (changed-blocks threshold, sync intervals, ...) affect
	// every migration, and version-config records which vJailbreak version
	// produced the bundle — always include both.
	configMapNames.add(constants.VjailbreakSettingsConfigMapName)
	configMapNames.add("version-config")

	configMaps := &unstructured.UnstructuredList{}
	configMaps.SetAPIVersion("v1")
	configMaps.SetKind("ConfigMapList")
	if err := c.List(ctx, configMaps, client.InNamespace(namespace)); err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to list configmaps: %v", err))
	}
	for i := range configMaps.Items {
		configMap := &configMaps.Items[i]
		if configMapNames.has(configMap.GetName()) || isOwnedBy(configMap, migrationOwnerKeys) {
			addEntry("configmaps", configMap)
			vmNames.add(nestedString(configMap.Object, "data", "VMWARE_MACHINE_OBJECT_NAME"))
			vmNames.add(nestedString(configMap.Object, "data", "SOURCE_VM_NAME"))
			vmNames.add(nestedString(configMap.Object, "data", "SOURCE_VM_KEY"))
			nodeNames.add(nestedString(configMap.Object, "data", "VJAILBREAK_NODE"))
		}
	}

	if podName != "" {
		pod := &unstructured.Unstructured{}
		pod.SetAPIVersion("v1")
		pod.SetKind("Pod")
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: podName}, pod); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to get pods/%s: %v", podName, err))
		} else {
			addEntry("pods", pod)
		}
	}

	templateNames := newStringSet()
	proxyVMNames := newStringSet()
	networkMappingNames := newStringSet()
	storageMappingNames := newStringSet()
	arrayCredsMappingNames := newStringSet()
	arrayCredsNames := newStringSet()
	openstackCredNames := newStringSet()
	vmwareCredNames := newStringSet()
	pcdClusterNames := newStringSet()
	pcdHostNames := newStringSet()
	rdmDiskNames := newStringSet()
	vmwareClusterNames := newStringSet()
	vmwareHostNames := newStringSet()
	volumeImageProfileNames := newStringSet()
	rollingMigrationPlanNames := newStringSet()
	bmConfigNames := newStringSet()

	for i := range resourceLists["migrationplans"] {
		plan := &resourceLists["migrationplans"][i]
		if !planNames.has(plan.GetName()) {
			continue
		}
		addEntry("migrationplans", plan)
		templateNames.add(nestedString(plan.Object, "spec", "migrationTemplate"))
		volumeImageProfileNames.addAll(nestedStringSlice(plan.Object, "spec", "advancedOptions", "imageProfiles"))
	}

	for i := range resourceLists["migrationtemplates"] {
		template := &resourceLists["migrationtemplates"][i]
		if !templateNames.has(template.GetName()) {
			continue
		}
		addEntry("migrationtemplates", template)
		networkMappingNames.add(nestedString(template.Object, "spec", "networkMapping"))
		storageMappingNames.add(nestedString(template.Object, "spec", "storageMapping"))
		arrayCredsMappingNames.add(nestedString(template.Object, "spec", "arrayCredsMapping"))
		openstackCredNames.add(nestedString(template.Object, "spec", "destination", "openstackRef"))
		vmwareCredNames.add(nestedString(template.Object, "spec", "source", "vmwareRef"))
		pcdClusterNames.add(nestedString(template.Object, "spec", "targetPCDClusterName"))
		proxyVMNames.add(nestedString(template.Object, "spec", "proxyVMRef", "name"))
	}

	// ProxyVMs (HotAdd data copy) are matched before the exact-name loop so
	// their VMware credential reference feeds into vmwarecreds matching.
	for i := range resourceLists["proxyvms"] {
		proxyVM := &resourceLists["proxyvms"][i]
		if proxyVMNames.has(proxyVM.GetName()) || isOwnedBy(proxyVM, migrationOwnerKeys) {
			addEntry("proxyvms", proxyVM)
			vmwareCredNames.add(nestedString(proxyVM.Object, "spec", "vmwareCredsRef", "name"))
		}
	}

	for i := range resourceLists["vmwaremachines"] {
		machine := &resourceLists["vmwaremachines"][i]
		machineVMName := nestedString(machine.Object, "spec", "vms", "name")
		if vmNames.has(machine.GetName()) ||
			vmNames.has(machineVMName) ||
			vmNames.has(machine.GetLabels()[vmKeyLabel]) {
			addEntry("vmwaremachines", machine)
			vmNames.add(machineVMName)
			rdmDiskNames.addAll(nestedStringSlice(machine.Object, "spec", "vms", "rdmDisks"))
			vmwareClusterNames.add(nestedString(machine.Object, "spec", "vms", "clusterName"))
			vmwareHostNames.add(nestedString(machine.Object, "spec", "vms", "esxiName"))
		}
	}

	exactResourceNames := map[string]stringSet{
		"networkmappings":     networkMappingNames,
		"storagemappings":     storageMappingNames,
		"arraycredsmappings":  arrayCredsMappingNames,
		"openstackcreds":      openstackCredNames,
		"vmwarecreds":         vmwareCredNames,
		"volumeimageprofiles": volumeImageProfileNames,
		"vjailbreaknodes":     nodeNames,
		"bmconfigs":           bmConfigNames,
	}
	for plural, names := range exactResourceNames {
		for i := range resourceLists[plural] {
			obj := &resourceLists[plural][i]
			if names.has(obj.GetName()) || (migrationUID != "" && isOwnedBy(obj, migrationOwnerKeys)) {
				addEntry(plural, obj)
			}
		}
	}

	for i := range resourceLists["arraycredsmappings"] {
		mapping := &resourceLists["arraycredsmappings"][i]
		if !arrayCredsMappingNames.has(mapping.GetName()) {
			continue
		}
		for _, item := range nestedSlice(mapping.Object, "spec", "mappings") {
			if entry, ok := item.(map[string]interface{}); ok {
				arrayCredsNames.add(nestedString(entry, "target"))
			}
		}
	}

	for i := range resourceLists["arraycreds"] {
		creds := &resourceLists["arraycreds"][i]
		if arrayCredsNames.has(creds.GetName()) || isOwnedBy(creds, migrationOwnerKeys) {
			addEntry("arraycreds", creds)
		}
	}

	for i := range resourceLists["rdmdisks"] {
		disk := &resourceLists["rdmdisks"][i]
		ownerMatches := false
		for _, ownerVM := range nestedSlice(disk.Object, "spec", "ownerVMs") {
			if str, ok := ownerVM.(string); ok && vmNames.has(str) {
				ownerMatches = true
				break
			}
		}
		if rdmDiskNames.has(disk.GetName()) ||
			rdmDiskNames.has(nestedString(disk.Object, "spec", "diskName")) ||
			ownerMatches ||
			isOwnedBy(disk, migrationOwnerKeys) {
			addEntry("rdmdisks", disk)
			openstackCredNames.add(nestedString(disk.Object, "spec", "openstackVolumeRef", "openstackCreds"))
		}
	}

	// OpenStack credential names can be discovered from RDM disks after the
	// first exact-name loop, so match them again.
	for i := range resourceLists["openstackcreds"] {
		cred := &resourceLists["openstackcreds"][i]
		if openstackCredNames.has(cred.GetName()) || (migrationUID != "" && isOwnedBy(cred, migrationOwnerKeys)) {
			addEntry("openstackcreds", cred)
		}
	}

	for i := range resourceLists["pcdclusters"] {
		cluster := &resourceLists["pcdclusters"][i]
		clusterCreds := strings.TrimSpace(cluster.GetLabels()[openstackCredsLabel])
		exactClusterMatch := pcdClusterNames.has(cluster.GetName())
		displayClusterMatch := pcdClusterNames.has(nestedString(cluster.Object, "spec", "clusterName"))
		credsMatch := clusterCreds == "" || openstackCredNames.has(clusterCreds)
		if exactClusterMatch || (displayClusterMatch && credsMatch) {
			addEntry("pcdclusters", cluster)
			pcdHostNames.addAll(nestedStringSlice(cluster.Object, "spec", "hosts"))
		}
	}

	for i := range resourceLists["pcdhosts"] {
		host := &resourceLists["pcdhosts"][i]
		if pcdHostNames.has(host.GetName()) || pcdHostNames.has(nestedString(host.Object, "spec", "hostName")) {
			addEntry("pcdhosts", host)
		}
	}

	for i := range resourceLists["vmwareclusters"] {
		cluster := &resourceLists["vmwareclusters"][i]
		if vmwareClusterNames.has(cluster.GetName()) || vmwareClusterNames.has(nestedString(cluster.Object, "spec", "name")) {
			addEntry("vmwareclusters", cluster)
			vmwareHostNames.addAll(nestedStringSlice(cluster.Object, "spec", "hosts"))
		}
	}

	for i := range resourceLists["vmwarehosts"] {
		host := &resourceLists["vmwarehosts"][i]
		if vmwareHostNames.has(host.GetName()) || vmwareHostNames.has(nestedString(host.Object, "spec", "name")) {
			addEntry("vmwarehosts", host)
		}
	}

	for i := range resourceLists["rollingmigrationplans"] {
		rollingPlan := &resourceLists["rollingmigrationplans"][i]
		linkedPlan := false
		for _, planName := range nestedSlice(rollingPlan.Object, "spec", "vmMigrationPlans") {
			if str, ok := planName.(string); ok && planNames.has(str) {
				linkedPlan = true
				break
			}
		}
		linkedVM := vmNames.has(nestedString(rollingPlan.Object, "status", "currentVM"))
		for _, statusField := range []string{"migratedVMs", "failedVMs"} {
			if linkedVM {
				break
			}
			for _, vm := range nestedSlice(rollingPlan.Object, "status", statusField) {
				if str, ok := vm.(string); ok && vmNames.has(str) {
					linkedVM = true
					break
				}
			}
		}
		if linkedPlan || linkedVM {
			addEntry("rollingmigrationplans", rollingPlan)
			rollingMigrationPlanNames.add(rollingPlan.GetName())
			bmConfigNames.add(nestedString(rollingPlan.Object, "spec", "bmConfigRef", "name"))
		}
	}

	for i := range resourceLists["bmconfigs"] {
		obj := &resourceLists["bmconfigs"][i]
		if bmConfigNames.has(obj.GetName()) {
			addEntry("bmconfigs", obj)
		}
	}

	for i := range resourceLists["clustermigrations"] {
		clusterMigration := &resourceLists["clustermigrations"][i]
		if rollingMigrationPlanNames.has(nestedString(clusterMigration.Object, "spec", "rollingMigrationPlanRef", "name")) {
			addEntry("clustermigrations", clusterMigration)
		}
	}

	for i := range resourceLists["esximigrations"] {
		esxiMigration := &resourceLists["esximigrations"][i]
		if rollingMigrationPlanNames.has(nestedString(esxiMigration.Object, "spec", "rollingMigrationPlanRef", "name")) ||
			vmwareHostNames.has(nestedString(esxiMigration.Object, "spec", "esxiName")) {
			addEntry("esximigrations", esxiMigration)
		}
	}

	for i := range resourceLists["esxisshcreds"] {
		sshCreds := &resourceLists["esxisshcreds"][i]
		if isOwnedBy(sshCreds, migrationOwnerKeys) {
			addEntry("esxisshcreds", sshCreds)
		}
	}

	// ClusterMigration, ESXIMigration and VjailbreakNode objects carry their
	// own credential references but are matched after the credential passes
	// above, so resolve credentials one final time from the collected entries.
	for key, entry := range entries {
		switch {
		case strings.HasPrefix(key, "clustermigrations/"), strings.HasPrefix(key, "esximigrations/"):
			openstackCredNames.add(nestedString(entry.Object.Object, "spec", "openstackCredsRef", "name"))
			vmwareCredNames.add(nestedString(entry.Object.Object, "spec", "vmwareCredsRef", "name"))
		case strings.HasPrefix(key, "vjailbreaknodes/"):
			openstackCredNames.add(nestedString(entry.Object.Object, "spec", "openstackCreds", "name"))
		}
	}
	for i := range resourceLists["openstackcreds"] {
		cred := &resourceLists["openstackcreds"][i]
		if openstackCredNames.has(cred.GetName()) {
			addEntry("openstackcreds", cred)
		}
	}
	for i := range resourceLists["vmwarecreds"] {
		cred := &resourceLists["vmwarecreds"][i]
		if vmwareCredNames.has(cred.GetName()) {
			addEntry("vmwarecreds", cred)
		}
	}

	out := make([]BundleEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, warnings
}
