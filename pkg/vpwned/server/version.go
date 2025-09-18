package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/juju/errors"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/platform9/vjailbreak/pkg/vpwned/upgrade"
	version "github.com/platform9/vjailbreak/pkg/vpwned/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

const progressConfigMapName = "vjailbreak-upgrade-progress"

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
	tags, err := upgrade.GetAllTags(ctx)
	if err != nil {
		log.Printf("Error fetching tags: %v", err)
		return nil, err
	}

	log.Printf("Found %d available tags", len(tags))

	var protoUpdates []*api.ReleaseInfo
	for _, tag := range tags {
		protoUpdates = append(protoUpdates, &api.ReleaseInfo{
			Version:      tag,
			ReleaseNotes: "",
		})
	}
	return &api.AvailableUpdatesResponse{Updates: protoUpdates}, nil
}

func (s *VpwnedVersion) InitiateUpgrade(ctx context.Context, in *api.UpgradeRequest) (*api.UpgradeResponse, error) {
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

	upgradeProgress = &UpgradeProgress{
		CurrentStep:    "Starting upgrade",
		TotalSteps:     5,
		CompletedSteps: 0,
		Status:         "in_progress",
		StartTime:      time.Now(),
	}

	saveProgress(ctx, kubeClient)

	err = func() error {

		upgradeProgress.CurrentStep = "Running pre-upgrade checks"
		saveProgress(ctx, kubeClient)
		checks, err := upgrade.RunPreUpgradeChecks(ctx, kubeClient, config, in.TargetVersion)
		if err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("Pre-upgrade checks failed: %v", err)
			return fmt.Errorf("pre-upgrade checks failed: %w", err)
		}
		upgradeProgress.CompletedSteps++

		if !checks.PassedAll && in.AutoCleanup {
			upgradeProgress.CurrentStep = "Performing automatic cleanup"
			saveProgress(ctx, kubeClient)
			if err := upgrade.CleanupResources(ctx, kubeClient, config); err != nil {
				upgradeProgress.Status = "failed"
				upgradeProgress.Error = fmt.Sprintf("Automatic cleanup failed: %v", err)
				return fmt.Errorf("automatic cleanup failed: %w", err)
			}
			checks, err = upgrade.RunPreUpgradeChecks(ctx, kubeClient, config, in.TargetVersion)
			if err != nil || !checks.PassedAll {
				upgradeProgress.Status = "failed"
				upgradeProgress.Error = "Checks failed even after cleanup"
				return fmt.Errorf("checks failed even after cleanup")
			}
		}

		if !checks.PassedAll {
			return errors.New("pre-upgrade checks did not pass, halting upgrade")
		}

		upgradeProgress.CurrentStep = "Verifying release images"
		saveProgress(ctx, kubeClient)
		if ok, err := upgrade.CheckImagesExist(ctx, in.TargetVersion); !ok {
			return fmt.Errorf("image validation failed: %w", err)
		}

		backupID := time.Now().UTC().Format("20060102T150405Z")
		upgradeProgress.CurrentStep = "Backing up resources"
		saveProgress(ctx, kubeClient)
		if err := upgrade.BackupResourcesWithID(ctx, kubeClient, config, backupID); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("Backup failed: %v", err)
			return fmt.Errorf("backup failed: %w", err)
		}
		upgradeProgress.CompletedSteps++

		upgradeProgress.CurrentStep = "Updating Custom Resource Definitions"
		saveProgress(ctx, kubeClient)
		if err := upgrade.ApplyAllCRDs(ctx, kubeClient, in.TargetVersion); err != nil {
			upgradeProgress.Status = "failed"
			upgradeProgress.Error = fmt.Sprintf("CRD update failed: %v", err)
			return fmt.Errorf("CRD update failed: %w", err)
		}
		upgradeProgress.CompletedSteps++

		upgradeProgress.CurrentStep = "Updating version configuration"
		saveProgress(ctx, kubeClient)
		if err := upgrade.UpdateVersionConfigMapFromGitHub(ctx, kubeClient, in.TargetVersion); err != nil {
			log.Printf("Warning: Failed to update version-config ConfigMap from GitHub: %v", err)
		}
		upgradeProgress.CompletedSteps++

		go func() {
			ctx := context.Background()
			err := func() error {
				time.Sleep(2 * time.Second)

				upgradeProgress.Status = "deploying"
				upgradeProgress.CurrentStep = "Upgrading"
				saveProgress(ctx, kubeClient)

				var controllerConfig DeploymentConfig
				for _, cfg := range deploymentConfigs {
					if cfg.Name == "migration-controller-manager" {
						controllerConfig = cfg
						break
					}
				}
				// Scale down controller
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					dep.Spec.Replicas = &[]int32{0}[0]
					return kubeClient.Update(ctx, dep)
				}); err != nil {
					return fmt.Errorf("failed to scale down controller: %w", err)
				}
				if err := waitForDeploymentScaledDown(ctx, kubeClient, controllerConfig); err != nil {
					return err
				}

				// Patch Controller Image
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					found := false
					newImage := fmt.Sprintf("%s:%s", controllerConfig.ImagePrefix, in.TargetVersion)
					for i, c := range dep.Spec.Template.Spec.Containers {
						if c.Name == controllerConfig.ContainerName {
							dep.Spec.Template.Spec.Containers[i].Image = newImage
							dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("container not found in %s", controllerConfig.Name)
					}
					return kubeClient.Update(ctx, dep)
				}); err != nil {
					return fmt.Errorf("failed to patch controller image: %w", err)
				}

				// Scale Up Controller
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					dep.Spec.Replicas = &[]int32{1}[0]
					return kubeClient.Update(ctx, dep)
				}); err != nil {
					return fmt.Errorf("failed to scale up controller: %w", err)
				}

				upgradeProgress.CurrentStep = "Waiting for deployments to be ready"
				saveProgress(ctx, kubeClient)
				if err := waitForDeploymentReady(ctx, kubeClient, controllerConfig); err != nil {
					return fmt.Errorf("controller deployment failed readiness check: %w", err)
				}

				var uiConfig DeploymentConfig
				for _, cfg := range deploymentConfigs {
					if cfg.Name == "vjailbreak-ui" {
						uiConfig = cfg
						break
					}
				}
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: uiConfig.Name, Namespace: uiConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					found := false
					newImage := fmt.Sprintf("%s:%s", uiConfig.ImagePrefix, in.TargetVersion)
					for i, c := range dep.Spec.Template.Spec.Containers {
						if c.Name == uiConfig.ContainerName {
							dep.Spec.Template.Spec.Containers[i].Image = newImage
							dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("container not found in %s", uiConfig.Name)
					}
					return kubeClient.Update(ctx, dep)
				}); err != nil {
					return fmt.Errorf("failed to update UI deployment: %w", err)
				}

				if err := waitForDeploymentReady(ctx, kubeClient, controllerConfig); err != nil {
					return err
				}
				if err := waitForDeploymentReady(ctx, kubeClient, uiConfig); err != nil {
					return err
				}

				upgradeProgress.Status = "server_restarting"
				upgradeProgress.CurrentStep = "Server restarting"
				saveProgress(ctx, kubeClient)
				log.Printf("UI and Controller are ready. Signaling server restart to UI.")

				var sdkConfig DeploymentConfig
				for _, cfg := range deploymentConfigs {
					if cfg.Name == "migration-vpwned-sdk" {
						sdkConfig = cfg
						break
					}
				}
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(ctx, client.ObjectKey{Name: sdkConfig.Name, Namespace: sdkConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					found := false
					newImage := fmt.Sprintf("%s:%s", sdkConfig.ImagePrefix, in.TargetVersion)
					for i, c := range dep.Spec.Template.Spec.Containers {
						if c.Name == sdkConfig.ContainerName {
							dep.Spec.Template.Spec.Containers[i].Image = newImage
							dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("container not found in %s", sdkConfig.Name)
					}
					return kubeClient.Update(ctx, dep)
				}); err != nil {
					log.Printf("Error updating SDK in the background: %v", err)
				}

				go func(localBackupID string) {
					time.Sleep(30 * time.Second)

					ok := true
					for _, cfg := range deploymentConfigs {
						dep := &appsv1.Deployment{}
						if err := kubeClient.Get(context.Background(),
							client.ObjectKey{Name: cfg.Name, Namespace: cfg.Namespace}, dep); err != nil {
							ok = false
							break
						}
						if dep.Status.ReadyReplicas != *dep.Spec.Replicas {
							ok = false
							break
						}
					}

					if ok {
						log.Println("Upgrade looks stable. Cleaning up backup ConfigMaps...")
						if err := upgrade.CleanupBackupConfigMaps(context.Background(), kubeClient, localBackupID); err != nil {
							log.Printf("Warning: Failed to cleanup backup ConfigMaps: %v", err)
						} else {
							log.Println("Backup ConfigMaps cleaned up.")
						}
						upgradeProgress.Status = "completed"
						upgradeProgress.CurrentStep = "Upgrade completed and backups cleaned"
						saveProgress(context.Background(), kubeClient)
					} else {
						log.Println("Post-upgrade stability checks failed; keeping backups for investigation.")
						upgradeProgress.Status = "deployments_ready_but_unstable"
						upgradeProgress.CurrentStep = "Deployments reported not stable; backups retained"
						saveProgress(context.Background(), kubeClient)
					}
				}(backupID)

				return nil
			}()

			if err != nil {
				log.Printf("Upgrade failed during deployment phase: %v. Rolling back.", err)
				upgradeProgress.Status = "failed"
				upgradeProgress.Error = err.Error()
				upgradeProgress.CurrentStep = "Deployment failed, rolling back..."
				saveProgress(ctx, kubeClient)
				if err := upgrade.RestoreResources(ctx, kubeClient); err != nil {
					log.Printf("CRITICAL: Rollback failed: %v", err)
					upgradeProgress.Status = "rollback_failed"
					upgradeProgress.Error = "Deployment failed and rollback also failed."
				} else {
					log.Printf("Rollback completed successfully.")
					upgradeProgress.Status = "rolled_back"
				}
				saveProgress(ctx, kubeClient)
				return
			}
			log.Printf("Upgrade process handed off to UI for finalization.")
		}()
		return nil

	}()

	if err != nil {
		log.Printf("Upgrade failed before deployment phase: %v. Rolling back.", err)
		upgradeProgress.Status = "failed"
		upgradeProgress.Error = err.Error()
		upgradeProgress.CurrentStep = "Upgrade failed, rolling back..."
		saveProgress(ctx, kubeClient)
		if err := upgrade.RestoreResources(ctx, kubeClient); err != nil {
			log.Printf("CRITICAL: Rollback failed: %v", err)
		} else {
			log.Printf("Rollback completed successfully.")
		}
		return nil, err
	}

	checks, _ := upgrade.RunPreUpgradeChecks(ctx, kubeClient, config, in.TargetVersion)
	protoChecks := &api.ValidationResult{
		NoMigrationPlans:        checks.NoMigrationPlans,
		NoRollingMigrationPlans: checks.NoRollingMigrationPlans,
		VmwareCredsDeleted:      checks.VMwareCredsDeleted,
		OpenstackCredsDeleted:   checks.OpenStackCredsDeleted,
		AgentsScaledDown:        checks.AgentsScaledDown,
		NoCustomResources:       checks.NoCustomResources,
		PassedAll:               checks.PassedAll,
	}

	return &api.UpgradeResponse{
		Checks:         protoChecks,
		UpgradeStarted: true,
	}, nil
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
	saveProgress(ctx, kubeClient)
	err = upgrade.RestoreResources(ctx, kubeClient)
	if err != nil {
		upgradeProgress.Status = "rollback_failed"
		upgradeProgress.Error = fmt.Sprintf("Rollback failed: %v", err)
		return &api.UpgradeProgressResponse{
			Status:      "rollback_failed",
			Error:       upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}
	upgradeProgress.Status = "rolled_back"
	upgradeProgress.Error = "Rollback completed successfully"
	saveProgress(ctx, kubeClient)
	return &api.UpgradeProgressResponse{
		Status:      "rolled_back",
		Error:       "",
		CurrentStep: upgradeProgress.CurrentStep,
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

	if err := upgrade.CleanupAllOldBackups(ctx, kubeClient); err != nil {
		log.Printf("Warning: failed to cleanup old backups: %v", err)
	}

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
			UpgradeStarted:  false,
			CleanupRequired: false,
		}, fmt.Errorf("checks failed after cleanup")
	}

	return s.InitiateUpgrade(ctx, in)
}

