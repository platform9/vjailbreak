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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	providers "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ESXIMigrationReconciler reconciles a ESXIMigration object
type ESXIMigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esximigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esximigrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esximigrations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ESXIMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ESXIMigrationControllerName)
	ctxlog.Info("Starting reconciliation", "esximigration", req.NamespacedName)

	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	if err := r.Get(ctx, req.NamespacedName, esxiMigration); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Resource not found, likely deleted", "esximigration", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get ESXIMigration resource", "esximigration", req.NamespacedName)
		return ctrl.Result{}, err
	}

	scope, err := scope.NewESXIMigrationScope(scope.ESXIMigrationScopeParams{
		Logger:        ctxlog,
		Client:        r.Client,
		ESXIMigration: esxiMigration,
	})
	if err != nil {
		ctxlog.Error(err, "Failed to create ESXIMigrationScope")
		return ctrl.Result{}, errors.Wrap(err, "failed to create ESXIMigrationScope")
	}

	rollingMigrationPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
	rollingMigrationPlanKey := client.ObjectKey{Namespace: esxiMigration.Namespace, Name: esxiMigration.Spec.RollingMigrationPlanRef.Name}
	if err := r.Get(ctx, rollingMigrationPlanKey, rollingMigrationPlan); err != nil {
		if apierrors.IsNotFound(err) {
			if !esxiMigration.ObjectMeta.DeletionTimestamp.IsZero() {
				ctxlog.Info("Resource is being deleted, reconciling deletion", "esximigration", req.NamespacedName)
				return r.reconcileDelete(ctx, scope)
			}
			return ctrl.Result{}, errors.Wrap(err, "failed to get RollingMigrationPlan")
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get RollingMigrationPlan")
	}

	scope.RollingMigrationPlan = rollingMigrationPlan

	// Always close the scope when exiting this function such that we can persist any ESXIMigration changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			ctxlog.Error(err, "Failed to close ESXIMigrationScope")
			reterr = err
		}
	}()

	if !esxiMigration.ObjectMeta.DeletionTimestamp.IsZero() {
		ctxlog.Info("Resource is being deleted, reconciling deletion", "esximigration", req.NamespacedName)
		return r.reconcileDelete(ctx, scope)
	}

	return r.reconcileNormal(ctx, scope)
}

func (r *ESXIMigrationReconciler) reconcileNormal(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Starting normal reconciliation", "esximigration", scope.ESXIMigration.Name, "namespace", scope.ESXIMigration.Namespace)
	controllerutil.AddFinalizer(scope.ESXIMigration, constants.ESXIMigrationFinalizer)
	if scope.ESXIMigration.Status.Phase == "" {
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseWaiting
		err := r.Status().Update(ctx, scope.ESXIMigration)
		if err != nil {
			log.Error(err, "Failed to update ESXIMigration status")
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
	} else if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded {
		log.Info("ESXIMigration already succeeded")
		return ctrl.Result{}, nil
	} else if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseFailed {
		log.Info("ESXIMigration already failed")
		return ctrl.Result{}, nil
	}

	if utils.IsESXIMigrationPaused(ctx, scope.ESXIMigration.Name, scope.Client) {
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhasePaused
		if err := scope.Client.Status().Update(ctx, scope.ESXIMigration); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
		log.Info(fmt.Sprintf("ESXi migration %s is paused, skipping reconciliation", scope.ESXIMigration.Name))
		return ctrl.Result{}, nil
	}

	bmConfig := &vjailbreakv1alpha1.BMConfig{}
	bmConfigKey := client.ObjectKey{Namespace: scope.ESXIMigration.Namespace, Name: scope.RollingMigrationPlan.Spec.BMConfigRef.Name}
	if err := r.Get(ctx, bmConfigKey, bmConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrap(err, "failed to get BMConfig")
		}
		log.Error(err, "Failed to get BMConfig", "bmconfig", bmConfigKey)
		return ctrl.Result{}, errors.Wrap(err, "failed to get BMConfig")
	}

	if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseCordoned {
		log.Info("ESXIMigration is in cordoned phase, initializing BM Provisioner", "providerType", bmConfig.Spec.ProviderType)

		// TODO:Omkar Assume this will be done by vPwned
		provider, err := providers.GetProvider(string(bmConfig.Spec.ProviderType))
		if err != nil {
			return ctrl.Result{}, err
		}
		err = utils.ConvertESXiToPCDHost(ctx, scope, provider)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Remove the ESXi host from vCenter before changing the phase
		err = utils.RemoveESXiFromVCenter(ctx, r.Client, scope)
		if err != nil {
			log.Error(err, "Failed to remove ESXi from vCenter", "esxiName", scope.ESXIMigration.Spec.ESXiName)
			return ctrl.Result{}, errors.Wrap(err, "failed to remove ESXi from vCenter")
		}
		log.Info("Successfully removed ESXi from vCenter", "esxiName", scope.ESXIMigration.Spec.ESXiName)

		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded
		err = r.Status().Update(ctx, scope.ESXIMigration)
		if err != nil {
			log.Error(err, "Failed to update ESXIMigration status", "esxiName", scope.ESXIMigration.Spec.ESXiName)
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
		return ctrl.Result{}, nil
	}

	inMaintenance, err := utils.CheckESXiInMaintenanceMode(ctx, r.Client, scope)
	if err != nil {
		log.Error(err, "Failed to check ESXi maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to check ESXi maintenance mode")
	}
	if inMaintenance {
		log.Info("ESXi is already in maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		vmCount, err := utils.CountVMsOnESXi(ctx, r.Client, scope)
		if err != nil {
			log.Error(err, "Failed to count VMs on ESXi", "esxiName", scope.ESXIMigration.Spec.ESXiName)
			return ctrl.Result{}, errors.Wrap(err, "failed to count VMs on ESXi")
		}
		log.Info("Counted VMs on ESXi", "esxiName", scope.ESXIMigration.Spec.ESXiName, "vmCount", vmCount)
		if vmCount != 0 {
			log.Info("VMs present on this ESXi host, waiting for VMs to be moved", "ESXiName", scope.ESXIMigration.Spec.ESXiName)
			// Omkar change back to 5 mins
			scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseWaitingForVMsToBeMoved
			err = r.Status().Update(ctx, scope.ESXIMigration)
			if err != nil {
				log.Error(err, "Failed to update ESXIMigration status")
				return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
		log.Info("No VMs on this ESXi host, removing from vCenter and converting to PCD host", "ESXiName", scope.ESXIMigration.Spec.ESXiName)

		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseCordoned
		err = r.Status().Update(ctx, scope.ESXIMigration)
		if err != nil {
			log.Error(err, "Failed to update ESXIMigration status")
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
		log.Info("Successfully updated ESXIMigration status to cordoned")
	} else {
		log.Info("Putting ESXi in maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		err = utils.PutESXiInMaintenanceMode(ctx, r.Client, scope)
		if err != nil {
			log.Error(err, "Failed to put ESXi in maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
			return ctrl.Result{}, errors.Wrap(err, "failed to put ESXi in maintenance mode")
		}
		log.Info("Successfully updated ESXIMigration status to maintenance")
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ESXIMigrationReconciler) reconcileDelete(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Reconciling deletion", "esximigration", scope.ESXIMigration.Name, "namespace", scope.ESXIMigration.Namespace)

	controllerutil.RemoveFinalizer(scope.ESXIMigration, constants.ESXIMigrationFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ESXIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ESXIMigration{}).
		Complete(r)
}
