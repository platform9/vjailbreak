package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/platform9/vjailbreak/pkg/vpwned/upgrade"
	version "github.com/platform9/vjailbreak/pkg/vpwned/version"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"io/ioutil"
	"sigs.k8s.io/yaml"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)
 
type DeploymentConfig struct {
	Namespace     string
	Name          string
	ContainerName string
	ImagePrefix   string
}

type UpgradeProgress struct {
	CurrentStep    string
	TotalSteps     int
	CompletedSteps int
	Status         string // "in_progress", "completed", "failed", "rolled_back"
	Error          string
	StartTime      time.Time
	EndTime        *time.Time
}

var (
	upgradeProgress *UpgradeProgress
	
	deploymentConfigs = []DeploymentConfig{
		{
			Namespace:     "migration-system",
			Name:          "migration-controller-manager",
			ContainerName: "manager",
			ImagePrefix:   "quay.io/platform9/vjailbreak-controller",
		},
		{
			Namespace:     "migration-system", 
			Name:          "migration-vpwned-sdk",
			ContainerName: "vpwned",
			ImagePrefix:   "quay.io/platform9/vjailbreak-vpwned",
		},
		{
			Namespace:     "migration-system",
			Name:          "vjailbreak-ui",
			ContainerName: "vjailbreak-ui-container", 
			ImagePrefix:   "quay.io/platform9/vjailbreak-ui",
		},
	}
)

type VpwnedVersion struct {
	api.UnimplementedVersionServer
}

func (s *VpwnedVersion) Version(ctx context.Context, in *api.VersionRequest) (*api.VersionResponse, error) {
	return &api.VersionResponse{Version: version.Version}, nil
}

