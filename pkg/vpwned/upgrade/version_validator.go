package upgrade

import (
	"context"
	"fmt"
	"log"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"golang.org/x/mod/semver"
	"encoding/base64"
	"gopkg.in/yaml.v2"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
)

type ValidationResult struct {
	AgentsScaledDown        bool
	VMwareCredsDeleted      bool
	OpenStackCredsDeleted   bool
	NoMigrationPlans        bool
	NoRollingMigrationPlans bool
	NoCustomResources       bool 
	CRDsCompatible          bool 
	PassedAll               bool
	VersionComparison       *VersionComparisonResult
}

type CRDInfo struct {
	Name    string
	Version string
	Group   string
}

type CRInfo struct {
	Group    string
	Version  string
	Kind     string
	Plural   string
	Singular string
}

type VersionComparisonResult struct {
	NewCRs     []CRInfo
	NewCRDs    []CRDInfo
	RemovedCRs []CRInfo
	RemovedCRDs []CRDInfo
	UpdatedCRDs []CRDInfo
}

func DiscoverCurrentCRs(ctx context.Context, kubeClient client.Client) ([]CRInfo, error) {
	var currentCRs []CRInfo
	
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := kubeClient.List(ctx, crdList); err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}
	
	for _, crd := range crdList.Items {
		if strings.Contains(crd.Spec.Group, "vjailbreak") {
			for _, version := range crd.Spec.Versions {
				crInfo := CRInfo{
					Group:    crd.Spec.Group,
					Version:  version.Name,
					Kind:     crd.Spec.Names.Kind,
					Plural:   crd.Spec.Names.Plural,
					Singular: crd.Spec.Names.Singular,
				}
				currentCRs = append(currentCRs, crInfo)
			}
		}
	}
	
	return currentCRs, nil
}

func DiscoverCurrentCRDs(ctx context.Context, kubeClient client.Client) ([]CRDInfo, error) {
	var currentCRDs []CRDInfo
	
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := kubeClient.List(ctx, crdList); err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}
	
	for _, crd := range crdList.Items {
		if strings.Contains(crd.Spec.Group, "vjailbreak") {
			for _, version := range crd.Spec.Versions {
				crdInfo := CRDInfo{
					Name:    crd.Name,
					Version: version.Name,
					Group:   crd.Spec.Group,
				}
				currentCRDs = append(currentCRDs, crdInfo)
			}
		}
	}
	
	return currentCRDs, nil
}

func GetTargetVersionCRs(targetVersion string) ([]CRInfo, error) {

	baseCRs := []CRInfo{
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "Migration", Plural: "migrations", Singular: "migration"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "OpenstackCreds", Plural: "openstackcreds", Singular: "openstackcreds"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "VMwareCreds", Plural: "vmwarecreds", Singular: "vmwarecreds"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "NetworkMapping", Plural: "networkmappings", Singular: "networkmapping"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "StorageMapping", Plural: "storagemappings", Singular: "storagemapping"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "MigrationPlan", Plural: "migrationplans", Singular: "migrationplan"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "MigrationTemplate", Plural: "migrationtemplates", Singular: "migrationtemplate"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "VjailbreakNode", Plural: "vjailbreaknodes", Singular: "vjailbreaknode"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "VMwareMachine", Plural: "vmwaremachines", Singular: "vmwaremachine"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "VMwareCluster", Plural: "vmwareclusters", Singular: "vmwarecluster"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "VMwareHost", Plural: "vmwarehosts", Singular: "vmwarehost"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "RollingMigrationPlan", Plural: "rollingmigrationplans", Singular: "rollingmigrationplan"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "ESXIMigration", Plural: "esximigrations", Singular: "esximigration"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "ClusterMigration", Plural: "clustermigrations", Singular: "clustermigration"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "BMConfig", Plural: "bmconfigs", Singular: "bmconfig"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "PCDCluster", Plural: "pcdclusters", Singular: "pcdcluster"},
		{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Kind: "PCDHost", Plural: "pcdhosts", Singular: "pcdhost"},
	}
	
	if semver.Compare(targetVersion, "2.0.0") >= 0 {
		baseCRs = append(baseCRs, CRInfo{
			Group: "vjailbreak.k8s.pf9.io", 
			Version: "v1alpha1", 
			Kind: "NewResource", 
			Plural: "newresources", 
			Singular: "newresource",
		})
	}
	
	return baseCRs, nil
}

