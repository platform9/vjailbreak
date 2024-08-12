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
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

const success = "Success"
const v2vimage = "platform9/v2v-helper:v0.1"

//+kubebuilder:rbac:groups=deploy.pf9.io,resources=sites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deploy.pf9.io,resources=sites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deploy.pf9.io,resources=sites/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
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
//+kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds,verbs=get;list
//+kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarecreds,verbs=get;list
//+kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarecreds/status,verbs=get;update;patch

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
func (r *MigrationReconciler) ReconcileMigrationJob(ctx context.Context,
	migration *vjailbreakv1alpha1.Migration, ctxlog logr.Logger) (ctrl.Result, error) {
	// Fetch VMwareCreds CR
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{}
	err := r.Get(ctx, types.NamespacedName{Name: migration.Spec.Source.VMwareRef, Namespace: migration.Namespace}, vmwcreds)
	if err != nil {
		ctxlog.Error(err, "Failed to retrieve VMware CR")
		return ctrl.Result{}, err
	}

	if vmwcreds.Status.VMwareValidationStatus != success {
		ctxlog.Info(fmt.Sprintf("VMwareCreds '%s' CR is not validated", vmwcreds.Name))
		return ctrl.Result{}, nil
	}

	// Fetch OpenStackCreds CR
	openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{}
	err = r.Get(ctx, types.NamespacedName{Name: migration.Spec.Destination.OpenstackRef, Namespace: migration.Namespace}, openstackcreds)
	if err != nil {
		ctxlog.Error(err, "Failed to retrieve OpenStack CR")
		return ctrl.Result{}, err
	}

	if openstackcreds.Status.OpenStackValidationStatus != success {
		ctxlog.Info(fmt.Sprintf("OpenstackCreds '%s' CR is not validated", openstackcreds.Name))
		return ctrl.Result{}, nil
	}
	newvmstat := []vjailbreakv1alpha1.VMMigrationStatus{}

	for _, vm := range migration.Spec.Source.VirtualMachines {
		vmname := strings.ReplaceAll(strings.ReplaceAll(vm, " ", "-"), "_", "-")
		configMapName := fmt.Sprintf("migration-config-%s", vmname)
		podName := fmt.Sprintf("v2v-helper-%s", vmname)
		virtiodrivers := ""
		if migration.Spec.Source.VirtioWinDriver == "" {
			virtiodrivers = "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso"
		} else {
			virtiodrivers = migration.Spec.Source.VirtioWinDriver
		}

		// Create ConfigMap
		configMap := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: migration.Namespace}, configMap)
		if err != nil && apierrors.IsNotFound(err) {
			ctxlog.Info(fmt.Sprintf("Creating new ConfigMap '%s' for VM '%s'", configMapName, vmname))
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: migration.Namespace,
				},
				Data: map[string]string{
					"CONVERT":               "true", // Assume that the vm always has to be converted
					"NEUTRON_NETWORK_NAMES": strings.Join(migration.Spec.Destination.NetworkNames, ","),
					"OS_AUTH_URL":           openstackcreds.Spec.OsAuthURL,
					"OS_DOMAIN_NAME":        openstackcreds.Spec.OsDomainName,
					"OS_PASSWORD":           openstackcreds.Spec.OsPassword,
					"OS_REGION_NAME":        openstackcreds.Spec.OsRegionName,
					"OS_TENANT_NAME":        openstackcreds.Spec.OsTenantName,
					"OS_TYPE":               migration.Spec.Source.OSType,
					"OS_USERNAME":           openstackcreds.Spec.OsUsername,
					"SOURCE_VM_NAME":        vm,
					"VCENTER_HOST":          vmwcreds.Spec.VcenterHost,
					"VCENTER_INSECURE":      strconv.FormatBool(vmwcreds.Spec.VcenterInsecure),
					"VCENTER_PASSWORD":      vmwcreds.Spec.VcenterPassword,
					"VCENTER_USERNAME":      vmwcreds.Spec.VcenterUsername,
					"VIRTIO_WIN_DRIVER":     virtiodrivers,
				},
			}
			err = r.createResource(ctx, migration, configMap)
			if err != nil {
				ctxlog.Error(err, fmt.Sprintf("Failed to create ConfigMap '%s'", configMapName))
				return ctrl.Result{}, err
			}
			ctxlog.Info(fmt.Sprintf("ConfigMap '%s' created for VM '%s'", configMapName, vmname))
		}

		// Create Pod
		pointtrue := true
		pod := &corev1.Pod{}
		err = r.Get(ctx, types.NamespacedName{Name: podName, Namespace: migration.Namespace}, pod)
		if err != nil && apierrors.IsNotFound(err) {
			ctxlog.Info(fmt.Sprintf("Creating new Pod '%s' for VM '%s'", podName, vmname))
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: migration.Namespace,
					Labels: map[string]string{
						"vm-name": vmname,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: "migration-controller-manager",
					Containers: []corev1.Container{
						{
							Name:            "fedora",
							Image:           v2vimage,
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"/home/fedora/manager"},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &pointtrue,
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
			if err := r.createResource(ctx, migration, pod); err != nil {
				ctxlog.Error(err, fmt.Sprintf("Failed to create Pod '%s'", podName))
				return ctrl.Result{}, err
			}
			ctxlog.Info(fmt.Sprintf("Pod '%s' queued for Migration '%s'", podName, migration.Name))
		} else {
			newvmstat = append(newvmstat, vjailbreakv1alpha1.VMMigrationStatus{
				VMName: vm,
				Status: "Migration " + string(pod.Status.Phase),
			})
		}
	}
	migration.Status.VMMigrationStatus = newvmstat
	if err := r.Status().Update(ctx, migration); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MigrationReconciler) createResource(ctx context.Context, owner metav1.Object, controlled client.Object) error {
	err := ctrl.SetControllerReference(owner, controlled, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set controller reference")
	}
	err = r.Create(ctx, controlled)
	if err != nil {
		return fmt.Errorf("failed to create resource")
	}
	return nil
}

func newHostPathType(pathType string) *corev1.HostPathType {
	hostPathType := corev1.HostPathType(pathType)
	return &hostPathType
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.Migration{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}).
		Complete(r)
}
