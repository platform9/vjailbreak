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
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=list;get
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=list;get;create;update;delete

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
	v2vimage := "tanaypf9/v2v:latest"

	// Fetch VMware secret
	vmwareSecret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: migration.Spec.Source.VMwareRef, Namespace: migration.Namespace}, vmwareSecret)
	if err != nil {
		ctxlog.Error(err, "Failed to retrieve VMware secret")
		return ctrl.Result{}, err
	}

	// Fetch OpenStack secret
	openstackSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: migration.Spec.Destination.OpenstackRef, Namespace: migration.Namespace}, openstackSecret)
	if err != nil {
		ctxlog.Error(err, "Failed to retrieve OpenStack secret")
		return ctrl.Result{}, err
	}

	for _, vm := range migration.Spec.Source.VirtualMachines {
		configMapName := fmt.Sprintf("migration-config-%s", vm)
		podName := fmt.Sprintf("v2v-helper-%s", vm)

		// Create ConfigMap
		configMap := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: migration.Namespace}, configMap)
		if err != nil && apierrors.IsNotFound(err) {
			ctxlog.Info(fmt.Sprintf("Creating new ConfigMap '%s' for VM '%s'", configMapName, vm))
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: migration.Namespace,
				},
				Data: map[string]string{
					"CONVERT":              "true",
					"NEUTRON_NETWORK_NAME": "vlan3002",
					"OS_AUTH_URL":          string(openstackSecret.Data["OS_AUTH_URL"]),
					"OS_DOMAIN_NAME":       string(openstackSecret.Data["OS_DOMAIN_NAME"]),
					"OS_PASSWORD":          string(openstackSecret.Data["OS_PASSWORD"]),
					"OS_REGION_NAME":       string(openstackSecret.Data["OS_REGION_NAME"]),
					"OS_TENANT_NAME":       string(openstackSecret.Data["OS_TENANT_NAME"]),
					"OS_TYPE":              "Windows",
					"OS_USERNAME":          string(openstackSecret.Data["OS_USERNAME"]),
					"SOURCE_VM_NAME":       vm,
					"VCENTER_HOST":         string(vmwareSecret.Data["VCENTER_HOST"]),
					"VCENTER_INSECURE":     string(vmwareSecret.Data["VCENTER_INSECURE"]),
					"VCENTER_PASSWORD":     string(vmwareSecret.Data["VCENTER_PASSWORD"]),
					"VCENTER_USERNAME":     string(vmwareSecret.Data["VCENTER_USERNAME"]),
					"VIRTIO_WIN_DRIVER":    "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.189-1/virtio-win-0.1.189.iso",
				},
			}
			err = ctrl.SetControllerReference(migration, configMap, r.Scheme)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.Create(ctx, configMap)
			if err != nil {
				ctxlog.Error(err, fmt.Sprintf("Failed to create ConfigMap '%s'", configMapName))
				return ctrl.Result{}, err
			}
			ctxlog.Info(fmt.Sprintf("ConfigMap '%s' created for VM '%s'", configMapName, vm))
		}

		// Create Pod
		pod := &corev1.Pod{}
		err = r.Get(ctx, types.NamespacedName{Name: podName, Namespace: migration.Namespace}, pod)
		if err != nil && apierrors.IsNotFound(err) {
			ctxlog.Info(fmt.Sprintf("Creating new Pod '%s' for VM '%s'", podName, vm))
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: migration.Namespace,
					Labels: map[string]string{
						"vm-name": vm,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "fedora",
							Image:           v2vimage,
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"/home/fedora/manager"},
							SecurityContext: &corev1.SecurityContext{
								Privileged: new(bool),
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "vddk",
									MountPath: "/home/fedora/vmware-vix-disklib-distrib",
								},
								{
									Name:      "dev",
									MountPath: "/dev",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "vddk",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/home/ubuntu/vmware-vix-disklib-distrib",
									Type: newHostPathType("Directory"),
								},
							},
						},
						{
							Name: "dev",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev",
									Type: newHostPathType("Directory"),
								},
							},
						},
					},
				},
			}
			err = ctrl.SetControllerReference(migration, pod, r.Scheme)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.Create(ctx, pod)
			if err != nil {
				ctxlog.Error(err, fmt.Sprintf("Failed to create Pod '%s'", podName))
				return ctrl.Result{}, err
			}
			ctxlog.Info(fmt.Sprintf("Pod '%s' queued for Migration '%s'", podName, migration.Name))
		}
	}

	return ctrl.Result{}, nil
}

func newHostPathType(pathType string) *corev1.HostPathType {
	hostPathType := corev1.HostPathType(pathType)
	return &hostPathType
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.Migration{}).
		Complete(r)
}
