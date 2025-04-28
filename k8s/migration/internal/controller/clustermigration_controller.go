/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
)

// ClusterMigrationReconciler reconciles a ClusterMigration object
type ClusterMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=clustermigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=clustermigrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=clustermigrations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterMigration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *ClusterMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ClusterMigrationControllerName)
	ctxlog.Info("Starting reconciliation", "clustermigration", req.NamespacedName)

	clusterMigration := &vjailbreakv1alpha1.ClusterMigration{}
	if err := r.Get(ctx, req.NamespacedName, clusterMigration); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Resource not found, likely deleted", "clustermigration", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get ClusterMigration resource", "clustermigration", req.NamespacedName)
		return ctrl.Result{}, err
	}
	ctxlog.V(1).Info("Retrieved ClusterMigration resource", "clustermigration", req.NamespacedName, "resourceVersion", clusterMigration.ResourceVersion)

	rollingMigrationPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
	rollingMigrationPlanKey := client.ObjectKey{Namespace: clusterMigration.Namespace, Name: clusterMigration.Spec.RollingMigrationPlanRef.Name}
	ctxlog.V(1).Info("Fetching referenced RollingMigrationPlan", "rollingMigrationPlan", rollingMigrationPlanKey)
	if err := r.Get(ctx, rollingMigrationPlanKey, rollingMigrationPlan); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Referenced RollingMigrationPlan not found", "rollingMigrationPlan", rollingMigrationPlanKey)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get RollingMigrationPlan", "rollingMigrationPlan", rollingMigrationPlanKey)
		return ctrl.Result{}, err
	}
	ctxlog.V(1).Info("Retrieved RollingMigrationPlan", "rollingMigrationPlan", rollingMigrationPlanKey, "resourceVersion", rollingMigrationPlan.ResourceVersion)

	scope, err := scope.NewClusterMigrationScope(scope.ClusterMigrationScopeParams{
		Logger:               ctxlog,
		Client:               r.Client,
		ClusterMigration:     clusterMigration,
		RollingMigrationPlan: rollingMigrationPlan,
	})
	if err != nil {
		ctxlog.Error(err, "Failed to create ClusterMigrationScope")
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function such that we can persist any ClusterMigration changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			ctxlog.Error(err, "Failed to close ClusterMigrationScope")
			reterr = err
		}
	}()

	if !clusterMigration.ObjectMeta.DeletionTimestamp.IsZero() {
		ctxlog.Info("Resource is being deleted, reconciling deletion", "clustermigration", req.NamespacedName)
		return r.reconcileDelete(ctx, scope)
	}

	ctxlog.Info("Reconciling normal state", "clustermigration", req.NamespacedName)
	return r.reconcileNormal(ctx, scope)
}