func (s *VpwnedVersion) GetAvailableTags(ctx context.Context, in *api.VersionRequest) (*api.AvailableUpdatesResponse, error) {
	owner := "platform9"
	repo := "vjailbreak"
	tags, err := upgrade.GetAllTags(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	var protoUpdates []*api.ReleaseInfo
	for _, tag := range tags {
		protoUpdates = append(protoUpdates, &api.ReleaseInfo{
			Version: tag,
			ReleaseNotes: "",
		})
	}
	return &api.AvailableUpdatesResponse{Updates: protoUpdates}, nil
}

func (s *VpwnedVersion) InitiateUpgrade(ctx context.Context, in *api.UpgradeRequest) (*api.UpgradeResponse, error) {
	upgradeProgress = &UpgradeProgress{
		CurrentStep:    "Starting upgrade",
		TotalSteps:     len(deploymentConfigs) + 4, // +4 for backup, pre-checks, CRD updates, and final validation
		CompletedSteps: 0,
		Status:         "in_progress",
		StartTime:      time.Now(),
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	upgradeProgress.CurrentStep = "Backing up resources"
	if err := upgrade.BackupResources(ctx, kubeClient, config); err != nil {
		upgradeProgress.Status = "failed"
		upgradeProgress.Error = fmt.Sprintf("Backup failed: %v", err)
		return nil, fmt.Errorf("backup failed: %w", err)
	}
	upgradeProgress.CompletedSteps++

	upgradeProgress.CurrentStep = "Running pre-upgrade checks"
	checks, err := upgrade.RunPreUpgradeChecks(ctx, kubeClient, config, in.TargetVersion)
	if err != nil {
		upgradeProgress.Status = "failed"
		upgradeProgress.Error = fmt.Sprintf("Pre-upgrade checks failed: %v", err)
		return nil, err
	}
	upgradeProgress.CompletedSteps++

	if !checks.NoCustomResources {
		currentCRs, _ := upgrade.DiscoverCurrentCRs(ctx, kubeClient)
		var crList []string
		for _, crInfo := range currentCRs {
			gvr := schema.GroupVersionResource{
				Group:    crInfo.Group,
				Version:  crInfo.Version,
				Resource: crInfo.Plural,
			}
			dynamicClient, err := dynamic.NewForConfig(config)
			if err != nil {
				continue
			}
			unstructuredList, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range unstructuredList.Items {
				crList = append(crList, crInfo.Kind+":"+item.GetName())
			}
		}
		return &api.UpgradeResponse{
			Checks: &api.ValidationResult{
				AgentsScaledDown:        checks.AgentsScaledDown,
				VmwareCredsDeleted:      checks.VMwareCredsDeleted,
				OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
				NoMigrationPlans:        checks.NoMigrationPlans,
				NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
				NoCustomResources:       checks.NoCustomResources,
				CrdsCompatible:          checks.CRDsCompatible,
				PassedAll:               false,
			},
			UpgradeStarted: false,
			CleanupRequired: true,
			CustomResourceList: crList,
		}, nil
	}

	if in.AutoCleanup {
		upgradeProgress.CurrentStep = "Performing automatic cleanup"
		if err := upgrade.CleanupResources(ctx, kubeClient, config); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("Automatic cleanup failed: %v", err)
			return nil, fmt.Errorf("automatic cleanup failed: %w", err)
		}
		checks, err = upgrade.RunPreUpgradeChecks(ctx, kubeClient, config, in.TargetVersion)
		if err != nil || !checks.PassedAll {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = "Checks failed even after cleanup"
			return nil, fmt.Errorf("checks failed even after cleanup")
		}
		upgradeProgress.CompletedSteps++
	}

	var protoChecks *api.ValidationResult
	if checks != nil {
		protoChecks = &api.ValidationResult{
			AgentsScaledDown:        checks.AgentsScaledDown,
			VmwareCredsDeleted:      checks.VMwareCredsDeleted,
			OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
			NoMigrationPlans:        checks.NoMigrationPlans,
			NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
			NoCustomResources:       checks.NoCustomResources,
			CrdsCompatible:          checks.CRDsCompatible,
			PassedAll:               checks.PassedAll,
		}
	}

	if checks.PassedAll {
		log.Printf("All checks passed. Starting upgrade to version: %s", in.TargetVersion)
		
		upgradeProgress.CurrentStep = "Updating deployment images"
		
		originalImages := make(map[string]string)
		
		for _, depConfig := range deploymentConfigs {
			upgradeProgress.CurrentStep = fmt.Sprintf("Updating %s deployment", depConfig.Name)
			
			currentImage, err := getCurrentDeploymentImage(ctx, kubeClient, depConfig)
			if err != nil {
				log.Printf("Warning: Could not get current image for %s: %v", depConfig.Name, err)
			} else {
				originalImages[depConfig.Name] = currentImage
			}
			
			newImage := fmt.Sprintf("%s:%s", depConfig.ImagePrefix, in.TargetVersion)
			if err := updateDeploymentImage(ctx, kubeClient, depConfig, newImage); err != nil {
				log.Printf("Failed to update %s, attempting rollback...", depConfig.Name)
				if rollbackErr := rollbackDeployment(ctx, kubeClient, depConfig, originalImages[depConfig.Name]); rollbackErr != nil {
					log.Printf("Rollback failed for %s: %v", depConfig.Name, rollbackErr)
				}
				
				upgradeProgress.Status = "failed"
				upgradeProgress.Error = fmt.Sprintf("Failed to update %s: %v", depConfig.Name, err)
				return nil, fmt.Errorf("failed to update %s: %w", depConfig.Name, err)
			}
			
			upgradeProgress.CompletedSteps++
		}
		
		upgradeProgress.CurrentStep = "Updating Custom Resource Definitions"
		if err := updateCRDs(ctx, kubeClient, in.TargetVersion); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("CRD update failed: %v", err)
			return nil, fmt.Errorf("CRD update failed: %w", err)
		}
		upgradeProgress.CompletedSteps++
		
		upgradeProgress.CurrentStep = "Validating upgrade"
		if err := validateUpgrade(ctx, kubeClient, in.TargetVersion); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("Upgrade validation failed: %v", err)
			return nil, fmt.Errorf("upgrade validation failed: %w", err)
		}
		upgradeProgress.CompletedSteps++
		
		now := time.Now()
		upgradeProgress.Status = "completed"
		upgradeProgress.CurrentStep = "Upgrade completed successfully"
		upgradeProgress.EndTime = &now

		log.Printf("Upgrade to version %s completed successfully", in.TargetVersion)
	} else {
		upgradeProgress.CurrentStep = "Rollback due to upgrade failure"
		upgradeProgress.Status = "rolled_back"
		upgradeProgress.Error = "Upgrade failed. Rolling back to previous state."
		now := time.Now()
		upgradeProgress.EndTime = &now
		log.Printf("Upgrade failed. Rolling back to previous state.")
		if err := upgrade.RestoreResources(ctx, kubeClient); err != nil {
			log.Printf("Rollback failed: %v", err)
			upgradeProgress.Error = fmt.Sprintf("Rollback failed: %v", err)
			upgradeProgress.Status = "rollback_failed"
		} else {
			log.Printf("Rollback completed successfully.")
		}
	}

	upgradeStarted := false
	if checks != nil {
		upgradeStarted = checks.PassedAll
	}
	
	return &api.UpgradeResponse{
		Checks:         protoChecks,
		UpgradeStarted: upgradeStarted,
	}, nil
}

