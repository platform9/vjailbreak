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
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	commonconfig "github.com/platform9/vjailbreak/pkg/common/config"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/vmware/govmomi/object"
	ovfimporter "github.com/vmware/govmomi/ovf/importer"
	"github.com/vmware/govmomi/vim25/mo"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ovaDefaultSSHUser     = "root"
	ovaDefaultSSHPassword = "password"
	ovaSSHConnectTimeout  = 30 * time.Second
	ovaSSHConnectRetries  = 2 * time.Minute
	ovaIPWaitTimeout      = 5 * time.Minute
	ovaIPPollInterval     = 10 * time.Second
)

// deployOVAIfNeeded deploys the Proxy VM from an OVA template when DeploymentMode is "ova".
// Idempotent: if the VM already exists in vCenter the function returns nil immediately.
// After deployment the VM is powered on, its guest IP is awaited, and the SSH public key
// from SSHKeyPairRef is injected via the OVA default credentials (root/password).
func (r *ProxyVMReconciler) deployOVAIfNeeded(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM, vcClient *vcenter.VCenterClient) error {
	ctxlog := log.FromContext(ctx)

	// Idempotency check.
	if _, err := vcClient.GetVMByName(ctx, proxyVM.Spec.VMName); err == nil {
		ctxlog.Info("OVA deployment: VM already exists, skipping", "vm", proxyVM.Spec.VMName)
		return nil
	}

	ovaURL, err := r.resolveOVAURL(ctx, proxyVM)
	if err != nil {
		return err
	}

	ctxlog.Info("Downloading OVA", "url", ovaURL)
	ovaPath, err := downloadToTempFile(ctx, ovaURL)
	if err != nil {
		return fmt.Errorf("failed to download OVA from %s: %w", ovaURL, err)
	}
	defer os.Remove(ovaPath)

	imp, opts, err := r.buildOVAImporter(ctx, proxyVM, vcClient)
	if err != nil {
		return fmt.Errorf("failed to prepare OVA importer: %w", err)
	}
	imp.Archive = &ovfimporter.TapeArchive{Path: ovaPath}

	ctxlog.Info("Importing OVA into vCenter", "vm", proxyVM.Spec.VMName)
	moRef, err := imp.Import(ctx, "*.ovf", opts)
	if err != nil {
		return fmt.Errorf("OVA import failed: %w", err)
	}

	vmObj := object.NewVirtualMachine(vcClient.VCClient, *moRef)
	powerTask, err := vmObj.PowerOn(ctx)
	if err != nil {
		return fmt.Errorf("failed to power on deployed VM: %w", err)
	}
	if err := powerTask.Wait(ctx); err != nil {
		return fmt.Errorf("failed waiting for VM power-on: %w", err)
	}
	ctxlog.Info("Deployed VM powered on, waiting for guest IP", "vm", proxyVM.Spec.VMName)

	ip, err := waitForGuestIP(ctx, vmObj, ovaIPWaitTimeout, ovaIPPollInterval)
	if err != nil {
		return fmt.Errorf("deployed VM did not report a guest IP within timeout: %w", err)
	}
	ctxlog.Info("Deployed VM has guest IP", "vm", proxyVM.Spec.VMName, "ip", ip)

	if proxyVM.Spec.SSHKeyPairRef != nil {
		if err := r.injectSSHPublicKey(ctx, proxyVM, ip); err != nil {
			return fmt.Errorf("failed to inject SSH public key: %w", err)
		}
		ctxlog.Info("SSH public key injected into deployed VM", "ip", ip)
	}

	return nil
}

// resolveOVAURL returns the OVA URL from the ProxyVM spec or from the vjailbreak-settings ConfigMap.
func (r *ProxyVMReconciler) resolveOVAURL(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM) (string, error) {
	if proxyVM.Spec.OVADeploymentSpec != nil && proxyVM.Spec.OVADeploymentSpec.OVAURL != "" {
		return proxyVM.Spec.OVADeploymentSpec.OVAURL, nil
	}
	settings, err := commonconfig.GetVjailbreakSettings(ctx, r.Client)
	if err != nil {
		return "", fmt.Errorf("failed to read vjailbreak-settings: %w", err)
	}
	if settings.ProxyVMOVAURL == "" {
		return "", fmt.Errorf("no OVA URL configured: set spec.ovaDeploymentSpec.ovaURL or PROXY_VM_OVA_URL in vjailbreak-settings")
	}
	return settings.ProxyVMOVAURL, nil
}