func (r *ClusterMigrationReconciler) reconcileNormal(ctx context.Context, scope *scope.ClusterMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	clusterMigration := scope.ClusterMigration
	log.Info("Starting normal reconciliation", "clustermigration", clusterMigration.Name, "namespace", clusterMigration.Namespace)

	controllerutil.AddFinalizer(clusterMigration, constants.ClusterMigrationFinalizer)
	if err := scope.Close(); err != nil {
		log.Error(err, "Failed to close ClusterMigrationScope")
		return ctrl.Result{}, errors.Wrap(err, "failed to close cluster migration scope")
	}
	if clusterMigration.Status.Phase == "" {
		log.Info("Initializing ClusterMigration phase", "newPhase", vjailbreakv1alpha1.ClusterMigrationPhasePending)
		clusterMigration.Status.Phase = vjailbreakv1alpha1.ClusterMigrationPhasePending
	} else if clusterMigration.Status.Phase == vjailbreakv1alpha1.ClusterMigrationPhaseSucceeded {
		log.Info("Cluster migration already succeeded")
		return ctrl.Result{}, nil
	} else if clusterMigration.Status.Phase == vjailbreakv1alpha1.ClusterMigrationPhaseFailed {
		log.Info("Cluster migration already failed")
		return ctrl.Result{}, nil
	}

	for _, esxi := range clusterMigration.Spec.ESXIMigrationSequence {
		esxiMigration, err := utils.GetESXIMigration(ctx, scope.Client, esxi, scope.RollingMigrationPlan)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("ESXIMigration not found, creating new one", "esxiName", esxi)
				if esxiMigration, err = utils.CreateESXIMigration(ctx, scope.Client, esxi, scope.RollingMigrationPlan); err != nil {
					log.Error(err, "Failed to create ESXIMigration", "esxiName", esxi)
					return ctrl.Result{}, errors.Wrap(err, "failed to create esxi migration")
				}
				log.Info("Successfully created ESXIMigration", "esxiName", esxi, "esximigration", esxiMigration.Name)
			} else {
				log.Error(err, "Failed to get ESXIMigration", "esxiName", esxi)
				return ctrl.Result{}, errors.Wrap(err, "failed to get esxi migration")
			}
		}
		log.Info("Retrieved ESXIMigration", "esxiName", esxi, "esximigration", esxiMigration.Name, "phase", esxiMigration.Status.Phase)

		if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseFailed {
			log.Info("ESXIMigration failed, updating ClusterMigration status", "esxiName", esxi, "message", esxiMigration.Status.Message)
			err = r.UpdateClusterMigrationStatus(ctx, scope, vjailbreakv1alpha1.ClusterMigrationPhaseFailed, esxiMigration.Status.Message, esxi)
			if err != nil {
				log.Error(err, "Failed to update ClusterMigration status", "desiredPhase", vjailbreakv1alpha1.ClusterMigrationPhaseFailed)
				return ctrl.Result{}, errors.Wrap(err, "failed to update cluster migration status")
			}
			log.Info("Successfully updated ClusterMigration status to failed")
			return ctrl.Result{}, nil
		} else if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded {
			log.Info("ESXIMigration succeeded, continuing to next ESXi", "esxiName", esxi)
			continue
		} else if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseRunning {
			log.Info("ESXIMigration is running, updating ClusterMigration status", "esxiName", esxi)
			err = r.UpdateClusterMigrationStatus(ctx, scope, vjailbreakv1alpha1.ClusterMigrationPhaseRunning, esxiMigration.Status.Message, esxi)
			if err != nil {
				log.Error(err, "Failed to update ClusterMigration status", "desiredPhase", vjailbreakv1alpha1.ClusterMigrationPhaseRunning)
				return ctrl.Result{}, errors.Wrap(err, "failed to update cluster migration status")
			}
			log.Info("Successfully updated ClusterMigration status to running")
			log.Info("Requeuing ClusterMigration for further processing", "requeueAfter", "1m")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		} else {
			log.Info("ESXIMigration is in another state, updating ClusterMigration status to pending", "esxiName", esxi, "esxiPhase", esxiMigration.Status.Phase)
			err = r.UpdateClusterMigrationStatus(ctx, scope, vjailbreakv1alpha1.ClusterMigrationPhasePending, esxiMigration.Status.Message, esxi)
			if err != nil {
				log.Error(err, "Failed to update ClusterMigration status", "desiredPhase", vjailbreakv1alpha1.ClusterMigrationPhasePending)
				return ctrl.Result{}, errors.Wrap(err, "failed to update cluster migration status")
			}
			log.Info("Successfully updated ClusterMigration status to pending")
			log.Info("Requeuing ClusterMigration for further processing", "requeueAfter", "1m")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *ClusterMigrationReconciler) reconcileDelete(ctx context.Context, scope *scope.ClusterMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Reconciling deletion", "clustermigration", scope.ClusterMigration.Name, "namespace", scope.ClusterMigration.Namespace)

	// Delete all ESXIMigrations
	for _, esxi := range scope.ClusterMigration.Spec.ESXIMigrationSequence {
		esxiMigration, err := utils.GetESXIMigration(ctx, r.Client, esxi, scope.RollingMigrationPlan)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to get ESXIMigration", "esxiName", esxi)
			return ctrl.Result{}, errors.Wrap(err, "failed to get esxi migration")
		}
		if err := r.Delete(ctx, esxiMigration); err != nil {
			log.Error(err, "Failed to delete ESXIMigration", "esxiName", esxi)
			return ctrl.Result{}, errors.Wrap(err, "failed to delete esxi migration")
		}
	}

	// Wait for all ESXIMigrations to be deleted
	for _, esxi := range scope.ClusterMigration.Spec.ESXIMigrationSequence {
		_, err := utils.GetESXIMigration(ctx, r.Client, esxi, scope.RollingMigrationPlan)
		if err == nil {
			// ESXIMigration still exists, requeue
			log.Info("ESXIMigration still exists, requeuing", "esxiName", esxi)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	controllerutil.RemoveFinalizer(scope.ClusterMigration, constants.ClusterMigrationFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ClusterMigration{}).
		Complete(r)
}

func (r *ClusterMigrationReconciler) UpdateClusterMigrationStatus(ctx context.Context, scope *scope.ClusterMigrationScope, status vjailbreakv1alpha1.ClusterMigrationPhase, message, currentESXi string) error {
	log := scope.Logger
	log.V(1).Info("Updating ClusterMigration status",
		"previousPhase", scope.ClusterMigration.Status.Phase,
		"newPhase", status,
		"message", message,
		"currentESXi", currentESXi)
	scope.ClusterMigration.Status.Phase = status
	scope.ClusterMigration.Status.Message = message
	scope.ClusterMigration.Status.CurrentESXi = currentESXi
	return r.Status().Update(ctx, scope.ClusterMigration)
}
