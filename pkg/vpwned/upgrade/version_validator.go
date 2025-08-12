package upgrade

import (
	"context"

	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	log.Println("Starting backup of resources...")
	backupLabel := map[string]string{"vjailbreak-backup": "true"}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := kubeClient.List(ctx, crdList); err == nil {
		for _, crd := range crdList.Items {
			if strings.Contains(crd.Spec.Group, "vjailbreak") {
				var buffer strings.Builder
				crd.GetObjectKind().SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
				if err := s.Encode(&crd, &buffer); err == nil {
					backupCM := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "backup-crd-" + strings.ReplaceAll(crd.Name, ".", "-"), Namespace: "migration-system", Labels: backupLabel},
					}
					_, _ = controllerutil.CreateOrUpdate(ctx, kubeClient, backupCM, func() error {
						backupCM.Data = map[string]string{"resource": buffer.String()}
						return nil
					})
				}
			}
		}
	}

	cmList := &corev1.ConfigMapList{}
	if err := kubeClient.List(ctx, cmList, client.InNamespace("migration-system")); err == nil {
		for _, cm := range cmList.Items {
			if _, ok := cm.Labels["vjailbreak-backup"]; ok {
				continue
			}
			var buffer strings.Builder
			cm.GetObjectKind().SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
			if err := s.Encode(&cm, &buffer); err == nil {
				backupCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "backup-cm-" + cm.Name, Namespace: "migration-system", Labels: backupLabel},
				}
				_, _ = controllerutil.CreateOrUpdate(ctx, kubeClient, backupCM, func() error {
					backupCM.Data = map[string]string{"resource": buffer.String()}
					return nil
				})
			}
		}
	}

	depList := &appsv1.DeploymentList{}
	if err := kubeClient.List(ctx, depList, client.InNamespace("migration-system")); err == nil {
		for _, dep := range depList.Items {
			var buffer strings.Builder
			dep.GetObjectKind().SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
			if err := s.Encode(&dep, &buffer); err == nil {
				backupCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "backup-deploy-" + dep.Name, Namespace: "migration-system", Labels: backupLabel},
				}
				_, _ = controllerutil.CreateOrUpdate(ctx, kubeClient, backupCM, func() error {
					backupCM.Data = map[string]string{"resource": buffer.String()}
					return nil
				})
			}
		}
	}
	log.Println("Backup completed.")
	return nil
}

func RestoreResources(ctx context.Context, kubeClient client.Client) error {
	log.Println("Restoring resources from backups...")

	backupLabelSelector := client.MatchingLabels{"vjailbreak-backup": "true"}
	backupCMList := &corev1.ConfigMapList{}
	if err := kubeClient.List(ctx, backupCMList, client.InNamespace("migration-system"), backupLabelSelector); err != nil {
		return fmt.Errorf("failed to list backup ConfigMaps: %w", err)
	}

	resourceTypes := []string{"crd", "configmap", "deployment"}
	for _, resourceType := range resourceTypes {
		for _, backupCM := range backupCMList.Items {
			if strings.HasPrefix(backupCM.Name, "backup-"+resourceType+"-") {
				yamlData, ok := backupCM.Data["resource"]
				if !ok {
					continue
				}
				if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
					log.Printf("Failed to restore resource from backup %s: %v", backupCM.Name, err)
				} else {
					log.Printf("Restored resource from backup %s", backupCM.Name)
				}
			}
		}
	}

	log.Println("Cleaning up backup ConfigMaps...")
	for _, backupCM := range backupCMList.Items {
		_ = kubeClient.Delete(ctx, &backupCM)
	}

	log.Println("Restore completed.")
	return nil
}

func applyRestoredObject(ctx context.Context, kubeClient client.Client, data []byte) error {
	unstructuredObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, unstructuredObj); err != nil {
		return err
	}

	objKey := client.ObjectKey{Name: unstructuredObj.GetName(), Namespace: unstructuredObj.GetNamespace()}
	existingObj := &unstructured.Unstructured{}
	existingObj.SetGroupVersionKind(unstructuredObj.GroupVersionKind())

	err := kubeClient.Get(ctx, objKey, existingObj)
	if err != nil {
		if kerrors.IsNotFound(err) {
			unstructuredObj.SetResourceVersion("")
			return kubeClient.Create(ctx, unstructuredObj)
		}
		return err
	}

	unstructuredObj.SetResourceVersion(existingObj.GetResourceVersion())
	return kubeClient.Update(ctx, unstructuredObj)
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
