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
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	storagesdk "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ArrayCredsReconciler reconciles an ArrayCreds object
type ArrayCredsReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=arraycreds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=arraycreds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=arraycreds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ArrayCredsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ArrayCredsControllerName)

	// Get the ArrayCreds object
	arraycreds := &vjailbreakv1alpha1.ArrayCreds{}
	if err := r.Get(ctx, req.NamespacedName, arraycreds); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Resource not found, likely deleted", "arraycreds", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get ArrayCreds resource", "arraycreds", req.NamespacedName)
		return ctrl.Result{}, err
	}

	ctxlog.V(1).Info("Retrieved ArrayCreds resource", "arraycreds", req.NamespacedName, "resourceVersion", arraycreds.ResourceVersion)

	scope, err := scope.NewArrayCredsScope(scope.ArrayCredsScopeParams{
		Logger:     ctxlog,
		Client:     r.Client,
		ArrayCreds: arraycreds,
	})
	if err != nil {
		ctxlog.Error(err, "Failed to create ArrayCredsScope")
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			ctxlog.Error(err, "Failed to close ArrayCredsScope")
			reterr = err
		}
	}()

	if !arraycreds.DeletionTimestamp.IsZero() {
		ctxlog.Info("Resource is being deleted, reconciling deletion", "arraycreds", req.NamespacedName)
		err := r.reconcileDelete(ctx, scope)
		return ctrl.Result{}, err
	}

	ctxlog.Info("Reconciling normal state", "arraycreds", req.NamespacedName)
	return r.reconcileNormal(ctx, scope)
}

func (r *ArrayCredsReconciler) reconcileNormal(ctx context.Context, scope *scope.ArrayCredsScope) (ctrl.Result, error) {
	ctxlog := scope.Logger
	ctxlog.Info("Reconciling ArrayCreds")
	arraycreds := scope.ArrayCreds

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(arraycreds, constants.ArrayCredsFinalizer) {
		controllerutil.AddFinalizer(arraycreds, constants.ArrayCredsFinalizer)
		if err := r.Update(ctx, arraycreds); err != nil {
			ctxlog.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get credentials from secret
	arrayCredential, err := utils.GetArrayCredentialsFromSecret(ctx, r.Client, arraycreds.Spec.SecretRef.Name)
	if err != nil {
		ctxlog.Error(err, "Failed to get storage array credentials from secret", "secretName", arraycreds.Spec.SecretRef.Name)
		scope.ArrayCreds.Status.ArrayValidationStatus = constants.ValidationStatusFailed
		scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Failed to get credentials from secret: %v", err)
		if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
			ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Validate credentials using storage SDK
	if err := r.validateArrayCredentials(ctx, arraycreds.Spec.VendorType, arrayCredential); err != nil {
		ctxlog.Error(err, "Error validating ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
		scope.ArrayCreds.Status.ArrayValidationStatus = constants.ValidationStatusFailed
		scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Validation failed: %v", err)
		ctxlog.Info("Updating status to failed", "arraycreds", scope.ArrayCreds.Name, "message", err.Error())
		if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
			ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Successfully validated
	ctxlog.Info("Successfully authenticated to storage array", "vendor", arraycreds.Spec.VendorType, "hostname", arrayCredential.Hostname)
	scope.ArrayCreds.Status.ArrayValidationStatus = string(corev1.PodSucceeded)
	scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Successfully authenticated to %s storage array", arraycreds.Spec.VendorType)
	ctxlog.Info("Updating status to success", "arraycreds", scope.ArrayCreds.Name)
	if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
		ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
		return ctrl.Result{}, err
	}
	ctxlog.Info("Successfully updated status to success")

	// Requeue periodically to re-validate credentials
	return ctrl.Result{Requeue: true, RequeueAfter: 15 * time.Minute}, nil
}

func (r *ArrayCredsReconciler) reconcileDelete(ctx context.Context, scope *scope.ArrayCredsScope) error {
	ctxlog := scope.Logger
	ctxlog.Info("Reconciling deletion", "arraycreds", scope.ArrayCreds.Name, "namespace", scope.ArrayCreds.Namespace)
	arraycreds := scope.ArrayCreds

	// Delete associated secret
	if secretName := arraycreds.Spec.SecretRef.Name; secretName != "" {
		ctxlog.Info("Deleting associated secret", "secretName", secretName)
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: constants.NamespaceMigrationSystem}}
		if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			ctxlog.Error(err, "Failed to delete associated secret", "secretName", secretName)
			return errors.Wrap(err, "failed to delete associated secret")
		}
		ctxlog.Info("Successfully deleted associated secret or it was already gone", "secretName", secretName)
	}

	ctxlog.Info("All cleanup successful. Removing finalizer.")
	if controllerutil.RemoveFinalizer(arraycreds, constants.ArrayCredsFinalizer) {
		if err := r.Update(ctx, arraycreds); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			ctxlog.Error(err, "failed to update resource to remove finalizer")
			return err
		}
	}

	return nil
}

// validateArrayCredentials validates storage array credentials using the storage SDK
func (r *ArrayCredsReconciler) validateArrayCredentials(ctx context.Context, vendorType string, creds vjailbreakv1alpha1.ArrayCredsInfo) error {
	// Get the storage provider
	provider, err := storagesdk.NewStorageProvider(vendorType)
	if err != nil {
		return errors.Wrapf(err, "failed to get storage provider for vendor type '%s'", vendorType)
	}

	// Create access info
	accessInfo := storagesdk.StorageAccessInfo{
		Hostname:            creds.Hostname,
		Username:            creds.Username,
		Password:            creds.Password,
		SkipSSLVerification: creds.SkipSSLVerification,
		VendorType:          vendorType,
	}

	// Connect to storage array
	if err := provider.Connect(ctx, accessInfo); err != nil {
		return errors.Wrap(err, "failed to connect to storage array")
	}
	defer func() {
		if err := provider.Disconnect(); err != nil {
			log.Error(err, "failed to disconnect from storage array")
		}
	}()

	// Validate credentials
	if err := provider.ValidateCredentials(ctx); err != nil {
		return errors.Wrap(err, "credential validation failed")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *ArrayCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ArrayCreds{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Complete(r)
}
