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
	"sync"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ESXiSSHCredsReconciler reconciles an ESXiSSHCreds object
type ESXiSSHCredsReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	MaxConcurrentReconciles int
}

// hostValidationResult is used internally to collect validation results
type hostValidationResult struct {
	vmwareHost  *vjailbreakv1alpha1.VMwareHost
	hostname    string
	status      string
	message     string
	esxiVersion string
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esxisshcreds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esxisshcreds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=esxisshcreds/finalizers,verbs=update
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarehosts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=vmwarehosts/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ESXiSSHCredsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctxlog := log.FromContext(ctx).WithName(constants.ESXiSSHCredsControllerName)

	// Get the ESXiSSHCreds object
	esxiSSHCreds := &vjailbreakv1alpha1.ESXiSSHCreds{}
	if err := r.Get(ctx, req.NamespacedName, esxiSSHCreds); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Resource not found, likely deleted", "esxisshcreds", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to get ESXiSSHCreds resource", "esxisshcreds", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !esxiSSHCreds.DeletionTimestamp.IsZero() {
		ctxlog.Info("Resource is being deleted, reconciling deletion", "esxisshcreds", req.NamespacedName)
		return r.reconcileDelete(ctx, esxiSSHCreds)
	}

	ctxlog.Info("Reconciling normal state", "esxisshcreds", req.NamespacedName)
	return r.reconcileNormal(ctx, esxiSSHCreds)
}

