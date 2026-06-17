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
	"strings"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	commonutils "github.com/platform9/vjailbreak/pkg/common/utils"
	vmwarepkg "github.com/platform9/vjailbreak/pkg/common/vmware"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	govmomitypes "github.com/vmware/govmomi/vim25/types"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ProxyVMReconciler reconciles a ProxyVM object
type ProxyVMReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms/finalizers,verbs=update

// Reconcile reconciles a ProxyVM object.
func (r *ProxyVMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ProxyVMControllerName)

	proxyVM := &vjailbreakv1alpha1.ProxyVM{}
	if err := r.Get(ctx, req.NamespacedName, proxyVM); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !proxyVM.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, proxyVM)
	}

	ctxlog.Info("Reconciling ProxyVM", "name", proxyVM.Name)
	return r.reconcileNormal(ctx, proxyVM)
}

func (r *ProxyVMReconciler) reconcileNormal(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(proxyVM, constants.ProxyVMFinalizer) {
		controllerutil.AddFinalizer(proxyVM, constants.ProxyVMFinalizer)
		if err := r.Update(ctx, proxyVM); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip Verifying broadcast for already-Ready VMs to avoid spurious UI flips.
	if proxyVM.Status.ValidationStatus != constants.ProxyVMStatusReady {
		proxyVM.Status.ValidationStatus = constants.ProxyVMStatusVerifying
		proxyVM.Status.ValidationMessage = "Verifying Proxy VM components..."
		if err := r.Status().Update(ctx, proxyVM); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	}

	// Fetch vCenter credentials using the shared helper (VMwareCreds CR → secret → parsed fields).
	vcCreds, err := vmwarepkg.GetVMwareCredsInfo(ctx, r.Client, proxyVM.Spec.VMwareCredsRef.Name)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to get VMwareCreds %q: %v", proxyVM.Spec.VMwareCredsRef.Name, err))
	}

	// Connect to vCenter
	vcClient, err := vcenter.VCenterClientBuilder(ctx, vcCreds.Username, vcCreds.Password, vcCreds.Host, vcCreds.Insecure)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to connect to vCenter: %v", err))
	}

	// Find the VM and get its guest IP
	vmObj, err := vcClient.GetVMByName(ctx, proxyVM.Spec.VMName)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("VM %q not found in vCenter: %v", proxyVM.Spec.VMName, err))
	}

	// Fetch guest properties: ipAddress and extraConfig
	var vmProps mo.VirtualMachine
	if err := vmObj.Properties(ctx, vmObj.Reference(), []string{"guest.ipAddress", "config.extraConfig"}, &vmProps); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to fetch VM properties: %v", err))
	}

	ipAddress := ""
	if vmProps.Guest != nil {
		ipAddress = vmProps.Guest.IpAddress
	}
	if ipAddress == "" {
		return r.failVerification(ctx, proxyVM, "Proxy VM has no guest IP address — ensure VMware Tools is running and the VM is powered on")
	}
	proxyVM.Status.IPAddress = ipAddress
	ctxlog.Info("Discovered Proxy VM IP", "ip", ipAddress)

	if vmProps.Config == nil || !isDiskEnableUUIDSet(vmProps.Config.ExtraConfig) {
		return r.setDiskEnableUUIDAndReboot(ctx, proxyVM, vmObj)
	}

	// Resolve SSH private key: prefer explicit SSHKeyPairRef, fall back to legacy per-ProxyVM secret.
	sshSecretName := commonutils.HotAddSSHSecretName(proxyVM.Name)
	if proxyVM.Spec.SSHKeyPairRef != nil {
		sshSecretName = proxyVM.Spec.SSHKeyPairRef.Name
	}
	sshSecret := &corev1.Secret{}
	if err := r.Get(ctx, k8stypes.NamespacedName{
		Name:      sshSecretName,
		Namespace: proxyVM.Namespace,
	}, sshSecret); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to get SSH secret %q: %v", sshSecretName, err))
	}
	privateKey, ok := sshSecret.Data["ssh-privatekey"]
	if !ok || len(privateKey) == 0 {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("secret %q is missing 'ssh-privatekey' key", sshSecretName))
	}

	// Connect via SSH
	sshClient := esxissh.NewClientWithTimeout(30 * time.Second)
	connectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := sshClient.Connect(connectCtx, ipAddress, "root", privateKey); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("SSH connection to Proxy VM %s failed: %v", ipAddress, err))
	}
	defer func() {
		if err := sshClient.Disconnect(); err != nil {
			ctxlog.V(1).Info("Failed to disconnect SSH client", "ip", ipAddress, "error", err)
		}
	}()

	checkCmd := `for cmd in ` + strings.Join(constants.ProxyVMRequiredComponents, " ") + `; do printf '%s:%s\n' "$cmd" "$(which "$cmd" 2>/dev/null || echo MISSING)"; done`
	checkOutput, execErr := sshClient.ExecuteCommand(checkCmd)
	if execErr != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to run component check on Proxy VM: %v", execErr))
	}

	componentResults, missingComponents, allPresent := parseComponentCheckOutput(checkOutput)
	now := metav1.Now()
	proxyVM.Status.ComponentsVerified = componentResults
	proxyVM.Status.LastValidationTime = &now

	if !allPresent {
		msg := fmt.Sprintf("Missing required components: %s", strings.Join(missingComponents, ", "))
		proxyVM.Status.ValidationStatus = constants.ProxyVMStatusVerificationFailed
		proxyVM.Status.ValidationMessage = msg
		if err := r.Status().Update(ctx, proxyVM); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		ctxlog.Info("ProxyVM verification failed", "reason", msg)
		return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
	}

	proxyVM.Status.ValidationStatus = constants.ProxyVMStatusReady
	proxyVM.Status.ValidationMessage = "All components verified. disk.EnableUUID=True."
	if err := r.Status().Update(ctx, proxyVM); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctxlog.Info("ProxyVM verification succeeded", "name", proxyVM.Name, "ip", ipAddress)
	return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
}

