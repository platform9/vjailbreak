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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
)

// RollingMigrationPlanReconciler reconciles a RollingMigrationPlan object
type RollingMigrationPlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=rollingmigrationplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=rollingmigrationplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=rollingmigrationplans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RollingMigrationPlan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *RollingMigrationPlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.RollingMigrationPlanControllerName)

	rollingmigrationplan := &vjailbreakv1alpha1.RollingMigrationPlan{}
	if err := r.Get(ctx, req.NamespacedName, rollingmigrationplan); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	scope, err := scope.NewRollingMigrationPlanScope(scope.RollingMigrationPlanScopeParams{
		Logger:               ctxlog,
		Client:               r.Client,
		RollingMigrationPlan: rollingmigrationplan,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function such that we can persist any RollingMigrationPlan changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !rollingmigrationplan.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, scope)
	}
	return r.reconcileNormal(ctx, scope)
}

func (r *RollingMigrationPlanReconciler) reconcileNormal(ctx context.Context, scope *scope.RollingMigrationPlanScope) (ctrl.Result, error) {
	log := scope.Logger
	migrationPlan := scope.RollingMigrationPlan
	log.Info(fmt.Sprintf("Reconciling RollingMigrationPlan '%s'", migrationPlan.Name))

	controllerutil.AddFinalizer(migrationPlan, constants.RollingMigrationPlanFinalizer)
	if err := scope.Close(); err != nil {
		log.Error(err, "Failed to close RollingMigrationPlanScope")
		return ctrl.Result{}, errors.Wrap(err, "failed to close rolling migration plan scope")
	}

	// validate environment is PCD
	if isPCD, err := utils.ValidateOpenstackIsPCD(ctx, r.Client, migrationPlan); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to validate environment")
	} else if !isPCD {
		log.Info("OpenStack environment is not PCD, skipping rolling migration plan")
		return ctrl.Result{}, nil
	}

	if migrationPlan.Status.Phase == "" {
		migrationPlan.Status.Phase = vjailbreakv1alpha1.RollingMigrationPlanPhaseWaiting
	} else if migrationPlan.Status.Phase == vjailbreakv1alpha1.RollingMigrationPlanPhaseSucceeded {
		log.Info("RollingMigrationPlan already succeeded")
		return ctrl.Result{}, nil
	} else if migrationPlan.Status.Phase == vjailbreakv1alpha1.RollingMigrationPlanPhaseFailed {
		log.Info("RollingMigrationPlan already failed")
		return ctrl.Result{}, nil
	}

	if migrationPlan.Spec.CloudInitConfigRef == nil {
		log.Info("CloudInitConfigRef is not set")
		err := utils.MergeCloudInitAndCreateSecret(ctx, scope)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to merge cloud-init config and create secret")
		}
	}

	// Update ESXi Name in RollingMigrationPlan for each VM in VM Sequence
	for i, cluster := range scope.RollingMigrationPlan.Spec.ClusterSequence {
		for j := range cluster.VMSequence {
			k8sVMName, err := utils.ConvertToK8sName(cluster.VMSequence[j].VMName)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to convert vm name to k8s name")
			}
			vm := &vjailbreakv1alpha1.VMwareMachine{}
			err = r.Get(ctx, client.ObjectKey{
				Name:      k8sVMName,
				Namespace: scope.Namespace(),
			}, vm)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error getting VMInfo for VM '%s'", cluster.VMSequence[j].VMName))
			}
			scope.RollingMigrationPlan.Spec.ClusterSequence[i].VMSequence[j].ESXiName = vm.Spec.VMInfo.ESXiName
		}
	}

	// execute rolling migration plan
	for _, cluster := range scope.RollingMigrationPlan.Spec.ClusterSequence {
		// TODO(vPwned): poweroff vms cannot be moved by the vmware vcenter
		// TODO(vPwned): DRS needs to be enabled and on fully automated mode
		clusterMigration, err := utils.GetClusterMigration(ctx, scope.Client, cluster.ClusterName, scope.RollingMigrationPlan)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if _, createErr := utils.CreateClusterMigration(ctx, scope.Client, cluster, scope.RollingMigrationPlan); createErr != nil {
					return ctrl.Result{}, errors.Wrap(createErr, "failed to create cluster migration")
				}
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			} else {
				return ctrl.Result{}, errors.Wrap(err, "failed to get cluster migration")
			}
		}
		if clusterMigration.Status.Phase == vjailbreakv1alpha1.ClusterMigrationPhaseFailed {
			log.Info("Cluster migration is in failed state, aborting rolling migration plan", "cluster", cluster, "message", clusterMigration.Status.Message)
			err = r.UpdateRollingMigrationPlanStatus(ctx, scope, vjailbreakv1alpha1.RollingMigrationPlanPhaseFailed, clusterMigration.Status.Message, cluster.ClusterName, clusterMigration.Status.CurrentESXi)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update rolling migration plan status")
			}
			return ctrl.Result{}, nil
		} else if clusterMigration.Status.Phase == vjailbreakv1alpha1.ClusterMigrationPhaseSucceeded {
			continue
		} else if clusterMigration.Status.Phase == vjailbreakv1alpha1.ClusterMigrationPhaseRunning {
			err = r.UpdateRollingMigrationPlanStatus(ctx, scope, vjailbreakv1alpha1.RollingMigrationPlanPhaseRunning, clusterMigration.Status.Message, cluster.ClusterName, clusterMigration.Status.CurrentESXi)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update rolling migration plan status")
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		} else {
			err = r.UpdateRollingMigrationPlanStatus(ctx, scope, vjailbreakv1alpha1.RollingMigrationPlanPhaseWaiting, clusterMigration.Status.Message, cluster.ClusterName, clusterMigration.Status.CurrentESXi)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update rolling migration plan status")
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *RollingMigrationPlanReconciler) reconcileDelete(ctx context.Context, scope *scope.RollingMigrationPlanScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Reconciling deletion", "rollingmigrationplan", scope.RollingMigrationPlan.Name, "namespace", scope.RollingMigrationPlan.Namespace)

	// Delete all ClusterMigrations
	for _, cluster := range scope.RollingMigrationPlan.Spec.ClusterSequence {
		clusterMigration, err := utils.GetClusterMigration(ctx, r.Client, cluster.ClusterName, scope.RollingMigrationPlan)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to get ClusterMigration", "cluster", cluster)
			return ctrl.Result{}, errors.Wrap(err, "failed to get cluster migration")
		}
		if err := r.Delete(ctx, clusterMigration); err != nil {
			log.Error(err, "Failed to delete ClusterMigration", "cluster", cluster)
			return ctrl.Result{}, errors.Wrap(err, "failed to delete cluster migration")
		}
	}

	// Wait for all ClusterMigrations to be deleted
	for _, cluster := range scope.RollingMigrationPlan.Spec.ClusterSequence {
		_, err := utils.GetClusterMigration(ctx, r.Client, cluster.ClusterName, scope.RollingMigrationPlan)
		if err == nil {
			// ClusterMigration still exists, requeue
			log.Info("ClusterMigration still exists, requeuing", "cluster", cluster)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	controllerutil.RemoveFinalizer(scope.RollingMigrationPlan, constants.RollingMigrationPlanFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RollingMigrationPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.RollingMigrationPlan{}).
		Complete(r)
}

func (r *RollingMigrationPlanReconciler) UpdateRollingMigrationPlanStatus(ctx context.Context, scope *scope.RollingMigrationPlanScope, status vjailbreakv1alpha1.RollingMigrationPlanPhase, message, currentCluster, currentESXi string) error {
	scope.RollingMigrationPlan.Status.Phase = status
	scope.RollingMigrationPlan.Status.Message = message
	scope.RollingMigrationPlan.Status.CurrentCluster = currentCluster
	scope.RollingMigrationPlan.Status.CurrentESXi = currentESXi
	return r.Status().Update(ctx, scope.RollingMigrationPlan)
}
