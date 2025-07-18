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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"os/exec"
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
	Status         string
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
		TotalSteps:     len(deploymentConfigs) + 4, // +4 for backup, pre-checks, CRD and deployment update, and final validation
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
				NoMigrationPlans:        checks.NoMigrationPlans,
				NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
				AgentsScaledDown:        checks.AgentsScaledDown,
				VmwareCredsDeleted:      checks.VMwareCredsDeleted,
				OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
				NoCustomResources:       checks.NoCustomResources,
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
			NoMigrationPlans:        checks.NoMigrationPlans,
			NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
			AgentsScaledDown:        checks.AgentsScaledDown,
			VmwareCredsDeleted:      checks.VMwareCredsDeleted,
			OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
			NoCustomResources:       checks.NoCustomResources,
			PassedAll:               checks.PassedAll,
		}
	}

	if checks.PassedAll {
		log.Printf("All checks passed. Starting upgrade to version: %s", in.TargetVersion)

		upgradeProgress.CurrentStep = "Updating Custom Resource Definitions"
		if err := upgrade.ApplyAllCRDs(ctx, kubeClient, in.TargetVersion); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("CRD update failed: %v", err)
			return nil, fmt.Errorf("CRD update failed: %w", err)
		}
		upgradeProgress.CompletedSteps++

		if err := upgrade.UpdateVersionConfigMapFromGitHub(ctx, kubeClient, in.TargetVersion); err != nil {
			log.Printf("Warning: Failed to update version-config ConfigMap from GitHub: %v", err)
		}

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
			
			if err := waitForDeploymentReady(ctx, kubeClient, depConfig); err != nil {
				log.Printf("Failed to wait for %s readiness after update: %v", depConfig.Name, err)
				upgradeProgress.Status = "failed"
				upgradeProgress.Error = fmt.Sprintf("Failed to wait for %s readiness after update: %v", depConfig.Name, err)
				return nil, fmt.Errorf("failed to wait for %s readiness after update: %w", depConfig.Name, err)
			}

			upgradeProgress.CompletedSteps++
		}

		upgradeProgress.CurrentStep = "Validating upgrade"
		if err := validateUpgrade(ctx, kubeClient, in.TargetVersion); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("Upgrade validation failed: %v", err)
			return nil, fmt.Errorf("upgrade validation failed: %w", err)
		}

		cmd := exec.CommandContext(ctx, "kubectl", "delete", "--all", "pods", "-n", "migration-system")
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: failed to delete all pods after upgrade: %v", err)
		}
		expectedPods := []string{"migration-controller-manager", "migration-vpwned-sdk", "vjailbreak-ui"}
		timeout := 5 * time.Minute
		interval := 5 * time.Second
		start := time.Now()
		for {
			allReady := true
			for _, name := range expectedPods {
				pods := &corev1.PodList{}
				if err := kubeClient.List(ctx, pods, client.InNamespace("migration-system"), client.MatchingLabels{"app": name}); err != nil {
					allReady = false
					break
				}
				foundReady := false
				for _, pod := range pods.Items {
					if pod.Status.Phase == corev1.PodRunning {
						for _, cond := range pod.Status.Conditions {
							if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
								foundReady = true
								break
							}
						}
					}
				}
				if !foundReady {
					allReady = false
					break
				}
			}
			if allReady {
				break
			}
			if time.Since(start) > timeout {
				return nil, fmt.Errorf("timeout waiting for all pods to be running and ready after upgrade")
			}
			time.Sleep(interval)
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
		success, msg = checkAndScaleDownAgent(ctx, kubeClient, config)
	case "vmware_creds_deleted":
		success, msg = checkAndDeleteSecret(ctx, kubeClient, config, "vmwarecreds")
	case "openstack_creds_deleted":
		success, msg = checkAndDeleteSecret(ctx, kubeClient, config, "openstackcreds")
	case "no_custom_resources":
		success, msg = checkAndDeleteAllCustomResources(ctx, kubeClient, config)
	default:
		return &api.CleanupStepResponse{Step: step, Success: false, Message: "Unknown step"}, nil
	}
	return &api.CleanupStepResponse{Step: step, Success: success, Message: msg}, nil
}

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
	time.Sleep(2 * time.Second)
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
	time.Sleep(2 * time.Second)
	list, err = dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to re-list RollingMigrationPlans"
	}
	if len(list.Items) == 0 {
		return true, "No RollingMigrationPlans remaining"
	}
	return false, "RollingMigrationPlans still exist"
}