func (s *VpwnedVersion) ConfirmCleanupAndUpgrade(ctx context.Context, in *api.UpgradeRequest) (*api.UpgradeResponse, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	_ = upgrade.CleanupResources(ctx, kubeClient, config)

	checks, err := upgrade.RunPreUpgradeChecks(ctx, kubeClient, config, in.TargetVersion)
	if err != nil || !checks.PassedAll {
		return &api.UpgradeResponse{
			Checks: &api.ValidationResult{
				AgentsScaledDown:        checks.AgentsScaledDown,
				VmwareCredsDeleted:      checks.VMwareCredsDeleted,
				OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
				NoMigrationPlans:        checks.NoMigrationPlans,
				NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
				NoCustomResources:       checks.NoCustomResources,
				CrdsCompatible:          checks.CRDsCompatible,
				PassedAll:               false,
			},
			UpgradeStarted: false,
			CleanupRequired: false,
		}, fmt.Errorf("checks failed after cleanup")
	}

	return s.InitiateUpgrade(ctx, in)
}

func (s *VpwnedVersion) RollbackUpgrade(ctx context.Context, in *api.VersionRequest) (*api.UpgradeProgressResponse, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	upgradeProgress.CurrentStep = "Restoring resources from backup"
	err = upgrade.RestoreResources(ctx, kubeClient)
	if err != nil {
		upgradeProgress.Status = "rollback_failed"
		upgradeProgress.Error = fmt.Sprintf("Rollback failed: %v", err)
		return &api.UpgradeProgressResponse{
			Status:  "rollback_failed",
			Error:   upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}
	upgradeProgress.Status = "rolled_back"
	upgradeProgress.Error = "Rollback completed successfully"
	return &api.UpgradeProgressResponse{
		Status:  "rolled_back",
		Error:   "",
		CurrentStep: upgradeProgress.CurrentStep,
	}, nil
}

func (s *VpwnedVersion) GetUpgradeProgress(ctx context.Context, in *api.VersionRequest) (*api.UpgradeProgressResponse, error) {
	if upgradeProgress == nil {
		return &api.UpgradeProgressResponse{
			Status: "no_upgrade_in_progress",
		}, nil
	}
	
	progress := float32(upgradeProgress.CompletedSteps) / float32(upgradeProgress.TotalSteps) * 100
	
	return &api.UpgradeProgressResponse{
		CurrentStep:    upgradeProgress.CurrentStep,
		Progress:       progress,
		Status:         upgradeProgress.Status,
		Error:          upgradeProgress.Error,
		StartTime:      upgradeProgress.StartTime.Format(time.RFC3339),
		EndTime:        func() string {
			if upgradeProgress.EndTime != nil {
				return upgradeProgress.EndTime.Format(time.RFC3339)
			}
			return ""
		}(),
	}, nil
}

func (s *VpwnedVersion) CleanupStep(ctx context.Context, in *api.CleanupStepRequest) (*api.CleanupStepResponse, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	step := in.Step
	var success bool
	var msg string
	switch step {
	case "no_migrationplans":
		success, msg = checkAndDeleteMigrationPlans(ctx, kubeClient, config)
	case "no_rollingmigrationplans":
		success, msg = checkAndDeleteRollingMigrationPlans(ctx, kubeClient, config)
	case "agent_scaled_down":
		success, msg = checkAndScaleDownAgent(ctx, kubeClient)
	case "vmware_creds_deleted":
		success, msg = checkAndDeleteSecret(ctx, kubeClient, "vmware-credentials")
	case "openstack_creds_deleted":
		success, msg = checkAndDeleteSecret(ctx, kubeClient, "openstack-credentials")
	case "no_custom_resources":
		success, msg = checkAndDeleteAllCustomResources(ctx, kubeClient, config)
	case "crds_compatible":
		success, msg = checkCRDCompatibility(ctx, kubeClient)
	default:
		return &api.CleanupStepResponse{Step: step, Success: false, Message: "Unknown step"}, nil
	}
	return &api.CleanupStepResponse{Step: step, Success: success, Message: msg}, nil
}

// Helper functions for each step
func checkAndDeleteMigrationPlans(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) (bool, string) {
	gvr := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "migrationplans"}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return false, "Failed to create dynamic client"
	}
	list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to list MigrationPlans"
	}
	for _, item := range list.Items {
		err := dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return false, "Failed to delete MigrationPlan: " + item.GetName()
		}
	}
	// Re-list to confirm
	list, err = dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to re-list MigrationPlans"
	}
	if len(list.Items) == 0 {
		return true, "No MigrationPlans remaining"
	}
	return false, "MigrationPlans still exist"
}

