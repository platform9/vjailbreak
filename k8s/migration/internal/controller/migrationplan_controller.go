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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// MigrationPlanReconciler reconciles a MigrationPlan object
type MigrationPlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	ctxlog logr.Logger
}

var migrationPlanFinalizer = "migrationplan.vjailbreak.pf9.io/finalizer"

const v2vimage = "platform9/v2v-helper:v0.1"

// Used to facilitate removal of our finalizer
func RemoveString(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationplans/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=migrationtemplates,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MigrationPlan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *MigrationPlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ctxlog = log.FromContext(ctx)
	migrationplan := &vjailbreakv1alpha1.MigrationPlan{}

	// Validate Time Field
	if migrationplan.Spec.MigrationStrategy.VMCutoverStart.After(migrationplan.Spec.MigrationStrategy.VMCutoverEnd.Time) {
		return ctrl.Result{}, fmt.Errorf("cutover start time is after cutover end time")
	}

	if err := r.Get(ctx, req.NamespacedName, migrationplan); err != nil {
		if apierrors.IsNotFound(err) {
			r.ctxlog.Info("Received ignorable event for a recently deleted MigrationPlan.")
			return ctrl.Result{}, nil
		}
		r.ctxlog.Error(err, fmt.Sprintf("Unexpected error reading MigrationPlan '%s' object", migrationplan.Name))
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion or not
	if migrationplan.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check if finalizer exists and if not, add one
		if !slices.Contains(migrationplan.ObjectMeta.Finalizers, migrationPlanFinalizer) {
			r.ctxlog.Info(fmt.Sprintf("MigrationPlan '%s' CR is being created or updated", migrationplan.Name))
			r.ctxlog.Info(fmt.Sprintf("Adding finalizer to MigrationPlan '%s'", migrationplan.Name))
			migrationplan.ObjectMeta.Finalizers = append(migrationplan.ObjectMeta.Finalizers, migrationPlanFinalizer)
			if err := r.Update(ctx, migrationplan); err != nil {
				return reconcile.Result{}, err
			}
		}

		if res, err := r.ReconcileMigrationPlanJob(ctx, migrationplan); err != nil {
			return res, err
		}
	} else {
		// The object is being deleted
		r.ctxlog.Info(fmt.Sprintf("MigrationPlan '%s' CR is being deleted", migrationplan.Name))

		// TODO implement finalizer logic

		// Now that the finalizer has completed deletion tasks, we can remove it
		// to allow deletion of the Migration object
		if slices.Contains(migrationplan.ObjectMeta.Finalizers, migrationPlanFinalizer) {
			r.ctxlog.Info(fmt.Sprintf("Removing finalizer from MigrationPlan '%s' so that it can be deleted", migrationplan.Name))
			migrationplan.ObjectMeta.Finalizers = RemoveString(migrationplan.ObjectMeta.Finalizers, migrationPlanFinalizer)
			if err := r.Update(ctx, migrationplan); err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// Similar to the Reconcile function above, but specifically for reconciling the Jobs
func (r *MigrationPlanReconciler) ReconcileMigrationPlanJob(ctx context.Context,
	migrationplan *vjailbreakv1alpha1.MigrationPlan) (ctrl.Result, error) {
	// Fetch MigrationTemplate CR
	migrationtemplate := &vjailbreakv1alpha1.MigrationTemplate{}
	if err := r.Get(ctx, types.NamespacedName{Name: migrationplan.Spec.MigrationTemplate, Namespace: migrationplan.Namespace},
		migrationtemplate); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get MigrationTemplate: %w", err)
	}
	// Fetch VMwareCreds CR
	vmwcreds := &vjailbreakv1alpha1.VMwareCreds{}
	if ok, err := r.checkStatusSuccess(ctx, migrationtemplate.Namespace, migrationtemplate.Spec.Source.VMwareRef, true, vmwcreds); !ok {
		return ctrl.Result{
			RequeueAfter: time.Minute,
		}, err
	}
	// Fetch OpenStackCreds CR
	openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{}
	if ok, err := r.checkStatusSuccess(ctx, migrationtemplate.Namespace, migrationtemplate.Spec.Destination.OpenstackRef,
		false, openstackcreds); !ok {
		return ctrl.Result{
			RequeueAfter: time.Minute,
		}, err
	}

	// Starting the Migrations
	if migrationplan.Status.MigrationStatus == "" {
		migrationplan.Status.MigrationStatus = string(corev1.PodRunning)
		migrationplan.Status.MigrationMessage = "Migration(s) in progress"
		err := r.Status().Update(ctx, migrationplan)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update MigrationPlan status: %w", err)
		}
	}

	for _, parallelvms := range migrationplan.Spec.VirtualMachines {
		migrationobjs := &vjailbreakv1alpha1.MigrationList{}
		for _, vm := range parallelvms {
			migrationobj, err := r.CreateMigration(ctx, migrationplan, vm)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create Migration for VM %s: %w", vm, err)
			}
			migrationobjs.Items = append(migrationobjs.Items, *migrationobj)
			cm, err := r.CreateConfigMap(ctx, migrationplan, migrationtemplate, migrationobj, openstackcreds, vmwcreds, vm)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create ConfigMap for VM %s: %w", vm, err)
			}
			err = r.CreatePod(ctx, migrationplan, migrationobj, vm, cm.Name)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create Pod for VM %s: %w", vm, err)
			}
		}
		for i := 0; i < len(migrationobjs.Items); i++ {
			switch migrationobjs.Items[i].Status.Phase {
			case string(corev1.PodFailed):
				r.ctxlog.Info(fmt.Sprintf("Migration for VM '%s' failed", migrationobjs.Items[i].Spec.VMName))
				migrationplan.Status.MigrationStatus = string(corev1.PodFailed)
				migrationplan.Status.MigrationMessage = fmt.Sprintf("Migration for VM '%s' failed", migrationobjs.Items[i].Spec.VMName)
				err := r.Status().Update(ctx, migrationplan)
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to update MigrationPlan status: %w", err)
				}
				return ctrl.Result{}, nil
			case string(corev1.PodSucceeded):
				continue
			default:
				r.ctxlog.Info(fmt.Sprintf("Waiting for all VMs in parallel batch %d to complete: %v", i+1, parallelvms))
				return ctrl.Result{}, nil
			}
		}
	}
	r.ctxlog.Info(fmt.Sprintf("All VMs in MigrationPlan '%s' have been successfully migrated", migrationplan.Name))
	migrationplan.Status.MigrationStatus = string(corev1.PodSucceeded)
	err := r.Status().Update(ctx, migrationplan)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update MigrationPlan status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *MigrationPlanReconciler) CreateMigration(ctx context.Context,
	migrationplan *vjailbreakv1alpha1.MigrationPlan,
	vm string) (*vjailbreakv1alpha1.Migration, error) {
	vmname := strings.ReplaceAll(strings.ReplaceAll(vm, " ", "-"), "_", "-")
	migrationobj := &vjailbreakv1alpha1.Migration{}
	err := r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("migration-%s", vmname), Namespace: migrationplan.Namespace}, migrationobj)
	if err != nil && apierrors.IsNotFound(err) {
		migrationobj = &vjailbreakv1alpha1.Migration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("migration-%s", vmname),
				Namespace: migrationplan.Namespace,
			},
			Spec: vjailbreakv1alpha1.MigrationSpec{
				MigrationPlan: migrationplan.Name,
				VMName:        vm,
				PodRef:        fmt.Sprintf("v2v-helper-%s", vmname),
			},
		}
		err = r.createResource(ctx, migrationplan, migrationobj)
		if err != nil {
			return nil, fmt.Errorf("failed to create Migration for VM %s: %w", vm, err)
		}
	}
	return migrationobj, nil
}

