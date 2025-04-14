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
	"fmt"
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

	clusterMigration := &vjailbreakv1alpha1.ClusterMigration{}
	if err := r.Get(ctx, req.NamespacedName, clusterMigration); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	scope, err := scope.NewClusterMigrationScope(scope.ClusterMigrationScopeParams{
		Logger:           ctxlog,
		Client:           r.Client,
		ClusterMigration: clusterMigration,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function such that we can persist any ClusterMigration changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !clusterMigration.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, scope)
	}

	return r.reconcileNormal(ctx, scope)
}

func (r *ClusterMigrationReconciler) reconcileNormal(ctx context.Context, scope *scope.ClusterMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	clusterMigration := scope.ClusterMigration
	log.Info(fmt.Sprintf("Reconciling ClusterMigration '%s'", clusterMigration.Name))
	controllerutil.AddFinalizer(clusterMigration, constants.ClusterMigrationFinalizer)
	var esxiMigration *vjailbreakv1alpha1.ESXIMigration
	var err error
	if clusterMigration.Status.Phase == "" {
		clusterMigration.Status.Phase = constants.ClusterMigrationPhaseWaiting
	} else if clusterMigration.Status.Phase == constants.ClusterMigrationPhaseSucceeded {
		log.Info("Cluster migration already succeeded")
		return ctrl.Result{}, nil
	} else if clusterMigration.Status.Phase == constants.ClusterMigrationPhaseFailed {
		log.Info("Cluster migration already failed")
		return ctrl.Result{}, nil
	}

	for _, esxi := range clusterMigration.Spec.ESXIMigrationSequence {
		esxiMigration, err = utils.GetESXIMigration(ctx, r.Client, esxi)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if esxiMigration, err = utils.CreateESXIMigration(ctx, r.Client, esxi); err != nil {
					return ctrl.Result{}, errors.Wrap(err, "failed to create esxi migration")
				}
			} else {
				return ctrl.Result{}, errors.Wrap(err, "failed to get esxi migration")
			}
		}

		if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseFailed {
			err = r.UpdateClusterMigrationStatus(ctx, scope, constants.ClusterMigrationPhaseFailed, esxiMigration.Status.Message, esxi)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update cluster migration status")
			}
			return ctrl.Result{}, nil
		} else if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded {
			continue
		} else if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseRunning {
			err = r.UpdateClusterMigrationStatus(ctx, scope, constants.ClusterMigrationPhaseRunning, esxiMigration.Status.Message, esxi)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update cluster migration status")
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		} else {
			err = r.UpdateClusterMigrationStatus(ctx, scope, constants.ClusterMigrationPhaseWaiting, esxiMigration.Status.Message, esxi)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update cluster migration status")
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *ClusterMigrationReconciler) reconcileDelete(ctx context.Context, scope *scope.ClusterMigrationScope) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ClusterMigration{}).
		Complete(r)
}

func (r *ClusterMigrationReconciler) UpdateClusterMigrationStatus(ctx context.Context, scope *scope.ClusterMigrationScope, status vjailbreakv1alpha1.ClusterMigrationPhase, message, currentESXi string) error {
	scope.ClusterMigration.Status.Phase = status
	scope.ClusterMigration.Status.Message = message
	scope.ClusterMigration.Status.CurrentESXi = currentESXi
	return r.Status().Update(ctx, scope.ClusterMigration)
}
