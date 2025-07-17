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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"encoding/base64"
	"gopkg.in/yaml.v2"
    "k8s.io/client-go/rest"
)
 
type ValidationResult struct {
	NoMigrationPlans        bool
	NoRollingMigrationPlans bool
	AgentsScaledDown        bool
	VMwareCredsDeleted      bool
	OpenStackCredsDeleted   bool
	NoCustomResources       bool 
	PassedAll               bool
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

func RunPreUpgradeChecks(ctx context.Context, kubeClient client.Client, restConfig *rest.Config, targetVersion string) (*ValidationResult, error) {
	result := &ValidationResult{}

	gvr := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "migrationplans"}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err == nil {
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err == nil && len(unstructuredList.Items) == 0 {
			result.NoMigrationPlans = true
		}
	}

	gvr = schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "rollingmigrationplans"}
	dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err == nil {
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err == nil && len(unstructuredList.Items) == 0 {
			result.NoRollingMigrationPlans = true
		}
	}

	gvr = schema.GroupVersionResource{
    Group:    "vjailbreak.k8s.pf9.io",
    Version:  "v1alpha1",
    Resource: "vjailbreaknodes",
	}
	dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result.AgentsScaledDown = len(list.Items) == 0

	vmwareSecret := &corev1.Secret{}
	err = kubeClient.Get(ctx, client.ObjectKey{Name: "vmware-credentials", Namespace: "migration-system"}, vmwareSecret)
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
	err = kubeClient.Get(ctx, client.ObjectKey{Name: "openstack-credentials", Namespace: "migration-system"}, openstackSecret)
	if err != nil {
		if kerrors.IsNotFound(err) {
			result.OpenStackCredsDeleted = true
		} else {
			return nil, err
		}
	} else {
		result.OpenStackCredsDeleted = false
	}

	result.NoCustomResources, err = checkForAnyCustomResources(ctx, kubeClient, restConfig)
	if err != nil {
		return nil, err
	}

	result.PassedAll = result.AgentsScaledDown && 
		result.VMwareCredsDeleted && 
		result.OpenStackCredsDeleted && 
		result.NoMigrationPlans && 
		result.NoRollingMigrationPlans &&
		result.NoCustomResources
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
		
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
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

func BackupResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) error {
	log.Println("Starting backup of CRDs, ConfigMaps, Deployments, and CRs")

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
	if err := kubeClient.List(ctx, cmList, client.InNamespace("migration-system")); err == nil {
		for _, cm := range cmList.Items {
			cmYaml, err := yaml.Marshal(cm)
			if err == nil {
				backup["configmap:"+cm.Name] = base64.StdEncoding.EncodeToString(cmYaml)
			}
		}
	}

	depList := &appsv1.DeploymentList{}
	if err := kubeClient.List(ctx, depList, client.InNamespace("migration-system")); err == nil {
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
			unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
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
			Namespace: "migration-system",
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
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: "vjailbreak-upgrade-backup", Namespace: "migration-system"}, backupCM); err != nil {
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

func CleanupResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) error {
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

	vmwareSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vmware-credentials", Namespace: "migration-system"}}
	if err := kubeClient.Delete(ctx, vmwareSecret); err != nil && !kerrors.IsNotFound(err) {
		log.Printf("Failed to delete vmware-credentials secret: %v", err)
	} else {
		log.Println("Secret vmware-credentials deleted.")
	}

	openstackSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "openstack-credentials", Namespace: "migration-system"}}
	if err := kubeClient.Delete(ctx, openstackSecret); err != nil && !kerrors.IsNotFound(err) {
		log.Printf("Failed to delete openstack-credentials secret: %v", err)
	} else {
		log.Println("Secret openstack-credentials deleted.")
	}

	gvr := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "migrationplans"}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err == nil {
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err == nil && len(unstructuredList.Items) > 0 {
			for _, mp := range unstructuredList.Items {
				planToDelete := mp
				if err := kubeClient.Delete(ctx, &planToDelete); err != nil {
					log.Printf("Failed to delete MigrationPlan %s: %v", planToDelete.GetName(), err)
				} else {
					log.Printf("Deleted MigrationPlan: %s", planToDelete.GetName())
				}
			}
		}
	}

	gvr = schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "rollingmigrationplans"}
	dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err == nil {
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err == nil && len(unstructuredList.Items) > 0 {
			for _, rmp := range unstructuredList.Items {
				planToDelete := rmp
				if err := kubeClient.Delete(ctx, &planToDelete); err != nil {
					log.Printf("Failed to delete RollingMigrationPlan %s: %v", planToDelete.GetName(), err)
				} else {
					log.Printf("Deleted RollingMigrationPlan: %s", planToDelete.GetName())
				}
			}
		}
	}

	if err := deleteAllCustomResources(ctx, kubeClient, restConfig); err != nil {
		log.Printf("Failed to delete all custom resources: %v", err)
		return err
	}

	log.Println("Resource cleanup completed.")
	return nil
}

func deleteAllCustomResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) error {
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
	
	unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list %s CRs: %w", crInfo.Kind, err)
	}
	
	for _, item := range unstructuredList.Items {
		err := dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
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
