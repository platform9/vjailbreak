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
	"k8s.io/client-go/util/retry"

)
 
type DeploymentConfig struct {
	Namespace     string
	Name          string
	ContainerName string
	ImagePrefix   string
	ManualScale   bool
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
			ManualScale:   true,
		},
		{
			Namespace:     "migration-system", 
			Name:          "migration-vpwned-sdk",
			ContainerName: "vpwned",
			ImagePrefix:   "quay.io/platform9/vjailbreak-vpwned",
			ManualScale:   false,
		},
		{
			Namespace:     "migration-system",
			Name:          "vjailbreak-ui",
			ContainerName: "vjailbreak-ui-container", 
			ImagePrefix:   "quay.io/platform9/vjailbreak-ui",
			ManualScale:   false,
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
	tags, err := upgrade.GetAllTags(ctx)
	if err != nil {
		log.Printf("Error fetching tags: %v", err)
		return nil, err
	}
	
	log.Printf("Found %d available tags", len(tags))
	
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
		TotalSteps:     5,
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
			VmwareCredsDeleted:      checks.VMwareCredsDeleted,
			OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
			AgentsScaledDown:        checks.AgentsScaledDown,
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

		upgradeProgress.CurrentStep = "Updating version configuration"
		if err := upgrade.UpdateVersionConfigMapFromGitHub(ctx, kubeClient, in.TargetVersion); err != nil {
			log.Printf("Warning: Failed to update version-config ConfigMap from GitHub: %v", err)
		}
		upgradeProgress.CompletedSteps++
		
		go func() {
			ctx := context.Background()
			err := func() error {
				time.Sleep(2 * time.Second)
			
				upgradeProgress.Status = "deploying"
				upgradeProgress.CurrentStep = "Updating deployments"
				
				for _, depConfig := range deploymentConfigs {
					if !depConfig.ManualScale {
						upgradeProgress.CurrentStep = fmt.Sprintf("Updating %s", depConfig.Name)
						err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							dep := &appsv1.Deployment{}
							if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); getErr != nil { return getErr }
							found := false
							newImage := fmt.Sprintf("%s:%s", depConfig.ImagePrefix, in.TargetVersion)
							for i, c := range dep.Spec.Template.Spec.Containers {
								if c.Name == depConfig.ContainerName {
									dep.Spec.Template.Spec.Containers[i].Image = newImage
									dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways 
									found = true
									break
								}
							}
							if !found { return fmt.Errorf("container not found in %s", depConfig.Name) }
							return kubeClient.Update(ctx, dep)
						})
						if err != nil {
							return fmt.Errorf("failed to update %s: %w", depConfig.Name, err)
						}
					}
				}
			
				for _, depConfig := range deploymentConfigs {
					if depConfig.ManualScale {
						upgradeProgress.CurrentStep = fmt.Sprintf("Scaling down %s", depConfig.Name)
						if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							dep := &appsv1.Deployment{}; if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); getErr != nil { return getErr }; dep.Spec.Replicas = &[]int32{0}[0]; return kubeClient.Update(ctx, dep)
						}); err != nil {
							return fmt.Errorf("failed to scale down %s: %w", depConfig.Name, err)
						}

						if err := waitForDeploymentScaledDown(ctx, kubeClient, depConfig); err != nil {
							return err
						}

						upgradeProgress.CurrentStep = fmt.Sprintf("Updating image for %s", depConfig.Name)
						if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							dep := &appsv1.Deployment{}; if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); getErr != nil { return getErr }; found := false; newImage := fmt.Sprintf("%s:%s", depConfig.ImagePrefix, in.TargetVersion); for i, c := range dep.Spec.Template.Spec.Containers { if c.Name == depConfig.ContainerName { dep.Spec.Template.Spec.Containers[i].Image = newImage; dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways; found = true; break } }; if !found { return fmt.Errorf("container not found in %s", depConfig.Name) }; return kubeClient.Update(ctx, dep)
						}); err != nil {
							return fmt.Errorf("failed to patch image for %s: %w", depConfig.Name, err)
						}
						
						upgradeProgress.CurrentStep = fmt.Sprintf("Scaling up %s", depConfig.Name)
						if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							dep := &appsv1.Deployment{}; if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); getErr != nil { return getErr }; dep.Spec.Replicas = &[]int32{1}[0]; return kubeClient.Update(ctx, dep)
						}); err != nil {
							return fmt.Errorf("failed to scale up %s: %w", depConfig.Name, err)
						}
					}
				}
			
				upgradeProgress.CurrentStep = "Waiting for deployments to be ready"
				for _, depConfig := range deploymentConfigs {
					if err := waitForDeploymentReady(ctx, kubeClient, depConfig); err != nil {
						return fmt.Errorf("deployment %s failed readiness check: %w", depConfig.Name, err)
					}
				}
				return nil 
			}()

			if err != nil {
				log.Printf("Upgrade failed: %v", err)
				upgradeProgress.Status = "failed"
				upgradeProgress.Error = err.Error()
				return 
			}

			upgradeProgress.CompletedSteps++ 
			upgradeProgress.Status = "deployments_ready"
			upgradeProgress.CurrentStep = "Upgrade completed successfully"
			log.Printf("Upgrade Successful: All deployments updated and ready")
		}()

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
				NoMigrationPlans:        checks.NoMigrationPlans,
				NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
				VmwareCredsDeleted:      checks.VMwareCredsDeleted,
				OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
				AgentsScaledDown:        checks.AgentsScaledDown,
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
	
	var progress float32
	if upgradeProgress.TotalSteps > 0 {
		progress = float32(upgradeProgress.CompletedSteps) / float32(upgradeProgress.TotalSteps) * 100
	}
	
	if upgradeProgress.Status == "completed" {
		progress = 100
	} else if upgradeProgress.Status == "deployments_ready" {
		progress = 100
	}
	
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
    if len(list.Items) == 0 || (len(list.Items) == 1 && list.Items[0].GetName() == "vjailbreak-master") {
        return true, "Agents scaled down"
    }
    
    return false, "Non-master agents still exist"
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

func waitForDeploymentScaledDown(ctx context.Context, kubeClient client.Client, depConfig DeploymentConfig) error {
	timeout := 5 * time.Minute
	interval := 10 * time.Second
	
	for start := time.Now(); time.Since(start) < timeout; {
		dep := &appsv1.Deployment{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: depConfig.Name, Namespace: depConfig.Namespace}, dep); err != nil {
			if kerrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		
		if dep.Status.Replicas == 0 {
			log.Printf("Deployment %s successfully scaled down.", depConfig.Name)
			return nil
		}
		
		time.Sleep(interval)
	}
	
	return fmt.Errorf("deployment %s not scaled down within timeout", depConfig.Name)
}