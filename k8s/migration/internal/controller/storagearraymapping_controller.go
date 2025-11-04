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

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type StorageArrayMappingReconciler struct {
	BaseReconciler
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=storagearraymappings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=storagearraymappings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=storagearraymappings/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *StorageArrayMappingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	storagearraymapping := &vjailbreakv1alpha1.StorageArrayMapping{}
	storagearraymapping.Name = req.Name
	storagearraymapping.Namespace = req.Namespace

	if err := r.Get(ctx, req.NamespacedName, storagearraymapping); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted storagearraymapping.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading storagearraymapping '%s' object", storagearraymapping.Name))
		return ctrl.Result{}, err
	}
	if storagearraymapping.DeletionTimestamp.IsZero() {
		ctxlog.Info(fmt.Sprintf("Reconciling storagearraymapping '%s'", storagearraymapping.Name))
		return r.ReconcileMapping(ctx, storagearraymapping, func() error {
			// Validate the storage array mapping
			validationErrors := r.validateStorageArrayMapping(storagearraymapping)

			if len(validationErrors) > 0 {
				storagearraymapping.Status.ValidationStatus = "Invalid"
				storagearraymapping.Status.ValidationMessage = strings.Join(validationErrors, "; ")
			} else {
				storagearraymapping.Status.ValidationStatus = "Valid"
				storagearraymapping.Status.ValidationMessage = "Storage array mapping is valid"
			}

			return r.Status().Update(ctx, storagearraymapping)
		})
	}
	return ctrl.Result{}, nil
}

// validateStorageArrayMapping performs validation checks on the StorageArrayMapping
func (r *StorageArrayMappingReconciler) validateStorageArrayMapping(mapping *vjailbreakv1alpha1.StorageArrayMapping) []string {
	var errors []string

	// Create a map of array names for quick lookup
	arrayMap := make(map[string]*vjailbreakv1alpha1.StorageArray)
	for i := range mapping.Spec.Arrays {
		array := &mapping.Spec.Arrays[i]

		// Check for duplicate array names
		if _, exists := arrayMap[array.Name]; exists {
			errors = append(errors, fmt.Sprintf("duplicate array name: %s", array.Name))
		} else {
			arrayMap[array.Name] = array
		}

		// Validate array configuration
		if array.Name == "" {
			errors = append(errors, "array name cannot be empty")
		}
		if array.Type == "" {
			errors = append(errors, fmt.Sprintf("array type cannot be empty for array: %s", array.Name))
		} else if array.Type != "pure" && array.Type != "netapp" {
			errors = append(errors, fmt.Sprintf("unsupported array type '%s' for array: %s (supported: pure, netapp)", array.Type, array.Name))
		}
		if array.ManagementEndpoint == "" {
			errors = append(errors, fmt.Sprintf("management endpoint cannot be empty for array: %s", array.Name))
		}
		if array.CredentialsSecret == "" {
			errors = append(errors, fmt.Sprintf("credentials secret cannot be empty for array: %s", array.Name))
		}
	}

	// Check for duplicate datastore names
	datastoreMap := make(map[string]bool)
	for _, datastore := range mapping.Spec.Datastores {
		if datastore.Name == "" {
			errors = append(errors, "datastore name cannot be empty")
			continue
		}

		if datastoreMap[datastore.Name] {
			errors = append(errors, fmt.Sprintf("duplicate datastore name: %s", datastore.Name))
		} else {
			datastoreMap[datastore.Name] = true
		}

		// Validate that referenced array exists
		if datastore.ArrayName == "" {
			errors = append(errors, fmt.Sprintf("array name cannot be empty for datastore: %s", datastore.Name))
		} else if _, exists := arrayMap[datastore.ArrayName]; !exists {
			errors = append(errors, fmt.Sprintf("datastore '%s' references undefined array: %s", datastore.Name, datastore.ArrayName))
		}

		// Validate array type matches
		if datastore.ArrayType == "" {
			errors = append(errors, fmt.Sprintf("array type cannot be empty for datastore: %s", datastore.Name))
		} else if array, exists := arrayMap[datastore.ArrayName]; exists && array.Type != datastore.ArrayType {
			errors = append(errors, fmt.Sprintf("datastore '%s' array type '%s' does not match array '%s' type '%s'",
				datastore.Name, datastore.ArrayType, datastore.ArrayName, array.Type))
		}
	}

	// Check that we have at least one datastore mapping
	if len(mapping.Spec.Datastores) == 0 {
		errors = append(errors, "at least one datastore mapping is required")
	}

	// Check that we have at least one array
	if len(mapping.Spec.Arrays) == 0 {
		errors = append(errors, "at least one storage array is required")
	}

	return errors
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageArrayMappingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.StorageArrayMapping{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