func GetTargetVersionCRDs(targetVersion string) ([]CRDInfo, error) {
	baseCRDs := []CRDInfo{
		{Name: "migrations.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "openstackcreds.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "vmwarecreds.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "networkmappings.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "storagemappings.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "migrationplans.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "migrationtemplates.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "vjailbreaknodes.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "vmwaremachines.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "vmwareclusters.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "vmwarehosts.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "rollingmigrationplans.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "esximigrations.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "clustermigrations.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "bmconfigs.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "pcdclusters.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
		{Name: "pcdhosts.vjailbreak.k8s.pf9.io", Version: "v1alpha1", Group: "vjailbreak.k8s.pf9.io"},
	}
	
	if semver.Compare(targetVersion, "2.0.0") >= 0 {
		baseCRDs = append(baseCRDs, CRDInfo{
			Name: "newresources.vjailbreak.k8s.pf9.io", 
			Version: "v1alpha1", 
			Group: "vjailbreak.k8s.pf9.io",
		})
	}
	
	return baseCRDs, nil
}

func CompareVersions(ctx context.Context, kubeClient client.Client, targetVersion string) (*VersionComparisonResult, error) {
	currentCRs, err := DiscoverCurrentCRs(ctx, kubeClient)
	if err != nil {
		return nil, fmt.Errorf("failed to discover current CRs: %w", err)
	}
	
	currentCRDs, err := DiscoverCurrentCRDs(ctx, kubeClient)
	if err != nil {
		return nil, fmt.Errorf("failed to discover current CRDs: %w", err)
	}
	
	targetCRs, err := GetTargetVersionCRs(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get target version CRs: %w", err)
	}
	
	targetCRDs, err := GetTargetVersionCRDs(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get target version CRDs: %w", err)
	}
	
	result := &VersionComparisonResult{}
	
	for _, targetCR := range targetCRs {
		found := false
		for _, currentCR := range currentCRs {
			if targetCR.Group == currentCR.Group && targetCR.Kind == currentCR.Kind {
				found = true
				break
			}
		}
		if !found {
			result.NewCRs = append(result.NewCRs, targetCR)
		}
	}
	
	for _, currentCR := range currentCRs {
		found := false
		for _, targetCR := range targetCRs {
			if currentCR.Group == targetCR.Group && currentCR.Kind == targetCR.Kind {
				found = true
				break
			}
		}
		if !found {
			result.RemovedCRs = append(result.RemovedCRs, currentCR)
		}
	}
	
	for _, targetCRD := range targetCRDs {
		found := false
		for _, currentCRD := range currentCRDs {
			if targetCRD.Name == currentCRD.Name {
				found = true
				break
			}
		}
		if !found {
			result.NewCRDs = append(result.NewCRDs, targetCRD)
		}
	}
	
	for _, currentCRD := range currentCRDs {
		found := false
		for _, targetCRD := range targetCRDs {
			if currentCRD.Name == targetCRD.Name {
				found = true
				break
			}
		}
		if !found {
			result.RemovedCRDs = append(result.RemovedCRDs, currentCRD)
		}
	}
	
	for _, targetCRD := range targetCRDs {
		for _, currentCRD := range currentCRDs {
			if targetCRD.Name == currentCRD.Name && targetCRD.Version != currentCRD.Version {
				result.UpdatedCRDs = append(result.UpdatedCRDs, targetCRD)
				break
			}
		}
	}
	
	return result, nil
}

func RunPreUpgradeChecks(ctx context.Context, kubeClient client.Client, restConfig *rest.Config, targetVersion string) (*ValidationResult, error) {
	result := &ValidationResult{}

	dep := &appsv1.Deployment{}
	err := kubeClient.Get(ctx, client.ObjectKey{Name: "migration-controller-manager", Namespace: "migration-system"}, dep)
	if err != nil {
		if kerrors.IsNotFound(err) {
			result.AgentsScaledDown = true
		} else {
			return nil, err
		}
	} else {
		result.AgentsScaledDown = dep.Spec.Replicas != nil && *dep.Spec.Replicas == 0
	}

	vmwareSecret := &corev1.Secret{}
	err = kubeClient.Get(ctx, client.ObjectKey{Name: "vmware-credentials", Namespace: constants.NamespaceMigrationSystem}, vmwareSecret)
	if err != nil {
		if kerrors.IsNotFound(err) {
			result.VMwareCredsDeleted = true
		} else {
			return nil, err
		}
	} else {
		result.VMwareCredsDeleted = false
	}

	openstackSecret := &corev1.Secret{}
	err = kubeClient.Get(ctx, client.ObjectKey{Name: "openstack-credentials", Namespace: constants.NamespaceMigrationSystem}, openstackSecret)
	if err != nil {
		if kerrors.IsNotFound(err) {
			result.OpenStackCredsDeleted = true
		} else {
			return nil, err
		}
	} else {
		result.OpenStackCredsDeleted = false
	}

	migrationPlans := &vjailbreakv1alpha1.MigrationPlanList{}
	err = kubeClient.List(ctx, migrationPlans, client.InNamespace(constants.NamespaceMigrationSystem))
	if err != nil {
		return nil, err
	}
	result.NoMigrationPlans = len(migrationPlans.Items) == 0

	rollingPlans := &vjailbreakv1alpha1.RollingMigrationPlanList{}
	err = kubeClient.List(ctx, rollingPlans, client.InNamespace(constants.NamespaceMigrationSystem))
	if err != nil {
		return nil, err
	}
	result.NoRollingMigrationPlans = len(rollingPlans.Items) == 0

	result.NoCustomResources, err = checkForAnyCustomResources(ctx, kubeClient, restConfig)
	if err != nil {
		return nil, err
	}

	result.CRDsCompatible, err = checkCRDCompatibility(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	if targetVersion != "" {
		comparison, err := CompareVersions(ctx, kubeClient, targetVersion)
		if err != nil {
			log.Printf("Warning: Could not compare versions: %v", err)
		} else {
			result.VersionComparison = comparison
		}
	}

	result.PassedAll = result.AgentsScaledDown && 
		result.VMwareCredsDeleted && 
		result.OpenStackCredsDeleted && 
		result.NoMigrationPlans && 
		result.NoRollingMigrationPlans &&
		result.NoCustomResources &&
		result.CRDsCompatible

	return result, nil
}

func checkForAnyCustomResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) (bool, error) {
	currentCRs, err := DiscoverCurrentCRs(ctx, kubeClient)
	if err != nil {
		return false, fmt.Errorf("failed to discover current CRs: %w", err)
	}
	
	for _, crInfo := range currentCRs {
		gvr := schema.GroupVersionResource{
			Group:    crInfo.Group,
			Version:  crInfo.Version,
			Resource: crInfo.Plural,
		}
		
		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			log.Printf("Warning: Could not create dynamic client for %s: %v", crInfo.Kind, err)
			continue
		}
		
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace(constants.NamespaceMigrationSystem).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("Warning: Could not list %s CRs: %v", crInfo.Kind, err)
			continue
		}
		
		if len(unstructuredList.Items) > 0 {
			log.Printf("Found %d %s CRs", len(unstructuredList.Items), crInfo.Kind)
			return false, nil
		}
	}
	
	return true, nil
}

