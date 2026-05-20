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
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
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

const proxyVMSSHKeyPath = "/home/ubuntu/.ssh/id_rsa"

// ProxyVMReconciler reconciles a ProxyVM object
type ProxyVMReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=proxyvms/finalizers,verbs=update

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

	// Mark Verifying
	proxyVM.Status.ValidationStatus = constants.ProxyVMStatusVerifying
	proxyVM.Status.ValidationMessage = "Verifying Proxy VM components..."
	if err := r.Status().Update(ctx, proxyVM); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch VMwareCreds secret
	vmwCreds := &vjailbreakv1alpha1.VMwareCreds{}
	if err := r.Get(ctx, k8stypes.NamespacedName{
		Name:      proxyVM.Spec.VMwareCredsRef.Name,
		Namespace: proxyVM.Namespace,
	}, vmwCreds); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to get VMwareCreds %q: %v", proxyVM.Spec.VMwareCredsRef.Name, err))
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, k8stypes.NamespacedName{
		Name:      vmwCreds.Spec.SecretRef.Name,
		Namespace: proxyVM.Namespace,
	}, secret); err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to get vCenter secret: %v", err))
	}

	username, password, host, err := extractVCenterCredentials(secret)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("invalid vCenter credentials: %v", err))
	}

	// Connect to vCenter
	vcClient, err := vcenter.VCenterClientBuilder(ctx, username, password, host, true)
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

	// Check disk.EnableUUID
	enableUUID := false
	if vmProps.Config != nil {
		for _, opt := range vmProps.Config.ExtraConfig {
			if ov, ok := opt.(*govmomitypes.OptionValue); ok {
				if strings.EqualFold(ov.Key, "disk.enableUUID") {
					if v, ok := ov.Value.(string); ok && strings.EqualFold(v, "TRUE") {
						enableUUID = true
					}
				}
			}
		}
	}
	if !enableUUID {
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
		// Reboot the VM so the change takes effect
		rebootTask, err := vmObj.Reset(ctx)
		if err != nil {
			return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to reboot Proxy VM after setting disk.EnableUUID: %v", err))
		}
		if err := rebootTask.Wait(ctx); err != nil {
			return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed waiting for VM reboot: %v", err))
		}
		// Re-queue after reboot to allow VMware Tools to come back up
		proxyVM.Status.ValidationMessage = "disk.EnableUUID set and VM rebooted — re-verifying shortly"
		_ = r.Status().Update(ctx, proxyVM)
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	// Load the vJailbreak appliance SSH private key
	privateKey, err := os.ReadFile(proxyVMSSHKeyPath)
	if err != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to read SSH private key at %s: %v", proxyVMSSHKeyPath, err))
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

	// Verify all required components in a single SSH session.
	// Output format per line: "<name>:<path-or-MISSING>"
	checkCmd := `for cmd in ` + strings.Join(constants.ProxyVMRequiredComponents, " ") + `; do printf '%s:%s\n' "$cmd" "$(which "$cmd" 2>/dev/null || echo MISSING)"; done`
	checkOutput, execErr := sshClient.ExecuteCommand(checkCmd)
	if execErr != nil {
		return r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to run component check on Proxy VM: %v", execErr))
	}

	componentResults := make([]vjailbreakv1alpha1.ProxyVMComponentCheck, 0, len(constants.ProxyVMRequiredComponents))
	allPresent := true
	missingComponents := []string{}

	for _, line := range strings.Split(strings.TrimSpace(checkOutput), "\n") {
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
			missingComponents = append(missingComponents, name)
		} else {
			check.Present = true
		}
		componentResults = append(componentResults, check)
	}

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
