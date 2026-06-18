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

// proxyVMVCState holds the vCenter discovery results for a Proxy VM.
type proxyVMVCState struct {
	ip      string
	vmObj   *object.VirtualMachine
	uuidSet bool
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

	state, res, done, err := r.discoverProxyVMState(ctx, proxyVM)
	if done {
		return res, err
	}
	ctxlog.Info("Discovered Proxy VM IP", "ip", state.ip)

	if !state.uuidSet {
		return r.setDiskEnableUUIDAndReboot(ctx, proxyVM, state.vmObj)
	}

	privateKey, res, done, err := r.loadSSHKey(ctx, proxyVM)
	if done {
		return res, err
	}

	sshClient := esxissh.NewClientWithTimeout(30 * time.Second)
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if connErr := sshClient.Connect(connectCtx, state.ip, "root", privateKey); connErr != nil {
		proxyVM.Status.ValidationMessage = fmt.Sprintf("SSH connection to Proxy VM %s failed (retrying): %v", state.ip, connErr)
		if updateErr := r.Status().Update(ctx, proxyVM); updateErr != nil && !apierrors.IsNotFound(updateErr) {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	defer func() {
		if discErr := sshClient.Disconnect(); discErr != nil {
			ctxlog.V(1).Info("Failed to disconnect SSH client", "ip", state.ip, "error", discErr)
		}
	}()

	componentResults, res, done, err := r.checkAndInstallComponents(ctx, proxyVM, sshClient)
	if done {
		return res, err
	}

	now := metav1.Now()
	proxyVM.Status.LastValidationTime = &now
	proxyVM.Status.ComponentsVerified = componentResults
	proxyVM.Status.ValidationStatus = constants.ProxyVMStatusReady
	proxyVM.Status.ValidationMessage = "All components verified. disk.EnableUUID=True."
	if err := r.Status().Update(ctx, proxyVM); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ctxlog.Info("ProxyVM verification succeeded", "name", proxyVM.Name, "ip", state.ip)
	return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
}

// discoverProxyVMState connects to vCenter, finds the Proxy VM, and returns its IP and
// disk.EnableUUID status. When done=true the caller must return (res, err) immediately.
func (r *ProxyVMReconciler) discoverProxyVMState(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM) (*proxyVMVCState, ctrl.Result, bool, error) {
	vcCreds, vcErr := vmwarepkg.GetVMwareCredsInfo(ctx, r.Client, proxyVM.Spec.VMwareCredsRef.Name)
	if vcErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to get VMwareCreds %q: %v", proxyVM.Spec.VMwareCredsRef.Name, vcErr))
		return nil, res, true, err
	}

	vcClient, vcErr := vcenter.VCenterClientBuilder(ctx, vcCreds.Username, vcCreds.Password, vcCreds.Host, vcCreds.Insecure)
	if vcErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to connect to vCenter: %v", vcErr))
		return nil, res, true, err
	}

	vmObj, vcErr := vcClient.GetVMByName(ctx, proxyVM.Spec.VMName)
	if vcErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("VM %q not found in vCenter: %v", proxyVM.Spec.VMName, vcErr))
		return nil, res, true, err
	}

	var vmProps mo.VirtualMachine
	if vcErr = vmObj.Properties(ctx, vmObj.Reference(), []string{"guest.ipAddress", "config.extraConfig"}, &vmProps); vcErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to fetch VM properties: %v", vcErr))
		return nil, res, true, err
	}

	ip := ""
	if vmProps.Guest != nil {
		ip = vmProps.Guest.IpAddress
	}
	if ip == "" {
		proxyVM.Status.ValidationMessage = "Waiting for Proxy VM guest IP (VMware Tools starting)..."
		if updErr := r.Status().Update(ctx, proxyVM); updErr != nil && !apierrors.IsNotFound(updErr) {
			return nil, ctrl.Result{}, true, updErr
		}
		return nil, ctrl.Result{RequeueAfter: 15 * time.Second}, true, nil
	}
	proxyVM.Status.IPAddress = ip

	uuidSet := vmProps.Config != nil && isDiskEnableUUIDSet(vmProps.Config.ExtraConfig)
	return &proxyVMVCState{ip: ip, vmObj: vmObj, uuidSet: uuidSet}, ctrl.Result{}, false, nil
}

// loadSSHKey resolves and returns the SSH private key for the Proxy VM.
// When done=true the caller must return (res, err) immediately.
func (r *ProxyVMReconciler) loadSSHKey(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM) ([]byte, ctrl.Result, bool, error) {
	sshSecretName := commonutils.HotAddSSHSecretName(proxyVM.Name)
	if proxyVM.Spec.SSHKeyPairRef != nil {
		sshSecretName = proxyVM.Spec.SSHKeyPairRef.Name
	}
	sshSecret := &corev1.Secret{}
	if getErr := r.Get(ctx, k8stypes.NamespacedName{Name: sshSecretName, Namespace: proxyVM.Namespace}, sshSecret); getErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to get SSH secret %q: %v", sshSecretName, getErr))
		return nil, res, true, err
	}
	key, ok := sshSecret.Data["ssh-privatekey"]
	if !ok || len(key) == 0 {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("secret %q is missing 'ssh-privatekey' key", sshSecretName))
		return nil, res, true, err
	}
	return key, ctrl.Result{}, false, nil
}

