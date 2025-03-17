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

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// OpenstackCredsReconciler reconciles a OpenstackCreds object
type OpenstackCredsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenstackCreds object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *OpenstackCredsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info(fmt.Sprintf("Reconciling OpenstackCreds '%s'", req.Name))
	// Get the OpenstackCreds object
	openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{}
	if err := r.Get(ctx, req.NamespacedName, openstackcreds); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted OpenstackCreds.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading OpenstackCreds '%s' object", openstackcreds.Name))
		return ctrl.Result{}, err
	}

	if openstackcreds.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check if speck matches with kubectl.kubernetes.io/last-applied-configuration
		ctxlog.Info(fmt.Sprintf("OpenstackCreds '%s' CR is being created or updated", openstackcreds.Name))
		ctxlog.Info(fmt.Sprintf("Validating OpenstackCreds '%s' object", openstackcreds.Name))
		if _, err := utils.ValidateAndGetProviderClient(ctx, openstackcreds); err != nil {
			// Update the status of the OpenstackCreds object
			ctxlog.Error(err, fmt.Sprintf("Error validating OpenstackCreds '%s'", openstackcreds.Name))
			openstackcreds.Status.OpenStackValidationStatus = "Failed"
			openstackcreds.Status.OpenStackValidationMessage = fmt.Sprintf("Error validating OpenstackCreds '%s'", openstackcreds.Name)
			if err := r.Status().Update(ctx, openstackcreds); err != nil {
				ctxlog.Error(err, fmt.Sprintf("Error updating status of OpenstackCreds '%s'", openstackcreds.Name))
				return ctrl.Result{}, err
			}
		} else {
			err := utils.UpdateMasterNodeImageID(ctx, r.Client, openstackcreds)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					ctxlog.Error(err, "failed to update master node image id and flavor list, skipping reconciliation")
				} else {
					return ctrl.Result{}, errors.Wrap(err, "failed to update master node image id")
				}
			}
			openstackCredential, err := utils.GetOpenstackCredentials(context.TODO(), openstackcreds.Spec.SecretRef.Name)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to get Openstack credentials from secret")
			}

			ctxlog.Info(fmt.Sprintf("Successfully authenticated to Openstack '%s'", openstackCredential.AuthURL))
			// Update the status of the OpenstackCreds object
			openstackcreds.Status.OpenStackValidationStatus = "Succeeded"
			openstackcreds.Status.OpenStackValidationMessage = "Successfully authenticated to Openstack"
			if err := r.Status().Update(ctx, openstackcreds); err != nil {
				ctxlog.Error(err, fmt.Sprintf("Error updating status of OpenstackCreds '%s'", openstackcreds.Name))
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenstackCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.OpenstackCreds{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
