package upgrade

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sigsyaml "sigs.k8s.io/yaml"

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

func BackupResourcesWithID(ctx context.Context, kubeClient client.Client, restConfig *rest.Config, backupID string) error {
	log.Println("Starting backup of resources...")
	backupLabel := map[string]string{"vjailbreak-backup": "true", "vjailbreak-backup-id": backupID}

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

	crdBackups := map[string]corev1.ConfigMap{}
	cmBackups := map[string]corev1.ConfigMap{}
	deployBackups := map[string]corev1.ConfigMap{}
	otherBackups := []corev1.ConfigMap{}

	for _, cm := range backupCMList.Items {
		switch {
		case strings.HasPrefix(cm.Name, "backup-crd-"):
			crdBackups[cm.Name] = cm
		case strings.HasPrefix(cm.Name, "backup-cm-"):
			cmBackups[cm.Name] = cm
		case strings.HasPrefix(cm.Name, "backup-deploy-"):
			deployBackups[cm.Name] = cm
		default:
			otherBackups = append(otherBackups, cm)
		}
	}

	for _, cm := range crdBackups {
		yamlData, ok := cm.Data["resource"]
		if !ok {
			log.Printf("backup %s missing resource key; skipping", cm.Name)
			continue
		}
		if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
			log.Printf("Failed to restore CRD from backup %s: %v", cm.Name, err)
		} else {
			log.Printf("Restored CRD from backup %s", cm.Name)
		}
	}

	if err := waitForCRDEstablished(ctx, kubeClient, 2*time.Minute); err != nil {
		log.Printf("Warning: CRDs not all established: %v", err)
	}
	for _, cm := range cmBackups {
		yamlData, ok := cm.Data["resource"]
		if !ok {
			continue
		}
		if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
			log.Printf("Failed to restore ConfigMap from backup %s: %v", cm.Name, err)
		} else {
			log.Printf("Restored ConfigMap from backup %s", cm.Name)
		}
	}

	controllerName := "migration-controller-manager"
	uiName := "vjailbreak-ui"
	sdkName := "migration-vpwned-sdk"
	ns := "migration-system"
	findDeployBackup := func(name string) (corev1.ConfigMap, bool) {
		key := "backup-deploy-" + name
		cm, ok := deployBackups[key]
		return cm, ok
	}

	if err := scaleDeploymentTo(ctx, kubeClient, controllerName, ns, 0); err != nil {
		log.Printf("Warning: failed to scale down controller %s: %v", controllerName, err)
	} else {
		log.Printf("Scaled down controller %s", controllerName)
		if err := waitForDeploymentScaledDownLocal(ctx, kubeClient, controllerName, ns, 60*time.Second); err != nil {
			log.Printf("Controller %s did not fully scale down: %v", controllerName, err)
		} else {
			log.Printf("Controller %s fully scaled down", controllerName)
		}
	}

	if cm, ok := findDeployBackup(controllerName); ok {
		yamlData := cm.Data["resource"]
		desired := parseReplicasFromDeploymentYAML([]byte(yamlData))
		if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
			log.Printf("Failed to apply controller deployment backup %s: %v", cm.Name, err)
		} else {
			log.Printf("Applied controller deployment backup %s", cm.Name)
			if desired < 1 {
				desired = 1
			}
			if err := scaleDeploymentTo(ctx, kubeClient, controllerName, ns, desired); err != nil {
				log.Printf("Failed to scale up controller %s to %d: %v", controllerName, desired, err)
			} else {
				if err := waitForDeploymentReadyLocal(ctx, kubeClient, controllerName, ns, 5*time.Minute); err != nil {
					log.Printf("Controller %s did not become ready: %v", controllerName, err)
				} else {
					log.Printf("Controller %s is ready", controllerName)
				}
			}
		}
	} else {
		log.Printf("No controller backup found for %s", controllerName)
	}

	if cm, ok := findDeployBackup(uiName); ok {
		yamlData := cm.Data["resource"]
		desired := parseReplicasFromDeploymentYAML([]byte(yamlData))
		if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
			log.Printf("Failed to apply UI deployment backup %s: %v", cm.Name, err)
		} else {
			if desired < 1 {
				desired = 1
			}
			if err := scaleDeploymentTo(ctx, kubeClient, uiName, ns, desired); err != nil {
				log.Printf("Failed to scale UI %s to %d: %v", uiName, desired, err)
			} else {
				if err := waitForDeploymentReadyLocal(ctx, kubeClient, uiName, ns, 5*time.Minute); err != nil {
					log.Printf("UI %s did not become ready: %v", uiName, err)
				} else {
					log.Printf("UI %s is ready", uiName)
				}
			}
		}
	} else {
		log.Printf("No UI backup found for %s", uiName)
	}

	if cm, ok := findDeployBackup(sdkName); ok {
		yamlData := cm.Data["resource"]
		desired := parseReplicasFromDeploymentYAML([]byte(yamlData))
		if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
			log.Printf("Failed to apply SDK deployment backup %s: %v", cm.Name, err)
		} else {
			if desired < 1 {
				desired = 1
			}
			if err := scaleDeploymentTo(ctx, kubeClient, sdkName, ns, desired); err != nil {
				log.Printf("Failed to scale SDK %s to %d: %v", sdkName, desired, err)
			} else {
				if err := waitForDeploymentReadyLocal(ctx, kubeClient, sdkName, ns, 5*time.Minute); err != nil {
					log.Printf("SDK %s did not become ready: %v", sdkName, err)
				} else {
					log.Printf("SDK %s is ready", sdkName)
				}
			}
		}
	} else {
		log.Printf("No SDK backup found for %s", sdkName)
	}

	for _, cm := range otherBackups {
		yamlData, ok := cm.Data["resource"]
		if !ok {
			continue
		}
		if err := applyRestoredObject(ctx, kubeClient, []byte(yamlData)); err != nil {
			log.Printf("Failed to restore resource from backup %s: %v", cm.Name, err)
		} else {
			log.Printf("Restored resource from backup %s", cm.Name)
		}
	}

	log.Println("Restore completed.")
	return nil
}