// checkAndInstallComponents runs the required-component check on the Proxy VM via SSH
// and attempts auto-installation of any missing packages.
// When done=true the caller must return (res, err) immediately.
func (r *ProxyVMReconciler) checkAndInstallComponents(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM, sshClient *esxissh.Client) ([]vjailbreakv1alpha1.ProxyVMComponentCheck, ctrl.Result, bool, error) {
	ctxlog := log.FromContext(ctx)

	checkCmd := `for cmd in ` + strings.Join(constants.ProxyVMRequiredComponents, " ") + `; do printf '%s:%s\n' "$cmd" "$(which "$cmd" 2>/dev/null || echo MISSING)"; done`
	checkOutput, execErr := sshClient.ExecuteCommand(checkCmd)
	if execErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("failed to run component check on Proxy VM: %v", execErr))
		return nil, res, true, err
	}

	componentResults, missingComponents, allPresent := parseComponentCheckOutput(checkOutput)
	if allPresent {
		return componentResults, ctrl.Result{}, false, nil
	}

	if !checkInternetReachable(sshClient) {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("missing components [%s] and internet not reachable for auto-install", strings.Join(missingComponents, ", ")))
		return nil, res, true, err
	}
	rawDistro, distroErr := detectOSDistro(sshClient)
	if distroErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("missing components and OS detection failed: %v", distroErr))
		return nil, res, true, err
	}
	distro, ok := normaliseOSDistro(rawDistro)
	if !ok {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("missing components and OS %q is not supported for auto-install", rawDistro))
		return nil, res, true, err
	}
	installCmd := proxyVMInstallCmds[distro]
	ctxlog.Info("auto-installing missing components", "distro", distro, "rawDistro", rawDistro, "missing", missingComponents)
	if installErr := installMissingComponents(sshClient, installCmd); installErr != nil {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("auto-install on %q failed: %v", distro, installErr))
		return nil, res, true, err
	}
	componentResults, missingComponents, allPresent = reVerifyComponents(sshClient)
	if !allPresent {
		res, err := r.failVerification(ctx, proxyVM, fmt.Sprintf("components still missing after auto-install: %s", strings.Join(missingComponents, ", ")))
		return nil, res, true, err
	}
	return componentResults, ctrl.Result{}, false, nil
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
	return ctrl.Result{RequeueAfter: 90 * time.Second}, nil
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

// proxyVMInstallCmds maps a normalised OS distro constant to the command that installs qemu-nbd.
var proxyVMInstallCmds = map[string]string{
	constants.ProxyVMOSDistroDebian: "apt-get install -y qemu-utils",
	constants.ProxyVMOSDistroAlpine: "apk add --no-cache qemu-nbd",
}

// normaliseOSDistro maps a raw /etc/os-release ID value to a normalised distro constant.
// Returns ("", false) for unsupported distributions.
func normaliseOSDistro(rawID string) (string, bool) {
	id := strings.ToLower(strings.TrimSpace(rawID))
	switch {
	case strings.Contains(id, "debian"), strings.Contains(id, "ubuntu"), strings.Contains(id, "mint"):
		return constants.ProxyVMOSDistroDebian, true
	case strings.Contains(id, "alpine"):
		return constants.ProxyVMOSDistroAlpine, true
	}
	return "", false
}

func checkInternetReachable(sshClient *esxissh.Client) bool {
	out, err := sshClient.ExecuteCommand("curl -sf --max-time 5 https://8.8.8.8 > /dev/null 2>&1 && echo ok || echo fail")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "ok"
}

func detectOSDistro(sshClient *esxissh.Client) (string, error) {
	out, err := sshClient.ExecuteCommand("cat /etc/os-release 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("failed to read /etc/os-release: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ID=") {
			continue
		}
		id := strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), `"' `))
		if id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("ID field not found in /etc/os-release")
}

func installMissingComponents(sshClient *esxissh.Client, installCmd string) error {
	out, err := sshClient.ExecuteCommand(installCmd)
	if err != nil {
		return fmt.Errorf("%v\noutput: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func reVerifyComponents(sshClient *esxissh.Client) ([]vjailbreakv1alpha1.ProxyVMComponentCheck, []string, bool) {
	checkCmd := `for cmd in ` + strings.Join(constants.ProxyVMRequiredComponents, " ") + `; do printf '%s:%s\n' "$cmd" "$(which "$cmd" 2>/dev/null || echo MISSING)"; done`
	out, err := sshClient.ExecuteCommand(checkCmd)
	if err != nil {
		return nil, constants.ProxyVMRequiredComponents, false
	}
	return parseComponentCheckOutput(out)
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