func checkAndDeleteRollingMigrationPlans(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) (bool, string) {
	gvr := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "rollingmigrationplans"}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return false, "Failed to create dynamic client"
	}
	list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to list RollingMigrationPlans"
	}
	for _, item := range list.Items {
		err := dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return false, "Failed to delete RollingMigrationPlan: " + item.GetName()
		}
	}
	list, err = dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to re-list RollingMigrationPlans"
	}
	if len(list.Items) == 0 {
		return true, "No RollingMigrationPlans remaining"
	}
	return false, "RollingMigrationPlans still exist"
}

func checkAndScaleDownAgent(ctx context.Context, kubeClient client.Client) (bool, string) {
	dep := &appsv1.Deployment{}
	err := kubeClient.Get(ctx, client.ObjectKey{Name: "migration-controller-manager", Namespace: "migration-system"}, dep)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return true, "Agent already scaled down"
		}
		return false, "Failed to get deployment"
	}
	var zero int32 = 0
	dep.Spec.Replicas = &zero
	err = kubeClient.Update(ctx, dep)
	if err != nil {
		return false, "Failed to scale down deployment"
	}
	if dep.Spec.Replicas != nil && *dep.Spec.Replicas == 0 {
		return true, "Agent scaled down"
	}
	return false, "Agent not scaled down"
}

func checkAndDeleteSecret(ctx context.Context, kubeClient client.Client, secretName string) (bool, string) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "migration-system"}}
	err := kubeClient.Delete(ctx, secret)
	if err != nil && !kerrors.IsNotFound(err) {
		return false, "Failed to delete secret: " + secretName
	}
	// Check if secret still exists
	err = kubeClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: "migration-system"}, secret)
	if kerrors.IsNotFound(err) {
		return true, secretName + " deleted"
	}
	return false, secretName + " still exists"
}

func checkAndDeleteAllCustomResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) (bool, string) {
	currentCRs, err := upgrade.DiscoverCurrentCRs(ctx, kubeClient)
	if err != nil {
		return false, "Failed to discover current CRs"
	}
	allDeleted := true
	var failed []string
	for _, crInfo := range currentCRs {
		gvr := schema.GroupVersionResource{
			Group:    crInfo.Group,
			Version:  crInfo.Version,
			Resource: crInfo.Plural,
		}
		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			failed = append(failed, crInfo.Kind+": dynamic client error")
			allDeleted = false
			continue
		}
		list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err != nil {
			failed = append(failed, crInfo.Kind+": list error")
			allDeleted = false
			continue
		}
		for _, item := range list.Items {
			err := dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
			if err != nil {
				failed = append(failed, crInfo.Kind+":"+item.GetName())
				allDeleted = false
			}
		}
		// Re-list to confirm
		list, err = dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err != nil || len(list.Items) > 0 {
			failed = append(failed, crInfo.Kind+": not empty after delete")
			allDeleted = false
		}
	}
	if allDeleted {
		return true, "No Custom Resources remaining"
	}
	return false, "Some Custom Resources could not be deleted: " + strings.Join(failed, ", ")
}