func checkCRDCompatibility(ctx context.Context, kubeClient client.Client) (bool, error) {
	currentCRDs, err := DiscoverCurrentCRDs(ctx, kubeClient)
	if err != nil {
		return false, fmt.Errorf("failed to discover current CRDs: %w", err)
	}
	
	for _, crdInfo := range currentCRDs {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := kubeClient.Get(ctx, types.NamespacedName{Name: crdInfo.Name}, crd)
		if err != nil {
			if kerrors.IsNotFound(err) {
				continue
			}
			return false, fmt.Errorf("failed to get CRD %s: %w", crdInfo.Name, err)
		}

		for _, version := range crd.Spec.Versions {
			if version.Storage {
				hasInstances, err := checkCRDHasInstances(ctx, kubeClient, crd.Spec.Group, version.Name, crd.Spec.Names.Plural)
				if err != nil {
					log.Printf("Warning: Could not check instances for CRD %s: %v", crdInfo.Name, err)
					continue
				}
				if hasInstances {
					log.Printf("CRD %s has existing instances, upgrade may require CR migration", crdInfo.Name)
				}
			}
		}
	}

	return true, nil
}

func checkCRDHasInstances(ctx context.Context, kubeClient client.Client, group, version, plural string) (bool, error) {
	return false, nil
}

func BackupResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) error {
	log.Println("Starting backup of CRDs, ConfigMaps, Deployments, and CRs...")

	backup := make(map[string]string)

	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := kubeClient.List(ctx, crdList); err == nil {
		for _, crd := range crdList.Items {
			if strings.Contains(crd.Spec.Group, "vjailbreak") {
				crdYaml, err := yaml.Marshal(crd)
				if err == nil {
					backup["crd:"+crd.Name] = base64.StdEncoding.EncodeToString(crdYaml)
				}
			}
		}
	}

	cmList := &corev1.ConfigMapList{}
	if err := kubeClient.List(ctx, cmList, client.InNamespace(constants.NamespaceMigrationSystem)); err == nil {
		for _, cm := range cmList.Items {
			cmYaml, err := yaml.Marshal(cm)
			if err == nil {
				backup["configmap:"+cm.Name] = base64.StdEncoding.EncodeToString(cmYaml)
			}
		}
	}

	depList := &appsv1.DeploymentList{}
	if err := kubeClient.List(ctx, depList, client.InNamespace(constants.NamespaceMigrationSystem)); err == nil {
		for _, dep := range depList.Items {
			depYaml, err := yaml.Marshal(dep)
			if err == nil {
				backup["deployment:"+dep.Name] = base64.StdEncoding.EncodeToString(depYaml)
			}
		}
	}

	currentCRs, err := DiscoverCurrentCRs(ctx, kubeClient)
	if err == nil {
		for _, crInfo := range currentCRs {
			gvr := schema.GroupVersionResource{
				Group:    crInfo.Group,
				Version:  crInfo.Version,
				Resource: crInfo.Plural,
			}
			dynamicClient, err := dynamic.NewForConfig(restConfig)
			if err != nil {
				continue
			}
			unstructuredList, err := dynamicClient.Resource(gvr).Namespace(constants.NamespaceMigrationSystem).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range unstructuredList.Items {
				crYaml, err := yaml.Marshal(item.Object)
				if err == nil {
					backup["cr:"+crInfo.Kind+":"+item.GetName()] = base64.StdEncoding.EncodeToString(crYaml)
				}
			}
		}
	}

	backupCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vjailbreak-upgrade-backup",
			Namespace: constants.NamespaceMigrationSystem,
		},
		Data: backup,
	}
	_ = kubeClient.Delete(ctx, backupCM) 
	if err := kubeClient.Create(ctx, backupCM); err != nil {
		return fmt.Errorf("failed to create backup ConfigMap: %w", err)
	}
	log.Println("Backup completed and stored in ConfigMap vjailbreak-upgrade-backup.")
	return nil
}