func (r *ESXiSSHCredsReconciler) reconcileNormal(ctx context.Context, esxiSSHCreds *vjailbreakv1alpha1.ESXiSSHCreds) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Reconciling ESXiSSHCreds", "name", esxiSSHCreds.Name)

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(esxiSSHCreds, constants.ESXiSSHCredsFinalizer) {
		controllerutil.AddFinalizer(esxiSSHCreds, constants.ESXiSSHCredsFinalizer)
		if err := r.Update(ctx, esxiSSHCreds); err != nil {
			ctxlog.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status to Validating
	esxiSSHCreds.Status.ValidationStatus = constants.ESXiSSHCredsStatusValidating
	esxiSSHCreds.Status.ValidationMessage = "Validating SSH connectivity to ESXi hosts..."
	if err := r.Status().Update(ctx, esxiSSHCreds); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get SSH credentials from secret
	sshCreds, err := r.getSSHCredentialsFromSecret(ctx, esxiSSHCreds)
	if err != nil {
		ctxlog.Error(err, "Failed to get SSH credentials from secret")
		esxiSSHCreds.Status.ValidationStatus = constants.ESXiSSHCredsStatusFailed
		esxiSSHCreds.Status.ValidationMessage = fmt.Sprintf("Failed to get SSH credentials: %v", err)
		if updateErr := r.Status().Update(ctx, esxiSSHCreds); updateErr != nil {
			if !apierrors.IsNotFound(updateErr) {
				ctxlog.Error(updateErr, "Failed to update status")
			}
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Get list of VMwareHosts to validate
	vmwareHosts, err := r.getVMwareHosts(ctx)
	if err != nil {
		ctxlog.Error(err, "Failed to get VMwareHosts")
		esxiSSHCreds.Status.ValidationStatus = constants.ESXiSSHCredsStatusFailed
		esxiSSHCreds.Status.ValidationMessage = fmt.Sprintf("Failed to get VMwareHosts: %v", err)
		if updateErr := r.Status().Update(ctx, esxiSSHCreds); updateErr != nil {
			if !apierrors.IsNotFound(updateErr) {
				ctxlog.Error(updateErr, "Failed to update status")
			}
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	if len(vmwareHosts) == 0 {
		ctxlog.Info("No VMwareHosts found to validate, will retry")
		esxiSSHCreds.Status.ValidationStatus = constants.ESXiSSHCredsStatusPending
		esxiSSHCreds.Status.ValidationMessage = "No VMwareHosts found. Waiting for VMwareCreds to discover hosts."
		esxiSSHCreds.Status.TotalHosts = 0
		esxiSSHCreds.Status.SuccessfulHosts = 0
		esxiSSHCreds.Status.FailedHosts = 0
		if updateErr := r.Status().Update(ctx, esxiSSHCreds); updateErr != nil {
			if !apierrors.IsNotFound(updateErr) {
				ctxlog.Error(updateErr, "Failed to update status")
			}
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	ctxlog.Info("Validating ESXi hosts", "count", len(vmwareHosts))

	// Validate SSH connectivity to all hosts in parallel with throttling
	results := r.validateHostsParallel(ctx, vmwareHosts, sshCreds)

	// Process results and update VMwareHost statuses
	successCount := 0
	failCount := 0

	for _, result := range results {
		// Update VMwareHost status
		if result.vmwareHost != nil {
			result.vmwareHost.Status.SSHStatus = result.status
			result.vmwareHost.Status.SSHMessage = result.message
			result.vmwareHost.Status.SSHLastChecked = metav1.Now()
			result.vmwareHost.Status.ESXiVersion = result.esxiVersion

			if err := r.Status().Update(ctx, result.vmwareHost); err != nil {
				if !apierrors.IsNotFound(err) {
					ctxlog.Error(err, "Failed to update VMwareHost status", "vmwareHost", result.vmwareHost.Name)
				}
			}
		}

		if result.status == constants.ESXiSSHCredsStatusSucceeded {
			successCount++
		} else {
			failCount++
		}
	}

	// Determine overall status
	var overallStatus, overallMessage string
	switch {
	case failCount == 0:
		overallStatus = constants.ESXiSSHCredsStatusSucceeded
		overallMessage = fmt.Sprintf("Successfully validated SSH connectivity to all %d ESXi hosts", len(vmwareHosts))
	case successCount == 0:
		overallStatus = constants.ESXiSSHCredsStatusFailed
		overallMessage = fmt.Sprintf("Failed to validate SSH connectivity to all %d ESXi hosts", len(vmwareHosts))
	default:
		overallStatus = constants.ESXiSSHCredsStatusPartiallySucceeded
		overallMessage = fmt.Sprintf("SSH validation partially succeeded: %d/%d hosts passed, %d failed", successCount, len(vmwareHosts), failCount)
	}

	// Update ESXiSSHCreds status (summary only)
	esxiSSHCreds.Status.ValidationStatus = overallStatus
	esxiSSHCreds.Status.ValidationMessage = overallMessage
	esxiSSHCreds.Status.TotalHosts = len(vmwareHosts)
	esxiSSHCreds.Status.SuccessfulHosts = successCount
	esxiSSHCreds.Status.FailedHosts = failCount
	esxiSSHCreds.Status.LastValidationTime = metav1.Now()

	if err := r.Status().Update(ctx, esxiSSHCreds); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	ctxlog.Info("ESXi SSH validation completed",
		"status", overallStatus,
		"total", len(vmwareHosts),
		"successful", successCount,
		"failed", failCount)

	// Requeue periodically to re-validate
	return ctrl.Result{RequeueAfter: 15 * time.Minute}, nil
}

func (r *ESXiSSHCredsReconciler) reconcileDelete(ctx context.Context, esxiSSHCreds *vjailbreakv1alpha1.ESXiSSHCreds) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Reconciling deletion", "esxisshcreds", esxiSSHCreds.Name)

	// Remove finalizer
	if controllerutil.RemoveFinalizer(esxiSSHCreds, constants.ESXiSSHCredsFinalizer) {
		if err := r.Update(ctx, esxiSSHCreds); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			ctxlog.Error(err, "failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getSSHCredentialsFromSecret retrieves SSH credentials from the referenced secret
func (r *ESXiSSHCredsReconciler) getSSHCredentialsFromSecret(ctx context.Context, esxiSSHCreds *vjailbreakv1alpha1.ESXiSSHCreds) (*vjailbreakv1alpha1.ESXiSSHCredsInfo, error) {
	secretName := esxiSSHCreds.Spec.SecretRef.Name
	secretNamespace := esxiSSHCreds.Spec.SecretRef.Namespace
	if secretNamespace == "" {
		secretNamespace = constants.NamespaceMigrationSystem
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: secretNamespace}, secret); err != nil {
		return nil, errors.Wrapf(err, "failed to get secret %s/%s", secretNamespace, secretName)
	}

	privateKey, ok := secret.Data["privateKey"]
	if !ok {
		// Try alternate key names
		privateKey, ok = secret.Data["ssh-privatekey"]
		if !ok {
			privateKey, ok = secret.Data["id_rsa"]
			if !ok {
				return nil, fmt.Errorf("secret %s/%s does not contain 'privateKey', 'ssh-privatekey', or 'id_rsa' key", secretNamespace, secretName)
			}
		}
	}

	username := esxiSSHCreds.Spec.Username
	if username == "" {
		username = "root"
	}

	return &vjailbreakv1alpha1.ESXiSSHCredsInfo{
		Username:   username,
		PrivateKey: privateKey,
	}, nil
}

// getVMwareHosts returns the list of VMwareHost objects to validate
func (r *ESXiSSHCredsReconciler) getVMwareHosts(ctx context.Context) ([]*vjailbreakv1alpha1.VMwareHost, error) {
	ctxlog := log.FromContext(ctx)

	// List all VMwareHosts in the system
	vmwareHostList := &vjailbreakv1alpha1.VMwareHostList{}
	if err := r.List(ctx, vmwareHostList); err != nil {
		return nil, errors.Wrap(err, "failed to list VMwareHosts")
	}

	hosts := make([]*vjailbreakv1alpha1.VMwareHost, 0, len(vmwareHostList.Items))
	for i := range vmwareHostList.Items {
		vmwareHost := &vmwareHostList.Items[i]
		// VMwareHost.Spec.Name contains the ESXi hostname (IP or FQDN)
		if vmwareHost.Spec.Name != "" {
			hosts = append(hosts, vmwareHost)
		} else {
			ctxlog.Info("VMwareHost has no hostname, skipping", "vmwareHost", vmwareHost.Name)
		}
	}

	ctxlog.Info("Discovered VMwareHosts", "hostCount", len(hosts))
	return hosts, nil
}

// validateHostsParallel validates SSH connectivity to multiple hosts in parallel with throttling
func (r *ESXiSSHCredsReconciler) validateHostsParallel(ctx context.Context, vmwareHosts []*vjailbreakv1alpha1.VMwareHost, sshCreds *vjailbreakv1alpha1.ESXiSSHCredsInfo) []hostValidationResult {
	ctxlog := log.FromContext(ctx)

	results := make([]hostValidationResult, len(vmwareHosts))
	resultsChan := make(chan struct {
		index  int
		result hostValidationResult
	}, len(vmwareHosts))

	// Create a semaphore to limit concurrent validations
	semaphore := make(chan struct{}, constants.ESXiSSHValidationConcurrency)

	var wg sync.WaitGroup

	for i, vmwareHost := range vmwareHosts {
		wg.Add(1)
		go func(index int, host *vjailbreakv1alpha1.VMwareHost) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			hostname := host.Spec.Name

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				resultsChan <- struct {
					index  int
					result hostValidationResult
				}{
					index: index,
					result: hostValidationResult{
						vmwareHost: host,
						hostname:   hostname,
						status:     constants.ESXiSSHCredsStatusFailed,
						message:    "Validation cancelled",
					},
				}
				return
			default:
			}

			// Validate this host
			result := r.validateSingleHost(ctx, hostname, sshCreds)
			result.vmwareHost = host
			resultsChan <- struct {
				index  int
				result hostValidationResult
			}{
				index:  index,
				result: result,
			}
		}(i, vmwareHost)
	}

	// Wait for all validations to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for res := range resultsChan {
		results[res.index] = res.result
		ctxlog.V(1).Info("Host validation result",
			"host", res.result.hostname,
			"status", res.result.status,
			"message", res.result.message)
	}

	return results
}

// validateSingleHost validates SSH connectivity to a single ESXi host
func (r *ESXiSSHCredsReconciler) validateSingleHost(ctx context.Context, hostname string, sshCreds *vjailbreakv1alpha1.ESXiSSHCredsInfo) hostValidationResult {
	ctxlog := log.FromContext(ctx)
	ctxlog.V(1).Info("Validating SSH connectivity", "host", hostname)

	result := hostValidationResult{
		hostname: hostname,
		status:   constants.ESXiSSHCredsStatusFailed,
	}

	// Create SSH client with a reasonable timeout
	sshClient := esxissh.NewClientWithTimeout(30 * time.Second)

	// Create a context with timeout for the connection
	connectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Connect to the ESXi host
	if err := sshClient.Connect(connectCtx, hostname, sshCreds.Username, sshCreds.PrivateKey); err != nil {
		result.message = fmt.Sprintf("SSH connection failed: %v", err)
		ctxlog.V(1).Info("SSH connection failed", "host", hostname, "error", err)
		return result
	}
	defer func() {
		if err := sshClient.Disconnect(); err != nil {
			ctxlog.V(1).Info("Failed to disconnect SSH client", "host", hostname, "error", err)
		}
	}()

	// Test the connection by running a simple command
	if err := sshClient.TestConnection(); err != nil {
		result.message = fmt.Sprintf("SSH connection test failed: %v", err)
		ctxlog.V(1).Info("SSH connection test failed", "host", hostname, "error", err)
		return result
	}

	// Try to get ESXi version
	output, err := sshClient.ExecuteCommand("esxcli system version get")
	if err == nil && output != "" {
		// Parse version from output
		version := parseESXiVersion(output)
		result.esxiVersion = version
	}

	result.status = constants.ESXiSSHCredsStatusSucceeded
	result.message = "SSH connectivity validated successfully"
	ctxlog.V(1).Info("SSH validation succeeded", "host", hostname, "version", result.esxiVersion)

	return result
}

// parseESXiVersion extracts the ESXi version from esxcli output
func parseESXiVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Version:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		}
	}
	return ""
}

// SetupWithManager sets up the controller with the Manager
func (r *ESXiSSHCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.ESXiSSHCreds{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Complete(r)
}