// buildOVAImporter constructs an ovfimporter.Importer with vCenter targets resolved from the spec.
// The caller must set imp.Archive before calling Import.
func (r *ProxyVMReconciler) buildOVAImporter(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM, vcClient *vcenter.VCenterClient) (*ovfimporter.Importer, ovfimporter.Options, error) {
	spec := proxyVM.Spec.OVADeploymentSpec
	finder := vcClient.VCFinder
	ctxlog := log.FromContext(ctx)

	if spec != nil && spec.Datacenter != "" {
		dc, err := finder.Datacenter(ctx, spec.Datacenter)
		if err != nil {
			return nil, ovfimporter.Options{}, fmt.Errorf("datacenter %q not found: %w", spec.Datacenter, err)
		}
		finder.SetDatacenter(dc)
	}

	poolPath := "*"
	if spec != nil && spec.Cluster != "" {
		poolPath = spec.Cluster
	}
	pool, err := finder.ResourcePool(ctx, poolPath)
	if err != nil {
		return nil, ovfimporter.Options{}, fmt.Errorf("resource pool %q not found: %w", poolPath, err)
	}

	dsPath := "*"
	if spec != nil && spec.Datastore != "" {
		dsPath = spec.Datastore
	}
	ds, err := finder.Datastore(ctx, dsPath)
	if err != nil {
		return nil, ovfimporter.Options{}, fmt.Errorf("datastore %q not found: %w", dsPath, err)
	}

	var folder *object.Folder
	if spec != nil && spec.Folder != "" {
		folder, err = finder.Folder(ctx, spec.Folder)
		if err != nil {
			return nil, ovfimporter.Options{}, fmt.Errorf("folder %q not found: %w", spec.Folder, err)
		}
	}

	logFn := func(msg string) (int, error) {
		ctxlog.Info(strings.TrimRight(msg, "\n"))
		return len(msg), nil
	}

	imp := &ovfimporter.Importer{
		Client:       vcClient.VCClient,
		Finder:       finder,
		ResourcePool: pool,
		Datastore:    ds,
		Folder:       folder,
		Log:          logFn,
	}

	vmName := proxyVM.Spec.VMName
	opts := ovfimporter.Options{
		Name:    &vmName,
		PowerOn: false, // powered on manually so we can capture the MoRef
	}
	if spec != nil && spec.Network != "" {
		opts.NetworkMapping = []ovfimporter.Network{
			{Name: "VM Network", Network: spec.Network},
		}
	}

	return imp, opts, nil
}

// downloadToTempFile downloads the OVA at url into a temp file and returns its path.
// The caller is responsible for removing the file.
func downloadToTempFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching OVA from %s", resp.StatusCode, url)
	}

	f, err := os.CreateTemp("", "vjailbreak-ova-*.ova")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("failed to write OVA: %w", err)
	}
	return f.Name(), nil
}

// waitForGuestIP polls the VM until VMware Tools reports a guest IP or the timeout expires.
func waitForGuestIP(ctx context.Context, vmObj *object.VirtualMachine, timeout, interval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var props mo.VirtualMachine
		if err := vmObj.Properties(ctx, vmObj.Reference(), []string{"guest.ipAddress"}, &props); err == nil {
			if props.Guest != nil && props.Guest.IpAddress != "" {
				return props.Guest.IpAddress, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}
	return "", fmt.Errorf("timed out waiting for guest IP after %s", timeout)
}

// injectSSHPublicKey connects to ip using the OVA default credentials and appends the
// public key from proxyVM.Spec.SSHKeyPairRef to ~/.ssh/authorized_keys.
func (r *ProxyVMReconciler) injectSSHPublicKey(ctx context.Context, proxyVM *vjailbreakv1alpha1.ProxyVM, ip string) error {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, k8stypes.NamespacedName{
		Name:      proxyVM.Spec.SSHKeyPairRef.Name,
		Namespace: proxyVM.Namespace,
	}, secret); err != nil {
		return fmt.Errorf("failed to get SSH key pair secret %q: %w", proxyVM.Spec.SSHKeyPairRef.Name, err)
	}
	pubKey, ok := secret.Data["ssh-publickey"]
	if !ok || len(pubKey) == 0 {
		return fmt.Errorf("secret %q missing 'ssh-publickey'", proxyVM.Spec.SSHKeyPairRef.Name)
	}

	cfg := &ssh.ClientConfig{
		User:            ovaDefaultSSHUser,
		Auth:            []ssh.AuthMethod{ssh.Password(ovaDefaultSSHPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // OVA first-boot credential injection
		Timeout:         ovaSSHConnectTimeout,
	}

	addr := ip + ":22"
	var sshClient *ssh.Client
	deadline := time.Now().Add(ovaSSHConnectRetries)
	for {
		var dialErr error
		sshClient, dialErr = ssh.Dial("tcp", addr, cfg)
		if dialErr == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("SSH connection to %s failed after retries: %w", addr, dialErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	defer sshClient.Close()

	sess, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to open SSH session: %w", err)
	}
	defer sess.Close()

	pubKeyLine := strings.TrimRight(string(pubKey), "\n\r") + "\n"
	cmd := fmt.Sprintf(
		`mkdir -p ~/.ssh && chmod 700 ~/.ssh && printf '%%s' %q >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
		pubKeyLine,
	)
	if out, err := sess.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("public key injection failed: %w — output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