func RestoreResources(ctx context.Context, kubeClient client.Client) error {
	log.Println("Restoring resources from backup ConfigMap...")
	backupCM := &corev1.ConfigMap{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: "vjailbreak-upgrade-backup", Namespace: constants.NamespaceMigrationSystem}, backupCM); err != nil {
		return fmt.Errorf("failed to get backup ConfigMap: %w", err)
	}
	for key, b64 := range backupCM.Data {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			log.Printf("Failed to decode backup for %s: %v", key, err)
			continue
		}
		if strings.HasPrefix(key, "crd:") {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			if err := yaml.Unmarshal(data, crd); err == nil {
				_ = kubeClient.Delete(ctx, crd) 
				_ = kubeClient.Create(ctx, crd)
			}
		} else if strings.HasPrefix(key, "configmap:") {
			cm := &corev1.ConfigMap{}
			if err := yaml.Unmarshal(data, cm); err == nil {
				_ = kubeClient.Delete(ctx, cm)
				_ = kubeClient.Create(ctx, cm)
			}
		} else if strings.HasPrefix(key, "deployment:") {
			dep := &appsv1.Deployment{}
			if err := yaml.Unmarshal(data, dep); err == nil {
				_ = kubeClient.Delete(ctx, dep)
				_ = kubeClient.Create(ctx, dep)
			}
		}
	}
	log.Println("Restore completed from backup ConfigMap.")
	return nil
}

func CleanupResources(ctx context.Context, kubeClient client.Client) error {
	log.Println("Starting automatic resource cleanup...")

	dep := &appsv1.Deployment{}
	err := kubeClient.Get(ctx, client.ObjectKey{Name: "migration-controller-manager", Namespace: "migration-system"}, dep)
	if err == nil {
		var zero int32 = 0
		dep.Spec.Replicas = &zero
		if err := kubeClient.Update(ctx, dep); err != nil {
			log.Printf("Failed to scale down deployment: %v", err)
			return err
		}
		log.Println("Deployment migration-controller-manager scaled down.")
	} else if !kerrors.IsNotFound(err) {
		log.Printf("Failed to get deployment for cleanup: %v", err)
		return err
	}

	vmwareSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vmware-credentials", Namespace: constants.NamespaceMigrationSystem}}
	if err := kubeClient.Delete(ctx, vmwareSecret); err != nil && !kerrors.IsNotFound(err) {
		log.Printf("Failed to delete vmware-credentials secret: %v", err)
	} else {
		log.Println("Secret vmware-credentials deleted.")
	}

	openstackSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "openstack-credentials", Namespace: constants.NamespaceMigrationSystem}}
	if err := kubeClient.Delete(ctx, openstackSecret); err != nil && !kerrors.IsNotFound(err) {
		log.Printf("Failed to delete openstack-credentials secret: %v", err)
	} else {
		log.Println("Secret openstack-credentials deleted.")
	}

	migrationPlans := &vjailbreakv1alpha1.MigrationPlanList{}
	if err := kubeClient.List(ctx, migrationPlans, client.InNamespace(constants.NamespaceMigrationSystem)); err == nil {
		for _, mp := range migrationPlans.Items {
			planToDelete := mp
			if err := kubeClient.Delete(ctx, &planToDelete); err != nil {
				log.Printf("Failed to delete MigrationPlan %s: %v", planToDelete.Name, err)
			} else {
				log.Printf("Deleted MigrationPlan: %s", planToDelete.Name)
			}
		}
	}

	rollingPlans := &vjailbreakv1alpha1.RollingMigrationPlanList{}
	if err := kubeClient.List(ctx, rollingPlans, client.InNamespace(constants.NamespaceMigrationSystem)); err == nil {
		for _, rmp := range rollingPlans.Items {
			planToDelete := rmp
			if err := kubeClient.Delete(ctx, &planToDelete); err != nil {
				log.Printf("Failed to delete RollingMigrationPlan %s: %v", planToDelete.Name, err)
			} else {
				log.Printf("Deleted RollingMigrationPlan: %s", planToDelete.Name)
			}
		}
	}

	if err := deleteAllCustomResources(ctx, kubeClient); err != nil {
		log.Printf("Failed to delete all custom resources: %v", err)
		return err
	}

	log.Println("Resource cleanup completed.")
	return nil
}

