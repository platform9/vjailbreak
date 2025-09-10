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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

const migrationFinalizer = "migration.vjailbreak.k8s.pf9.io/finalizer"

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwaremachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwaremachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=bmconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=bmconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=bmconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles a Migration object
func (r *MigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.MigrationControllerName)

	migration := &vjailbreakv1alpha1.Migration{}

	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if apierrors.IsNotFound(err) {
			// Object deleted successfully
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

	defer func() {
		if err := migrationScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deletion reconciliation
	if !migration.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(migration, migrationFinalizer) {
			if err := r.reconcileDelete(ctx, migration); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(migration, migrationFinalizer)
			if err := r.Update(ctx, migration); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Adding finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(migration, migrationFinalizer) {
		controllerutil.AddFinalizer(migration, migrationFinalizer)
		if err := r.Update(ctx, migration); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	ctxlog.Info("Reconciling Migration object")

	// Get the pod phase
	pod, err := r.GetPod(ctx, migrationScope)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Migration pod not found yet, requeuing", "migration", migration.Name)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	ctxlog.Info("Updating migration spec podref", "migration", migration.Name, "podRef", migration.Spec.PodRef)
	if migration.Spec.PodRef != pod.Name {
		migration.Spec.PodRef = pod.Name
		if err := r.Update(ctx, migration); err != nil {
			return ctrl.Result{}, err
		}
	}

	pod.Labels["startCutover"] = utils.SetCutoverLabel(migration.Spec.InitiateCutover, pod.Labels["startCutover"])
	if err = r.Update(ctx, pod); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to update Pod '%s'", pod.Name))
		return ctrl.Result{}, err
	}

	if constants.VMMigrationStatesEnum[migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseValidating] {
		migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseValidating
	}
	// Check if the pod is in a valid state only then continue
	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
		return ctrl.Result{}, fmt.Errorf("pod is not Running, Failed nor Succeeded for migration %s", migration.Name)
	}

	filteredEvents, err := r.GetEventsSorted(ctx, migrationScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed getting pod events")
	}
	// Create status conditions
	migration.Status.Conditions = utils.CreateValidatedCondition(migration, filteredEvents)
	migration.Status.Conditions = utils.CreateDataCopyCondition(migration, filteredEvents)
	migration.Status.Conditions = utils.CreateMigratingCondition(migration, filteredEvents)
	migration.Status.Conditions = utils.CreateFailedCondition(migration, filteredEvents)

	migration.Status.AgentName = pod.Spec.NodeName
	err = r.SetupMigrationPhase(ctx, migrationScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error setting migration phase")
	}
	if err := r.Status().Update(ctx, migration); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to update status of Migration '%s'", migration.Name))
		return ctrl.Result{}, err
	}

	if string(migration.Status.Phase) != string(vjailbreakv1alpha1.VMMigrationPhaseFailed) &&
		string(migration.Status.Phase) != string(vjailbreakv1alpha1.VMMigrationPhaseSucceeded) {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles the cleanup logic when Migration object is deleted.
func (r *MigrationReconciler) reconcileDelete(ctx context.Context, migration *vjailbreakv1alpha1.Migration) error {
	ctxlog := log.FromContext(ctx).WithName(constants.MigrationControllerName)
	ctxlog.Info("Reconciling deletion of Migration, resetting VMwareMachine status", "MigrationName", migration.Name)

	if migration.Spec.VMName == "" {
		ctxlog.Info("VMName is empty in Migration spec, nothing to do.")
		return nil
	}

	vmwareCredsName, err := utils.GetVMwareCredsNameFromMigration(ctx, r.Client, migration)
	if err != nil {
		ctxlog.Error(err, "Failed to get VMware credentials name for migration")
		return nil
	}

	// Then use it to get the k8s compatible name
	vmwMachineName, err := utils.GetK8sCompatibleVMWareObjectName(migration.Spec.VMName, vmwareCredsName)
	if err != nil {
		ctxlog.Error(err, "Could not determine VMwareMachine name from VM name", "VMName", migration.Spec.VMName)
		return nil
	}

	vmwMachine := &vjailbreakv1alpha1.VMwareMachine{}
	err = r.Get(ctx, types.NamespacedName{Name: vmwMachineName, Namespace: migration.GetNamespace()}, vmwMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("VMwareMachine not found during migration deletion, nothing to do.", "VMwareMachineName", vmwMachineName)
			return nil
		}
		return errors.Wrap(err, "failed to get VMwareMachine for cleanup")
	}

	if vmwMachine.Status.Migrated {
		ctxlog.Info("Setting VMwareMachine status.migrated to false", "VMwareMachineName", vmwMachineName)
		vmwMachine.Status.Migrated = false
		if err := r.Status().Update(ctx, vmwMachine); err != nil {
			return errors.Wrap(err, "failed to update VMwareMachine status")
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.Migration{}).
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
//
//nolint:gocyclo
func (r *MigrationReconciler) SetupMigrationPhase(ctx context.Context, scope *scope.MigrationScope) error {
	events, err := r.GetEventsSorted(ctx, scope)
	if err != nil {
		return err
	}

	// Get the pod to check startCutover label
	pod, err := r.GetPod(ctx, scope)
	if err != nil {
		return err
	}

	IgnoredPhases := []vjailbreakv1alpha1.VMMigrationPhase{
		vjailbreakv1alpha1.VMMigrationPhaseValidating,
		vjailbreakv1alpha1.VMMigrationPhasePending}

loop:
	for i := range events.Items {
		switch {
		// In reverse order, because the events are sorted by timestamp latest to oldest
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageMigrationSucessful) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseSucceeded]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseSucceeded
			if err := r.markMigrationSuccessful(ctx, scope); err != nil {
				return err
			}
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCreatingVM) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseCreatingVM]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseCreatingVM
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageWaitingForAdminCutOver) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseAwaitingAdminCutOver]:
			// Only stay in AwaitingAdminCutOver if cutover hasn't been triggered yet
			if pod.Labels["startCutover"] != "yes" {
				scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseAwaitingAdminCutOver
				break loop
			}
			// Admin cutover was triggered, reset to a lower phase so it can progress normally
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseCopying

			// If startCutover is "yes", don't set phase here - let it progress to next phase
			// by continuing to check other events
			continue
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageWaitingForCutOverStart) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseAwaitingCutOverStartTime]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseAwaitingCutOverStartTime
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageConvertingDisk) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCopyingChangedBlocksWithIteration) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseCopyingChangedBlocks]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseCopyingChangedBlocks
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCopyingDisk) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseCopying]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseCopying
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageWaitingForDataCopyStart) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseAwaitingDataCopyStart]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseAwaitingDataCopyStart
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCreatingVolumes) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseCreatingVolumes]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseCreatingVolumes
			break loop
		case strings.Contains(events.Items[i].Message, openstackconst.EventMessageCreatingPorts) &&
			constants.VMMigrationStatesEnum[scope.Migration.Status.Phase] <= constants.VMMigrationStatesEnum[vjailbreakv1alpha1.VMMigrationPhaseCreatingPorts]:
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseCreatingPorts
			break loop
		case strings.Contains(strings.TrimSpace(events.Items[i].Message), openstackconst.EventMessageMigrationFailed) ||
			strings.Contains(strings.TrimSpace(events.Items[i].Message), openstackconst.EventMessageFailed):
			scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseFailed
			break loop
		case slices.Contains(IgnoredPhases, scope.Migration.Status.Phase):
			break loop
		default:
			continue
		}
	}
	return nil
}

