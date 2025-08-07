package upgrade

import (
	"context"
	"fmt"
	"log"
	"strings"

	"encoding/base64"
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ValidationResult struct {
	NoMigrationPlans        bool
	NoRollingMigrationPlans bool
	VMwareCredsDeleted      bool
	OpenStackCredsDeleted   bool
	AgentsScaledDown        bool
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
	if len(list.Items) == 0 {
		result.AgentsScaledDown = true
	} else if len(list.Items) == 1 && list.Items[0].GetName() == "vjailbreak-master" {
		result.AgentsScaledDown = true
	} else {
		result.AgentsScaledDown = false
	}

	result.NoCustomResources, err = checkForAnyCustomResources(ctx, kubeClient, restConfig)
	if err != nil {
		return nil, err
	}

	result.PassedAll = result.NoMigrationPlans &&
		result.NoRollingMigrationPlans &&
		result.VMwareCredsDeleted &&
		result.OpenStackCredsDeleted &&
		result.AgentsScaledDown &&
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

		for _, item := range unstructuredList.Items {
			if crInfo.Kind == "VjailbreakNode" && item.GetName() == "vjailbreak-master" {
				continue
			}
			log.Printf("Found custom resource %s: %s", crInfo.Kind, item.GetName())
			return false, nil
		}
	}

	return true, nil
}

func BackupResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) error {
	log.Println("Starting backup of CRDs, ConfigMaps, Deployments, and CRs")

	backup := make(map[string]string)
	totalSize := 0
	const maxConfigMapSize = 1000000

	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := kubeClient.List(ctx, crdList); err == nil {
		for _, crd := range crdList.Items {
			if strings.Contains(crd.Spec.Group, "vjailbreak") {
				crdYaml, err := yaml.Marshal(crd)
				if err == nil {
					key := "crd-" + strings.ReplaceAll(crd.Name, ":", "-")
					value := base64.StdEncoding.EncodeToString(crdYaml)
					if totalSize+len(key)+len(value) > maxConfigMapSize {
						log.Printf("Warning: ConfigMap size limit reached, skipping remaining CRDs")
						break
					}

					backup[key] = value
					totalSize += len(key) + len(value)
				}
			}
		}
	}

	cmList := &corev1.ConfigMapList{}
	if err := kubeClient.List(ctx, cmList, client.InNamespace("migration-system")); err == nil {
		for _, cm := range cmList.Items {
			cmYaml, err := yaml.Marshal(cm)
			if err == nil {
				key := "configmap-" + strings.ReplaceAll(cm.Name, ":", "-")
				value := base64.StdEncoding.EncodeToString(cmYaml)
				if totalSize+len(key)+len(value) > maxConfigMapSize {
					log.Printf("Warning: ConfigMap size limit reached, skipping remaining ConfigMaps")
					break
				}

				backup[key] = value
				totalSize += len(key) + len(value)
			}
		}
	}

	depList := &appsv1.DeploymentList{}
	if err := kubeClient.List(ctx, depList, client.InNamespace("migration-system")); err == nil {
		for _, dep := range depList.Items {
			depYaml, err := yaml.Marshal(dep)
			if err == nil {
				key := "deployment-" + strings.ReplaceAll(dep.Name, ":", "-")
				value := base64.StdEncoding.EncodeToString(depYaml)
				if totalSize+len(key)+len(value) > maxConfigMapSize {
					log.Printf("Warning: ConfigMap size limit reached, skipping remaining Deployments")
					break
				}

				backup[key] = value
				totalSize += len(key) + len(value)
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
					key := "cr-" + strings.ReplaceAll(crInfo.Kind, ":", "-") + "-" + strings.ReplaceAll(item.GetName(), ":", "-")
					value := base64.StdEncoding.EncodeToString(crYaml)
					if totalSize+len(key)+len(value) > maxConfigMapSize {
						log.Printf("Warning: ConfigMap size limit reached, skipping remaining CRs")
						break
					}

					backup[key] = value
					totalSize += len(key) + len(value)
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
	log.Printf("Backup completed and stored in ConfigMap vjailbreak-upgrade-backup. Total size: %d bytes", totalSize)
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
		if strings.HasPrefix(key, "crd-") {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			if err := yaml.Unmarshal(data, crd); err == nil {
				_ = kubeClient.Delete(ctx, crd)
				_ = kubeClient.Create(ctx, crd)
			}
		} else if strings.HasPrefix(key, "configmap-") {
			cm := &corev1.ConfigMap{}
			if err := yaml.Unmarshal(data, cm); err == nil {
				_ = kubeClient.Delete(ctx, cm)
				_ = kubeClient.Create(ctx, cm)
			}
		} else if strings.HasPrefix(key, "deployment-") {
			dep := &appsv1.Deployment{}
			if err := yaml.Unmarshal(data, dep); err == nil {
				_ = kubeClient.Delete(ctx, dep)
				_ = kubeClient.Create(ctx, dep)
			}
		} else if strings.HasPrefix(key, "cr-") {
			var obj map[string]interface{}
			if err := yaml.Unmarshal(data, &obj); err == nil {
				unstructured := &unstructured.Unstructured{}
				if err := yaml.Unmarshal(data, unstructured); err == nil {
					_ = kubeClient.Delete(ctx, unstructured)
					_ = kubeClient.Create(ctx, unstructured)
				}
			}
		}
	}
	log.Println("Restore completed from backup ConfigMap.")
	return nil
}

func CleanupResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) error {
	log.Println("Starting automatic resource cleanup...")
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gvrMigrationPlans := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "migrationplans"}
	mpList, err := dynamicClient.Resource(gvrMigrationPlans).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range mpList.Items {
			_ = dynamicClient.Resource(gvrMigrationPlans).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		}
		log.Println("Deleted MigrationPlans.")
	}

	gvrRollingPlans := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "rollingmigrationplans"}
	rmpList, err := dynamicClient.Resource(gvrRollingPlans).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range rmpList.Items {
			_ = dynamicClient.Resource(gvrRollingPlans).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		}
		log.Println("Deleted RollingMigrationPlans.")
	}

	gvrNodes := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "vjailbreaknodes"}
	nodeList, err := dynamicClient.Resource(gvrNodes).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range nodeList.Items {
			if item.GetName() != "vjailbreak-master" {
				_ = dynamicClient.Resource(gvrNodes).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
			}
		}
		log.Println("Scaled down agents by deleting non-master nodes.")
	}

	vmwareSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vmware-credentials", Namespace: "migration-system"}}
	if err := kubeClient.Delete(ctx, vmwareSecret); err == nil || kerrors.IsNotFound(err) {
		log.Println("Secret vmware-credentials deleted.")
	}

	openstackSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "openstack-credentials", Namespace: "migration-system"}}
	if err := kubeClient.Delete(ctx, openstackSecret); err == nil || kerrors.IsNotFound(err) {
		log.Println("Secret openstack-credentials deleted.")
	}

	if err := deleteAllCustomResources(ctx, kubeClient, restConfig); err != nil {
		log.Printf("Failed to delete all custom resources: %v", err)
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

func fetchCRDsFromGitHub(tag string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/platform9/vjailbreak/%s/deploy/upgrade-all-resources.yaml", tag)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch CRDs: %s", resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

func ApplyAllCRDs(ctx context.Context, kubeClient client.Client, tag string) error {
	data, err := fetchCRDsFromGitHub(tag)
	if err != nil {
		return err
	}
	docs := strings.Split(string(data), "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal([]byte(doc), &crd); err == nil && crd.Kind == "CustomResourceDefinition" {
			err := kubeClient.Create(ctx, &crd)
			if err != nil {
				if kerrors.IsAlreadyExists(err) {
					existing := &apiextensionsv1.CustomResourceDefinition{}
					if err := kubeClient.Get(ctx, client.ObjectKey{Name: crd.Name}, existing); err != nil {
						return err
					}
					existing.Spec = crd.Spec
					if err := kubeClient.Update(ctx, existing); err != nil {
						return err
					}
					log.Printf("Updated CRD: %s", crd.Name)
				} else {
					return err
				}
			} else {
				log.Printf("Created CRD: %s", crd.Name)
			}
		}
	}
	return nil
}

func fetchVersionConfigFromGitHub(tag string) ([]byte, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/platform9/vjailbreak/%s/image_builder/configs/version-config.yaml", tag)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch version-config: %s", resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

func UpdateVersionConfigMapFromGitHub(ctx context.Context, kubeClient client.Client, tag string) error {
	data, err := fetchVersionConfigFromGitHub(tag)
	if err != nil {
		return err
	}
	rendered := strings.ReplaceAll(string(data), "${TAG}", tag)
	cm := &corev1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(rendered), cm); err != nil {
		return err
	}
	cm.Namespace = "migration-system"
	cm.Name = "version-config"
	if err := kubeClient.Update(ctx, cm); err != nil {
		if kerrors.IsNotFound(err) {
			return kubeClient.Create(ctx, cm)
		}
		return err
	}
	return nil
}
