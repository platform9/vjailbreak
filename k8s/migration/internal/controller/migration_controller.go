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
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// MigrationReconciler reconciles a Migration object
type MigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const migrationReason = "Migration"
const startCutoveryes = "yes"
const startCutoverno = "no"

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Migration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *MigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)

	ctxlog.Info("Reconciling Migration object")
	migration := &vjailbreakv1alpha1.Migration{}

	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted Migration.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading Migration '%s' object", migration.Name))
		return ctrl.Result{}, err
	}

	// Get the pod phase
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: migration.Namespace, Name: migration.Spec.PodRef}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Failed to get Pod '%s'", migration.Spec.PodRef))
		return ctrl.Result{}, err
	}

	pod.Labels["startCutover"] = setCutoverLabel(migration.Spec.InitiateCutover, pod.Labels["startCutover"])
	if err := r.Update(ctx, pod); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to update Pod '%s'", pod.Name))
		return ctrl.Result{}, err
	}

	// Get events of this pod
	allevents := &corev1.EventList{}
	if err := r.List(ctx, allevents, client.InNamespace(migration.Namespace)); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to get events for Pod '%s'", pod.Name))
		return ctrl.Result{}, err
	}
	filteredEvents := &corev1.EventList{}
	for i := 0; i < len(allevents.Items); i++ {
		if allevents.Items[i].InvolvedObject.Name == pod.Name && string(allevents.Items[i].InvolvedObject.UID) == string(pod.UID) {
			filteredEvents.Items = append(filteredEvents.Items, allevents.Items[i])
		}
	}

	// Create status conditions
	statusconditions := []corev1.PodCondition{}
	if validatedCondition := createValidatedCondition(filteredEvents); validatedCondition != nil {
		statusconditions = append(statusconditions, *validatedCondition)
	}
	statusconditions = append(statusconditions, createDataCopyCondition(filteredEvents)...)
	if migratedCondition := createMigratedCondition(filteredEvents); migratedCondition != nil {
		statusconditions = append(statusconditions, *migratedCondition)
	}

	// Sort status conditions by LastTransitionTime
	// This is necessary because the order of the events is not guaranteed
	sortConditionsByLastTransitionTime(statusconditions)

	for i := 0; i < len(statusconditions); i++ {
		if i != len(statusconditions)-1 {
			statusconditions[i].Status = "True"
		} else if string(pod.Status.Phase) == string(corev1.PodSucceeded) {
			statusconditions[i].Status = "True"
		} else if string(pod.Status.Phase) == string(corev1.PodFailed) {
			statusconditions[i].Status = "False"
		}
	}

	// Update the status of the Migration object
	migration.Status.Phase = string(pod.Status.Phase)
	migration.Status.Conditions = statusconditions
	migration.Status.AgentName = string(pod.Spec.NodeName)
	if err := r.Status().Update(ctx, migration); err != nil {
		ctxlog.Error(err, fmt.Sprintf("Failed to update status of Migration '%s'", migration.Name))
		return ctrl.Result{}, err
	}
	ctxlog.Info("Updated status of Migration object")

	return ctrl.Result{}, nil
}

func createValidatedCondition(eventList *corev1.EventList) *corev1.PodCondition {
	statuscondition := &corev1.PodCondition{}
	for i := 0; i < len(eventList.Items); i++ {
		if !(eventList.Items[i].Reason == migrationReason && eventList.Items[i].Message == "Creating volumes in OpenStack") {
			continue
		}
		statuscondition.Type = "Validated"
		statuscondition.Status = corev1.ConditionUnknown
		statuscondition.Reason = migrationReason
		statuscondition.Message = "Migration validated successfully"
		statuscondition.LastTransitionTime = eventList.Items[i].LastTimestamp
		return statuscondition
	}
	return nil
}

// SortConditionsByLastTransitionTime sorts conditions by LastTransitionTime
func sortConditionsByLastTransitionTime(conditions []corev1.PodCondition) {
	sort.Slice(conditions, func(i, j int) bool {
		return conditions[i].LastTransitionTime.Before(&conditions[j].LastTransitionTime)
	})
}

func createDataCopyCondition(eventList *corev1.EventList) []corev1.PodCondition {
	statusconditions := []corev1.PodCondition{}
	for i := 0; i < len(eventList.Items); i++ {
		if !(eventList.Items[i].Reason == migrationReason && strings.Contains(eventList.Items[i].Message, "Copying disk")) {
			continue
		}
		statuscondition := &corev1.PodCondition{}
		statuscondition.Type = "DataCopy"
		statuscondition.Status = corev1.ConditionUnknown
		statuscondition.Reason = migrationReason
		statuscondition.Message = eventList.Items[i].Message
		statuscondition.LastTransitionTime = eventList.Items[i].LastTimestamp
		statusconditions = append(statusconditions, *statuscondition)
	}
	return statusconditions
}

func createMigratedCondition(eventList *corev1.EventList) *corev1.PodCondition {
	statuscondition := &corev1.PodCondition{}
	for i := 0; i < len(eventList.Items); i++ {
		if !(eventList.Items[i].Reason == migrationReason && eventList.Items[i].Message == "Converting disk") {
			continue
		}
		statuscondition.Type = "Migrated"
		statuscondition.Status = corev1.ConditionUnknown
		statuscondition.Reason = migrationReason
		statuscondition.Message = "Migrating VM from VMware to Openstack"
		statuscondition.LastTransitionTime = eventList.Items[i].LastTimestamp
		return statuscondition
	}
	return nil
}

func setCutoverLabel(initiateCutover bool, currentLabel string) string {
	if initiateCutover {
		if currentLabel != startCutoveryes {
			currentLabel = startCutoveryes
		}
	} else {
		if currentLabel != startCutoverno {
			currentLabel = startCutoverno
		}
	}
	return currentLabel
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.Migration{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}, builder.WithPredicates(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldpod := e.ObjectOld.(*corev1.Pod)
					newpod := e.ObjectNew.(*corev1.Pod)
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