func checkCRDCompatibility(ctx context.Context, kubeClient client.Client) (bool, string) {
	currentCRDs, err := upgrade.DiscoverCurrentCRDs(ctx, kubeClient)
	if err != nil {
		return false, "Failed to discover current CRDs"
	}
	for _, crdInfo := range currentCRDs {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := kubeClient.Get(ctx, types.NamespacedName{Name: crdInfo.Name}, crd)
		if err != nil {
			if kerrors.IsNotFound(err) {
				continue
			}
			return false, "Failed to get CRD: " + crdInfo.Name
		}
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				// Optionally, check for instances or other compatibility logic
			}
		}
	}
	return true, "CRDs compatible"
}

func getCurrentDeploymentImage(ctx context.Context, kubeClient client.Client, depConfig DeploymentConfig) (string, error) {
	dep := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); err != nil {
		return "", err
	}
	
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == depConfig.ContainerName {
			return container.Image, nil
		}
	}
	
	return "", fmt.Errorf("container %s not found in deployment %s", depConfig.ContainerName, depConfig.Name)
}

func updateDeploymentImage(ctx context.Context, kubeClient client.Client, depConfig DeploymentConfig, newImage string) error {
	dep := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); err != nil {
		return fmt.Errorf("failed to get deployment %s: %w", depConfig.Name, err)
	}

	updated := false
	for i, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == depConfig.ContainerName {
			dep.Spec.Template.Spec.Containers[i].Image = newImage
			updated = true 
			break
		}
	}

	if updated {
		if err := kubeClient.Update(ctx, dep); err != nil {
			return fmt.Errorf("failed to update deployment %s: %w", depConfig.Name, err)
		}
		log.Printf("Updated deployment %s container %s to image %s", depConfig.Name, depConfig.ContainerName, newImage)
	} else {
		log.Printf("Container %s not found in deployment %s, skipping update", depConfig.ContainerName, depConfig.Name)
	}

	return nil
}

func rollbackDeployment(ctx context.Context, kubeClient client.Client, depConfig DeploymentConfig, originalImage string) error {
	if originalImage == "" {
		return fmt.Errorf("no original image available for rollback")
	}
	
	log.Printf("Rolling back %s to image: %s", depConfig.Name, originalImage)
	return updateDeploymentImage(ctx, kubeClient, depConfig, originalImage)
}

func validateUpgrade(ctx context.Context, kubeClient client.Client, targetVersion string) error {
	for _, depConfig := range deploymentConfigs {
		if err := waitForDeploymentReady(ctx, kubeClient, depConfig); err != nil {
			return fmt.Errorf("deployment %s not ready after upgrade: %w", depConfig.Name, err)
		}
		
		currentImage, err := getCurrentDeploymentImage(ctx, kubeClient, depConfig)
		if err != nil {
			return fmt.Errorf("failed to verify image for %s: %w", depConfig.Name, err)
		}
		
		if !strings.Contains(currentImage, targetVersion) {
			return fmt.Errorf("deployment %s image not updated to version %s, current: %s", 
				depConfig.Name, targetVersion, currentImage)
		}
	}
	
	return nil
}

func waitForDeploymentReady(ctx context.Context, kubeClient client.Client, depConfig DeploymentConfig) error {
	timeout := 5 * time.Minute
	interval := 10 * time.Second
	
	for start := time.Now(); time.Since(start) < timeout; {
		dep := &appsv1.Deployment{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("deployment %s not found", depConfig.Name)
			}
			return err
		}
		
		if dep.Status.ReadyReplicas == *dep.Spec.Replicas && dep.Status.UpdatedReplicas == *dep.Spec.Replicas {
			return nil
		}
		
		time.Sleep(interval)
	}
	
	return fmt.Errorf("deployment %s not ready within timeout", depConfig.Name)
}

