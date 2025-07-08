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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
)

// OpenstackCredsReconciler reconciles a OpenstackCreds object
type OpenstackCredsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Local  bool
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenstackCreds object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
//
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=pcdhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=pcdhosts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=pcdhosts/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=pcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=pcdclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=pcdclusters/finalizers,verbs=update
func (r *OpenstackCredsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.OpenstackCredsControllerName)
	// Get the OpenstackCreds object
	openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{}
	if err := r.Get(ctx, req.NamespacedName, openstackcreds); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Resource not found, likely deleted", "openstackcreds", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get OpenstackCreds resource", "openstackcreds", req.NamespacedName)
		return ctrl.Result{}, err
	}
	ctxlog.V(1).Info("Retrieved OpenstackCreds resource", "openstackcreds", req.NamespacedName, "resourceVersion", openstackcreds.ResourceVersion)
	scope, err := scope.NewOpenstackCredsScope(scope.OpenstackCredsScopeParams{
		Logger:         ctxlog,
		Client:         r.Client,
		OpenstackCreds: openstackcreds,
	})
	if err != nil {
		ctxlog.Error(err, "Failed to create OpenstackCredsScope")
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function such that we can persist any OpenstackCreds changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			ctxlog.Error(err, "Failed to close OpenstackCredsScope")
			reterr = err
		}
	}()

	if !openstackcreds.DeletionTimestamp.IsZero() {
		ctxlog.Info("Resource is being deleted, reconciling deletion", "openstackcreds", req.NamespacedName)
		if err := r.reconcileDelete(ctx, scope); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	ctxlog.Info("Reconciling normal state", "openstackcreds", req.NamespacedName)
	return r.reconcileNormal(ctx, scope)
}

func (r *OpenstackCredsReconciler) reconcileNormal(ctx context.Context,
	scope *scope.OpenstackCredsScope) (ctrl.Result, error) {
	ctxlog := scope.Logger
	ctxlog.Info("Starting normal reconciliation", "openstackcreds", scope.OpenstackCreds.Name, "namespace", scope.OpenstackCreds.Namespace)

	if !controllerutil.ContainsFinalizer(scope.OpenstackCreds, constants.OpenstackCredsFinalizer) {
		ctxlog.Info("Adding finalizer to OpenstackCreds")
		controllerutil.AddFinalizer(scope.OpenstackCreds, constants.OpenstackCredsFinalizer)
		if err := r.Update(ctx, scope.OpenstackCreds); err != nil {
			ctxlog.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if _, err := utils.ValidateAndGetProviderClient(ctx, r.Client, scope.OpenstackCreds); err != nil {
		ctxlog.Error(err, "Error validating OpenstackCreds", "openstackcreds", scope.OpenstackCreds.Name)

		scope.OpenstackCreds.Status.OpenStackValidationStatus = "Failed"
		scope.OpenstackCreds.Status.OpenStackValidationMessage = err.Error()       // Directly use the error message.
		scope.OpenstackCreds.Status.Openstack = vjailbreakv1alpha1.OpenstackInfo{} // Clear previous info.

		ctxlog.Info("Updating status to failed", "openstackcreds", scope.OpenstackCreds.Name)
		if updateErr := r.Status().Update(ctx, scope.OpenstackCreds); updateErr != nil {
			ctxlog.Error(updateErr, "Failed to update status of OpenstackCreds")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}
	if scope.OpenstackCreds.Status.OpenStackValidationStatus != string(corev1.PodSucceeded) {
		ctxlog.Info("Successfully authenticated to OpenStack")
		scope.OpenstackCreds.Status.OpenStackValidationStatus = string(corev1.PodSucceeded)
		scope.OpenstackCreds.Status.OpenStackValidationMessage = "Successfully authenticated to Openstack"
	}

	// Get OpenStack info and update status
	openstackinfo, err := utils.GetOpenstackInfo(ctx, r.Client, scope.OpenstackCreds)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Openstack info: %w", err)
	}
	scope.OpenstackCreds.Status.Openstack = *openstackinfo

	// List flavors and update the spec
	flavors, err := utils.ListAllFlavors(ctx, r.Client, scope.OpenstackCreds)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get flavors: %w", err)
	}
	scope.OpenstackCreds.Spec.Flavors = flavors

	if err := r.Update(ctx, scope.OpenstackCreds); err != nil {
		ctxlog.Error(err, "Failed to update OpenstackCreds spec")
		return ctrl.Result{}, err
	}
	if err := r.Status().Update(ctx, scope.OpenstackCreds); err != nil {
		ctxlog.Error(err, "Failed to update OpenstackCreds status")
		return ctrl.Result{}, err
	}

	// Create a dummy PCD cluster entry if needed
	err = utils.CreateEntryForNoPCDCluster(ctx, r.Client, scope.OpenstackCreds)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("failed to create dummy PCD cluster: %w", err)
	}

	// Update master node image ID (can be done in the background)
	if err := utils.UpdateMasterNodeImageID(ctx, r.Client, r.Local); err != nil {
		ctxlog.Error(err, "Failed to update master node image ID and flavor list")
	}

	// Label VMware machines with flavor info
	vmwaremachineList := &vjailbreakv1alpha1.VMwareMachineList{}
	if err := r.List(ctx, vmwaremachineList); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list vmwaremachine objects: %w", err)
	}

	computeClient, err := utils.GetOpenStackClients(ctx, r.Client, scope.OpenstackCreds)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get OpenStack clients: %w", err)
	}
	for i := range vmwaremachineList.Items {
		vmwaremachine := &vmwaremachineList.Items[i]
		cpu := vmwaremachine.Spec.VMInfo.CPU
		memory := vmwaremachine.Spec.VMInfo.Memory

		flavor, err := utils.GetClosestFlavour(ctx, cpu, memory, computeClient.ComputeClient)
		if err != nil && !strings.Contains(err.Error(), "no suitable flavor found") {
			return ctrl.Result{}, fmt.Errorf("failed to get closest flavor for %s: %w", vmwaremachine.Name, err)
		}

		labelValue := "NOT_FOUND"
		if flavor != nil {
			labelValue = flavor.ID
		}
		if err := utils.CreateOrUpdateLabel(ctx, r.Client, vmwaremachine, scope.OpenstackCreds.Name, labelValue); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update label on vmwaremachine %s: %w", vmwaremachine.Name, err)
		}
	}

	// Sync PCD info if applicable
	if utils.IsOpenstackPCD(*scope.OpenstackCreds) {
		ctxlog.Info("Syncing PCD info", "openstackcreds", scope.OpenstackCreds.Name)
		if err = utils.SyncPCDInfo(ctx, r.Client, *scope.OpenstackCreds); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to sync PCD info: %w", err)
		}
	}

	ctxlog.Info("Successfully reconciled OpenstackCreds")
	return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
}

