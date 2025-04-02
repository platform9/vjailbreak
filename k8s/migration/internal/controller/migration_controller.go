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
	"slices"
	"sort"
	"strings"
	"time"

	openstackconst "github.com/platform9/vjailbreak/v2v-helper/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
)

// MigrationReconciler reconciles a Migration object
type MigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles a Migration object
func (r *MigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.MigrationControllerName)

	ctxlog.Info("Reconciling Migration object")
	migration := &vjailbreakv1alpha1.Migration{}

	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading Migration '%s' object", migration.Name))
		return ctrl.Result{}, err
	}
	migrationScope, err := scope.NewMigrationScope(scope.MigrationScopeParams{
		Logger:    ctxlog,
		Client:    r.Client,
		Migration: migration,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Get the pod phase
	pod, err := r.GetPod(ctx, migrationScope)
	if err != nil {
		return ctrl.Result{}, err
	}

	pod.Labels["startCutover"] = utils.SetCutoverLabel(migration.Spec.InitiateCutover, pod.Labels["startCutover"])
	if err = r.Update(ctx, pod); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to update Pod '%s'", pod.Name))
		return ctrl.Result{}, err
	}

	if constants.StatesEnum[migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseValidating] {
		migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseValidating
	}
	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
		return ctrl.Result{}, fmt.Errorf("pod is not Running nor Succeeded for migration %s", migration.Name)
	}

	filteredEvents, err := r.GetEventsSorted(ctx, migrationScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed getting pod events")
	}

	// Create status conditions
	migration.Status.Conditions = utils.CreateValidatedCondition(migration, filteredEvents)
	migration.Status.Conditions = utils.CreateDataCopyCondition(migration, filteredEvents)
	migration.Status.Conditions = utils.CreateMigratingCondition(migration, filteredEvents)

	migration.Status.AgentName = pod.Spec.NodeName

	err = r.SetupMigrationPhase(ctx, migrationScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error setting migration phase")
	}

	if err := r.Status().Update(ctx, migration); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to update status of Migration '%s'", migration.Name))
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function such that we can persist any Migration changes.
	defer func() {
		if err := migrationScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if string(pod.Status.Phase) != string(corev1.PodSucceeded) {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.Migration{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}, builder.WithPredicates(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldpod, ok := e.ObjectOld.(*corev1.Pod)
					if !ok {
						return false
					}
					newpod, ok := e.ObjectNew.(*corev1.Pod)
					if !ok {
						return false
					}
					for _, condition := range newpod.Status.Conditions {
						// Ignores the disk percentage updates in the pod custom conditions
						if condition.Type == "Progressing" && !strings.Contains(condition.Message, "Progress:") {
							return true
						}
					}
					return oldpod.Status.Phase != newpod.Status.Phase
				},
			},
		)).
		Complete(r)
}

// SetupMigrationPhase sets up the migration phase based on current state
func (r *MigrationReconciler) SetupMigrationPhase(ctx context.Context, scope *scope.MigrationScope) error {
	events, err := r.GetEventsSorted(ctx, scope)
	if err != nil {
		return err
	}
	IgnoredPhases := []vjailbreakv1alpha1.MigrationPhase{
		vjailbreakv1alpha1.MigrationPhaseValidating,
		vjailbreakv1alpha1.MigrationPhasePending}

loop:
	for i := range events.Items {
		switch {
		// In reverse order, because the events are sorted by timestamp latest to oldest
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageMigrationSucessful) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseSucceeded]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseSucceeded
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageWaitingForAdminCutOver) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseAwaitingAdminCutOver]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseAwaitingAdminCutOver
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageWaitingForCutOverStart) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseAwaitingCutOverStartTime]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseAwaitingCutOverStartTime
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageConvertingDisk) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseConvertingDisk]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseConvertingDisk
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCopyingChangedBlocksWithIteration) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseCopyingChangedBlocks]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseCopyingChangedBlocks
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCopyingDisk) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseCopying]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseCopying
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageWaitingForDataCopyStart) &&
			constants.StatesEnum[scope.Migration.Status.Phase] <= constants.StatesEnum[vjailbreakv1alpha1.MigrationPhaseAwaitingDataCopyStart]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseAwaitingDataCopyStart
			break loop
		case strings.Contains(strings.TrimSpace(events.Items[i].Message), openstackconst.EventMessageMigrationFailed):
			scope.Migration.Status.Phase = vjailbreakv1alpha1.MigrationPhaseFailed
			break loop
			// If none of the above phases matched
		case slices.Contains(IgnoredPhases, scope.Migration.Status.Phase):
			break loop
		default:
			continue
		}
	}
	return nil
}

// GetEventsSorted retrieves sorted events for a migration
func (r *MigrationReconciler) GetEventsSorted(ctx context.Context, scope *scope.MigrationScope) (*corev1.EventList, error) {
	migration := scope.Migration
	ctxlog := scope.Logger

	pod, err := r.GetPod(ctx, scope)
	if err != nil {
		return nil, err
	}

	// Get events of this pod
	allevents := &corev1.EventList{}
	if err := r.List(ctx, allevents, client.InNamespace(migration.Namespace)); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to get events for Pod '%s'", migration.Spec.PodRef))
		return nil, err
	}

	filteredEvents := &corev1.EventList{}
	for i := 0; i < len(allevents.Items); i++ {
		if allevents.Items[i].InvolvedObject.Name == migration.Spec.PodRef && string(allevents.Items[i].InvolvedObject.UID) == string(pod.UID) {
			filteredEvents.Items = append(filteredEvents.Items, allevents.Items[i])
		}
	}

	// Sort filteredEvents by creation timestamp
	sort.Slice(filteredEvents.Items, func(i, j int) bool {
		return !filteredEvents.Items[i].CreationTimestamp.Before(&filteredEvents.Items[j].CreationTimestamp)
	})

	return filteredEvents, nil
}

// GetPod retrieves the pod associated with a migration
func (r *MigrationReconciler) GetPod(ctx context.Context, scope *scope.MigrationScope) (*corev1.Pod, error) {
	migration := scope.Migration
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(migration.Namespace),
		client.MatchingLabels(map[string]string{"vm-name": migration.Spec.VMName})); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get pod with label '%s'", migration.Spec.VMName))
	}
	if len(podList.Items) == 0 {
		return nil, errors.New("migration pod not found")
	}
	scope.Migration.Spec.PodRef = podList.Items[0].Name
	return &podList.Items[0], nil
}