func updateCRDs(ctx context.Context, kubeClient client.Client, targetVersion string) error {
	log.Printf("Updating CRDs to version: %s", targetVersion)
	
	comparison, err := upgrade.CompareVersions(ctx, kubeClient, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}
	
	if len(comparison.NewCRDs) > 0 {
		log.Printf("New CRDs to be created: %d", len(comparison.NewCRDs))
		for _, crd := range comparison.NewCRDs {
			log.Printf("  - %s (version: %s)", crd.Name, crd.Version)
		}
	}
	
	if len(comparison.UpdatedCRDs) > 0 {
		log.Printf("CRDs to be updated: %d", len(comparison.UpdatedCRDs))
		for _, crd := range comparison.UpdatedCRDs {
			log.Printf("  - %s (version: %s)", crd.Name, crd.Version)
		}
	}
	
	if len(comparison.RemovedCRDs) > 0 {
		log.Printf("CRDs to be removed: %d", len(comparison.RemovedCRDs))
		for _, crd := range comparison.RemovedCRDs {
			log.Printf("  - %s (version: %s)", crd.Name, crd.Version)
		}
	}
	
	for _, newCRD := range comparison.NewCRDs {
		if err := applyNewCRD(ctx, kubeClient, newCRD, targetVersion); err != nil {
			return fmt.Errorf("failed to apply new CRD %s: %w", newCRD.Name, err)
		}
		log.Printf("Successfully applied new CRD: %s", newCRD.Name)
	}
	
	for _, updatedCRD := range comparison.UpdatedCRDs {
		if err := updateExistingCRD(ctx, kubeClient, updatedCRD, targetVersion); err != nil {
			return fmt.Errorf("failed to update CRD %s: %w", updatedCRD.Name, err)
		}
		log.Printf("Successfully updated CRD: %s", updatedCRD.Name)
	}
	
	log.Printf("CRD update step completed successfully")
	return nil
}

func loadCRDsFromFile() ([]apiextensionsv1.CustomResourceDefinition, error) {
	data, err := ioutil.ReadFile("deploy/00crds.yaml")
	if err != nil {
		return nil, err
	}
	docs := strings.Split(string(data), "---")
	var crds []apiextensionsv1.CustomResourceDefinition
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" { continue }
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal([]byte(doc), &crd); err == nil && crd.Kind == "CustomResourceDefinition" {
			crds = append(crds, crd)
		}
	}
	return crds, nil
}

func applyNewCRD(ctx context.Context, kubeClient client.Client, crdInfo upgrade.CRDInfo, targetVersion string) error {
	crds, err := loadCRDsFromFile()
	if err != nil {
		return err
	}
	for _, crd := range crds {
		if crd.Name == crdInfo.Name {
			if err := kubeClient.Create(ctx, &crd); err != nil {
				return err
			}
			log.Printf("Applied new CRD: %s", crd.Name)
			return nil
		}
	}
	return fmt.Errorf("CRD manifest for %s not found in deploy/00crds.yaml", crdInfo.Name)
}

func updateExistingCRD(ctx context.Context, kubeClient client.Client, crdInfo upgrade.CRDInfo, targetVersion string) error {
	crds, err := loadCRDsFromFile()
	if err != nil {
		return err
	}
	for _, newCRD := range crds {
		if newCRD.Name == crdInfo.Name {
			existingCRD := &apiextensionsv1.CustomResourceDefinition{}
			if err := kubeClient.Get(ctx, client.ObjectKey{Name: crdInfo.Name}, existingCRD); err != nil {
				return err
			}
			existingCRD.Spec = newCRD.Spec
			if err := kubeClient.Update(ctx, existingCRD); err != nil {
				return err
			}
			log.Printf("Updated CRD: %s", newCRD.Name)
			return nil
		}
	}
	return fmt.Errorf("CRD manifest for %s not found in deploy/00crds.yaml", crdInfo.Name)
}