func parseReplicasFromDeploymentYAML(data []byte) int32 {
	var dep appsv1.Deployment
	if err := yaml.Unmarshal(data, &dep); err != nil {
		return 1
	}
	if dep.Spec.Replicas == nil {
		return 1
	}
	return *dep.Spec.Replicas
}

func scaleDeploymentTo(ctx context.Context, kubeClient client.Client, name, namespace string, target int32) error {
	dep := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
		return err
	}
	dep.Spec.Replicas = &target
	return kubeClient.Update(ctx, dep)
}

func waitForDeploymentReadyLocal(ctx context.Context, kubeClient client.Client, name, namespace string, timeout time.Duration) error {
	interval := 10 * time.Second
	for start := time.Now(); time.Since(start) < timeout; {
		dep := &appsv1.Deployment{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
			if kerrors.IsNotFound(err) {
				return fmt.Errorf("deployment %s not found", name)
			}
			return err
		}
		desired := int32(1)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		if dep.Status.ReadyReplicas == desired && dep.Status.UpdatedReplicas == desired {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("deployment %s not ready within timeout", name)
}

func waitForDeploymentScaledDownLocal(ctx context.Context, kubeClient client.Client, name, namespace string, timeout time.Duration) error {
	interval := 5 * time.Second
	for start := time.Now(); time.Since(start) < timeout; {
		dep := &appsv1.Deployment{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dep); err != nil {
			if kerrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if dep.Status.Replicas == 0 {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("deployment %s not scaled down within timeout", name)
}

func waitForCRDEstablished(ctx context.Context, kubeClient client.Client, timeout time.Duration) error {
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timed out waiting for CRDs to become Established")
		}
		crdList := &apiextensionsv1.CustomResourceDefinitionList{}
		if err := kubeClient.List(ctx, crdList); err != nil {
			log.Printf("Error listing CRDs while waiting for Established: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		allEstablished := true
		for _, crd := range crdList.Items {
			if !strings.Contains(crd.Spec.Group, "vjailbreak") {
				continue
			}
			established := false
			for _, c := range crd.Status.Conditions {
				if c.Type == apiextensionsv1.Established && c.Status == apiextensionsv1.ConditionTrue {
					established = true
					break
				}
			}
			if !established {
				allEstablished = false
				break
			}
		}
		if allEstablished {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}

func CleanupBackupConfigMaps(ctx context.Context, kubeClient client.Client, backupID string) error {
	selector := client.MatchingLabels{"vjailbreak-backup": "true"}
	if backupID != "" {
		selector = client.MatchingLabels{"vjailbreak-backup": "true", "vjailbreak-backup-id": backupID}
	}
	backupCMList := &corev1.ConfigMapList{}
	if err := kubeClient.List(ctx, backupCMList, client.InNamespace("migration-system"), selector); err != nil {
		return fmt.Errorf("failed to list backup ConfigMaps for cleanup: %w", err)
	}
	for _, cm := range backupCMList.Items {
		if err := kubeClient.Delete(ctx, &cm); err != nil && !kerrors.IsNotFound(err) {
			log.Printf("Failed to delete backup ConfigMap %s: %v", cm.Name, err)
		}
	}
	return nil
}

func applyRestoredObject(ctx context.Context, kubeClient client.Client, data []byte) error {
	jsonData, err := sigsyaml.YAMLToJSON(data)
	if err != nil {
		return fmt.Errorf("failed to convert yaml to json: %w", err)
	}
	unstructuredObj := &unstructured.Unstructured{}
	if err := unstructuredObj.UnmarshalJSON(jsonData); err != nil {
		return fmt.Errorf("failed to unmarshal into unstructured: %w", err)
	}

	objKey := client.ObjectKey{Name: unstructuredObj.GetName(), Namespace: unstructuredObj.GetNamespace()}
	existingObj := &unstructured.Unstructured{}
	existingObj.SetGroupVersionKind(unstructuredObj.GroupVersionKind())

	err = kubeClient.Get(ctx, objKey, existingObj)
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
		if err := deleteCRInstances(ctx, restConfig, crInfo); err != nil {
			log.Printf("Warning: Failed to delete %s CRs: %v", crInfo.Kind, err)
		}
	}

	return nil
}

func deleteCRInstances(ctx context.Context, restConfig *rest.Config, crInfo CRInfo) error {
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

func CleanupAllOldBackups(ctx context.Context, kubeClient client.Client) error {
	log.Println("Cleaning up old backup ConfigMaps...")

	backupCMList := &corev1.ConfigMapList{}
	if err := kubeClient.List(ctx, backupCMList, client.MatchingLabels{"vjailbreak-backup": "true"}); err != nil {
		return fmt.Errorf("failed to list backup ConfigMaps: %w", err)
	}

	for _, cm := range backupCMList.Items {
		if err := kubeClient.Delete(ctx, &cm); err != nil {
			log.Printf("Failed to delete old backup ConfigMap %s/%s: %v", cm.Namespace, cm.Name, err)
		} else {
			log.Printf("Deleted old backup ConfigMap %s/%s", cm.Namespace, cm.Name)
		}
	}

	log.Println("Old backup ConfigMaps cleanup complete.")
	return nil
}