func (s *VpwnedVersion) GetUpgradeProgress(ctx context.Context, in *api.VersionRequest) (*api.UpgradeProgressResponse, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Error getting in-cluster config: %v", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Printf("Error creating kubeClient: %v", err)
	}

	if kubeClient != nil {
		loadProgress(ctx, kubeClient)
	}

	if upgradeProgress == nil {
		return &api.UpgradeProgressResponse{
			Status: "no_upgrade_in_progress",
		}, nil
	}

	var progress float32
	if upgradeProgress.TotalSteps > 0 {
		progress = float32(upgradeProgress.CompletedSteps) / float32(upgradeProgress.TotalSteps) * 100
	}

	if upgradeProgress.Status == "completed" || upgradeProgress.Status == "deployments_ready" {
		progress = 100
	}

	return &api.UpgradeProgressResponse{
		CurrentStep: upgradeProgress.CurrentStep,
		Progress:    progress,
		Status:      upgradeProgress.Status,
		Error:       upgradeProgress.Error,
		StartTime:   upgradeProgress.StartTime.Format(time.RFC3339),
		EndTime: func() string {
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
		success, msg = checkAndDeleteMigrationPlans(ctx, config)
	case "no_rollingmigrationplans":
		success, msg = checkAndDeleteRollingMigrationPlans(ctx, config)
	case "agent_scaled_down":
		success, msg = checkAndScaleDownAgent(ctx, config)
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

func checkAndDeleteMigrationPlans(ctx context.Context, restConfig *rest.Config) (bool, string) {
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
		_ = dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
	}

	timeout := 5 * time.Minute
	interval := 5 * time.Second
	for start := time.Now(); time.Since(start) < timeout; {
		list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "Failed to re-list MigrationPlans during polling"
		}
		if len(list.Items) == 0 {
			return true, "No MigrationPlans remaining"
		}
		time.Sleep(interval)
	}

	return false, "MigrationPlans still exist after 5 minute timeout"
}

