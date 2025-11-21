package vmware

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	sdkPath                  = "/sdk"
	namespaceMigrationSystem = "migration-system"
	defaultMaxRetries        = 5
	vjailbreakSettingsName   = "vjailbreak-settings"
	retryLimitKey            = "VCENTER_LOGIN_RETRY_LIMIT"
)

var vmwareClientMap *sync.Map

// ValidationResult holds the outcome of credential validation
type ValidationResult struct {
	Valid   bool
	Message string
	Error   error
}

// getRetryLimitFromSettings fetches VCENTER_LOGIN_RETRY_LIMIT from vjailbreak-settings
func getRetryLimitFromSettings(ctx context.Context, k8sClient client.Client) int {
	configMap := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, k8stypes.NamespacedName{
		Name:      vjailbreakSettingsName,
		Namespace: namespaceMigrationSystem,
	}, configMap)
	
	if err != nil {
		return defaultMaxRetries
	}
	
	if retryLimitStr, ok := configMap.Data[retryLimitKey]; ok {
		var retryLimit int
		if _, err := fmt.Sscanf(retryLimitStr, "%d", &retryLimit); err == nil && retryLimit > 0 {
			return retryLimit
		}
	}
	
	return defaultMaxRetries
}

// Validate performs complete VMware credential validation with all edge cases
func Validate(ctx context.Context, k8sClient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, maxRetries ...int) ValidationResult {
	// Determine retry limit: use passed value, else fetch from ConfigMap, else use default
	retryLimit := defaultMaxRetries
	if len(maxRetries) > 0 && maxRetries[0] > 0 {
		retryLimit = maxRetries[0]
	} else {
		retryLimit = getRetryLimitFromSettings(ctx, k8sClient)
	}

	// Get credentials from secret
	vmwareCredsinfo, err := getCredentialsFromSecret(ctx, k8sClient, vmwcreds.Spec.SecretRef.Name)
	if err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to get vCenter credentials from secret: %s", err.Error()),
			Error:   err,
		}
	}

	host := vmwareCredsinfo.Host
	username := vmwareCredsinfo.Username
	password := vmwareCredsinfo.Password
	disableSSLVerification := vmwareCredsinfo.Insecure
	datacenter := vmwareCredsinfo.Datacenter

	// Normalize URL
	if host[:4] != "http" {
		host = "https://" + host
	}
	if host[len(host)-4:] != sdkPath {
		host += sdkPath
	}

	u, err := url.Parse(host)
	if err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to parse URL: %s", err.Error()),
			Error:   err,
		}
	}
	u.User = url.UserPassword(username, password)

	// Connect and log in to ESX or vCenter
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
		Reauth:   true,
	}

	mapKey := string(vmwcreds.UID)
	var c *vim25.Client

	// Initialize map if needed
	if vmwareClientMap == nil {
		vmwareClientMap = &sync.Map{}
	}

	// Check cache for existing authenticated client
	if val, ok := vmwareClientMap.Load(mapKey); ok {
		cachedClient, valid := val.(*vim25.Client)
		if valid && cachedClient != nil && cachedClient.Client != nil {
			c = cachedClient
			sessMgr := session.NewManager(c)
			userSession, err := sessMgr.UserSession(ctx)
			if err == nil && userSession != nil {
				// Cached client is still valid
				return ValidationResult{
					Valid:   true,
					Message: "Successfully authenticated to VMware",
					Error:   nil,
				}
			}
			// Cached client is no longer valid, remove it
			vmwareClientMap.Delete(mapKey)
			c = nil // Will create fresh client below
		}
	}

	// Exponential retry logic with retry limit from ConfigMap or passed parameter
	var lastErr error
	ctxlog := log.FromContext(ctx)

	for attempt := 1; attempt <= retryLimit; attempt++ {
		// Ensure we have a client for this attempt
		if c == nil {
			c = new(vim25.Client)
		}
		err = s.Login(ctx, c, nil)
		if err == nil {
			// Login successful
			ctxlog.Info("Login successful", "attempt", attempt)
			break
		} else if strings.Contains(err.Error(), "incorrect user name or password") {
			return ValidationResult{
				Valid:   false,
				Message: "Authentication failed: invalid username or password. Please verify your credentials",
				Error:   err,
			}
		}
		// Save the error and log it
		lastErr = err
		ctxlog.Info("Login attempt failed", "attempt", attempt, "error", err)
		// Retry with exponential backoff
		if attempt < retryLimit {
			delayNum := math.Pow(2, float64(attempt)) * 500
			ctxlog.Info("Retrying login after delay", "delayMs", delayNum)
			time.Sleep(time.Duration(delayNum) * time.Millisecond)
			c = nil // Force fresh client on next attempt
		}
	}

	// Check if all login attempts failed
	if lastErr != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to login to vCenter after %d attempts: %s", retryLimit, lastErr.Error()),
			Error:   lastErr,
		}
	}

	// Check if the datacenter exists
	finder := find.NewFinder(c, false)
	_, err = finder.Datacenter(context.Background(), datacenter)
	if err != nil {
		return ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("Failed to find datacenter: %s", err.Error()),
			Error:   err,
		}
	}

	// All validations passed - cache the fully validated client
	vmwareClientMap.Store(mapKey, c)

	return ValidationResult{
		Valid:   true,
		Message: "Successfully authenticated to VMware",
		Error:   nil,
	}
}

