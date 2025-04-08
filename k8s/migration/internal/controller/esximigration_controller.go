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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ESXIMigration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *ESXIMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ESXIMigrationControllerName)
	ctxlog.Info(fmt.Sprintf("Reconciling ESXIMigration '%s'", req.Name))

	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	if err := r.Get(ctx, req.NamespacedName, esxiMigration); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted esxi migration.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading esxi migration '%s' object", esxiMigration.Name))
		return ctrl.Result{}, err
	}

	scope, err := scope.NewESXIMigrationScope(scope.ESXIMigrationScopeParams{
		Logger:        ctxlog,
		Client:        r.Client,
		ESXIMigration: esxiMigration,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function such that we can persist any ESXIMigration changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !esxiMigration.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, esxiMigration)
	}

	return r.reconcileNormal(ctx, esxiMigration)
}

func (r *ESXIMigrationReconciler) reconcileNormal(ctx context.Context, esxiMigration *vjailbreakv1alpha1.ESXIMigration) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info(fmt.Sprintf("Reconciling ESXIMigration '%s'", esxiMigration.Name))
	controllerutil.AddFinalizer(esxiMigration, constants.ESXIMigrationFinalizer)
	if esxiMigration.Status.Phase == "" {
		esxiMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseWaiting
	} else if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded {
		log.Info("ESXIMigration already succeeded")
		return ctrl.Result{}, nil
	} else if esxiMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseFailed {
		log.Info("ESXIMigration already failed")
		return ctrl.Result{}, nil
	}

	// TODO(vPwned): put esxi cordoning logic here
	log.Info("Cordoned ESXI", "ESXIName", esxiMigration.Spec.ESXIName)

	return ctrl.Result{}, nil
}

func (r *ESXIMigrationReconciler) reconcileDelete(ctx context.Context, esxiMigration *vjailbreakv1alpha1.ESXIMigration) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info(fmt.Sprintf("Reconciling ESXIMigration '%s'", esxiMigration.Name))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ESXIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ESXIMigration{}).
		Complete(r)
}
