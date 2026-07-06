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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	providers "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
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
func (r *ESXIMigrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ESXIMigrationControllerName)
	ctxlog.Info("Starting reconciliation", "esximigration", req.NamespacedName)

	esxiMigration := &vjailbreakv1alpha1.ESXIMigration{}
	if err := r.Get(ctx, req.NamespacedName, esxiMigration); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Resource not found, likely deleted", "esximigration", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get ESXIMigration resource", "esximigration", req.NamespacedName)
		return ctrl.Result{}, err
	}

	scope, err := scope.NewESXIMigrationScope(scope.ESXIMigrationScopeParams{
		Logger:        ctxlog,
		Client:        r.Client,
		ESXIMigration: esxiMigration,
	})
	if err != nil {
		ctxlog.Error(err, "Failed to create ESXIMigrationScope")
		return ctrl.Result{}, errors.Wrap(err, "failed to create ESXIMigrationScope")
	}

	if esxiMigration.Spec.RollingMigrationPlanRef.Name != "" {
		rollingMigrationPlan := &vjailbreakv1alpha1.RollingMigrationPlan{}
		rollingMigrationPlanKey := client.ObjectKey{Namespace: esxiMigration.Namespace, Name: esxiMigration.Spec.RollingMigrationPlanRef.Name}
		if err := r.Get(ctx, rollingMigrationPlanKey, rollingMigrationPlan); err != nil {
			if apierrors.IsNotFound(err) {
				if !esxiMigration.DeletionTimestamp.IsZero() {
					ctxlog.Info("Resource is being deleted, reconciling deletion", "esximigration", req.NamespacedName)
					return r.reconcileDelete(ctx, scope)
				}
				return ctrl.Result{}, errors.Wrap(err, "failed to get RollingMigrationPlan")
			}
			return ctrl.Result{}, errors.Wrap(err, "failed to get RollingMigrationPlan")
		}
		scope.RollingMigrationPlan = rollingMigrationPlan
	}

	// Always close the scope when exiting this function such that we can persist any ESXIMigration changes.
	defer func() {
		if err := scope.Close(); err != nil && reterr == nil {
			ctxlog.Error(err, "Failed to close ESXIMigrationScope")
			reterr = err
		}
	}()

	if !esxiMigration.DeletionTimestamp.IsZero() {
		ctxlog.Info("Resource is being deleted, reconciling deletion", "esximigration", req.NamespacedName)
		return r.reconcileDelete(ctx, scope)
	}

	return r.reconcileNormal(ctx, scope)
}