func (r *ProxyVMReconciler) reconcileDelete(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	sshSecretName := proxyVM.Name + "-" + constants.HotAddSSHSecretSuffix
	sshSecret := &corev1.Secret{}
	if err := r.Get(ctx, k8stypes.NamespacedName{
		Name:      sshSecretName,
		Namespace: proxyVM.Namespace,
	}, sshSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			ctxlog.V(1).Info("Failed to get SSH secret for deletion", "secret", sshSecretName, "error", err)
		}
	} else {
		if err := r.Delete(ctx, sshSecret); err != nil && !apierrors.IsNotFound(err) {
			ctxlog.V(1).Info("Failed to delete SSH secret", "secret", sshSecretName, "error", err)
		}
	}

	if controllerutil.RemoveFinalizer(proxyVM, constants.ProxyVMFinalizer) {
		if err := r.Update(ctx, proxyVM); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// failVerification sets VerificationFailed status and requeues after 10 minutes.
func (r *ProxyVMReconciler) failVerification(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM, msg string) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("ProxyVM verification failed", "name", proxyVM.Name, "reason", msg)
	proxyVM.Status.ValidationStatus = constants.ProxyVMStatusVerificationFailed
	proxyVM.Status.ValidationMessage = msg
	now := metav1.Now()
	proxyVM.Status.LastValidationTime = &now
	if err := r.Status().Update(ctx, proxyVM); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, errors.Wrap(err, "failed to update ProxyVM status")
	}
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// isDiskEnableUUIDSet reports whether disk.enableUUID is set to TRUE in the VM's ExtraConfig.
func isDiskEnableUUIDSet(extraConfig []govmomitypes.BaseOptionValue) bool {
	for _, opt := range extraConfig {
		if ov, ok := opt.(*govmomitypes.OptionValue); ok {
			if strings.EqualFold(ov.Key, "disk.enableUUID") {
				if v, ok := ov.Value.(string); ok && strings.EqualFold(v, "TRUE") {
					return true
				}
			}
		}
	}
	return false
}

func (r *ProxyVMReconciler) setDiskEnableUUIDAndReboot(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM, vmObj *object.VirtualMachine) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("disk.EnableUUID not set on Proxy VM — setting it and rebooting", "vm", proxyVM.Spec.VMName)
	spec := govmomitypes.VirtualMachineConfigSpec{
		ExtraConfig: []govmomitypes.BaseOptionValue{
			&govmomitypes.OptionValue{Key: "disk.enableUUID", Value: "TRUE"},
		},
	}
	reconfTask, err := vmObj.Reconfigure(ctx, spec)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to set disk.EnableUUID: %v", err))
	}
	if err := reconfTask.Wait(ctx); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed waiting for disk.EnableUUID reconfigure: %v", err))
	}
	rebootTask, err := vmObj.Reset(ctx)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to reboot Proxy VM after setting disk.EnableUUID: %v", err))
	}
	if err := rebootTask.Wait(ctx); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed waiting for VM reboot: %v", err))
	}
	proxyVM.Status.ValidationMessage = "disk.EnableUUID set and VM rebooted — re-verifying shortly"
	if err := r.Status().Update(ctx, proxyVM); err != nil && !apierrors.IsNotFound(err) {
		ctxlog.V(1).Info("Failed to update status after reboot", "error", err)
	}
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func parseComponentCheckOutput(output string) ([]vjailbreakv1alpha1.ProxyVMComponentCheck, []string, bool) {
	results := make([]vjailbreakv1alpha1.ProxyVMComponentCheck, 0)
	missing := []string{}
	allPresent := true
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		name, path := parts[0], parts[1]
		check := vjailbreakv1alpha1.ProxyVMComponentCheck{Name: name}
		if path == "MISSING" || path == "" {
			check.Present = false
			check.Message = fmt.Sprintf("%s not found in PATH — install the required package", name)
			allPresent = false
			missing = append(missing, name)
		} else {
			check.Present = true
		}
		results = append(results, check)
	}
	return results, missing, allPresent
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxyVMReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ProxyVM{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
		)).
		Complete(r)
}
