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
	constants "github.com/platform9/vjailbreak/pkg/common/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	storagesdk "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	// Import storage providers to register them
	netappsdk "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/netapp"
	_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage/providers"

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

	// Check if secretRef is provided - if not, this is an auto-discovered array awaiting credentials
	if arraycreds.Spec.SecretRef.Name == "" {
		ctxlog.Info("ArrayCreds is awaiting credentials (no secretRef)", "arraycreds", arraycreds.Name)

		// Update status to indicate waiting for credentials
		if scope.ArrayCreds.Status.Phase != constants.ArrayCredsPhaseDiscovered {
			scope.ArrayCreds.Status.Phase = constants.ArrayCredsPhaseDiscovered
			scope.ArrayCreds.Status.ArrayValidationStatus = constants.ArrayCredsStatusAwaitingCredentials
			scope.ArrayCreds.Status.ArrayValidationMessage = "Array discovered from OpenStack. Awaiting storage array credentials."

			if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
				// If the resource was deleted during reconciliation, ignore the error
				if apierrors.IsNotFound(err) {
					ctxlog.Info("ArrayCreds was deleted during reconciliation, skipping status update", "arraycreds", scope.ArrayCreds.Name)
					return ctrl.Result{}, nil
				}
				ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
				return ctrl.Result{}, err
			}
		}

		// Don't requeue - wait for user to add secretRef
		return ctrl.Result{}, nil
	}

	// Get credentials from secret
	arrayCredential, err := utils.GetArrayCredentialsFromSecret(ctx, r.Client, arraycreds.Spec.SecretRef.Name)
	if err != nil {
		ctxlog.Error(err, "Failed to get storage array credentials from secret", "secretName", arraycreds.Spec.SecretRef.Name)
		scope.ArrayCreds.Status.Phase = constants.ArrayCredsPhaseFailed
		scope.ArrayCreds.Status.ArrayValidationStatus = constants.ArrayCredsStatusFailed
		scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Failed to get credentials from secret: %v", err)
		if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
			// If the resource was deleted during reconciliation, ignore the error
			if apierrors.IsNotFound(err) {
				ctxlog.Info("ArrayCreds was deleted during reconciliation, skipping status update", "arraycreds", scope.ArrayCreds.Name)
				return ctrl.Result{}, nil
			}
			ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Build vendor-specific provider options from the spec (e.g., NetApp SVM/FlexVol).
	providerOptions := buildProviderOptionsFromSpec(arraycreds.Spec)

	// Validate credentials using storage SDK
	if err := r.validateArrayCredentials(ctx, arraycreds.Spec.VendorType, arrayCredential, providerOptions); err != nil {
		ctxlog.Error(err, "Error validating ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
		scope.ArrayCreds.Status.Phase = constants.ArrayCredsPhaseFailed
		scope.ArrayCreds.Status.ArrayValidationStatus = constants.ArrayCredsStatusFailed
		scope.ArrayCreds.Status.ArrayValidationMessage = fmt.Sprintf("Validation failed: %v", err)
		ctxlog.Info("Updating status to failed", "arraycreds", scope.ArrayCreds.Name, "message", err.Error())
		if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
			// If the resource was deleted during reconciliation, ignore the error
			if apierrors.IsNotFound(err) {
				ctxlog.Info("ArrayCreds was deleted during reconciliation, skipping status update", "arraycreds", scope.ArrayCreds.Name)
				return ctrl.Result{}, nil
			}
			ctxlog.Error(err, "Error updating status of ArrayCreds", "arraycreds", scope.ArrayCreds.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Successfully validated - now discover datastores
	ctxlog.Info("Successfully authenticated to storage array", "vendor", arraycreds.Spec.VendorType, "hostname", arrayCredential.Hostname)

	// Discover datastores backed by this array
	datastores, err := r.discoverDatastores(ctx, arraycreds.Spec.VendorType, arrayCredential, scope)
	if err != nil {
		ctxlog.Error(err, "Failed to discover datastores", "arraycreds", scope.ArrayCreds.Name)
		// Don't fail validation, just log warning
		scope.ArrayCreds.Status.DataStore = []vjailbreakv1alpha1.DatastoreInfo{}
	} else {
		ctxlog.Info("Discovered datastores", "count", len(datastores), "datastores", datastores)
		scope.ArrayCreds.Status.DataStore = datastores
	}

	// Discover selectable backend targets (SVMs/FlexVols for NetApp). Pure
	// and other providers without a target hierarchy will return (nil, nil).
	backendTargets, err := r.discoverBackendTargets(ctx, arraycreds.Spec.VendorType, arrayCredential, providerOptions)
	if err != nil {
		// Non-fatal: log and clear targets so UI shows empty state rather than stale data.
		ctxlog.Error(err, "Failed to discover backend targets", "arraycreds", scope.ArrayCreds.Name)
		scope.ArrayCreds.Status.BackendTargets = nil
	} else {
		scope.ArrayCreds.Status.BackendTargets = backendTargets
	}

	// For NetApp specifically: verify the user's SVM/FlexVol selection (if any)
	// exists on the array, and flag when a selection is still required.
	phase := constants.ArrayCredsPhaseValidated
	validationStatus := constants.ArrayCredsStatusSucceeded
	validationMessage := fmt.Sprintf("Successfully authenticated to %s storage array. Discovered %d datastores.", arraycreds.Spec.VendorType, len(datastores))

	if arraycreds.Spec.VendorType == "netapp" {
		cfg := arraycreds.Spec.NetAppConfig
		hasSelection := cfg != nil && cfg.SVM != "" && cfg.FlexVol != ""
		if hasSelection {
			if err := validateNetAppTargetSelection(cfg, backendTargets); err != nil {
				ctxlog.Error(err, "NetApp SVM/FlexVol selection invalid", "arraycreds", scope.ArrayCreds.Name)
				phase = constants.ArrayCredsPhaseFailed
				validationStatus = constants.ArrayCredsStatusFailed
				validationMessage = fmt.Sprintf("Invalid NetApp target selection: %v", err)
			}
		} else {
			phase = constants.ArrayCredsPhaseNeedsBackendSelection
			validationMessage = fmt.Sprintf(
				"Credentials validated. Select a NetApp SVM and FlexVol from %d discovered SVM(s) to complete setup.",
				len(backendTargets),
			)
		}
	}

	scope.ArrayCreds.Status.Phase = phase
	scope.ArrayCreds.Status.ArrayValidationStatus = validationStatus
	scope.ArrayCreds.Status.ArrayValidationMessage = validationMessage
	ctxlog.Info("Updating status to success", "arraycreds", scope.ArrayCreds.Name)
	if err := r.Status().Update(ctx, scope.ArrayCreds); err != nil {
		// If the resource was deleted during reconciliation, ignore the error
		if apierrors.IsNotFound(err) {
			ctxlog.Info("ArrayCreds was deleted during reconciliation, skipping status update", "arraycreds", scope.ArrayCreds.Name)
			return ctrl.Result{}, nil
		}
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

// validateArrayCredentials validates storage array credentials using the storage SDK.
// providerOptions carries vendor-specific configuration (e.g., NetApp SVM/FlexVol);
// pass nil when no extra options apply.
func (r *ArrayCredsReconciler) validateArrayCredentials(ctx context.Context, vendorType string, creds vjailbreakv1alpha1.ArrayCredsInfo, providerOptions map[string]string) error {
	ctxlog := log.FromContext(ctx)

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
		ProviderOptions:     providerOptions,
	}

	// Connect to storage array
	if err := provider.Connect(ctx, accessInfo); err != nil {
		return errors.Wrap(err, "failed to connect to storage array")
	}
	defer func() {
		if err := provider.Disconnect(); err != nil {
			ctxlog.Error(err, "failed to disconnect from storage array")
		}
	}()

	// Validate credentials
	if err := provider.ValidateCredentials(ctx); err != nil {
		return errors.Wrap(err, "credential validation failed")
	}

	return nil
}

// buildProviderOptionsFromSpec translates a CRD spec into the generic
// ProviderOptions map consumed by the storage SDK. This is the one place
// where per-vendor CRD fields are projected into runtime options; new
// vendors add their case here.
func buildProviderOptionsFromSpec(spec vjailbreakv1alpha1.ArrayCredsSpec) map[string]string {
	opts := map[string]string{}
	switch spec.VendorType {
	case "netapp":
		if spec.NetAppConfig != nil {
			if spec.NetAppConfig.SVM != "" {
				opts[netappsdk.OptionSVM] = spec.NetAppConfig.SVM
			}
			if spec.NetAppConfig.FlexVol != "" {
				opts[netappsdk.OptionFlexVol] = spec.NetAppConfig.FlexVol
			}
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// discoverBackendTargets queries the array for its selectable target hierarchy
// (e.g., NetApp SVMs -> FlexVols). Returns nil when the provider does not
// implement BackendTargetDiscoverer (e.g., Pure today).
func (r *ArrayCredsReconciler) discoverBackendTargets(ctx context.Context, vendorType string, creds vjailbreakv1alpha1.ArrayCredsInfo, providerOptions map[string]string) ([]vjailbreakv1alpha1.BackendTargetGroup, error) {
	ctxlog := log.FromContext(ctx)

	provider, err := storagesdk.NewStorageProvider(vendorType)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get storage provider for vendor type '%s'", vendorType)
	}

	discoverer, ok := provider.(storagesdk.BackendTargetDiscoverer)
	if !ok {
		return nil, nil
	}

	accessInfo := storagesdk.StorageAccessInfo{
		Hostname:            creds.Hostname,
		Username:            creds.Username,
		Password:            creds.Password,
		SkipSSLVerification: creds.SkipSSLVerification,
		VendorType:          vendorType,
		ProviderOptions:     providerOptions,
	}
	if err := provider.Connect(ctx, accessInfo); err != nil {
		return nil, errors.Wrap(err, "failed to connect to storage array")
	}
	defer func() {
		if err := provider.Disconnect(); err != nil {
			ctxlog.Error(err, "failed to disconnect from storage array")
		}
	}()

	groups, err := discoverer.DiscoverBackendTargets(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]vjailbreakv1alpha1.BackendTargetGroup, 0, len(groups))
	for _, g := range groups {
		children := make([]vjailbreakv1alpha1.BackendTarget, 0, len(g.Children))
		for _, c := range g.Children {
			children = append(children, vjailbreakv1alpha1.BackendTarget{Name: c.Name, UUID: c.UUID})
		}
		out = append(out, vjailbreakv1alpha1.BackendTargetGroup{
			Name:     g.Name,
			UUID:     g.UUID,
			Children: children,
		})
	}
	return out, nil
}

// validateNetAppTargetSelection checks that the SVM and FlexVol named in
// spec.NetAppConfig exist in the discovered target tree. Returns a
// user-facing error when the selection is invalid.
func validateNetAppTargetSelection(cfg *vjailbreakv1alpha1.NetAppConfig, targets []vjailbreakv1alpha1.BackendTargetGroup) error {
	if cfg == nil || cfg.SVM == "" || cfg.FlexVol == "" {
		return nil
	}
	for _, g := range targets {
		if g.Name != cfg.SVM {
			continue
		}
		for _, c := range g.Children {
			if c.Name == cfg.FlexVol {
				return nil
			}
		}
		return fmt.Errorf("FlexVol %q not found on SVM %q", cfg.FlexVol, cfg.SVM)
	}
	return fmt.Errorf("SVM %q not found on the NetApp array", cfg.SVM)
}

// discoverDatastores discovers vCenter datastores backed by volumes from this storage array
func (r *ArrayCredsReconciler) discoverDatastores(ctx context.Context, vendorType string, creds vjailbreakv1alpha1.ArrayCredsInfo, scope *scope.ArrayCredsScope) ([]vjailbreakv1alpha1.DatastoreInfo, error) {
	ctxlog := scope.Logger
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
	defer func() {
		if err := provider.Disconnect(); err != nil {
			ctxlog.Error(err, "failed to disconnect from storage array")
		}
	}()

	naaIdentifiers, err := provider.GetAllVolumeNAAs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get volume NAAs from storage array")
	}

	if len(naaIdentifiers) == 0 {
		return []vjailbreakv1alpha1.DatastoreInfo{}, nil
	}
	ctxlog.Info("Found volume NAAs", "naaIdentifiers", naaIdentifiers)

	// Step 2: Get vmware credentials to query datastores
	vmwareCreds, err := r.getVMwareCredentials(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vmware credentials")
	}
	if vmwareCreds == nil {
		return nil, errors.New("vmware credentials not found")
	}

	datastoresPresentInarray := []vjailbreakv1alpha1.DatastoreInfo{}

	for _, vmwareCred := range vmwareCreds.Items {
		// Get VMware credentials from secret to extract datacenter
		vmwareCredsInfo, err := utils.GetVMwareCredentialsFromSecret(ctx, r.Client, vmwareCred.Spec.SecretRef.Name)
		if err != nil {
			ctxlog.Error(err, "Failed to get VMware credentials from secret", "secretName", vmwareCred.Spec.SecretRef.Name)
			continue // Skip this VMware credential and try the next one
		}

		// If datacenter is not specified, query all datacenters
		if vmwareCredsInfo.Datacenter == "" {
			vmwareCredsInfo.Datacenter = "*"
		}

		_, finder, err := utils.GetFinderForVMwareCreds(ctx, r.Client, &vmwareCred, vmwareCredsInfo.Datacenter)
		if err != nil {
			ctxlog.Error(err, "Failed to get finder for vmware credentials", "datacenter", vmwareCredsInfo.Datacenter)
			continue // Skip this VMware credential and try the next one
		}

		// 1. Get all datastores
		datastores, err := finder.DatastoreList(ctx, "*")
		if err != nil {
			ctxlog.Error(err, "Failed to list datastores", "datacenter", vmwareCredsInfo.Datacenter)
			continue // Skip this VMware credential and try the next one
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
					// Check if datastore is already present in the list
					if !utils.Contains(datastoresPresentInarray, datastoreInfo) {
						datastoresPresentInarray = append(datastoresPresentInarray, datastoreInfo)
					}
				}
			}
		}
	}
	return datastoresPresentInarray, nil
}

// getDatastoreInfo gets the backing info etc of a datastore.
func getDatastoreInfo(ctx context.Context, ds *object.Datastore) (vjailbreakv1alpha1.DatastoreInfo, error) {
	var mds mo.Datastore

	// Retrieve the datastore managed object properties we need
	err := ds.Properties(ctx, ds.Reference(), []string{"summary", "info"}, &mds)
	if err != nil {
		return vjailbreakv1alpha1.DatastoreInfo{}, fmt.Errorf("failed to get datastore properties: %w", err)
	}

	info := vjailbreakv1alpha1.DatastoreInfo{
		Name:      mds.Summary.Name,
		Type:      mds.Summary.Type,
		Capacity:  mds.Summary.Capacity,
		FreeSpace: mds.Summary.FreeSpace,
	}

	// Extract VMFS or NFS specific backing details
	if dsInfo, ok := mds.Info.(*types.VmfsDatastoreInfo); ok {
		if dsInfo.Vmfs != nil && len(dsInfo.Vmfs.Extent) > 0 {
			extent := dsInfo.Vmfs.Extent[0]
			info.BackingNAA = extent.DiskName
			info.BackingUUID = dsInfo.Vmfs.Uuid
			info.MoID = ds.Reference().Value
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