// getCredentialsFromSecret retrieves VMware credentials from a Kubernetes secret
func getCredentialsFromSecret(ctx context.Context, k8sClient client.Client, secretName string) (vjailbreakv1alpha1.VMwareCredsInfo, error) {
	var vmwareCredsInfo vjailbreakv1alpha1.VMwareCredsInfo
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: secretName, Namespace: namespaceMigrationSystem}, secret)
	if err != nil {
		return vmwareCredsInfo, errors.Wrap(err, "failed to get secret")
	}

	if secret.Data == nil {
		return vmwareCredsInfo, fmt.Errorf("no data in secret '%s'", secretName)
	}

	vmwareCredsInfo.Host = string(secret.Data["VCENTER_HOST"])
	vmwareCredsInfo.Username = string(secret.Data["VCENTER_USERNAME"])
	vmwareCredsInfo.Password = string(secret.Data["VCENTER_PASSWORD"])
	vmwareCredsInfo.Datacenter = string(secret.Data["VCENTER_DATACENTER"])

	if vmwareCredsInfo.Host == "" {
		return vmwareCredsInfo, fmt.Errorf("VCENTER_HOST is missing in secret '%s'", secretName)
	}

	insecureStr := string(secret.Data["VCENTER_INSECURE"])
	vmwareCredsInfo.Insecure = strings.EqualFold(strings.TrimSpace(insecureStr), "true")

	return vmwareCredsInfo, nil
}

// PostValidate performs resource discovery after successful VMware validation
func PostValidate(ctx context.Context, k8sClient client.Client, vmwareCreds *vjailbreakv1alpha1.VMwareCreds) error {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Starting VMware resource discovery", "name", vmwareCreds.Name)

	// Create scope for utils functions
	vmwareScope, err := scope.NewVMwareCredsScope(scope.VMwareCredsScopeParams{
		Client:      k8sClient,
		VMwareCreds: vmwareCreds,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create VMwareCreds scope")
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(vmwareCreds, constants.VMwareCredsFinalizer) {
		controllerutil.AddFinalizer(vmwareCreds, constants.VMwareCredsFinalizer)
		if err := k8sClient.Update(ctx, vmwareCreds); err != nil {
			return errors.Wrap(err, "failed to add finalizer")
		}
		ctxlog.Info("Added finalizer to VMwareCreds", "name", vmwareCreds.Name)
	}

	// Create clusters and hosts
	ctxlog.Info("Creating VMware clusters and hosts", "name", vmwareCreds.Name)
	if err := utils.CreateVMwareClustersAndHosts(ctx, vmwareScope); err != nil {
		return errors.Wrap(err, "failed to create VMware clusters and hosts")
	}

	// Get all VMs
	ctxlog.Info("Fetching all VMs from VMware", "name", vmwareCreds.Name, "datacenter", vmwareCreds.Spec.DataCenter)
	vminfo, rdmDiskMap, err := utils.GetAllVMs(ctx, vmwareScope, vmwareCreds.Spec.DataCenter)
	if err != nil {
		return errors.Wrap(err, "failed to get all VMs")
	}
	ctxlog.Info(fmt.Sprintf("Fetched %d VMs from VMware", len(vminfo)), "name", vmwareCreds.Name)

	// Create or update RDM disks
	ctxlog.Info("Creating/updating RDM disk CRs", "name", vmwareCreds.Name)
	if err := utils.CreateOrUpdateRDMDisks(ctx, k8sClient, vmwareCreds, rdmDiskMap); err != nil {
		return errors.Wrap(err, "failed to create/update RDM disks")
	}

	// Delete stale VMware machines
	ctxlog.Info("Cleaning up stale VMware machines", "name", vmwareCreds.Name)
	if err := utils.DeleteStaleVMwareMachines(ctx, k8sClient, vmwareCreds, vminfo); err != nil {
		return errors.Wrap(err, "failed to delete stale VMware machines")
	}

	// Delete stale clusters and hosts
	ctxlog.Info("Cleaning up stale clusters and hosts", "name", vmwareCreds.Name)
	if err := utils.DeleteStaleVMwareClustersAndHosts(ctx, vmwareScope); err != nil {
		return errors.Wrap(err, "failed to delete stale clusters and hosts")
	}

	ctxlog.Info("VMware resource discovery completed successfully", "name", vmwareCreds.Name)
	return nil
}