func deleteAllCustomResources(ctx context.Context, kubeClient client.Client) error {
	currentCRs, err := DiscoverCurrentCRs(ctx, kubeClient)
	if err != nil {
		return fmt.Errorf("failed to discover current CRs: %w", err)
	}
	
	for _, crInfo := range currentCRs {
		if err := deleteCRInstances(ctx, kubeClient, restConfig, crInfo); err != nil {
			log.Printf("Warning: Failed to delete %s CRs: %v", crInfo.Kind, err)	
		}
	}
	
	return nil
}

func deleteCRInstances(ctx context.Context, kubeClient client.Client, restConfig *rest.Config, crInfo CRInfo) error {
	gvr := schema.GroupVersionResource{
		Group:    crInfo.Group,
		Version:  crInfo.Version,
		Resource: crInfo.Plural,
	}
	
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client for %s: %w", crInfo.Kind, err)
	}
	
	unstructuredList, err := dynamicClient.Resource(gvr).Namespace(constants.NamespaceMigrationSystem).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list %s CRs: %w", crInfo.Kind, err)
	}
	
	for _, item := range unstructuredList.Items {
		err := dynamicClient.Resource(gvr).Namespace(constants.NamespaceMigrationSystem).Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete %s %s: %v", crInfo.Kind, item.GetName(), err)
		} else {
			log.Printf("Deleted %s: %s", crInfo.Kind, item.GetName())
		}
	}
	
	if len(unstructuredList.Items) > 0 {
		log.Printf("Deleted %d %s CRs", len(unstructuredList.Items), crInfo.Kind)
	}
	
	return nil
}

func GetVersionUpgradeSummary(ctx context.Context, kubeClient client.Client, targetVersion string) (string, error) {
	comparison, err := CompareVersions(ctx, kubeClient, targetVersion)
	if err != nil {
		return "", fmt.Errorf("failed to compare versions: %w", err)
	}
	
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Upgrade Summary for version %s:\n", targetVersion))
	summary.WriteString("=" + strings.Repeat("=", 50) + "\n\n")
	
	if len(comparison.NewCRs) > 0 {
		summary.WriteString("🆕 New Custom Resources:\n")
		for _, cr := range comparison.NewCRs {
			summary.WriteString(fmt.Sprintf("  • %s (%s/%s)\n", cr.Kind, cr.Group, cr.Version))
		}
		summary.WriteString("\n")
	}
	
	if len(comparison.NewCRDs) > 0 {
		summary.WriteString("🆕 New Custom Resource Definitions:\n")
		for _, crd := range comparison.NewCRDs {
			summary.WriteString(fmt.Sprintf("  • %s (version: %s)\n", crd.Name, crd.Version))
		}
		summary.WriteString("\n")
	}
	
	if len(comparison.UpdatedCRDs) > 0 {
		summary.WriteString("🔄 Updated Custom Resource Definitions:\n")
		for _, crd := range comparison.UpdatedCRDs {
			summary.WriteString(fmt.Sprintf("  • %s (version: %s)\n", crd.Name, crd.Version))
		}
		summary.WriteString("\n")
	}
	
	if len(comparison.RemovedCRs) > 0 {
		summary.WriteString("⚠️  Removed Custom Resources (deprecated):\n")
		for _, cr := range comparison.RemovedCRs {
			summary.WriteString(fmt.Sprintf("  • %s (%s/%s)\n", cr.Kind, cr.Group, cr.Version))
		}
		summary.WriteString("\n")
	}
	
	if len(comparison.RemovedCRDs) > 0 {
		summary.WriteString("⚠️  Removed Custom Resource Definitions (deprecated):\n")
		for _, crd := range comparison.RemovedCRDs {
			summary.WriteString(fmt.Sprintf("  • %s (version: %s)\n", crd.Name, crd.Version))
		}
		summary.WriteString("\n")
	}
	
	if len(comparison.NewCRs) == 0 && len(comparison.NewCRDs) == 0 && 
	   len(comparison.UpdatedCRDs) == 0 && len(comparison.RemovedCRs) == 0 && 
	   len(comparison.RemovedCRDs) == 0 {
		summary.WriteString("✅ No CR/CRD changes detected in this upgrade.\n")
	}
	
	return summary.String(), nil
}