// Extracted function to handle successful migration updates
func (r *MigrationReconciler) markMigrationSuccessful(ctx context.Context, scope *scope.MigrationScope) error {
	scope.Migration.Status.Phase = vjailbreakv1alpha1.VMMigrationPhaseSucceeded
	vmwareCredsName, err := utils.GetVMwareCredsNameFromMigration(ctx, r.Client, scope.Migration)
	if err != nil {
		return errors.Wrap(err, "failed to get vmware credentials name")
	}
	name, err := utils.GetK8sCompatibleVMWareObjectName(scope.Migration.Spec.VMName, vmwareCredsName)
	if err != nil {
		return errors.Wrap(err, "failed to get vmware machine name")
	}

	vmwvm := &vjailbreakv1alpha1.VMwareMachine{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: scope.Migration.Namespace}, vmwvm); err != nil {
		return errors.Wrap(err, "failed to get vmware machine")
	}

	vmwvm.Status.Migrated = true
	return r.Status().Update(ctx, vmwvm)
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
	vmwareCredsName, err := utils.GetVMwareCredsNameFromMigration(ctx, r.Client, scope.Migration)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vmware credentials name")
	}
	vmname, err := utils.GetK8sCompatibleVMWareObjectName(migration.Spec.VMName, vmwareCredsName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vm name")
	}
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(migration.Namespace),
		client.MatchingLabels(map[string]string{constants.VMNameLabel: vmname})); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get pod with label '%s=%s'", constants.VMNameLabel, vmname))
	}
	if len(podList.Items) == 0 {
		return nil, apierrors.NewNotFound(corev1.Resource("pods"), fmt.Sprintf("migration pod not found for vm %s", migration.Spec.VMName))
	}
	return &podList.Items[0], nil
}
