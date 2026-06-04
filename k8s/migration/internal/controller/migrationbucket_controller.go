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

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// MigrationBucketReconciler reconciles a MigrationBucket object. It keeps the bucket's status
// phase defaulted and surfaces invariant violations (e.g. an empty bucket) in the status.
// It does NOT modify the existing Migration/MigrationPlan workflow; at trigger time buckets are
// compiled into those existing objects elsewhere.
type MigrationBucketReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationbuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationbuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationbuckets/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwaremachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds,verbs=get;list;watch

// Reconcile keeps a MigrationBucket's status consistent with its spec.
func (r *MigrationBucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx).WithName(constants.MigrationBucketControllerName)

	bucket := &vjailbreakv1alpha1.MigrationBucket{}
	if err := r.Get(ctx, req.NamespacedName, bucket); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Default the phase, and surface the "no empty bucket" invariant (FR-012) in the status.
	desiredPhase := bucket.Status.Phase
	if desiredPhase == "" {
		desiredPhase = vjailbreakv1alpha1.MigrationBucketPhaseNotMigrated
	}

	message := ""
	if len(bucket.Spec.VMs) == 0 {
		message = "bucket has no VMs; a bucket must contain at least one VM"
	}

	if bucket.Status.Phase != desiredPhase || bucket.Status.Message != message {
		bucket.Status.Phase = desiredPhase
		bucket.Status.Message = message
		if err := r.Status().Update(ctx, bucket); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update MigrationBucket status")
		}
	}

	ctxlog.Info("Reconciled MigrationBucket", "name", bucket.Name, "vmCount", len(bucket.Spec.VMs))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationBucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.MigrationBucket{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		)).
		Complete(r)
}
