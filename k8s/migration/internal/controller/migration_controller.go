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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

// MigrationReconciler reconciles a Migration object
type MigrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var migrationFinalizer = "migration.vjailbreak.pf9.io/finalizer"

//+kubebuilder:rbac:groups=deploy.pf9.io,resources=sites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deploy.pf9.io,resources=sites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deploy.pf9.io,resources=sites/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Used to facilitate removal of our finalizer
func RemoveString(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrations/finalizers,verbs=update

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
	migration := &vjailbreakv1alpha1.Migration{}

	if err := r.Get(ctx, req.NamespacedName, migration); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted Migration.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading Migration '%s' object", migration.Name))
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion or not
	if migration.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check if finalizer exists and if not, add one
		if !slices.Contains(migration.ObjectMeta.Finalizers, migrationFinalizer) {
			ctxlog.Info(fmt.Sprintf("Migration '%s' CR is being created or updated", migration.Name))
			ctxlog.Info(fmt.Sprintf("Adding finalizer to Migration '%s'", migration.Name))
			migration.ObjectMeta.Finalizers = append(migration.ObjectMeta.Finalizers, migrationFinalizer)
			if err := r.Update(context.Background(), migration); err != nil {
				return reconcile.Result{}, err
			}
		}

		if res, err := r.ReconcileMigrationJob(ctx, migration, ctxlog); err != nil {
			return res, err
		}
	} else {
		// The object is being deleted
		ctxlog.Info(fmt.Sprintf("Migration '%s' CR is being deleted", migration.Name))

		// TODO implement finalizer logic

		// Now that the finalizer has completed deletion tasks, we can remove it
		// to allow deletion of the Migration object
		if slices.Contains(migration.ObjectMeta.Finalizers, migrationFinalizer) {
			ctxlog.Info(fmt.Sprintf("Removing finalizer from Migration '%s' so that it can be deleted", migration.Name))
			migration.ObjectMeta.Finalizers = RemoveString(migration.ObjectMeta.Finalizers, migrationFinalizer)
			if err := r.Update(context.Background(), migration); err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// Similar to the Reconcile function above, but specifically for reconciling the Jobs
func (r *MigrationReconciler) ReconcileMigrationJob(ctx context.Context, migration *vjailbreakv1alpha1.Migration, ctxlog logr.Logger) (ctrl.Result, error) {

	migrationJobName := migration.Name
	var backOffLimit int32 = 0
	migrationJob := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: migrationJobName, Namespace: migration.Namespace}, migrationJob)
	if err != nil && apierrors.IsNotFound(err) {
		ctxlog.Info(fmt.Sprintf("Creating new Job '%s'", migrationJobName))
		migrationJob = &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      migrationJobName,
				Namespace: migration.Namespace,
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: &backOffLimit,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"vjailbreak.pf9.io/migration": migrationJobName},
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Volumes: []corev1.Volume{
							{
								Name: "adminrc",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: migration.Spec.OpenstackSecretRef,
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "virt-v2v",
								Image:           "ubuntu:latest",
								ImagePullPolicy: "Always",
								Command:         []string{"/bin/echo"},
								Args:            []string{"hello", " world!"},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(50), resource.DecimalSI),
										corev1.ResourceMemory: *resource.NewScaledQuantity(int64(250), resource.Mega),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "adminrc",
										MountPath: "/home/migration/admin.rc",
										ReadOnly:  true,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "VJAILBREAK_VMWARE_USER",
										Value: "",
									},
									{
										Name:  "VJAILBREAK_VMWARE_PASSWORD",
										Value: "",
									},
									{
										Name:  "VJAILBREAK_VMWARE_VCENTER",
										Value: migration.Spec.Source.VCenter,
									},
									{
										Name:  "VJAILBREAK_VMWARE_CLUSTER",
										Value: migration.Spec.Source.Cluster,
									},
									{
										Name:  "VJAILBREAK_VMWARE_DATACENTER",
										Value: migration.Spec.Source.DataCenter,
									},
									{
										Name:  "VJAILBREAK_VMWARE_ESXNODE",
										Value: migration.Spec.Source.ESXNode,
									},
									{
										Name:  "VJAILBREAK_VMWARE_THUMBPRINT",
										Value: migration.Spec.Source.VCenterThumbPrint,
									},
								},
							},
						},
					},
				},
			},
		}
		err = ctrl.SetControllerReference(migration, migrationJob, r.Scheme)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Create(ctx, migrationJob)
		if err != nil {
			ctxlog.Error(err, fmt.Sprintf("Failed to create Job '%s'", migrationJobName))
			return ctrl.Result{}, err
		}
		ctxlog.Info(fmt.Sprintf("Job '%s' queued for Migration '%s'", migrationJobName, migration.Name))
	}

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.Migration{}).
		Complete(r)
}