func (r *ESXIMigrationReconciler) reconcileNormal(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Starting normal reconciliation", "esximigration", scope.ESXIMigration.Name, "namespace", scope.ESXIMigration.Namespace)
	controllerutil.AddFinalizer(scope.ESXIMigration, constants.ESXIMigrationFinalizer)
	switch scope.ESXIMigration.Status.Phase {
	case "":
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseWaiting
		err := r.Status().Update(ctx, scope.ESXIMigration)
		if err != nil {
			log.Error(err, "Failed to update ESXIMigration status")
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
	case vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded:
		log.Info("ESXIMigration already succeeded")
		return ctrl.Result{}, nil
	case vjailbreakv1alpha1.ESXIMigrationPhaseFailed:
		log.Info("ESXIMigration already failed")
		return ctrl.Result{}, nil
	}

	if utils.IsESXIMigrationPaused(ctx, scope.ESXIMigration.Name, scope.Client) {
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhasePaused
		if err := scope.Client.Status().Update(ctx, scope.ESXIMigration); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
		log.Info(fmt.Sprintf("ESXi migration %s is paused, skipping reconciliation", scope.ESXIMigration.Name))
		return ctrl.Result{}, nil
	}

	bmConfigName, err := resolveBMConfigName(scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		log.Error(err, "Failed to resolve BMConfig name")
		return ctrl.Result{}, err
	}
	bmConfig := &vjailbreakv1alpha1.BMConfig{}
	bmConfigKey := client.ObjectKey{Namespace: scope.ESXIMigration.Namespace, Name: bmConfigName}
	if err := r.Get(ctx, bmConfigKey, bmConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, errors.Wrap(err, "failed to get BMConfig")
		}
		log.Error(err, "Failed to get BMConfig", "bmconfig", bmConfigKey)
		return ctrl.Result{}, errors.Wrap(err, "failed to get BMConfig")
	}

	if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseCordoned {
		return r.handleESXiCordoned(ctx, scope, bmConfig)
	}

	if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseWaitingForPCDHost {
		return r.handleESXiWaitingForPCDHost(ctx, scope)
	}

	if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseConfiguringPCDHost {
		return r.handleESXiConfiguringPCDHost(ctx, scope)
	}

	if scope.ESXIMigration.Status.Phase == vjailbreakv1alpha1.ESXIMigrationPhaseAssigningRole {
		return r.handleESXiAssigningRole(ctx, scope)
	}

	inMaintenance, err := utils.CheckESXiInMaintenanceMode(ctx, r.Client, scope)
	if err != nil {
		log.Error(err, "Failed to check ESXi maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to check ESXi maintenance mode")
	}
	if inMaintenance {
		return r.handleESXiInMaintenanceMode(ctx, scope)
	}

	log.Info("Putting ESXi in maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
	err = utils.PutESXiInMaintenanceMode(ctx, r.Client, scope)
	if err != nil {
		log.Error(err, "Failed to put ESXi in maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseFailed
		updateErr := scope.Client.Status().Update(ctx, scope.ESXIMigration)
		if updateErr != nil {
			return ctrl.Result{}, errors.Wrap(updateErr, "failed to update ESXi migration status to failed")
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to put ESXi in maintenance mode")
	}
	log.Info("Successfully updated ESXIMigration status to maintenance")
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// nolint:unparam
func (r *ESXIMigrationReconciler) reconcileDelete(_ context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Reconciling deletion", "esximigration", scope.ESXIMigration.Name, "namespace", scope.ESXIMigration.Namespace)

	controllerutil.RemoveFinalizer(scope.ESXIMigration, constants.ESXIMigrationFinalizer)
	return ctrl.Result{}, nil
}

// resolveBMConfigName returns the BMConfig name for an ESXIMigration.
// Prefers spec.bmConfigRef (new ClusterConversionBatch flow) over the RollingMigrationPlan's bmConfigRef.
func resolveBMConfigName(esxiMig *vjailbreakv1alpha1.ESXIMigration, rmp *vjailbreakv1alpha1.RollingMigrationPlan) (string, error) {
	if esxiMig.Spec.BMConfigRef != nil && esxiMig.Spec.BMConfigRef.Name != "" {
		return esxiMig.Spec.BMConfigRef.Name, nil
	}
	if rmp != nil && rmp.Spec.BMConfigRef.Name != "" {
		return rmp.Spec.BMConfigRef.Name, nil
	}
	return "", errors.New("no BMConfig reference: set spec.bmConfigRef on ESXIMigration or ensure RollingMigrationPlanRef is valid")
}

// resolveVMwareCreds returns VMwareCreds for an ESXIMigration.
// Prefers spec.vmwareCredsRef (ClusterConversionBatch flow) over the RollingMigrationPlan path.
//
//nolint:dupl
func resolveVMwareCreds(ctx context.Context, k8sClient client.Client, esxiMig *vjailbreakv1alpha1.ESXIMigration, rmp *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.VMwareCreds, error) {
	if esxiMig.Spec.VMwareCredsRef.Name != "" {
		creds := &vjailbreakv1alpha1.VMwareCreds{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceMigrationSystem, Name: esxiMig.Spec.VMwareCredsRef.Name}, creds); err != nil {
			return nil, errors.Wrap(err, "failed to get VMwareCreds from spec.vmwareCredsRef")
		}
		return creds, nil
	}
	if rmp != nil {
		return utils.GetVMwareCredsFromRollingMigrationPlan(ctx, k8sClient, rmp)
	}
	return nil, errors.New("no VMwareCreds: set spec.vmwareCredsRef or ensure RollingMigrationPlanRef is valid")
}

// resolveOpenstackCreds returns OpenstackCreds for an ESXIMigration.
// Prefers spec.openstackCredsRef (ClusterConversionBatch flow) over the RollingMigrationPlan path.
//
//nolint:dupl
func resolveOpenstackCreds(ctx context.Context, k8sClient client.Client, esxiMig *vjailbreakv1alpha1.ESXIMigration, rmp *vjailbreakv1alpha1.RollingMigrationPlan) (*vjailbreakv1alpha1.OpenstackCreds, error) {
	if esxiMig.Spec.OpenstackCredsRef.Name != "" {
		creds := &vjailbreakv1alpha1.OpenstackCreds{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceMigrationSystem, Name: esxiMig.Spec.OpenstackCredsRef.Name}, creds); err != nil {
			return nil, errors.Wrap(err, "failed to get OpenstackCreds from spec.openstackCredsRef")
		}
		return creds, nil
	}
	if rmp != nil {
		return utils.GetOpenstackCredsFromRollingMigrationPlan(ctx, k8sClient, rmp)
	}
	return nil, errors.New("no OpenstackCreds: set spec.openstackCredsRef or ensure RollingMigrationPlanRef is valid")
}

// SetupWithManager sets up the controller with the Manager.
func (r *ESXIMigrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ESXIMigration{}).
		Complete(r)
}

func (r *ESXIMigrationReconciler) handleESXiCordoned(ctx context.Context, scope *scope.ESXIMigrationScope, bmConfig *vjailbreakv1alpha1.BMConfig) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("ESXi is cordoned", "esxiName", scope.ESXIMigration.Spec.ESXiName)
	provider, err := providers.GetProvider(string(bmConfig.Spec.ProviderType))
	if err != nil {
		return ctrl.Result{}, err
	}
	err = utils.ConvertESXiToPCDHost(ctx, scope, provider)
	if err != nil {
		return ctrl.Result{}, err
	}

	scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseWaitingForPCDHost
	err = r.Status().Update(ctx, scope.ESXIMigration)
	if err != nil {
		log.Error(err, "Failed to update ESXIMigration status", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
	}
	return ctrl.Result{}, nil
}

func (r *ESXIMigrationReconciler) handleESXiWaitingForPCDHost(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("ESXi is waiting for PCD host", "esxiName", scope.ESXIMigration.Spec.ESXiName)
	destOpenstackCreds, err := resolveOpenstackCreds(ctx, r.Client, scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		log.Error(err, "Failed to get destination openstack credentials")
		return ctrl.Result{}, errors.Wrap(err, "failed to get destination openstack credentials")
	}
	sourceVMwareCreds, err := resolveVMwareCreds(ctx, r.Client, scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		log.Error(err, "Failed to get source vmware credentials")
		return ctrl.Result{}, errors.Wrap(err, "failed to get source vmware credentials")
	}
	vmwareHost, err := utils.GetVMwareHostFromESXiName(ctx, r.Client, scope.ESXIMigration.Spec.ESXiName, sourceVMwareCreds.Name)
	if err != nil {
		log.Error(err, "Failed to get VMware host", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to get VMware host")
	}
	showedUp, err := utils.WaitforHostToShowUpOnPCD(ctx, r.Client, destOpenstackCreds.Name, vmwareHost.Spec.HardwareUUID)
	if err != nil {
		log.Error(err, "Failed to wait for host to show up on PCD", "hostID", vmwareHost.Spec.HardwareUUID)
		return ctrl.Result{}, errors.Wrap(err, "failed to wait for host to show up on PCD")
	}
	if !showedUp {
		log.Info("Host did not show up on PCD, waiting for it to show up", "hostID", vmwareHost.Spec.HardwareUUID)
		return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
	}
	log.Info("Host showed up on PCD", "hostName", vmwareHost.Spec.Name)
	scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseConfiguringPCDHost
	err = r.Status().Update(ctx, scope.ESXIMigration)
	if err != nil {
		log.Error(err, "Failed to update ESXIMigration status", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
	}
	return ctrl.Result{}, nil
}

func (r *ESXIMigrationReconciler) handleESXiConfiguringPCDHost(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("ESXi is configuring PCD host", "esxiName", scope.ESXIMigration.Spec.ESXiName)
	var pcdClusterName string

	destOpenstackCreds, err := resolveOpenstackCreds(ctx, r.Client, scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		log.Error(err, "Failed to get destination openstack credentials")
		return ctrl.Result{}, errors.Wrap(err, "failed to get destination openstack credentials")
	}
	sourceVMwareCreds, err := resolveVMwareCreds(ctx, r.Client, scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		log.Error(err, "Failed to get source vmware credentials")
		return ctrl.Result{}, errors.Wrap(err, "failed to get source vmware credentials")
	}
	vmwareHost, err := utils.GetVMwareHostFromESXiName(ctx, r.Client, scope.ESXIMigration.Spec.ESXiName, sourceVMwareCreds.Name)
	if err != nil {
		log.Error(err, "Failed to get VMware host", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to get VMware host")
	}
	if vmwareHost.Spec.HostConfigID == "" {
		log.Info("Host config ID is empty, waiting for host config assignment", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		if scope.RollingMigrationPlan != nil {
			scope.RollingMigrationPlan.Labels[constants.PauseMigrationLabel] = constants.PauseMigrationValue
			if err = r.Update(ctx, scope.RollingMigrationPlan); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to update RollingMigrationPlan")
			}
		}
		return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
	}
	if scope.RollingMigrationPlan != nil && len(scope.RollingMigrationPlan.Spec.ClusterMapping) > 0 {
		for _, mapping := range scope.RollingMigrationPlan.Spec.ClusterMapping {
			if mapping.VMwareClusterName == vmwareHost.Spec.ClusterName || mapping.VMwareClusterName == vmwareHost.Labels[constants.VMwareClusterLabel] {
				pcdClusterName = mapping.PCDClusterName
				break
			}
		}
	}
	if pcdClusterName == "" {
		log.Info("PCD cluster name is empty, pausing ESXi migration. please assign PCD cluster to ESXi to continue", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		pcdClusterList := &vjailbreakv1alpha1.PCDClusterList{}
		err = r.List(ctx, pcdClusterList)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to list PCD clusters")
		}
		if len(pcdClusterList.Items) == 0 {
			log.Info("No PCD clusters found, pausing ESXi migration. please create PCD cluster to continue", "esxiName", scope.ESXIMigration.Spec.ESXiName)
			return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
		}
		pcdClusterName = pcdClusterList.Items[0].Spec.ClusterName
	}

	if err := utils.AssignHostConfigToHost(ctx, r.Client, destOpenstackCreds.Name, vmwareHost.Spec.HardwareUUID, vmwareHost.Spec.HostConfigID); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to assign host config to PCD host")
	}
	if err := utils.AssignHypervisorRoleToHost(ctx, r.Client, destOpenstackCreds.Name, vmwareHost.Spec.HardwareUUID, pcdClusterName); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to assign hypervisor role to PCD host")
	}

	scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseAssigningRole
	err = r.Status().Update(ctx, scope.ESXIMigration)
	if err != nil {
		log.Error(err, "Failed to update ESXIMigration status", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
	}
	return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
}

func (r *ESXIMigrationReconciler) handleESXiAssigningRole(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("Waiting for hypervisor role assignment", "esxiName", scope.ESXIMigration.Spec.ESXiName)

	destOpenstackCreds, err := resolveOpenstackCreds(ctx, r.Client, scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get destination openstack credentials")
	}
	sourceVMwareCreds, err := resolveVMwareCreds(ctx, r.Client, scope.ESXIMigration, scope.RollingMigrationPlan)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get source vmware credentials")
	}
	vmwareHost, err := utils.GetVMwareHostFromESXiName(ctx, r.Client, scope.ESXIMigration.Spec.ESXiName, sourceVMwareCreds.Name)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get VMware host")
	}

	assigned, err := utils.WaitForHypervisorRoleAssignment(ctx, r.Client, destOpenstackCreds.Name, vmwareHost.Spec.HardwareUUID)
	if err != nil {
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseFailed
		if updateErr := r.Status().Update(ctx, scope.ESXIMigration); updateErr != nil {
			log.Error(updateErr, "Failed to update ESXIMigration status to Failed", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		}
		return ctrl.Result{}, errors.Wrap(err, "hypervisor role assignment failed")
	}
	if !assigned {
		return ctrl.Result{RequeueAfter: constants.CredsRequeueAfter}, nil
	}
	log.Info("Assigned Hypervisor Role to PCD Host", "hostName", vmwareHost.Spec.Name)

	scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseSucceeded
	err = r.Status().Update(ctx, scope.ESXIMigration)
	if err != nil {
		log.Error(err, "Failed to update ESXIMigration status", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
	}

	err = utils.RemoveESXiFromVCenter(ctx, r.Client, scope)
	if err != nil {
		log.Error(err, "Failed to remove ESXi from vCenter, retrying after one minute", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	log.Info("Successfully removed ESXi from vCenter", "esxiName", scope.ESXIMigration.Spec.ESXiName)

	return ctrl.Result{}, nil
}

func (r *ESXIMigrationReconciler) handleESXiInMaintenanceMode(ctx context.Context, scope *scope.ESXIMigrationScope) (ctrl.Result, error) {
	log := scope.Logger
	log.Info("ESXi is in maintenance mode", "esxiName", scope.ESXIMigration.Spec.ESXiName)
	vmCount, err := utils.CountVMsOnESXi(ctx, r.Client, scope)
	if err != nil {
		log.Error(err, "Failed to count VMs on ESXi", "esxiName", scope.ESXIMigration.Spec.ESXiName)
		return ctrl.Result{}, errors.Wrap(err, "failed to count VMs on ESXi")
	}
	log.Info("Counted VMs on ESXi", "esxiName", scope.ESXIMigration.Spec.ESXiName, "vmCount", vmCount)
	if vmCount != 0 {
		log.Info("VMs present on this ESXi host, waiting for VMs to be moved", "ESXiName", scope.ESXIMigration.Spec.ESXiName)
		// Omkar change back to 5 mins
		scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseWaitingForVMsToBeMoved
		err = r.Status().Update(ctx, scope.ESXIMigration)
		if err != nil {
			log.Error(err, "Failed to update ESXIMigration status")
			return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	log.Info("No VMs on this ESXi host, removing from vCenter and converting to PCD host", "ESXiName", scope.ESXIMigration.Spec.ESXiName)

	scope.ESXIMigration.Status.Phase = vjailbreakv1alpha1.ESXIMigrationPhaseCordoned
	err = r.Status().Update(ctx, scope.ESXIMigration)
	if err != nil {
		log.Error(err, "Failed to update ESXIMigration status")
		return ctrl.Result{}, errors.Wrap(err, "failed to update ESXi migration status")
	}
	log.Info("Successfully updated ESXIMigration status to cordoned")
	return ctrl.Result{}, nil
}