func checkAndScaleDownAgent(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) (bool, string) {
    gvr := schema.GroupVersionResource{
        Group:    "vjailbreak.k8s.pf9.io",
        Version:  "v1alpha1",
        Resource: "vjailbreaknodes",
    }
    dynamicClient, err := dynamic.NewForConfig(restConfig)
    if err != nil {
        return false, "Failed to create dynamic client"
    }
    list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
    if err != nil {
        return false, "Failed to list VjailbreakNodes"
    }
    for _, item := range list.Items {
        err := dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
        if err != nil {
            return false, "Failed to delete VjailbreakNode: " + item.GetName()
        }
    }
    time.Sleep(2 * time.Second)
    list, err = dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
    if err != nil {
        return false, "Failed to re-list VjailbreakNodes"
    }
    if len(list.Items) == 0 {
        return true, "All agents scaled down"
    }
    return false, "Agents still exist"
}

func checkAndDeleteSecret(ctx context.Context, kubeClient client.Client, restConfig *rest.Config, credType string) (bool, string) {
	gvr := schema.GroupVersionResource{
		Group:    "vjailbreak.k8s.pf9.io",
		Version:  "v1alpha1",
		Resource: credType,
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return false, "Failed to create dynamic client"
	}
	list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to list " + credType + " CRs"
	}
	allDeleted := true
	var failed []string
	for _, item := range list.Items {
		spec, ok := item.UnstructuredContent()["spec"].(map[string]interface{})
		if !ok {
			failed = append(failed, item.GetName()+": no spec")
			allDeleted = false
			continue
		}
		secretRef, ok := spec["secretRef"].(map[string]interface{})
		if !ok {
			failed = append(failed, item.GetName()+": no secretRef")
			allDeleted = false
			continue
		}
		secretName, ok := secretRef["name"].(string)
		if !ok || secretName == "" {
			failed = append(failed, item.GetName()+": no secretRef.name")
			allDeleted = false
			continue
		}
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "migration-system"}}
		_ = kubeClient.Delete(ctx, secret)
		err := dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			failed = append(failed, item.GetName()+": CR delete failed")
			allDeleted = false
		}
	}
	if allDeleted {
		return true, "All " + credType + " and their secrets deleted"
	}
	return false, "Some " + credType + " or their secrets could not be deleted: " + strings.Join(failed, ", ")
}

func checkAndDeleteAllCustomResources(ctx context.Context, kubeClient client.Client, restConfig *rest.Config) (bool, string) {
	currentCRs, err := upgrade.DiscoverCurrentCRs(ctx, kubeClient)
	if err != nil {
		return false, "Failed to discover current CRs: " + err.Error()
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
		time.Sleep(2 * time.Second)
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

func getCurrentDeploymentImage(ctx context.Context, kubeClient client.Client, depConfig DeploymentConfig) (string, error) {
	dep := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); err != nil {
		log.Printf("Failed to get deployment %s in namespace %s: %v", depConfig.Name, depConfig.Namespace, err)
		return "", fmt.Errorf("failed to get deployment %s in namespace %s: %w", depConfig.Name, depConfig.Namespace, err)
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
    err := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep)
    if err != nil {
        return fmt.Errorf("failed to get deployment: %w", err)
    }

    found := false
    for i, c := range dep.Spec.Template.Spec.Containers {
        if c.Name == depConfig.ContainerName {
            dep.Spec.Template.Spec.Containers[i].Image = newImage
            dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("container %s not found in deployment %s", depConfig.ContainerName, depConfig.Name)
    }
    if err := kubeClient.Update(ctx, dep); err != nil {
        return fmt.Errorf("failed to update deployment image: %w", err)
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

