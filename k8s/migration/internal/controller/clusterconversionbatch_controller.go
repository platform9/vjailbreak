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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	batchutils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	"github.com/platform9/vjailbreak/pkg/common/constants"
)

// EligibilityChecker evaluates whether a given ESXi host is ready for conversion.
type EligibilityChecker interface {
	CheckPerHostEligibility(
		ctx context.Context,
		vmwareClient interface{},
		batch *vjailbreakv1alpha1.ClusterConversionBatch,
		esxiName string,
	) (vjailbreakv1alpha1.EligibilityStatus, string, error)
}

// ClusterConversionBatchReconciler reconciles a ClusterConversionBatch object.
type ClusterConversionBatchReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	EligibilityChecker EligibilityChecker
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=clusterconversionbatches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=clusterconversionbatches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=clusterconversionbatches/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esximigrations,verbs=get;list;watch;create;update;patch;delete

const batchRequeueAfter = 30 * time.Second

// Reconcile drives a ClusterConversionBatch toward its desired state.
func (r *ClusterConversionBatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ClusterConversionBatchControllerName)
	ctxlog.Info("reconciling", "batch", req.NamespacedName)

	batch := &vjailbreakv1alpha1.ClusterConversionBatch{}
	if err := r.Get(ctx, req.NamespacedName, batch); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !batch.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, batch)
	}
	return r.reconcileNormal(ctx, batch)
}

// reconcileDelete removes the finalizer without cascading to ESXIMigrations.
func (r *ClusterConversionBatchReconciler) reconcileDelete(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(batch, constants.ClusterConversionBatchFinalizer)
	if err := r.Update(ctx, batch); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileNormal handles steady-state reconciliation.
func (r *ClusterConversionBatchReconciler) reconcileNormal(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ClusterConversionBatchControllerName)
	needsMetadataUpdate := false

	if !controllerutil.ContainsFinalizer(batch, constants.ClusterConversionBatchFinalizer) {
		controllerutil.AddFinalizer(batch, constants.ClusterConversionBatchFinalizer)
		needsMetadataUpdate = true
	}

	// Initialize per-host statuses on first reconcile.
	if len(batch.Status.Hosts) == 0 {
		batch.Status.TotalHosts = len(batch.Spec.Hosts)
		batch.Status.Hosts = make([]vjailbreakv1alpha1.HostConversionStatus, len(batch.Spec.Hosts))
		for i, h := range batch.Spec.Hosts {
			batch.Status.Hosts[i] = vjailbreakv1alpha1.HostConversionStatus{
				ESXiName: h.ESXiName,
				Phase:    vjailbreakv1alpha1.HostConversionPhaseCheckingEligibility,
			}
		}
	}

	// Process operator annotations (trigger/retry/skip). Annotations are removed in-place.
	actions := batchutils.ProcessBatchAnnotations(batch)
	if len(actions) > 0 {
		needsMetadataUpdate = true
	}
	for _, action := range actions {
		if err := r.applyAction(ctx, batch, action); err != nil {
			ctxlog.Error(err, "failed to apply action", "action", action.Type, "host", action.ESXiName)
		}
	}

	// Drive each host through its lifecycle. Errors are isolated per-host (sibling isolation).
	for i := range batch.Status.Hosts {
		if err := r.processHost(ctx, batch, &batch.Status.Hosts[i]); err != nil {
			ctxlog.Error(err, "processHost error (sibling isolated)", "host", batch.Status.Hosts[i].ESXiName)
		}
	}

	updateBatchAggregates(batch)

	// Persist metadata changes (finalizer + cleared annotations).
	// The fake client (and real client with status subresource) overwrites in-memory
	// status with tracker state on r.Update; save and restore it so Status().Update
	// sees the freshly computed status.
	if needsMetadataUpdate {
		savedStatus := batch.Status.DeepCopy()
		if err := r.Update(ctx, batch); err != nil {
			return ctrl.Result{}, err
		}
		batch.Status = *savedStatus
	}

	if err := r.Status().Update(ctx, batch); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: batchRequeueAfter}, nil
}