func checkAndDeleteRollingMigrationPlans(ctx context.Context, restConfig *rest.Config) (bool, string) {
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
		_ = dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
	}

	timeout := 5 * time.Minute
	interval := 5 * time.Second
	for start := time.Now(); time.Since(start) < timeout; {
		list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "Failed to re-list RollingMigrationPlans during polling"
		}
		if len(list.Items) == 0 {
			return true, "No RollingMigrationPlans remaining"
		}
		time.Sleep(interval)
	}

	return false, "RollingMigrationPlans still exist after 5 minute timeout"
}

func checkAndScaleDownAgent(ctx context.Context, restConfig *rest.Config) (bool, string) {
	gvr := schema.GroupVersionResource{Group: "vjailbreak.k8s.pf9.io", Version: "v1alpha1", Resource: "vjailbreaknodes"}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return false, "Failed to create dynamic client"
	}

	list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "Failed to list VjailbreakNodes"
	}

	for _, item := range list.Items {
		if item.GetName() == "vjailbreak-master" {
			continue
		}
		_ = dynamicClient.Resource(gvr).Namespace("migration-system").Delete(ctx, item.GetName(), metav1.DeleteOptions{})
	}

	timeout := 5 * time.Minute
	interval := 5 * time.Second
	for start := time.Now(); time.Since(start) < timeout; {
		list, err := dynamicClient.Resource(gvr).Namespace("migration-system").List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "Failed to re-list VjailbreakNodes"
		}

		if len(list.Items) == 0 || (len(list.Items) == 1 && list.Items[0].GetName() == "vjailbreak-master") {
			return true, "Agents scaled down"
		}
		time.Sleep(interval)
	}

	return false, "Non-master agents still exist after 5 minute timeout"
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

func saveProgress(ctx context.Context, kubeClient client.Client) {
	if upgradeProgress == nil {
		return
	}
	progressJSON, err := json.Marshal(upgradeProgress)
	if err != nil {
		log.Printf("Error marshaling progress: %v", err)
		return
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      progressConfigMapName,
			Namespace: "migration-system",
		},
		Data: map[string]string{"progress": string(progressJSON)},
	}
	err = kubeClient.Update(ctx, cm)
	if err != nil && kerrors.IsNotFound(err) {
		err = kubeClient.Create(ctx, cm)
	}
	if err != nil {
		log.Printf("Error saving upgrade progress to ConfigMap: %v", err)
	}
}

func loadProgress(ctx context.Context, kubeClient client.Client) {
	cm := &corev1.ConfigMap{}
	err := kubeClient.Get(ctx, client.ObjectKey{Name: progressConfigMapName, Namespace: "migration-system"}, cm)
	if err != nil {
		return
	}
	if progressJSON, ok := cm.Data["progress"]; ok {
		var loadedProgress UpgradeProgress
		if err := json.Unmarshal([]byte(progressJSON), &loadedProgress); err == nil {
			upgradeProgress = &loadedProgress
		}
	}
}
