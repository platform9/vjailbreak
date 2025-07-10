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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
)

// VMwareCredsReconciler reconciles a VMwareCreds object
type VMwareCredsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarecreds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarecreds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarecreds/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarehosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwareclusters,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *VMwareCredsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx)

	// Get the VMwareCreds object
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{}
	if err := r.Get(ctx, req.NamespacedName, vmwcreds); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted VMWareCreds.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading VMWareCreds '%s' object", vmwcreds.Name))
		return ctrl.Result{}, err
	}
	scope, err := scope.NewVMwareCredsScope(scope.VMwareCredsScopeParams{
		Logger:      ctxlog,
		Client:      r.Client,
		VMwareCreds: vmwcreds,
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always close the scope when exiting this function such that we can persist any vmwarecreds changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if vmwcreds.DeletionTimestamp.IsZero() {
		return r.reconcileNormal(ctx, scope)
	}
	return r.reconcileDelete(ctx, scope)
}

func (r *VMwareCredsReconciler) reconcileNormal(ctx context.Context, scope *scope.VMwareCredsScope) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info(fmt.Sprintf("Reconciling VMwareCreds '%s' object", scope.Name()))

	if _, err := utils.ValidateVMwareCreds(ctx, r.Client, scope.VMwareCreds); err != nil {
		// Update the status of the VMwareCreds object
		scope.VMwareCreds.Status.VMwareValidationStatus = string(corev1.PodFailed)
		scope.VMwareCreds.Status.VMwareValidationMessage = fmt.Sprintf("Error validating VMwareCreds '%s': %s", scope.Name(), err)
		if updateErr := r.Status().Update(ctx, scope.VMwareCreds); updateErr != nil {
			return ctrl.Result{}, errors.Wrap(
				errors.Wrap(updateErr, fmt.Sprintf("Error updating status of VMwareCreds '%s'", scope.Name())),
				err.Error())
		}
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error validating VMwareCreds '%s'", scope.Name()))
	}
	ctxlog.Info(fmt.Sprintf("Successfully authenticated to VMware '%s'", scope.Name()))
	// Update the status of the VMwareCreds object
	scope.VMwareCreds.Status.VMwareValidationStatus = "Succeeded"
	scope.VMwareCreds.Status.VMwareValidationMessage = "Successfully authenticated to VMware"
	if err := r.Status().Update(ctx, scope.VMwareCreds); err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error updating status of VMwareCreds '%s'", scope.Name()))
	}

	ctxlog.Info("Successfully validated VMwareCreds, adding finalizer", "name", scope.Name(), "finalizers", scope.VMwareCreds.Finalizers)
	controllerutil.AddFinalizer(scope.VMwareCreds, constants.VMwareCredsFinalizer)

	err := utils.CreateVMwareClustersAndHosts(ctx, r.Client, scope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error creating VMs for VMwareCreds '%s'", scope.Name()))
	}

	vminfo, err := utils.GetAllVMs(ctx, r.Client, scope.VMwareCreds, scope.VMwareCreds.Spec.DataCenter)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error getting info of all VMs for VMwareCreds '%s'", scope.Name()))
	}
	err = utils.CreateOrUpdateVMwareMachines(ctx, r.Client, scope.VMwareCreds, vminfo)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error creating VMs for VMwareCreds '%s'", scope.Name()))
	}
	err = utils.DeleteStaleVMwareMachines(ctx, r.Client, scope.VMwareCreds, vminfo)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error finding deleted VMs for VMwareCreds '%s'", scope.Name()))
	}

	err = utils.DeleteStaleVMwareMachines(ctx, r.Client, scope.VMwareCreds, vminfo)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error finding deleted VMs for VMwareCreds '%s'", scope.Name()))
	}
	err = utils.DeleteStaleVMwareClustersAndHosts(ctx, r.Client, scope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error finding deleted clusters and hosts for VMwareCreds '%s'", scope.Name()))
	}

	return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
}

// nolint:unparam
func (r *VMwareCredsReconciler) reconcileDelete(ctx context.Context, scope *scope.VMwareCredsScope) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info(fmt.Sprintf("Reconciling deletion of VMwareCreds '%s' object", scope.Name()))

	err := utils.DeleteDependantObjectsForVMwareCreds(ctx, scope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("Error deleting dependant objects for VMwareCreds '%s'", scope.Name()))
	}

	// Always remove the finalizer to ensure the resource can be deleted for cred with unknown status
	if controllerutil.ContainsFinalizer(scope.VMwareCreds, constants.VMwareCredsFinalizer) {
		controllerutil.RemoveFinalizer(scope.VMwareCreds, constants.VMwareCredsFinalizer)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VMwareCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.VMwareCreds{}).
		Complete(r)
}