// applyAction executes a single operator-driven annotation action.
func (r *ClusterConversionBatchReconciler) applyAction(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch, action batchutils.BatchAction) error {
	h := findHostStatus(batch, action.ESXiName)
	if h == nil {
		return nil
	}

	switch action.Type {
	case batchutils.BatchActionTypeTrigger:
		if h.Phase != vjailbreakv1alpha1.HostConversionPhaseReady {
			return nil
		}
		mig, err := batchutils.CreateESXIMigrationForBatch(ctx, r.Client, batch, action.ESXiName)
		if err != nil {
			return err
		}
		now := metav1.Now()
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseConverting
		h.ESXIMigrationRef = &corev1.LocalObjectReference{Name: mig.Name}
		h.StartedAt = &now

	case batchutils.BatchActionTypeRetry:
		h.RetryCount = 0
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseCheckingEligibility
		h.ESXIMigrationRef = nil
		h.NextRetryAt = nil
		h.Message = ""

	case batchutils.BatchActionTypeSkip:
		now := metav1.Now()
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseSkipped
		h.SkippedAt = &now
	}
	return nil
}

// processHost dispatches a host to the correct lifecycle handler based on its current phase.
func (r *ClusterConversionBatchReconciler) processHost(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch, h *vjailbreakv1alpha1.HostConversionStatus) error {
	switch h.Phase {
	case vjailbreakv1alpha1.HostConversionPhaseSucceeded,
		vjailbreakv1alpha1.HostConversionPhaseNeedsAttention,
		vjailbreakv1alpha1.HostConversionPhaseSkipped:
		return nil

	case vjailbreakv1alpha1.HostConversionPhaseCheckingEligibility,
		vjailbreakv1alpha1.HostConversionPhaseNotReady,
		vjailbreakv1alpha1.HostConversionPhaseReady,
		"":
		return r.processEligibilityPhase(ctx, batch, h)

	case vjailbreakv1alpha1.HostConversionPhaseConverting:
		return r.processConvertingPhase(ctx, batch, h)

	case vjailbreakv1alpha1.HostConversionPhaseFailed:
		return r.processFailedPhase(ctx, batch, h)
	}
	return nil
}

// processEligibilityPhase re-evaluates eligibility and optionally auto-starts conversion.
func (r *ClusterConversionBatchReconciler) processEligibilityPhase(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch, h *vjailbreakv1alpha1.HostConversionStatus) error {
	status, reason, err := r.EligibilityChecker.CheckPerHostEligibility(ctx, nil, batch, h.ESXiName)
	if err != nil {
		h.EligibilityStatus = vjailbreakv1alpha1.EligibilityStatusUnknown
		h.EligibilityReason = err.Error()
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseCheckingEligibility
		return nil
	}

	h.EligibilityStatus = status
	h.EligibilityReason = reason

	if status != vjailbreakv1alpha1.EligibilityStatusReady {
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseNotReady
		return nil
	}

	h.Phase = vjailbreakv1alpha1.HostConversionPhaseReady

	if batch.Spec.AutoStart == vjailbreakv1alpha1.AutoStartModeAuto {
		mig, err := batchutils.CreateESXIMigrationForBatch(ctx, r.Client, batch, h.ESXiName)
		if err != nil {
			return err
		}
		now := metav1.Now()
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseConverting
		h.ESXIMigrationRef = &corev1.LocalObjectReference{Name: mig.Name}
		h.StartedAt = &now
	}
	return nil
}

// processConvertingPhase watches the active ESXIMigration and advances on terminal phases.
func (r *ClusterConversionBatchReconciler) processConvertingPhase(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch, h *vjailbreakv1alpha1.HostConversionStatus) error {
	if h.ESXIMigrationRef == nil {
		return nil
	}

	mig := &vjailbreakv1alpha1.ESXIMigration{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      h.ESXIMigrationRef.Name,
		Namespace: batch.Namespace,
	}, mig)
	if err != nil {
		if apierrors.IsNotFound(err) {
			h.Phase = vjailbreakv1alpha1.HostConversionPhaseCheckingEligibility
			h.ESXIMigrationRef = nil
			return nil
		}
		return err
	}

	switch mig.Status.Phase {
	case vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded:
		now := metav1.Now()
		h.Phase = vjailbreakv1alpha1.HostConversionPhaseSucceeded
		h.CompletedAt = &now

	case vjailbreakv1alpha1.ESXIMigrationPhaseFailed:
		if h.RetryCount >= batch.Spec.MaxRetries {
			h.Phase = vjailbreakv1alpha1.HostConversionPhaseNeedsAttention
			h.Message = "max retries exhausted"
		} else {
			backoff := batchutils.ComputeRetryBackoff(batch.Spec.RetryBackoffSeconds, h.RetryCount+1)
			nextRetry := metav1.NewTime(time.Now().Add(backoff))
			h.RetryCount++
			h.NextRetryAt = &nextRetry
			h.Phase = vjailbreakv1alpha1.HostConversionPhaseFailed
		}
	}
	return nil
}