func (r *MigrationPlanReconciler) CreatePod(ctx context.Context,
	migrationplan *vjailbreakv1alpha1.MigrationPlan,
	migrationobj *vjailbreakv1alpha1.Migration, vm string, configMapName string) error {
	vmname := strings.ReplaceAll(strings.ReplaceAll(vm, " ", "-"), "_", "-")
	podName := fmt.Sprintf("v2v-helper-%s", vmname)
	pointtrue := true
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: migrationplan.Namespace}, pod)
	if err != nil && apierrors.IsNotFound(err) {
		r.ctxlog.Info(fmt.Sprintf("Creating new Pod '%s' for VM '%s'", podName, vmname))
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: migrationplan.Namespace,
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
		if err := r.createResource(ctx, migrationobj, pod); err != nil {
			r.ctxlog.Error(err, fmt.Sprintf("Failed to create Pod '%s'", podName))
			return err
		}
	}
	return nil
}

func (r *MigrationPlanReconciler) CreateConfigMap(ctx context.Context,
	migrationplan *vjailbreakv1alpha1.MigrationPlan,
	migrationtemplate *vjailbreakv1alpha1.MigrationTemplate,
	migrationobj *vjailbreakv1alpha1.Migration,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds, vm string) (*corev1.ConfigMap, error) {
	vmname := strings.ReplaceAll(strings.ReplaceAll(vm, " ", "-"), "_", "-")
	configMapName := fmt.Sprintf("migration-config-%s", vmname)
	virtiodrivers := ""
	if migrationtemplate.Spec.VirtioWinDriver == "" {
		virtiodrivers = "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso"
	} else {
		virtiodrivers = migrationtemplate.Spec.VirtioWinDriver
	}
	openstacknws, openstackvolumetypes, err := r.reconcileMapping(ctx, migrationtemplate, openstackcreds, vmwcreds, vm)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile mapping: %w", err)
	}

	// Create ConfigMap
	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: migrationplan.Namespace}, configMap)
	if err != nil && apierrors.IsNotFound(err) {
		r.ctxlog.Info(fmt.Sprintf("Creating new ConfigMap '%s' for VM '%s'", configMapName, vmname))
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: migrationplan.Namespace,
			},
			Data: map[string]string{
				"CONVERT":               "true", // Assume that the vm always has to be converted
				"TYPE":                  migrationplan.Spec.MigrationStrategy.Type,
				"DATACOPYSTART":         migrationplan.Spec.MigrationStrategy.DataCopyStart.Format(time.RFC3339),
				"CUTOVERSTART":          migrationplan.Spec.MigrationStrategy.VMCutoverStart.Format(time.RFC3339),
				"CUTOVEREND":            migrationplan.Spec.MigrationStrategy.VMCutoverEnd.Format(time.RFC3339),
				"NEUTRON_NETWORK_NAMES": strings.Join(openstacknws, ","),
				"CINDER_VOLUME_TYPES":   strings.Join(openstackvolumetypes, ","),
				"OS_AUTH_URL":           openstackcreds.Spec.OsAuthURL,
				"OS_DOMAIN_NAME":        openstackcreds.Spec.OsDomainName,
				"OS_PASSWORD":           openstackcreds.Spec.OsPassword,
				"OS_REGION_NAME":        openstackcreds.Spec.OsRegionName,
				"OS_TENANT_NAME":        openstackcreds.Spec.OsTenantName,
				"OS_TYPE":               migrationtemplate.Spec.OSType,
				"OS_USERNAME":           openstackcreds.Spec.OsUsername,
				"SOURCE_VM_NAME":        vm,
				"VCENTER_HOST":          vmwcreds.Spec.VcenterHost,
				"VCENTER_INSECURE":      strconv.FormatBool(vmwcreds.Spec.VcenterInsecure),
				"VCENTER_PASSWORD":      vmwcreds.Spec.VcenterPassword,
				"VCENTER_USERNAME":      vmwcreds.Spec.VcenterUsername,
				"VIRTIO_WIN_DRIVER":     virtiodrivers,
			},
		}
		err = r.createResource(ctx, migrationobj, configMap)
		if err != nil {
			r.ctxlog.Error(err, fmt.Sprintf("Failed to create ConfigMap '%s'", configMapName))
			return nil, err
		}
	}
	return configMap, nil
}