func (r *OpenstackCredsReconciler) reconcileDelete(ctx context.Context, scope *scope.OpenstackCredsScope) error {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Reconciling deletion for OpenstackCreds")

	// --- Step 1: Perform all cleanup tasks first ---
	if err := utils.DeleteEntryForNoPCDCluster(ctx, r.Client, scope.OpenstackCreds); err != nil {
		ctxlog.Error(err, "Failed to delete PCD cluster entry")
		return fmt.Errorf("failed to delete PCD cluster: %w", err)
	}
	ctxlog.Info("Successfully deleted PCD cluster entry")

	secretName := scope.OpenstackCreds.Spec.SecretRef.Name
	if secretName != "" {
		ctxlog.Info("Deleting associated secret", "secretName", secretName)
		err := r.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: constants.NamespaceMigrationSystem,
			},
		})
		if err != nil && !apierrors.IsNotFound(err) {
			ctxlog.Error(err, "Failed to delete associated secret", "secretName", secretName)
			return fmt.Errorf("failed to delete associated secret: %w", err)
		}
		ctxlog.Info("Successfully deleted associated secret or it was already gone", "secretName", secretName)
	}

	// --- Step 2: Remove the finalizer and save the change ---
	ctxlog.Info("All cleanup successful. Removing finalizer.")
	if controllerutil.RemoveFinalizer(scope.OpenstackCreds, constants.OpenstackCredsFinalizer) {
		// This Update call is crucial to persist the change.
		if err := r.Update(ctx, scope.OpenstackCreds); err != nil {
			// If the resource is already gone, it's a success.
			if apierrors.IsNotFound(err) {
				return nil
			}
			ctxlog.Error(err, "Failed to update resource to remove finalizer")
			return err
		}
	}

	ctxlog.Info("Successfully reconciled deletion of OpenstackCreds")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenstackCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.OpenstackCreds{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