// processFailedPhase retries conversion after the backoff window expires.
func (r *ClusterConversionBatchReconciler) processFailedPhase(ctx context.Context, batch *vjailbreakv1alpha1.ClusterConversionBatch, h *vjailbreakv1alpha1.HostConversionStatus) error {
	if h.NextRetryAt != nil && time.Now().Before(h.NextRetryAt.Time) {
		return nil
	}

	if h.ESXIMigrationRef != nil {
		old := &vjailbreakv1alpha1.ESXIMigration{}
		err := r.Get(ctx, types.NamespacedName{Name: h.ESXIMigrationRef.Name, Namespace: batch.Namespace}, old)
		if err == nil {
			if delErr := r.Delete(ctx, old); delErr != nil && !apierrors.IsNotFound(delErr) {
				return delErr
			}
		}
		h.ESXIMigrationRef = nil
	}

	mig, err := batchutils.CreateESXIMigrationForBatch(ctx, r.Client, batch, h.ESXiName)
	if err != nil {
		return err
	}
	now := metav1.Now()
	h.Phase = vjailbreakv1alpha1.HostConversionPhaseConverting
	h.ESXIMigrationRef = &corev1.LocalObjectReference{Name: mig.Name}
	h.StartedAt = &now
	h.NextRetryAt = nil
	return nil
}

// updateBatchAggregates recomputes batch-level counters and phase from per-host statuses.
// Failed is NOT terminal — batch stays Running while retries remain.
func updateBatchAggregates(batch *vjailbreakv1alpha1.ClusterConversionBatch) {
	var succeeded, needsAttention, skipped, running, pending int
	for _, h := range batch.Status.Hosts {
		switch h.Phase {
		case vjailbreakv1alpha1.HostConversionPhaseSucceeded:
			succeeded++
		case vjailbreakv1alpha1.HostConversionPhaseNeedsAttention:
			needsAttention++
		case vjailbreakv1alpha1.HostConversionPhaseSkipped:
			skipped++
		case vjailbreakv1alpha1.HostConversionPhaseConverting:
			running++
		default:
			pending++
		}
	}

	batch.Status.SucceededHosts = succeeded
	batch.Status.NeedsAttentionHosts = needsAttention
	batch.Status.SkippedHosts = skipped
	batch.Status.RunningHosts = running
	batch.Status.PendingHosts = pending
	batch.Status.TotalHosts = len(batch.Status.Hosts)

	terminal := succeeded + needsAttention + skipped
	total := len(batch.Status.Hosts)

	switch {
	case terminal < total:
		batch.Status.Phase = vjailbreakv1alpha1.ClusterConversionBatchPhaseRunning
	case succeeded == total:
		batch.Status.Phase = vjailbreakv1alpha1.ClusterConversionBatchPhaseSucceeded
	case succeeded > 0:
		batch.Status.Phase = vjailbreakv1alpha1.ClusterConversionBatchPhasePartialFail
	default:
		batch.Status.Phase = vjailbreakv1alpha1.ClusterConversionBatchPhaseFailed
	}
}

// findHostStatus returns a pointer to the HostConversionStatus matching esxiName, or nil.
func findHostStatus(batch *vjailbreakv1alpha1.ClusterConversionBatch, esxiName string) *vjailbreakv1alpha1.HostConversionStatus {
	for i := range batch.Status.Hosts {
		if batch.Status.Hosts[i].ESXiName == esxiName {
			return &batch.Status.Hosts[i]
		}
	}
	return nil
}

// esxiMigrationToBatch maps an ESXIMigration event to the ClusterConversionBatch that owns it.
func (r *ClusterConversionBatchReconciler) esxiMigrationToBatch(_ context.Context, obj client.Object) []reconcile.Request {
	batchName := obj.GetLabels()[constants.ClusterConversionBatchLabel]
	if batchName == "" {
		return nil
	}
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: batchName, Namespace: obj.GetNamespace()}},
	}
}

// SetupWithManager registers the controller and adds an ESXIMigration watch.
func (r *ClusterConversionBatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ClusterConversionBatch{}).
		Watches(
			&vjailbreakv1alpha1.ESXIMigration{},
			handler.EnqueueRequestsFromMapFunc(r.esxiMigrationToBatch),
		).
		Complete(r)
}