func (r *MigrationPlanReconciler) createResource(ctx context.Context, owner metav1.Object, controlled client.Object) error {
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

func (r *MigrationPlanReconciler) checkStatusSuccess(ctx context.Context,
	namespace, credsname string,
	isvmware bool,
	credsobj client.Object) (bool, error) {
	err := r.Get(ctx, types.NamespacedName{Name: credsname, Namespace: namespace}, credsobj)
	if err != nil {
		return false, fmt.Errorf("failed to get VMwareCreds: %w", err)
	}

	if isvmware && credsobj.(*vjailbreakv1alpha1.VMwareCreds).Status.VMwareValidationStatus != string(corev1.PodSucceeded) {
		return false, fmt.Errorf("VMwareCreds '%s' CR is not validated", credsobj.(*vjailbreakv1alpha1.VMwareCreds).Name)
	} else if !isvmware && credsobj.(*vjailbreakv1alpha1.OpenstackCreds).Status.OpenStackValidationStatus != string(corev1.PodSucceeded) {
		return false, fmt.Errorf("OpenstackCreds '%s' CR is not validated", credsobj.(*vjailbreakv1alpha1.OpenstackCreds).Name)
	}
	return true, nil
}

func (r *MigrationPlanReconciler) reconcileMapping(ctx context.Context,
	migrationtemplate *vjailbreakv1alpha1.MigrationTemplate,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds,
	vm string) (openstacknws, openstackvolumetypes []string, err error) {
	openstacknws, err = r.reconcileNetwork(ctx, migrationtemplate, openstackcreds, vmwcreds, vm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reconcile network: %w", err)
	}
	openstackvolumetypes, err = r.reconcileStorage(ctx, migrationtemplate, vmwcreds, openstackcreds, vm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reconcile storage: %w", err)
	}
	return openstacknws, openstackvolumetypes, nil
}

//nolint:dupl // Similar logic to storages reconciliation, excluding from linting to keep it readable
func (r *MigrationPlanReconciler) reconcileNetwork(ctx context.Context,
	migrationtemplate *vjailbreakv1alpha1.MigrationTemplate,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds,
	vm string) ([]string, error) {
	vmnws, err := GetVMwNetworks(ctx, vmwcreds, migrationtemplate.Spec.Source.DataCenter, vm)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	// Fetch the networkmap
	networkmap := &vjailbreakv1alpha1.NetworkMapping{}
	err = r.Get(ctx, types.NamespacedName{Name: migrationtemplate.Spec.NetworkMapping, Namespace: migrationtemplate.Namespace}, networkmap)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve NetworkMapping CR: %w", err)
	}

	openstacknws := []string{}
	for _, vmnw := range vmnws {
		for _, nwm := range networkmap.Spec.Networks {
			if vmnw == nwm.Source {
				openstacknws = append(openstacknws, nwm.Target)
			}
		}
	}
	if len(openstacknws) != len(vmnws) {
		return nil, fmt.Errorf("VMware Network(s) not found in NetworkMapping")
	}

	if networkmap.Status.NetworkmappingValidationStatus != string(corev1.PodSucceeded) {
		err = VerifyNetworks(ctx, openstackcreds, openstacknws)
		if err != nil {
			return nil, fmt.Errorf("failed to verify networks: %w", err)
		}
		networkmap.Status.NetworkmappingValidationStatus = string(corev1.PodSucceeded)
		networkmap.Status.NetworkmappingValidationMessage = "NetworkMapping validated"
		err = r.Status().Update(ctx, networkmap)
		if err != nil {
			return nil, fmt.Errorf("failed to update networkmapping status: %w", err)
		}
	}
	return openstacknws, nil
}

