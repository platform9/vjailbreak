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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	storagesdk "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

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
	Logger                  logr.Logger
}

// DatastoreInfo holds datastore information including backing device NAA
type DatastoreInfo struct {
	Name        string
	Type        string
	Capacity    int64
	FreeSpace   int64
	BackingNAA  string // NAA identifier of the backing LUN (for VMFS datastores)
	BackingUUID string // UUID of the backing device
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
		scope.ArrayCreds.Status.ArrayValidationStatus = "Failed"
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
		scope.ArrayCreds.Status.ArrayValidationStatus = "Failed"
		scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Validation failed: %v", err)
		ctxlog.Info("Updating status to failed", "arraycreds", scope.ArrayCreds.Name, "message", err.Error())
		if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
			ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Successfully validated - now discover datastores
	ctxlog.Info("Successfully authenticated to storage array", "vendor", arraycreds.Spec.VendorType, "hostname", arrayCredential.Hostname)

	// Discover datastores backed by this array
	datastores, err := r.discoverDatastores(ctx, arraycreds.Spec.VendorType, arrayCredential)
	if err != nil {
		ctxlog.Error(err, "Failed to discover datastores", "arraycreds", scope.ArrayCreds.Name)
		// Don't fail validation, just log warning
		scope.ArrayCreds.Status.DataStores = []string{}
	} else {
		ctxlog.Info("Discovered datastores", "count", len(datastores), "datastores", datastores)
		scope.ArrayCreds.Status.DataStores = datastores
	}

	scope.ArrayCreds.Status.ArrayValidationStatus = string(corev1.PodSucceeded)
	scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Successfully authenticated to %s storage array. Discovered %d datastores.", arraycreds.Spec.VendorType, len(datastores))
	ctxlog.Info("Updating status to success", "arraycreds", scope.ArrayCreds.Name)
	if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
		ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
		return ctrl.Result{}, err
	}
	ctxlog.Info("Successfully updated status to success")

	// Requeue periodically to re-validate credentials and refresh datastore list
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
	defer provider.Disconnect()

	// Validate credentials
	if err := provider.ValidateCredentials(ctx); err != nil {
		return errors.Wrap(err, "credential validation failed")
	}

	return nil
}

// discoverDatastores discovers vCenter datastores backed by volumes from this storage array
func (r *ArrayCredsReconciler) discoverDatastores(ctx context.Context, vendorType string, creds vjailbreakv1alpha1.ArrayCredsInfo) ([]string, error) {
	ctxlog := r.Logger
	ctxlog.Info("Discovering datastores", "vendorType", vendorType)
	// Step 1: Get all volume NAAs from the storage array
	// Get the storage provider
	provider, err := storagesdk.NewStorageProvider(vendorType)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get storage provider for vendor type '%s'", vendorType)
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
		return nil, errors.Wrap(err, "failed to connect to storage array")
	}
	defer provider.Disconnect()

	naaIdentifiers, err := provider.GetAllVolumeNAAs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get volume NAAs from storage array")
	}

	if len(naaIdentifiers) == 0 {
		return []string{}, nil
	}

	// Step 2: Get vmware credentials to query datastores
	vmwareCreds, err := r.getVMwareCredentials(ctx)
	if vmwareCreds == nil {
		return nil, errors.New("vmware credentials not found")
	}

	datastoresPresentInarray := []string{}

	for _, vmwareCred := range vmwareCreds.Items {
		_, finder, err := utils.GetFinderForVMwareCreds(ctx, r.Client, &vmwareCred, "")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get finder for vmware credentials")
		}

		// 1. Get all datastores
		datastores, err := finder.DatastoreList(ctx, "*")
		if err != nil {
			return nil, errors.Wrap(err, "failed to list datastores")
		}

		// 2. Get datastore info
		for _, ds := range datastores {
			// 3. Get datastore info
			datastoreInfo, err := getDatastoreInfo(ctx, ds)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get datastore info")
			}
			ctxlog.Info("Datastore info", "datastore", datastoreInfo.Name, "backingNAA", datastoreInfo.BackingNAA)

			// 4. Check if datastore is backed by any of the volume NAAs
			for _, naa := range naaIdentifiers {
				if datastoreInfo.BackingNAA == naa {
					datastoresPresentInarray = append(datastoresPresentInarray, datastoreInfo.Name)
				}
			}

		}
	}
	return datastoresPresentInarray, nil
}

// getDatastoreInfo gets the backing info etc of a datastore.
func getDatastoreInfo(ctx context.Context, ds *object.Datastore) (DatastoreInfo, error) {
	var mds mo.Datastore

	// Retrieve the datastore managed object properties we need
	err := ds.Properties(ctx, ds.Reference(), []string{"summary", "info"}, &mds)
	if err != nil {
		return DatastoreInfo{}, fmt.Errorf("failed to get datastore properties: %w", err)
	}

	info := DatastoreInfo{
		Name:      mds.Summary.Name,
		Type:      mds.Summary.Type,
		Capacity:  mds.Summary.Capacity,
		FreeSpace: mds.Summary.FreeSpace,
	}

	// Extract VMFS or NFS specific backing details
	switch dsInfo := mds.Info.(type) {
	case *types.VmfsDatastoreInfo:
		if dsInfo.Vmfs != nil && len(dsInfo.Vmfs.Extent) > 0 {
			extent := dsInfo.Vmfs.Extent[0]
			info.BackingNAA = extent.DiskName
			info.BackingUUID = dsInfo.Vmfs.Uuid
		}
	}

	return info, nil
}

// getVMwareCredentials retrieves vmware credentials from the secret
func (r *ArrayCredsReconciler) getVMwareCredentials(ctx context.Context) (*vjailbreakv1alpha1.VMwareCredsList, error) {
	// Get vmwarecreds
	vmwareCredsList := &vjailbreakv1alpha1.VMwareCredsList{}
	// get all the vmwarecreds
	if err := r.List(ctx, vmwareCredsList); err != nil {
		return nil, errors.Wrap(err, "failed to list vmware credentials")
	}
	return vmwareCredsList, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *ArrayCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ArrayCreds{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Complete(r)
}
