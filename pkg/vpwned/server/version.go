package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
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
	"k8s.io/client-go/kubernetes"
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
	CurrentStep     string
	TotalSteps      int
	CompletedSteps  int
	Status          string
	Error           string
	StartTime       time.Time
	EndTime         *time.Time
	PreviousVersion string
	TargetVersion   string
}

const progressConfigMapName = "vjailbreak-upgrade-progress"

var (
	progressMu      sync.RWMutex
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

// updateProgress safely updates upgradeProgress with proper locking
func updateProgress(fn func(p *UpgradeProgress)) {
	progressMu.Lock()
	defer progressMu.Unlock()
	if upgradeProgress == nil {
		return
	}
	fn(upgradeProgress)
}

// readProgress safely reads from upgradeProgress with proper locking
func readProgress(fn func(p *UpgradeProgress)) {
	progressMu.RLock()
	defer progressMu.RUnlock()
	if upgradeProgress == nil {
		return
	}
	fn(upgradeProgress)
}

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
	upgradeCtx := context.WithoutCancel(ctx)
	upgradeCtx, cancel := context.WithTimeout(upgradeCtx, 30*time.Minute)
	defer cancel()

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	config.QPS = 100
	config.Burst = 200

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	currentVersion, err := upgrade.GetCurrentVersion(upgradeCtx, clientset)
	if err != nil {
		log.Printf("Warning: Could not get current version: %v", err)
		currentVersion = "unknown"
	}

	progressMu.Lock()
	upgradeProgress = &UpgradeProgress{
		CurrentStep:     "Starting upgrade",
		TotalSteps:      5,
		CompletedSteps:  0,
		Status:          "in_progress",
		StartTime:       time.Now(),
		PreviousVersion: currentVersion,
		TargetVersion:   in.TargetVersion,
	}
	progressMu.Unlock()

	log.Printf("Starting upgrade from %s to %s", currentVersion, in.TargetVersion)
	saveProgress(upgradeCtx, kubeClient)

	err = func() error {

		updateProgress(func(p *UpgradeProgress) {
			p.CurrentStep = "Running pre-upgrade checks"
		})
		saveProgress(upgradeCtx, kubeClient)
		checks, err := upgrade.RunPreUpgradeChecks(upgradeCtx, kubeClient, config, in.TargetVersion)
		if err != nil {
			updateProgress(func(p *UpgradeProgress) {
				p.Status = "failed"
				p.Error = fmt.Sprintf("Pre-upgrade checks failed: %v", err)
			})
			return fmt.Errorf("pre-upgrade checks failed: %w", err)
		}
		updateProgress(func(p *UpgradeProgress) {
			p.CompletedSteps++
		})

		if !checks.PassedAll && in.AutoCleanup {
			updateProgress(func(p *UpgradeProgress) {
				p.CurrentStep = "Performing automatic cleanup"
			})
			saveProgress(upgradeCtx, kubeClient)
			if err := upgrade.CleanupResources(upgradeCtx, kubeClient, config); err != nil {
				updateProgress(func(p *UpgradeProgress) {
					p.Status = "failed"
					p.Error = fmt.Sprintf("Automatic cleanup failed: %v", err)
				})
				return fmt.Errorf("automatic cleanup failed: %w", err)
			}
			checks, err = upgrade.RunPreUpgradeChecks(upgradeCtx, kubeClient, config, in.TargetVersion)
			if err != nil || !checks.PassedAll {
				updateProgress(func(p *UpgradeProgress) {
					p.Status = "failed"
					p.Error = "Checks failed even after cleanup"
				})
				return fmt.Errorf("checks failed even after cleanup")
			}
		}

		if !checks.PassedAll {
			return errors.New("pre-upgrade checks did not pass, halting upgrade")
		}

		updateProgress(func(p *UpgradeProgress) {
			p.CurrentStep = "Verifying release images"
		})
		saveProgress(upgradeCtx, kubeClient)
		if ok, err := upgrade.CheckImagesExist(upgradeCtx, in.TargetVersion); !ok {
			return fmt.Errorf("image validation failed: %w", err)
		}

		backupID := time.Now().UTC().Format("20060102T150405Z")
		updateProgress(func(p *UpgradeProgress) {
			p.CurrentStep = "Backing up resources"
		})
		saveProgress(upgradeCtx, kubeClient)
		if err := upgrade.BackupResourcesWithID(upgradeCtx, kubeClient, config, backupID); err != nil {
			updateProgress(func(p *UpgradeProgress) {
				p.Status = "failed"
				p.Error = fmt.Sprintf("Backup failed: %v", err)
			})
			return fmt.Errorf("backup failed: %w", err)
		}
		updateProgress(func(p *UpgradeProgress) {
			p.CompletedSteps++
			p.CurrentStep = "Updating Custom Resource Definitions"
		})
		saveProgress(upgradeCtx, kubeClient)
		if err := upgrade.ApplyAllCRDs(upgradeCtx, kubeClient, in.TargetVersion); err != nil {
			updateProgress(func(p *UpgradeProgress) {
				p.Status = "failed"
				p.Error = fmt.Sprintf("CRD update failed: %v", err)
			})
			return fmt.Errorf("CRD update failed: %w", err)
		}
		updateProgress(func(p *UpgradeProgress) {
			p.CompletedSteps++
		})
		saveProgress(upgradeCtx, kubeClient)

		updateProgress(func(p *UpgradeProgress) {
			p.CurrentStep = "Updating version configuration"
		})
		saveProgress(upgradeCtx, kubeClient)
		if err := upgrade.UpdateVersionConfigMapFromGitHub(upgradeCtx, kubeClient, in.TargetVersion); err != nil {
			log.Printf("Warning: Failed to update version-config ConfigMap from GitHub: %v", err)
		}

		updateProgress(func(p *UpgradeProgress) {
			p.CurrentStep = "Updating vjailbreak settings"
		})
		saveProgress(upgradeCtx, kubeClient)
		if err := upgrade.UpdateVjailbreakSettingsFromGitHub(upgradeCtx, kubeClient, in.TargetVersion); err != nil {
			log.Printf("Warning: Failed to update vjailbreak-settings ConfigMap from GitHub: %v", err)
		}

		updateProgress(func(p *UpgradeProgress) {
			p.CompletedSteps++
		})

		go func() {
			// Use upgradeCtx directly (already detached from client cancellation and has bounded timeout)
			err := func() error {
				time.Sleep(2 * time.Second)

				updateProgress(func(p *UpgradeProgress) {
					p.Status = "deploying"
					p.CurrentStep = "Upgrading"
				})
				saveProgress(upgradeCtx, kubeClient)

				var controllerConfig DeploymentConfig
				for _, cfg := range deploymentConfigs {
					if cfg.Name == "migration-controller-manager" {
						controllerConfig = cfg
						break
					}
				}
				if controllerConfig.Name == "" {
					return fmt.Errorf("controller deployment config not found")
				}

				var originalReplicas int32

				// Scale down controller
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Scaling down controller"
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(upgradeCtx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					if dep.Spec.Replicas != nil {
						originalReplicas = *dep.Spec.Replicas
					} else {
						originalReplicas = 1
					}
					dep.Spec.Replicas = &[]int32{0}[0]
					return kubeClient.Update(upgradeCtx, dep)
				}); err != nil {
					return fmt.Errorf("failed to scale down controller: %w", err)
				}
				if err := waitForDeploymentScaledDown(upgradeCtx, kubeClient, controllerConfig); err != nil {
					return err
				}

				// Apply controller deployment
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Applying controller deployment from GitHub"
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := upgrade.ApplyManifestFromGitHub(upgradeCtx, kubeClient, in.TargetVersion, "deploy/05controller-deployment.yaml"); err != nil {
					return fmt.Errorf("failed to apply controller deployment from GitHub: %w", err)
				}

				// Scale up controller
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Scaling up controller"
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					dep := &appsv1.Deployment{}
					if getErr := kubeClient.Get(upgradeCtx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
						return getErr
					}
					dep.Spec.Replicas = &originalReplicas
					return kubeClient.Update(upgradeCtx, dep)
				}); err != nil {
					return fmt.Errorf("failed to scale up controller: %w", err)
				}

				// Wait for controller ready
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Waiting for controller to be ready"
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := waitForDeploymentReady(upgradeCtx, kubeClient, controllerConfig); err != nil {
					return fmt.Errorf("controller deployment failed readiness check: %w", err)
				}

				// Apply vpwned deployment
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Applying vpwned deployment from GitHub"
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := upgrade.ApplyManifestFromGitHub(upgradeCtx, kubeClient, in.TargetVersion, "deploy/06vpwned-deployment.yaml"); err != nil {
					return fmt.Errorf("failed to apply vpwned deployment: %w", err)
				}

				// Apply UI deployment
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Applying UI deployment from GitHub"
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := upgrade.ApplyManifestFromGitHub(upgradeCtx, kubeClient, in.TargetVersion, "deploy/07ui-deployment.yaml"); err != nil {
					return fmt.Errorf("failed to apply UI deployment: %w", err)
				}

				// Wait for all deployments ready
				updateProgress(func(p *UpgradeProgress) {
					p.CurrentStep = "Waiting for all deployments to be ready"
				})
				saveProgress(upgradeCtx, kubeClient)
				for _, cfg := range []DeploymentConfig{
					controllerConfig,
					{Name: "migration-vpwned-sdk", Namespace: "migration-system"},
					{Name: "vjailbreak-ui", Namespace: "migration-system"},
				} {
					if err := waitForDeploymentReady(upgradeCtx, kubeClient, cfg); err != nil {
						return fmt.Errorf("deployment %s not ready: %w", cfg.Name, err)
					}
				}

				updateProgress(func(p *UpgradeProgress) {
					p.Status = "server_restarting"
					p.CurrentStep = "Server restarting"
				})
				saveProgress(upgradeCtx, kubeClient)
				log.Printf("All deployments are ready. Signaling server restart to UI.")

				go func(localBackupID string) {
					time.Sleep(30 * time.Second)

					ok := true
					for _, cfg := range deploymentConfigs {
						dep := &appsv1.Deployment{}
						if err := kubeClient.Get(upgradeCtx,
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
						if err := upgrade.CleanupBackupConfigMaps(upgradeCtx, kubeClient, localBackupID); err != nil {
							log.Printf("Warning: Failed to cleanup backup ConfigMaps: %v", err)
						} else {
							log.Println("Backup ConfigMaps cleaned up.")
						}
						updateProgress(func(p *UpgradeProgress) {
							p.CompletedSteps++
							p.Status = "completed"
							p.CurrentStep = "Upgrade completed and backups cleaned"
							now := time.Now()
							p.EndTime = &now
						})
						saveProgress(upgradeCtx, kubeClient)
					} else {
						log.Println("Post-upgrade stability checks failed; keeping backups for investigation.")
						updateProgress(func(p *UpgradeProgress) {
							p.Status = "deployments_ready_but_unstable"
							p.CurrentStep = "Deployments reported not stable; backups retained"
						})
						saveProgress(upgradeCtx, kubeClient)
					}
				}(backupID)

				return nil
			}()

			if err != nil {
				log.Printf("Upgrade failed during deployment phase: %v. Rolling back.", err)
				updateProgress(func(p *UpgradeProgress) {
					p.Status = "failed"
					p.Error = err.Error()
					p.CurrentStep = "Deployment failed, rolling back..."
				})
				saveProgress(upgradeCtx, kubeClient)
				if err := upgrade.RestoreResources(upgradeCtx, kubeClient); err != nil {
					log.Printf("CRITICAL: Rollback failed: %v", err)
					updateProgress(func(p *UpgradeProgress) {
						p.Status = "rollback_failed"
						p.Error = "Deployment failed and rollback also failed."
						now := time.Now()
						p.EndTime = &now
					})
				} else {
					log.Printf("Rollback completed successfully.")
					updateProgress(func(p *UpgradeProgress) {
						p.Status = "rolled_back"
						now := time.Now()
						p.EndTime = &now
					})
				}
				saveProgress(upgradeCtx, kubeClient)
				return
			}
			log.Printf("Upgrade process handed off to UI for finalization.")
		}()
		return nil

	}()

	if err != nil {
		log.Printf("Upgrade failed before deployment phase: %v. Rolling back.", err)
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "failed"
			p.Error = err.Error()
			p.CurrentStep = "Upgrade failed, rolling back..."
			now := time.Now()
			p.EndTime = &now
		})
		saveProgress(upgradeCtx, kubeClient)
		if err := upgrade.RestoreResources(upgradeCtx, kubeClient); err != nil {
			log.Printf("CRITICAL: Rollback failed: %v", err)
		} else {
			log.Printf("Rollback completed successfully.")
		}
		return nil, err
	}

	checks, _ := upgrade.RunPreUpgradeChecks(upgradeCtx, kubeClient, config, in.TargetVersion)
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

	rollbackCtx := context.WithoutCancel(ctx)
	rollbackCtx, cancel := context.WithTimeout(rollbackCtx, 20*time.Minute)
	defer cancel()

	var previousVersion, targetVersion string
	readProgress(func(p *UpgradeProgress) {
		previousVersion = p.PreviousVersion
		targetVersion = p.TargetVersion
	})

	if previousVersion == "" || previousVersion == "unknown" {
		log.Printf("WARNING: Previous version unknown, falling back to snapshot-based rollback")
		updateProgress(func(p *UpgradeProgress) {
			p.CurrentStep = "Restoring resources from backup (fallback)"
		})
		saveProgress(rollbackCtx, kubeClient)
		err = upgrade.RestoreResources(rollbackCtx, kubeClient)
		if err != nil {
			updateProgress(func(p *UpgradeProgress) {
				p.Status = "rollback_failed"
				p.Error = fmt.Sprintf("Rollback failed: %v", err)
				now := time.Now()
				p.EndTime = &now
			})
			return &api.UpgradeProgressResponse{
				Status:      "rollback_failed",
				Error:       upgradeProgress.Error,
				CurrentStep: upgradeProgress.CurrentStep,
			}, err
		}
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "rolled_back"
			p.Error = "Rollback completed successfully (snapshot-based)"
			p.CurrentStep = "Rollback completed successfully"
			now := time.Now()
			p.EndTime = &now
		})
		saveProgress(rollbackCtx, kubeClient)
		return &api.UpgradeProgressResponse{
			Status:      "rolled_back",
			Error:       "",
			CurrentStep: upgradeProgress.CurrentStep,
		}, nil
	}

	log.Printf("Starting manifest-driven rollback from %s to %s", targetVersion, previousVersion)
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Rolling back using GitHub manifests"
	})
	saveProgress(rollbackCtx, kubeClient)

	// Scale down controller
	var controllerConfig DeploymentConfig
	for _, cfg := range deploymentConfigs {
		if cfg.Name == "migration-controller-manager" {
			controllerConfig = cfg
			break
		}
	}
	if controllerConfig.Name == "" {
		return nil, fmt.Errorf("controller deployment config not found")
	}

	var originalReplicas int32
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Scaling down controller for rollback"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		dep := &appsv1.Deployment{}
		if getErr := kubeClient.Get(rollbackCtx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
			return getErr
		}
		if dep.Spec.Replicas != nil {
			originalReplicas = *dep.Spec.Replicas
		} else {
			originalReplicas = 1
		}
		dep.Spec.Replicas = &[]int32{0}[0]
		return kubeClient.Update(rollbackCtx, dep)
	}); err != nil {
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "rollback_failed"
			p.Error = fmt.Sprintf("Failed to scale down controller: %v", err)
			now := time.Now()
			p.EndTime = &now
		})
		return &api.UpgradeProgressResponse{
			Status:      "rollback_failed",
			Error:       upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}
	if err := waitForDeploymentScaledDown(rollbackCtx, kubeClient, controllerConfig); err != nil {
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "rollback_failed"
			p.Error = fmt.Sprintf("Failed waiting for controller scale down: %v", err)
			now := time.Now()
			p.EndTime = &now
		})
		return &api.UpgradeProgressResponse{
			Status:      "rollback_failed",
			Error:       upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}

	// Apply CRDs from previous version
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Restoring CRDs from previous version"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := upgrade.ApplyAllCRDs(rollbackCtx, kubeClient, previousVersion); err != nil {
		log.Printf("Warning: Failed to restore CRDs: %v", err)
	}

	// Apply controller deployment from previous version
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Restoring controller deployment from GitHub"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := upgrade.ApplyManifestFromGitHub(rollbackCtx, kubeClient, previousVersion, "deploy/05controller-deployment.yaml"); err != nil {
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "rollback_failed"
			p.Error = fmt.Sprintf("Failed to restore controller: %v", err)
			now := time.Now()
			p.EndTime = &now
		})
		return &api.UpgradeProgressResponse{
			Status:      "rollback_failed",
			Error:       upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}

	// Scale up controller
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Scaling up controller"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		dep := &appsv1.Deployment{}
		if getErr := kubeClient.Get(rollbackCtx, client.ObjectKey{Name: controllerConfig.Name, Namespace: controllerConfig.Namespace}, dep); getErr != nil {
			return getErr
		}
		dep.Spec.Replicas = &originalReplicas
		return kubeClient.Update(rollbackCtx, dep)
	}); err != nil {
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "rollback_failed"
			p.Error = fmt.Sprintf("Failed to scale up controller: %v", err)
			now := time.Now()
			p.EndTime = &now
		})
		return &api.UpgradeProgressResponse{
			Status:      "rollback_failed",
			Error:       upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}

	// Wait for controller ready
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Waiting for controller to be ready"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := waitForDeploymentReady(rollbackCtx, kubeClient, controllerConfig); err != nil {
		updateProgress(func(p *UpgradeProgress) {
			p.Status = "rollback_failed"
			p.Error = fmt.Sprintf("Controller not ready: %v", err)
			now := time.Now()
			p.EndTime = &now
		})
		return &api.UpgradeProgressResponse{
			Status:      "rollback_failed",
			Error:       upgradeProgress.Error,
			CurrentStep: upgradeProgress.CurrentStep,
		}, err
	}

	// Apply vpwned deployment
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Restoring vpwned deployment from GitHub"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := upgrade.ApplyManifestFromGitHub(rollbackCtx, kubeClient, previousVersion, "deploy/06vpwned-deployment.yaml"); err != nil {
		log.Printf("Warning: Failed to restore vpwned deployment: %v", err)
	}

	// Apply UI deployment
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Restoring UI deployment from GitHub"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := upgrade.ApplyManifestFromGitHub(rollbackCtx, kubeClient, previousVersion, "deploy/07ui-deployment.yaml"); err != nil {
		log.Printf("Warning: Failed to restore UI deployment: %v", err)
	}

	// Wait for all deployments ready
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Waiting for all deployments to be ready"
	})
	saveProgress(rollbackCtx, kubeClient)
	for _, cfg := range []DeploymentConfig{
		controllerConfig,
		{Name: "migration-vpwned-sdk", Namespace: "migration-system"},
		{Name: "vjailbreak-ui", Namespace: "migration-system"},
	} {
		if err := waitForDeploymentReady(rollbackCtx, kubeClient, cfg); err != nil {
			log.Printf("Warning: Deployment %s not ready: %v", cfg.Name, err)
		}
	}

	// Restore version-config ConfigMap
	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Restoring version configuration"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := upgrade.UpdateVersionConfigMapFromGitHub(rollbackCtx, kubeClient, previousVersion); err != nil {
		log.Printf("Warning: Failed to restore version-config: %v", err)
	}

	updateProgress(func(p *UpgradeProgress) {
		p.CurrentStep = "Restoring vjailbreak settings"
	})
	saveProgress(rollbackCtx, kubeClient)
	if err := upgrade.UpdateVjailbreakSettingsFromGitHub(rollbackCtx, kubeClient, previousVersion); err != nil {
		log.Printf("Warning: Failed to restore vjailbreak-settings: %v", err)
	}

	updateProgress(func(p *UpgradeProgress) {
		p.Status = "rolled_back"
		p.Error = ""
		p.CurrentStep = "Rollback completed successfully"
		now := time.Now()
		p.EndTime = &now
	})
	saveProgress(rollbackCtx, kubeClient)

	log.Printf("Manifest-driven rollback completed: %s -> %s", targetVersion, previousVersion)

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
	progressMu.RLock()
	if upgradeProgress == nil {
		progressMu.RUnlock()
		return
	}
	progressJSON, err := json.Marshal(upgradeProgress)
	progressMu.RUnlock()
	if err != nil {
		log.Printf("Error marshaling progress: %v", err)
		return
	}

	key := client.ObjectKey{Name: progressConfigMapName, Namespace: "migration-system"}
	existing := &corev1.ConfigMap{}

	if err := kubeClient.Get(ctx, key, existing); err == nil {
		// ConfigMap exists, use Patch to avoid resourceVersion conflicts
		patch := client.MergeFrom(existing.DeepCopy())
		existing.Data = map[string]string{"progress": string(progressJSON)}
		if err := kubeClient.Patch(ctx, existing, patch); err != nil {
			log.Printf("Error patching upgrade progress ConfigMap: %v", err)
		}
	} else if kerrors.IsNotFound(err) {
		// ConfigMap doesn't exist, create it
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      progressConfigMapName,
				Namespace: "migration-system",
			},
			Data: map[string]string{"progress": string(progressJSON)},
		}
		if err := kubeClient.Create(ctx, cm); err != nil {
			log.Printf("Error creating upgrade progress ConfigMap: %v", err)
		}
	} else {
		log.Printf("Error getting upgrade progress ConfigMap: %v", err)
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
			progressMu.Lock()
			upgradeProgress = &loadedProgress
			progressMu.Unlock()
		}
	}
}