//nolint:dupl // Similar logic to networks reconciliation, excluding from linting to keep it readable
func (r *MigrationPlanReconciler) reconcileStorage(ctx context.Context,
	migrationtemplate *vjailbreakv1alpha1.MigrationTemplate,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds,
	vm string) ([]string, error) {
	vmds, err := GetVMwDatastore(ctx, vmwcreds, migrationtemplate.Spec.Source.DataCenter, vm)
	if err != nil {
		return nil, fmt.Errorf("failed to get datastores: %w", err)
	}
	// Fetch the StorageMap
	storagemap := &vjailbreakv1alpha1.StorageMapping{}
	err = r.Get(ctx, types.NamespacedName{Name: migrationtemplate.Spec.StorageMapping, Namespace: migrationtemplate.Namespace}, storagemap)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve StorageMapping CR: %w", err)
	}

	openstackvolumetypes := []string{}
	for _, vmdatastore := range vmds {
		for _, storagemaptype := range storagemap.Spec.Storages {
			if vmdatastore == storagemaptype.Source {
				openstackvolumetypes = append(openstackvolumetypes, storagemaptype.Target)
			}
		}
	}
	if len(openstackvolumetypes) != len(vmds) {
		return nil, fmt.Errorf("VMware Datastore(s) not found in StorageMapping")
	}
	if storagemap.Status.StoragemappingValidationStatus != string(corev1.PodSucceeded) {
		err = VerifyStorage(ctx, openstackcreds, openstackvolumetypes)
		if err != nil {
			return nil, fmt.Errorf("failed to verify datastores: %w", err)
		}
		storagemap.Status.StoragemappingValidationStatus = string(corev1.PodSucceeded)
		storagemap.Status.StoragemappingValidationMessage = "StorageMapping validated"
		err = r.Status().Update(ctx, storagemap)
		if err != nil {
			return nil, fmt.Errorf("failed to update storagemapping status: %w", err)
		}
	}
	return openstackvolumetypes, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigrationPlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.MigrationPlan{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&vjailbreakv1alpha1.Migration{}, builder.WithPredicates(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return e.ObjectOld.(*vjailbreakv1alpha1.Migration).Status.Phase != e.ObjectNew.(*vjailbreakv1alpha1.Migration).Status.Phase
				},
			})).
		Complete(r)
}