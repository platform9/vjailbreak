// Package utils provides utility functions for handling credentials and other operations
package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	gophercloud "github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/schedulerstats"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/pkg/errors"
	netutils "github.com/platform9/vjailbreak/common/utils"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/k8sutils"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	trueString  = "true" // Define at package level
	falseString = "false"
	sdkPath     = "/sdk" // SDK path constant
)

// GetVMwareCredsInfo retrieves vCenter credentials from a secret
func GetVMwareCredsInfo(ctx context.Context, k3sclient client.Client, credsName string) (vjailbreakv1alpha1.VMwareCredsInfo, error) {
	creds := vjailbreakv1alpha1.VMwareCreds{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: credsName}, &creds); err != nil {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Wrapf(err, "failed to get VMware credentials '%s'", credsName)
	}
	return GetVMwareCredentialsFromSecret(ctx, k3sclient, creds.Spec.SecretRef.Name)
}

// GetOpenstackCredsInfo retrieves OpenStack credentials from a secret
func GetOpenstackCredsInfo(ctx context.Context, k3sclient client.Client, credsName string) (vjailbreakv1alpha1.OpenStackCredsInfo, error) {
	creds := vjailbreakv1alpha1.OpenstackCreds{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: credsName}, &creds); err != nil {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Wrapf(err, "failed to get OpenStack credentials '%s'", credsName)
	}
	return GetOpenstackCredentialsFromSecret(ctx, k3sclient, creds.Spec.SecretRef.Name)
}

// GetArrayCredsInfo retrieves storage array credentials from a secret
func GetArrayCredsInfo(ctx context.Context, k3sclient client.Client, credsName string) (vjailbreakv1alpha1.ArrayCredsInfo, error) {
	creds := vjailbreakv1alpha1.ArrayCreds{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: credsName}, &creds); err != nil {
		return vjailbreakv1alpha1.ArrayCredsInfo{}, errors.Wrapf(err, "failed to get storage array credentials '%s'", credsName)
	}
	return GetArrayCredentialsFromSecret(ctx, k3sclient, creds.Spec.SecretRef.Name)
}

// GetArrayCredentialsFromSecret retrieves storage array credentials from a secret
func GetArrayCredentialsFromSecret(ctx context.Context, k3sclient client.Client, secretName string) (vjailbreakv1alpha1.ArrayCredsInfo, error) {
	secret := &corev1.Secret{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return vjailbreakv1alpha1.ArrayCredsInfo{}, errors.Wrapf(err, "failed to get secret '%s'", secretName)
	}

	if secret.Data == nil {
		return vjailbreakv1alpha1.ArrayCredsInfo{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	hostname := string(secret.Data["ARRAY_HOSTNAME"])
	username := string(secret.Data["ARRAY_USERNAME"])
	password := string(secret.Data["ARRAY_PASSWORD"])
	insecureStr := string(secret.Data["ARRAY_INSECURE"])

	if hostname == "" {
		return vjailbreakv1alpha1.ArrayCredsInfo{}, errors.Errorf("ARRAY_HOSTNAME is missing in secret '%s'", secretName)
	}
	if username == "" {
		return vjailbreakv1alpha1.ArrayCredsInfo{}, errors.Errorf("ARRAY_USERNAME is missing in secret '%s'", secretName)
	}
	if password == "" {
		return vjailbreakv1alpha1.ArrayCredsInfo{}, errors.Errorf("ARRAY_PASSWORD is missing in secret '%s'", secretName)
	}

	skipSSL := strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

	return vjailbreakv1alpha1.ArrayCredsInfo{
		Hostname:            hostname,
		Username:            username,
		Password:            password,
		SkipSSLVerification: skipSSL,
	}, nil
}

// GetVMwareCredentialsFromSecret retrieves vCenter credentials from a secret
func GetVMwareCredentialsFromSecret(ctx context.Context, k3sclient client.Client, secretName string) (vjailbreakv1alpha1.VMwareCredsInfo, error) {
	secret := &corev1.Secret{}

	// Get In cluster client
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Wrapf(err, "failed to get secret '%s'", secretName)
	}

	if secret.Data == nil {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	host := string(secret.Data["VCENTER_HOST"])
	username := string(secret.Data["VCENTER_USERNAME"])
	password := string(secret.Data["VCENTER_PASSWORD"])
	insecureStr := string(secret.Data["VCENTER_INSECURE"])
	datacenter := string(secret.Data["VCENTER_DATACENTER"])

	if host == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_HOST is missing in secret '%s'", secretName)
	}
	if username == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_USERNAME is missing in secret '%s'", secretName)
	}
	if password == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_PASSWORD is missing in secret '%s'", secretName)
	}

	insecure := strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

	return vjailbreakv1alpha1.VMwareCredsInfo{
		Host:       host,
		Username:   username,
		Password:   password,
		Datacenter: datacenter,
		Insecure:   insecure,
	}, nil
}

// GetOpenstackCredentialsFromSecret retrieves and checks the secret
func GetOpenstackCredentialsFromSecret(ctx context.Context, k3sclient client.Client, secretName string) (vjailbreakv1alpha1.OpenStackCredsInfo, error) {
	secret := &corev1.Secret{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Wrap(err, "failed to get secret")
	}

	// Extract and validate each field
	fields := map[string]string{
		"AuthURL":    string(secret.Data["OS_AUTH_URL"]),
		"DomainName": string(secret.Data["OS_DOMAIN_NAME"]),
		"Username":   string(secret.Data["OS_USERNAME"]),
		"Password":   string(secret.Data["OS_PASSWORD"]),
		"TenantName": string(secret.Data["OS_TENANT_NAME"]),
		"RegionName": string(secret.Data["OS_REGION_NAME"]),
	}

	for key, value := range fields {
		if value == "" {
			return vjailbreakv1alpha1.OpenStackCredsInfo{}, errors.Errorf("%s is missing in secret '%s'", key, secretName)
		}
	}

	insecureStr := string(secret.Data["OS_INSECURE"])
	insecure := strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

	return vjailbreakv1alpha1.OpenStackCredsInfo{
		AuthURL:    fields["AuthURL"],
		DomainName: fields["DomainName"],
		Username:   fields["Username"],
		Password:   fields["Password"],
		RegionName: fields["RegionName"],
		TenantName: fields["TenantName"],
		Insecure:   insecure,
	}, nil
}

// GetCert retrieves an X.509 certificate from an endpoint
func GetCert(endpoint string) (*x509.Certificate, error) {
	conf := &tls.Config{
		//nolint:gosec // This is required to skip certificate verification
		InsecureSkipVerify: true,
	}
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing URL")
	}
	hostname := parsedURL.Hostname()
	conn, err := tls.Dial("tcp", hostname+":443", conf)
	if err != nil {
		return nil, errors.Wrapf(err, "error connecting to %s", hostname)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Log.Info("Error closing connection", "error", err, "endpoint", endpoint)
		}
	}()
	cert := conn.ConnectionState().PeerCertificates[0]
	return cert, nil
}

// VerifyNetworks verifies the existence of specified networks in OpenStack
func VerifyNetworks(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetnetworks []string) error {
	openstackClients, err := GetOpenStackClients(ctx, k3sclient, openstackcreds)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack clients")
	}
	allPages, err := networks.List(openstackClients.NetworkingClient, nil).AllPages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list networks")
	}

	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return errors.Wrap(err, "failed to extract all networks")
	}

	// Build a map of all networks
	networkMap := make(map[string]bool)
	for i := 0; i < len(allNetworks); i++ {
		networkMap[allNetworks[i].Name] = true
	}

	// Verify that all network names in targetnetworks exist in the openstack networks
	for _, targetNetwork := range targetnetworks {
		if _, found := networkMap[targetNetwork]; !found {
			return fmt.Errorf("network '%s' not found in OpenStack", targetNetwork)
		}
	}
	return nil
}

// VerifyPorts verifies the existence of specified ports in OpenStack
func VerifyPorts(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetports []string) error {
	openstackClients, err := GetOpenStackClients(ctx, k3sclient, openstackcreds)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack clients")
	}

	allPages, err := ports.List(openstackClients.NetworkingClient, nil).AllPages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list ports")
	}

	allPorts, err := ports.ExtractPorts(allPages)
	if err != nil {
		return errors.Wrap(err, "failed to extract all ports")
	}

	// Build a map of all ports
	portMap := make(map[string]bool)
	for i := 0; i < len(allPorts); i++ {
		portMap[allPorts[i].ID] = true
	}

	// Verify that all port names in targetports exist in the openstack ports
	for _, targetPort := range targetports {
		if _, found := portMap[targetPort]; !found {
			return errors.Wrap(fmt.Errorf("port '%s' not found in OpenStack", targetPort), "failed to verify ports")
		}
	}
	return nil
}

// VerifyStorage verifies the existence of specified storage in OpenStack
func VerifyStorage(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetstorages []string) error {
	openstackClients, err := GetOpenStackClients(ctx, k3sclient, openstackcreds)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack clients")
	}
	allPages, err := volumetypes.List(openstackClients.BlockStorageClient, nil).AllPages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list volume types")
	}

	allvoltypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil {
		return errors.Wrap(err, "failed to extract all volume types")
	}
	// Verify that all volume types in targetstorage exist in the openstack volume types
	for _, targetstorage := range targetstorages {
		found := false
		for i := 0; i < len(allvoltypes); i++ {
			if allvoltypes[i].Name == targetstorage {
				found = true
				break
			}
		}
		if !found {
			return errors.Wrap(fmt.Errorf("volume type '%s' not found in OpenStack", targetstorage), "failed to verify volume types")
		}
	}
	return nil
}

// GetOpenstackInfo retrieves OpenStack information using provided credentials
func GetOpenstackInfo(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*vjailbreakv1alpha1.OpenstackInfo, error) {
	openstackClients, err := GetOpenStackClients(ctx, k3sclient, openstackcreds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get openstack clients")
	}
	var openstackvoltypes []string
	var openstacknetworks []string

	allVolumeTypePages, err := volumetypes.List(openstackClients.BlockStorageClient, nil).AllPages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list volume types")
	}

	allvoltypes, err := volumetypes.ExtractVolumeTypes(allVolumeTypePages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract all volume types")
	}

	for i := 0; i < len(allvoltypes); i++ {
		openstackvoltypes = append(openstackvoltypes, allvoltypes[i].Name)
	}

	allNetworkPages, err := networks.List(openstackClients.NetworkingClient, nil).AllPages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list networks")
	}

	allNetworks, err := networks.ExtractNetworks(allNetworkPages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract all networks")
	}
	volumeBackendPools, err := getCinderVolumeBackendPools(ctx, openstackClients)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cinder volume backend pools")
	}
	for i := 0; i < len(allNetworks); i++ {
		openstacknetworks = append(openstacknetworks, allNetworks[i].Name)
	}

	credsInfo, err := GetOpenstackCredentialsFromSecret(ctx, k3sclient, openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get openstack credentials for project lookup")
	}

	identityClient, err := openstack.NewIdentityV3(openstackClients.BlockStorageClient.ProviderClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create identity client")
	}

	listOpts := projects.ListOpts{Name: credsInfo.TenantName}
	allPages, err := projects.List(identityClient, listOpts).AllPages(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list projects with name %s", credsInfo.TenantName)
	}

	allProjects, err := projects.ExtractProjects(allPages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract projects")
	}
	if len(allProjects) == 0 {
		return nil, fmt.Errorf("no project found with name %s", credsInfo.TenantName)
	}
	projectID := allProjects[0].ID

	allSecGroupPages, err := groups.List(openstackClients.NetworkingClient, groups.ListOpts{
		TenantID: projectID,
	}).AllPages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list security groups for project")
	}

	allSecGroups, err := groups.ExtractGroups(allSecGroupPages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract all security groups")
	}

	nameCounts := make(map[string]int)
	for _, group := range allSecGroups {
		nameCounts[group.Name]++
	}

	openstacksecuritygroups := make([]vjailbreakv1alpha1.SecurityGroupInfo, 0, len(allSecGroups))

	for _, group := range allSecGroups {
		openstacksecuritygroups = append(openstacksecuritygroups, vjailbreakv1alpha1.SecurityGroupInfo{
			Name:              group.Name,
			ID:                group.ID,
			RequiresIDDisplay: nameCounts[group.Name] > 1,
		})
	}

	// Fetch server groups
	openstackservergroups := make([]vjailbreakv1alpha1.ServerGroupInfo, 0)
	allServerGroupPages, err := servergroups.List(openstackClients.ComputeClient, servergroups.ListOpts{}).AllPages(ctx)
	if err == nil {
		allServerGroups, err := servergroups.ExtractServerGroups(allServerGroupPages)
		if err == nil {
			for _, group := range allServerGroups {
				openstackservergroups = append(openstackservergroups, vjailbreakv1alpha1.ServerGroupInfo{
					Name:    group.Name,
					ID:      group.ID,
					Policy:  strings.Join(group.Policies, ","),
					Members: len(group.Members),
				})
			}
		}
	}

	return &vjailbreakv1alpha1.OpenstackInfo{
		VolumeTypes:    openstackvoltypes,
		Networks:       openstacknetworks,
		VolumeBackends: volumeBackendPools,
		SecurityGroups: openstacksecuritygroups,
		ServerGroups:   openstackservergroups,
	}, nil
}

// GetOpenStackClients is a function to create openstack clients
func GetOpenStackClients(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*OpenStackClients, error) {
	if openstackcreds == nil {
		return nil, errors.New("openstackcreds cannot be nil")
	}

	openstackCredential, err := GetOpenstackCredentialsFromSecret(ctx, k3sclient, openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get openstack credentials from secret")
	}

	endpoint := gophercloud.EndpointOpts{
		Region: openstackCredential.RegionName,
	}
	providerClient, err := ValidateAndGetProviderClient(ctx, k3sclient, openstackcreds)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get provider client for region '%s'", openstackCredential.RegionName))
	}
	if providerClient == nil {
		return nil, fmt.Errorf("failed to get provider client for region '%s'", openstackCredential.RegionName)
	}
	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create openstack compute client for region '%s'", openstackCredential.RegionName))
	}
	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create openstack block storage client for region '%s'",
			openstackCredential.RegionName))
	}
	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create openstack networking client for region '%s'",
			openstackCredential.RegionName))
	}

	return &OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
	}, nil
}

// ValidateAndGetProviderClient is a function to get provider client
func ValidateAndGetProviderClient(ctx context.Context, k3sclient client.Client,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*gophercloud.ProviderClient, error) {
	openstackCredential, err := GetOpenstackCredentialsFromSecret(ctx, k3sclient, openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get openstack credentials from secret")
	}

	providerClient, err := openstack.NewClient(openstackCredential.AuthURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create openstack client")
	}
	vjbNet := netutils.NewVjbNet()
	if openstackCredential.Insecure {
		vjbNet.Insecure = true
	} else {
		fmt.Printf("Warning: TLS verification is enforced by default. If you encounter certificate errors, set OS_INSECURE=true to skip verification.\n")
	}
	vjbNet.SetTimeout(60 * time.Second)

	if vjbNet.CreateSecureHTTPClient() == nil {
		providerClient.HTTPClient = *vjbNet.GetClient()
	} else {
		return nil, fmt.Errorf("failed to create secure HTTP client")
	}

	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: openstackCredential.AuthURL,
		Username:         openstackCredential.Username,
		Password:         openstackCredential.Password,
		DomainName:       openstackCredential.DomainName,
		TenantName:       openstackCredential.TenantName,
	}
	if err := openstack.Authenticate(ctx, providerClient, authOpts); err != nil {
		switch {
		case strings.Contains(err.Error(), "401"):
			return nil, fmt.Errorf("authentication failed: invalid username, password, or project/domain. Please verify your credentials")
		case strings.Contains(err.Error(), "404"):
			return nil, fmt.Errorf("authentication failed: the authentication URL or tenant/project name is incorrect")
		case strings.Contains(err.Error(), "timeout"):
			return nil, fmt.Errorf("connection timeout: unable to reach the OpenStack authentication service. Please check your network connection and Auth URL")
		default:
			return nil, fmt.Errorf("authentication failed: %w. Please verify your OpenStack credentials", err)
		}
	}

	_, err = VerifyCredentialsMatchCurrentEnvironment(providerClient, openstackCredential.RegionName)
	if err != nil {
		if strings.Contains(err.Error(), "Credentials are valid but for a different OpenStack environment") {
			return nil, err
		}
		return nil, fmt.Errorf("failed to verify credentials against current environment: %w", err)
	}

	return providerClient, nil
}

var vmwareClientMap *sync.Map

// ValidateVMwareCreds validates the VMware credentials
func ValidateVMwareCreds(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vim25.Client, error) {
	vmwareCredsinfo, err := GetVMwareCredentialsFromSecret(ctx, k3sclient, vmwcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get vCenter credentials from secret: %w", err)
	}
	host := vmwareCredsinfo.Host
	username := vmwareCredsinfo.Username
	password := vmwareCredsinfo.Password
	disableSSLVerification := vmwareCredsinfo.Insecure
	datacenter := vmwareCredsinfo.Datacenter

	u, err := netutils.NormalizeVCenterURL(host)
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(username, password)
	// Connect and log in to ESX or vCenter
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
		Reauth:   true,
	}
	mapKey := string(vmwcreds.UID)
	// Initialize map if needed
	if vmwareClientMap == nil {
		vmwareClientMap = &sync.Map{}
	}
	// Check cache for existing authenticated client
	if val, ok := vmwareClientMap.Load(mapKey); ok {
		cachedClient, valid := val.(*vim25.Client)
		if valid && cachedClient != nil && cachedClient.Client != nil {
			sessMgr := session.NewManager(cachedClient)
			userSession, err := sessMgr.UserSession(ctx)
			if err == nil && userSession != nil {
				// Cached client is still valid, return it
				return cachedClient, nil
			}
			// Cached client is no longer valid, remove it
			vmwareClientMap.Delete(mapKey)
		}
	}
	settings, err := k8sutils.GetVjailbreakSettings(ctx, k3sclient)
	if err != nil {
		return nil, fmt.Errorf("failed to get vjailbreak settings: %w", err)
	}
	// Exponential retry logic
	maxRetries := settings.VCenterLoginRetryLimit
	var lastErr error
	var c *vim25.Client
	ctxlog := log.FromContext(ctx)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a new empty client struct for Login to populate
		c = &vim25.Client{}
		err = s.Login(ctx, c, nil)
		if err == nil {
			// Login successful
			ctxlog.Info("Login successful", "attempt", attempt)
			break
		} else if strings.Contains(err.Error(), "incorrect user name or password") {
			return nil, fmt.Errorf("authentication failed: invalid username or password. Please verify your credentials")
		}
		ctxlog := log.FromContext(ctx)
		// Log the error and retry after a delay
		ctxlog.Info("Login attempt failed", "attempt", attempt, "error", err, "maxRetries", maxRetries)
		if attempt < maxRetries {
			delayNum := math.Pow(2, float64(attempt)) * 500
			ctxlog.Info("Retrying login after delay", "delayMs", delayNum)
			time.Sleep(time.Duration(delayNum) * time.Millisecond)
		}
	}
	// Check if all login attempts failed
	if lastErr != nil {
		return nil, fmt.Errorf("failed to login to vCenter after %d attempts: %w", maxRetries, lastErr)
	}
	if datacenter != "" {
		finder := find.NewFinder(c, false)
		_, err = finder.Datacenter(ctx, datacenter)
		if err != nil {
			return nil, fmt.Errorf("failed to find datacenter: %w", err)
		}
	}
	// All validations passed - cache the fully validated client
	vmwareClientMap.Store(mapKey, c)
	return c, nil
}

// GetVMwNetworks gets the networks of a VM
func GetVMwNetworks(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	// Pre-allocate networks slice to avoid append allocations
	networks := make([]string, 0)
	c, finder, err := GetFinderForVMwareCreds(ctx, k3sclient, vmwcreds, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to get finder: %w", err)
	}

	// Get the vm
	vm, err := finder.VirtualMachine(ctx, vmname)
	if err != nil {
		return nil, fmt.Errorf("failed to find vm: %w", err)
	}

	// Get the network name of the VM
	var o mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config", "network"}, &o)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	// Get the network interfaces
	// Get the virtual NICs
	nicList, err := ExtractVirtualNICs(&o)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual NICs for vm %s: %w", vmname, err)
	}

	pc := property.DefaultCollector(c)
	for _, nic := range nicList {
		var netObj mo.Network
		netRef := types.ManagedObjectReference{Type: "Network", Value: nic.Network}
		err := pc.RetrieveOne(ctx, netRef, []string{"name"}, &netObj)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve network name for %s: %w", nic.Network, err)
		}
		networks = append(networks, netObj.Name)
	}
	return networks, nil
}

// GetVMwDatastore gets the datastores of a VM
func GetVMwDatastore(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	c, finder, err := GetFinderForVMwareCreds(ctx, k3sclient, vmwcreds, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to get finder: %w", err)
	}

	// Get the vm
	vm, err := finder.VirtualMachine(ctx, vmname)
	if err != nil {
		return nil, fmt.Errorf("failed to find vm: %w", err)
	}

	var vmProps mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config"}, &vmProps)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	var datastores []string
	var ds mo.Datastore
	var dsref types.ManagedObjectReference
	for _, device := range vmProps.Config.Hardware.Device {
		if _, ok := device.(*types.VirtualDisk); ok {
			switch backing := device.GetVirtualDevice().Backing.(type) {
			case *types.VirtualDiskFlatVer2BackingInfo:
				dsref = backing.Datastore.Reference()
			case *types.VirtualDiskSparseVer2BackingInfo:
				dsref = backing.Datastore.Reference()
			case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
				continue
			default:
				return nil, fmt.Errorf("unsupported disk backing type: %T", device.GetVirtualDevice().Backing)
			}
			err := property.DefaultCollector(c).RetrieveOne(ctx, dsref, []string{"name"}, &ds)
			if err != nil {
				return nil, fmt.Errorf("failed to get datastore: %w", err)
			}

			datastores = append(datastores, ds.Name)
		}
	}
	return datastores, nil
}

// GetAndCreateAllVMs gets all the VMs in a datacenter.
func GetAndCreateAllVMs(ctx context.Context, scope *scope.VMwareCredsScope, datacenter string) ([]vjailbreakv1alpha1.VMInfo, *sync.Map, error) {
	log := scope.Logger

	vjailbreakSettings, err := k8sutils.GetVjailbreakSettings(ctx, scope.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get vjailbreak settings: %w", err)
	}
	log.Info("Fetched vjailbreak settings for vcenter scan concurrency limit", "vcenter_scan_concurrency_limit", vjailbreakSettings.VCenterScanConcurrencyLimit, "datacenter", datacenter)

	// Get VMware client and finder
	c, finder, err := GetFinderForVMwareCreds(ctx, scope.Client, scope.VMwareCreds, datacenter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get finder for VMware credentials: %w", err)
	}

	// Get all VMs
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	c.Timeout = 5 * time.Minute
	defer c.CloseIdleConnections()
	vmData, rdmDiskMap, err := getVMDetails(ctx, scope, vms, c)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch vm data: %w", err)
	}

	return vmData, rdmDiskMap, nil
}

// CountGPUs counts the number of GPU devices attached to a VM.
// It separately counts PCI passthrough GPUs and vGPU devices.
func CountGPUs(vmProps *mo.VirtualMachine) vjailbreakv1alpha1.GPUInfo {
	info := vjailbreakv1alpha1.GPUInfo{}

	if vmProps.Config == nil || vmProps.Config.Hardware.Device == nil {
		return info
	}

	for _, device := range vmProps.Config.Hardware.Device {
		if pciDevice, ok := device.(*types.VirtualPCIPassthrough); ok {
			if pciDevice.Backing != nil {
				// VirtualPCIPassthroughVmiopBackingInfo indicates vGPU
				if _, isVGPU := pciDevice.Backing.(*types.VirtualPCIPassthroughVmiopBackingInfo); isVGPU {
					info.VGPUCount++
				} else {
					// Regular PCI passthrough (likely GPU)
					info.PassthroughCount++
				}
			} else {
				// PCI passthrough without specific backing
				info.PassthroughCount++
			}
		}
	}

	return info
}

// DetectGPUUsage checks if the VM has any GPU devices attached.
// It detects PCI passthrough devices (including GPUs) and vGPU profiles.
//
// Deprecated: Use CountGPUs() and GPUInfo.HasGPU() instead.
func DetectGPUUsage(vmProps *mo.VirtualMachine) bool {
	gpuInfo := CountGPUs(vmProps)
	return gpuInfo.HasGPU()
}

// ExtractVirtualNICs retrieves the virtual NICs defined in the VM hardware (config.hardware.device).
// It returns a list of NICs with MAC addresses, backing network identifiers, and index order.
func ExtractVirtualNICs(vmProps *mo.VirtualMachine) ([]vjailbreakv1alpha1.NIC, error) {
	nicList := []vjailbreakv1alpha1.NIC{}
	nicsIndex := 0

	for _, device := range vmProps.Config.Hardware.Device {
		var nic *types.VirtualEthernetCard

		switch d := device.(type) {
		case *types.VirtualE1000,
			*types.VirtualE1000e,
			*types.VirtualVmxnet,
			*types.VirtualVmxnet2,
			*types.VirtualVmxnet3,
			*types.VirtualPCNet32:
			if ethCard, ok := d.(types.BaseVirtualEthernetCard); ok {
				nic = ethCard.GetVirtualEthernetCard()
			}
		}

		if nic != nil && nic.Backing != nil {
			var network string
			switch backing := device.GetVirtualDevice().Backing.(type) {
			case *types.VirtualEthernetCardNetworkBackingInfo:
				if backing.Network != nil {
					network = backing.Network.Value
				}
			case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
				network = backing.Port.PortgroupKey
			case *types.VirtualEthernetCardOpaqueNetworkBackingInfo:
				network = backing.OpaqueNetworkId
			}
			nicList = append(nicList, vjailbreakv1alpha1.NIC{
				MAC:     strings.ToLower(nic.MacAddress),
				Index:   nicsIndex,
				Network: network,
			})
			nicsIndex++
		}
	}
	return nicList, nil
}

// ExtractGuestNetworkInfo retrieves the runtime guest network configuration (guest.net)
// reported by VMware Tools. Returns MAC, IP, DNS, and origin for each NIC in the guest.
func ExtractGuestNetworkInfo(vmProps *mo.VirtualMachine) ([]vjailbreakv1alpha1.GuestNetwork, error) {
	guestNetworks := []vjailbreakv1alpha1.GuestNetwork{}

	for i, guestNet := range vmProps.Guest.Net {
		if guestNet.IpConfig == nil {
			continue
		}

		for _, ip := range guestNet.IpConfig.IpAddress {
			dnsConfigList := []string{}
			if guestNet.DnsConfig != nil {
				dnsConfigList = guestNet.DnsConfig.IpAddress
			}

			guestNetworks = append(guestNetworks, vjailbreakv1alpha1.GuestNetwork{
				MAC:          strings.ToLower(guestNet.MacAddress),
				IP:           ip.IpAddress,
				Origin:       ip.Origin,
				PrefixLength: ip.PrefixLength,
				DNS:          dnsConfigList,
				Device:       fmt.Sprintf("%d", i),
			})
		}
	}

	return guestNetworks, nil
}

// processVMDisk processes a single virtual disk device and updates the disk information
// it returns the datastore reference, RDM disk info, a skip flag, and any error encountered
// It checks if the disk is backed by a shared SCSI controller and skips the VM.
func processVMDisk(ctx context.Context, disk *types.VirtualDisk, hostStorageInfo *types.HostStorageDeviceInfo, vmName string) (dsref *types.ManagedObjectReference, rdmDisk vjailbreakv1alpha1.RDMDisk, err error) {
	ctxlog := log.FromContext(ctx)
	switch backing := disk.Backing.(type) {
	case *types.VirtualDiskFlatVer2BackingInfo:
		ref := backing.Datastore.Reference()
		dsref = &ref
	case *types.VirtualDiskSparseVer2BackingInfo:
		ref := backing.Datastore.Reference()
		dsref = &ref
	case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
		if hostStorageInfo != nil {
			rdmDisk = vjailbreakv1alpha1.RDMDisk{
				Spec: vjailbreakv1alpha1.RDMDiskSpec{
					DiskSize: int(disk.CapacityInBytes),
					DiskName: disk.DeviceInfo.GetDescription().Label,
				},
			}
			for _, scsiDisk := range hostStorageInfo.ScsiLun {
				lunDetails := scsiDisk.GetScsiLun()
				if backing.LunUuid == lunDetails.Uuid {
					rdmDisk.Spec.DisplayName = lunDetails.DisplayName
					rdmDisk.Spec.UUID = lunDetails.Uuid
					rdmDisk.Spec.OwnerVMs = []string{vmName}
					rdmDisk.Name = fmt.Sprintf("vml.%s", lunDetails.Uuid)
				}
			}
		}
	default:
		ctxlog.Error(fmt.Errorf("unsupported disk backing type: %T", disk.Backing), "VM", vmName, "disk", disk.DeviceInfo.GetDescription().Label)
		return nil, vjailbreakv1alpha1.RDMDisk{}, fmt.Errorf("unsupported disk backing type: %T", disk.Backing)
	}

	return dsref, rdmDisk, nil
}

// AppendUnique appends unique values to a slice
func AppendUnique(slice []string, values ...string) []string {
	for _, value := range values {
		if !slices.Contains(slice, value) {
			slice = append(slice, value)
		}
	}
	return slice
}

// CreateOrUpdateVMwareMachine creates or updates a VMwareMachine object for the given VM
func CreateOrUpdateVMwareMachine(ctx context.Context, client client.Client,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds, vminfo *vjailbreakv1alpha1.VMInfo, datacenter string) error {
	sanitizedVMName, err := GetK8sCompatibleVMWareObjectName(vminfo.Name, vmwcreds.Name)
	if err != nil {
		return fmt.Errorf("failed to get VM name: %w", err)
	}
	esxiK8sName, err := GetK8sCompatibleVMWareObjectName(vminfo.ESXiName, vmwcreds.Name)
	if err != nil {
		return errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	clusterK8sID := GetClusterK8sID(vminfo.ClusterName, datacenter)
	clusterK8sName, err := GetK8sCompatibleVMWareObjectName(clusterK8sID, vmwcreds.Name)
	if err != nil {
		return errors.Wrap(err, "failed to convert cluster name to k8s name")
	}
	// We need this flag because, there can be multiple VMwarecreds and each will
	// trigger its own reconciliation loop,
	// so we need to know if the object is new or not. if it is new we mark the migrated
	// field to false and powerstate to the current state of the vm.
	// If the object is not new, we update the status and persist the migrated status.
	init := false

	vmwvm := &vjailbreakv1alpha1.VMwareMachine{}
	vmwvmKey := k8stypes.NamespacedName{Name: sanitizedVMName, Namespace: vmwcreds.Namespace}

	// Try to fetch existing resource
	err = client.Get(ctx, vmwvmKey, vmwvm)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get VMwareMachine: %w", err)
	}

	for _, data := range vmwvm.Spec.VMInfo.RDMDisks {
		if !slices.Contains(vminfo.RDMDisks, data) {
			vminfo.RDMDisks = append(vminfo.RDMDisks, data)
			log.FromContext(ctx).Info("RDM disk cannot be removed from VM, delete vmware custom resource if wanted to exclude rdm disks after detachment from VM and remove owner VM's reference from RDM disk", "disk", data, "vm", vminfo.Name)
		}
	}

	// Check if the object is present or not if not present create a new object and set init to true.
	if apierrors.IsNotFound(err) {
		// If not found, create a new object
		vmwvm = &vjailbreakv1alpha1.VMwareMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmwvmKey.Name,
				Namespace: vmwcreds.Namespace,
				Labels: map[string]string{
					constants.VMwareCredsLabel:   vmwcreds.Name,
					constants.ESXiNameLabel:      esxiK8sName,
					constants.VMwareClusterLabel: clusterK8sName,
				},
				Annotations: map[string]string{
					constants.VMwareDatacenterLabel: datacenter,
				},
			},
			Spec: vjailbreakv1alpha1.VMwareMachineSpec{
				VMInfo: *vminfo,
			},
		}
		// Create the new object
		if err := client.Create(ctx, vmwvm); err != nil {
			return fmt.Errorf("failed to create VMwareMachine: %w", err)
		}
		init = true
	} else {
		// Initialize labels map if needed
		label := fmt.Sprintf("%s-%s", constants.VMwareCredsLabel, vmwcreds.Name)
		currentOSFamily := vmwvm.Spec.VMInfo.OSFamily
		// Check if label already exists with same value
		if vmwvm.Labels == nil || vmwvm.Labels[label] != trueString {
			// Initialize labels map if needed
			if vmwvm.Labels == nil {
				vmwvm.Labels = make(map[string]string)
			}
			vmwvm.Labels[label] = trueString
			// Update only if we made changes
			if err = client.Update(ctx, vmwvm); err != nil {
				return fmt.Errorf("failed to update VMwareMachine label: %w", err)
			}
		}
		// Set the new label
		vmwvm.Labels[constants.VMwareCredsLabel] = vmwcreds.Name

		if !reflect.DeepEqual(vmwvm.Spec.VMInfo, *vminfo) || !reflect.DeepEqual(vmwvm.Labels[constants.ESXiNameLabel], esxiK8sName) || !reflect.DeepEqual(vmwvm.Labels[constants.VMwareClusterLabel], clusterK8sName) {
			// update vminfo in case the VM has been moved by vMotion
			assignedIP := ""
			osType := ""

			if vmwvm.Spec.VMInfo.AssignedIP != "" {
				assignedIP = vmwvm.Spec.VMInfo.AssignedIP
			}
			if vmwvm.Spec.VMInfo.OSFamily != "" {
				osType = vmwvm.Spec.VMInfo.OSFamily
			}
			vmwvm.Spec.VMInfo = *vminfo
			if assignedIP != "" {
				vmwvm.Spec.VMInfo.AssignedIP = assignedIP
			}
			if osType != "" && vmwvm.Spec.VMInfo.OSFamily == "" {
				vmwvm.Spec.VMInfo.OSFamily = osType
			}
			vmwvm.Labels[constants.ESXiNameLabel] = esxiK8sName
			vmwvm.Labels[constants.VMwareClusterLabel] = clusterK8sName

			if vmwvm.Annotations == nil {
				vmwvm.Annotations = make(map[string]string)
			}
			vmwvm.Annotations[constants.VMwareDatacenterLabel] = datacenter

			if vmwvm.Spec.VMInfo.OSFamily == "" {
				vmwvm.Spec.VMInfo.OSFamily = currentOSFamily
			}
			// Update only if we made changes
			if err = client.Update(ctx, vmwvm); err != nil {
				return fmt.Errorf("failed to update VMwareMachine: %w", err)
			}
		}
	}

	// Assumption is if init is true, the object is new and it is not migrated hence mark migrated to false.
	if init {
		vmwvm.Status = vjailbreakv1alpha1.VMwareMachineStatus{
			PowerState: vminfo.VMState,
			Migrated:   false,
		}
	} else {
		// If the object is not new, update the status and persist migrated status.
		currentMigratedStatus := vmwvm.Status.Migrated
		if vmwvm.Status.PowerState != vminfo.VMState {
			vmwvm.Status.PowerState = vminfo.VMState
		}
		vmwvm.Status.Migrated = currentMigratedStatus
	}

	// Update the status
	if err := client.Status().Update(ctx, vmwvm); err != nil {
		return fmt.Errorf("failed to update VMwareMachine status: %w", err)
	}
	return nil
}

// CreateOrUpdateLabel creates or updates a label on a VMwareMachine resource
func CreateOrUpdateLabel(ctx context.Context, client client.Client,
	vmwvm *vjailbreakv1alpha1.VMwareMachine, key, value string) error {
	_, err := controllerutil.CreateOrUpdate(ctx, client, vmwvm, func() error {
		if vmwvm.Labels == nil {
			vmwvm.Labels = make(map[string]string)
		}
		if vmwvm.Labels[key] == value {
			return nil
		}
		vmwvm.Labels[key] = value
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update VMwareMachine labels: %w", err)
	}
	return nil
}

// FilterVMwareMachinesForCreds returns all VMwareMachine objects associated with a VMwareCreds resource
func FilterVMwareMachinesForCreds(ctx context.Context, k8sClient client.Client,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vjailbreakv1alpha1.VMwareMachineList, error) {
	vmList := vjailbreakv1alpha1.VMwareMachineList{}
	if err := k8sClient.List(ctx, &vmList, client.InNamespace(constants.NamespaceMigrationSystem), client.MatchingLabels{constants.VMwareCredsLabel: vmwcreds.Name}); err != nil {
		return nil, errors.Wrap(err, "Error listing VMs")
	}
	return &vmList, nil
}

// FilterVMwareHostsForCreds filters VMwareHost objects for the given credentials
func FilterVMwareHostsForCreds(ctx context.Context, k8sClient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vjailbreakv1alpha1.VMwareHostList, error) {
	hostList := vjailbreakv1alpha1.VMwareHostList{}
	if err := k8sClient.List(ctx, &hostList, client.InNamespace(constants.NamespaceMigrationSystem), client.MatchingLabels{constants.VMwareCredsLabel: vmwcreds.Name}); err != nil {
		return nil, errors.Wrap(err, "Error listing VMs")
	}
	return &hostList, nil
}

// FilterVMwareClustersForCreds filters VMwareCluster objects for the given credentials
func FilterVMwareClustersForCreds(ctx context.Context, k8sClient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vjailbreakv1alpha1.VMwareClusterList, error) {
	clusterList := vjailbreakv1alpha1.VMwareClusterList{}
	if err := k8sClient.List(ctx, &clusterList, client.InNamespace(constants.NamespaceMigrationSystem), client.MatchingLabels{constants.VMwareCredsLabel: vmwcreds.Name}); err != nil {
		return nil, errors.Wrap(err, "Error listing VMs")
	}
	return &clusterList, nil
}

// FindVMwareMachinesNotInVcenter finds VMwareMachine objects that are not present in the vCenter
func FindVMwareMachinesNotInVcenter(ctx context.Context, client client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, vcenterVMs []vjailbreakv1alpha1.VMInfo) ([]vjailbreakv1alpha1.VMwareMachine, error) {
	vmList, err := FilterVMwareMachinesForCreds(ctx, client, vmwcreds)
	if err != nil {
		return nil, errors.Wrap(err, "Error filtering VMs")
	}
	var staleVMs []vjailbreakv1alpha1.VMwareMachine
	for _, vm := range vmList.Items {
		if !VMExistsInVcenter(vm.Spec.VMInfo.Name, vcenterVMs) {
			staleVMs = append(staleVMs, vm)
		}
	}
	return staleVMs, nil
}

// FindVMwareHostsNotInVcenter finds VMwareHost objects that are not present in the vCenter
func FindVMwareHostsNotInVcenter(ctx context.Context, client client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, clusterInfo []VMwareClusterInfo) ([]vjailbreakv1alpha1.VMwareHost, error) {
	hostList, err := FilterVMwareHostsForCreds(ctx, client, vmwcreds)
	if err != nil {
		return nil, errors.Wrap(err, "Error filtering VMs")
	}
	var staleHosts []vjailbreakv1alpha1.VMwareHost
	for _, host := range hostList.Items {
		if !HostExistsInVcenter(host.Name, clusterInfo) {
			staleHosts = append(staleHosts, host)
		}
	}
	return staleHosts, nil
}

// DeleteStaleVMwareMachines deletes VMwareMachine objects that are not present in the vCenter
func DeleteStaleVMwareMachines(ctx context.Context, client client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, vcenterVMs []vjailbreakv1alpha1.VMInfo) error {
	staleVMs, err := FindVMwareMachinesNotInVcenter(ctx, client, vmwcreds, vcenterVMs)
	if err != nil {
		return errors.Wrap(err, "Error finding stale VMs")
	}
	for _, vm := range staleVMs {
		if err := client.Delete(ctx, &vm); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, fmt.Sprintf("Error deleting stale VM '%s'", vm.Name))
			}
		}
	}
	return nil
}

// VMExistsInVcenter checks if a VM exists in the vCenter
func VMExistsInVcenter(vmName string, vcenterVMs []vjailbreakv1alpha1.VMInfo) bool {
	for _, vm := range vcenterVMs {
		if vm.Name == vmName {
			return true
		}
	}
	return false
}

// HostExistsInVcenter checks if a host exists in the vCenter
func HostExistsInVcenter(hostName string, clusterInfo []VMwareClusterInfo) bool {
	for _, cluster := range clusterInfo {
		for _, host := range cluster.Hosts {
			if host.Name == hostName {
				return true
			}
		}
	}
	return false
}

// DeleteDependantObjectsForVMwareCreds removes all objects dependent on a VMwareCreds resource
func DeleteDependantObjectsForVMwareCreds(ctx context.Context, scope *scope.VMwareCredsScope) error {
	log := scope.Logger
	log.Info("Deleting dependant objects for VMwareCreds", "vmwarecreds", scope.Name())
	if err := DeleteVMwareMachinesForVMwareCreds(ctx, scope); err != nil {
		return errors.Wrap(err, "Error deleting VMs")
	}
	if err := DeleteVMwareHostsForVMwareCreds(ctx, scope); err != nil {
		return errors.Wrap(err, "Error deleting hosts")
	}
	if err := DeleteVMwareClustersForVMwareCreds(ctx, scope); err != nil {
		return errors.Wrap(err, "Error deleting clusters")
	}

	if err := DeleteVMwarecredsSecret(ctx, scope); err != nil {
		return errors.Wrap(err, "Error deleting secret")
	}

	return nil
}

// DeleteVMwarecredsSecret removes the secret associated with a VMwareCreds resource
func DeleteVMwarecredsSecret(ctx context.Context, scope *scope.VMwareCredsScope) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scope.VMwareCreds.Spec.SecretRef.Name,
			Namespace: constants.NamespaceMigrationSystem,
		},
	}
	if err := scope.Client.Delete(ctx, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to delete associated secret")
		}
	}
	return nil
}

// DeleteVMwareMachinesForVMwareCreds removes all VMwareMachine objects associated with a VMwareCreds resource
func DeleteVMwareMachinesForVMwareCreds(ctx context.Context, scope *scope.VMwareCredsScope) error {
	vmList, err := FilterVMwareMachinesForCreds(ctx, scope.Client, scope.VMwareCreds)
	if err != nil {
		return errors.Wrap(err, "Error filtering VMs")
	}
	for _, vm := range vmList.Items {
		if err := scope.Client.Delete(ctx, &vm); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, fmt.Sprintf("error deleting VM '%s'", vm.Name))
			}
		}
	}
	return nil
}

// DeleteVMwareClustersForVMwareCreds removes all VMwareCluster objects associated with a VMwareCreds resource
func DeleteVMwareClustersForVMwareCreds(ctx context.Context, scope *scope.VMwareCredsScope) error {
	clusterList, err := FilterVMwareClustersForCreds(ctx, scope.Client, scope.VMwareCreds)
	if err != nil {
		return errors.Wrap(err, "Error filtering VMs")
	}
	for _, cluster := range clusterList.Items {
		if err := scope.Client.Delete(ctx, &cluster); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, fmt.Sprintf("error deleting VM '%s'", cluster.Name))
			}
		}
	}
	return nil
}

// DeleteVMwareHostsForVMwareCreds removes all VMwareHost objects associated with a VMwareCreds resource
func DeleteVMwareHostsForVMwareCreds(ctx context.Context, scope *scope.VMwareCredsScope) error {
	hostList, err := FilterVMwareHostsForCreds(ctx, scope.Client, scope.VMwareCreds)
	if err != nil {
		return errors.Wrap(err, "Error filtering VMs")
	}
	for _, host := range hostList.Items {
		if err := scope.Client.Delete(ctx, &host); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, fmt.Sprintf("error deleting VM '%s'", host.Name))
			}
		}
	}
	return nil
}

// containsString checks if a string exists in a slice
func containsString(slice []string, target string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

// Helper to check if Phase is not in managing/managed/error
func isPhaseUpdatable(phase string) bool {
	return phase != constants.RDMPhaseManaging &&
		phase != constants.RDMPhaseManaged &&
		phase != constants.RDMPhaseError
}

// syncRDMDisks handles synchronization of RDM disk information between VMInfo and VMwareMachine
func syncRDMDisks(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, rdmInfo []vjailbreakv1alpha1.RDMDisk) error {
	// Both have RDM disks - preserve OpenStack related information
	// Create a map of existing VMware Machine RDM disks by disk name
	existingDisks := make(map[string]vjailbreakv1alpha1.RDMDisk)
	for _, disk := range rdmInfo {
		rdmDiskCR := &vjailbreakv1alpha1.RDMDisk{}
		err := k3sclient.Get(ctx, k8stypes.NamespacedName{
			Name:      strings.TrimSpace(disk.Name),
			Namespace: constants.NamespaceMigrationSystem,
		}, rdmDiskCR)

		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to get existing RDM disk CR", "name", disk.Name)
		} else {
			existingDisks[disk.Name] = *rdmDiskCR
		}
	}

	// Update VMInfo RDM disks while preserving OpenStack information
	for i := range rdmInfo {
		if existingDisk, ok := existingDisks[rdmInfo[i].Name]; ok {
			// Preserve OpenStack volume reference if new one is nil
			if reflect.DeepEqual(rdmInfo[i].Spec.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) &&
				!reflect.DeepEqual(existingDisk.Spec.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) {
				rdmInfo[i].Spec.OpenstackVolumeRef = existingDisk.Spec.OpenstackVolumeRef
			} else {
				// Preserve CinderBackendPool if new one is nil
				if rdmInfo[i].Spec.OpenstackVolumeRef.CinderBackendPool == "" &&
					existingDisk.Spec.OpenstackVolumeRef.CinderBackendPool != "" {
					rdmInfo[i].Spec.OpenstackVolumeRef.CinderBackendPool = existingDisk.Spec.OpenstackVolumeRef.CinderBackendPool
				}

				// Preserve VolumeType if new one is nil
				if rdmInfo[i].Spec.OpenstackVolumeRef.VolumeType == "" &&
					existingDisk.Spec.OpenstackVolumeRef.VolumeType != "" {
					rdmInfo[i].Spec.OpenstackVolumeRef.VolumeType = existingDisk.Spec.OpenstackVolumeRef.VolumeType
				}

				// Update Openstack Volume Reference if existing disk doesn't have it but new one does
				if reflect.DeepEqual(existingDisk.Spec.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) &&
					!reflect.DeepEqual(rdmInfo[i].Spec.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) {
					existingDisk.Spec.OpenstackVolumeRef = rdmInfo[i].Spec.OpenstackVolumeRef
				}

				// Update RDM disk CR if existing and new OpenStack volume reference does not match, and existing disk is not in managing, managed or error phase
				if !reflect.DeepEqual(existingDisk.Spec.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) &&
					!reflect.DeepEqual(rdmInfo[i].Spec.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) &&
					!reflect.DeepEqual(existingDisk.Spec.OpenstackVolumeRef, rdmInfo[i].Spec.OpenstackVolumeRef) &&
					isPhaseUpdatable(existingDisk.Status.Phase) {
					existingDisk.Spec.OpenstackVolumeRef = rdmInfo[i].Spec.OpenstackVolumeRef
				}

				// Add owner VMs if new reference is added
				for _, ownerVM := range rdmInfo[i].Spec.OwnerVMs {
					if !slices.Contains(existingDisk.Spec.OwnerVMs, ownerVM) {
						existingDisk.Spec.OwnerVMs = AppendUnique(existingDisk.Spec.OwnerVMs, ownerVM)
					}
				}

				// Ensure owner reference is set to VMwareCreds for existing RDM disks
				ownerRefExists := false
				for _, ownerRef := range existingDisk.OwnerReferences {
					if ownerRef.UID == vmwcreds.UID &&
						ownerRef.Name == vmwcreds.GetName() &&
						ownerRef.Kind == "VMwareCreds" {
						ownerRefExists = true
						break
					}
				}
				if !ownerRefExists {
					if err := controllerutil.SetOwnerReference(vmwcreds, &existingDisk, k3sclient.Scheme()); err != nil {
						return fmt.Errorf("failed to set owner reference on existing RDM disk CR '%s': %w", existingDisk.Name, err)
					}
				}

				err := k3sclient.Update(ctx, &existingDisk)
				if err != nil {
					return fmt.Errorf("failed to update existing RDM disk CR with new OpenStack volume reference: %w", err)
				}
				log.FromContext(ctx).Info("Updated existing RDM disk CR with new OpenStack volume reference", "name", existingDisk.Name)
			}
		} else {
			// Create RDM disk CR if it doesn't exist
			rdmDiskCR := &vjailbreakv1alpha1.RDMDisk{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.TrimSpace(rdmInfo[i].Name),
					Namespace: constants.NamespaceMigrationSystem,
					Labels: map[string]string{
						constants.VMwareCredsLabel: vmwcreds.Name,
					},
				},
				Spec: vjailbreakv1alpha1.RDMDiskSpec{
					DiskName:           rdmInfo[i].Spec.DiskName,
					DiskSize:           rdmInfo[i].Spec.DiskSize,
					DisplayName:        rdmInfo[i].Spec.DisplayName,
					UUID:               rdmInfo[i].Spec.UUID,
					OwnerVMs:           rdmInfo[i].Spec.OwnerVMs,
					OpenstackVolumeRef: rdmInfo[i].Spec.OpenstackVolumeRef,
				},
			}

			// Set the owner reference to VMwareCreds so RDM disks are deleted when VMwareCreds is deleted
			if err := controllerutil.SetOwnerReference(vmwcreds, rdmDiskCR, k3sclient.Scheme()); err != nil {
				return fmt.Errorf("failed to set owner reference on RDM disk CR '%s': %w", rdmDiskCR.Name, err)
			}
			err := k3sclient.Create(ctx, rdmDiskCR)
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return fmt.Errorf("failed to create RDM disk CR: %w", err)
				}
				// If it already exists, update the existing CR with new information
				err = k3sclient.Update(ctx, rdmDiskCR)
				if err != nil {
					return fmt.Errorf("failed to update existing RDM disk CR: %w", err)
				}
			}
			log := log.FromContext(ctx)
			log.Info("Created new RDM disk CR", "name", rdmDiskCR.Name)
		}
	}
	return nil
}

// getHostStorageDeviceInfo retrieves the storage device information for the host of a given VM
func getHostStorageDeviceInfo(ctx context.Context, vm *object.VirtualMachine, hostStorageMap *sync.Map) (*types.HostStorageDeviceInfo, error) {
	hostSystem, err := vm.HostSystem(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host system: %v", err)
	}
	var hostStorageDevice *types.HostStorageDeviceInfo
	hostStorageDevicefromMap, ok := hostStorageMap.Load(hostSystem.String())
	if ok {
		hostStorageDevice, ok = hostStorageDevicefromMap.(*types.HostStorageDeviceInfo)
		if !ok {
			return nil, fmt.Errorf("invalid type assertion for host system from map")
		}
	} else {
		var hs mo.HostSystem
		err = hostSystem.Properties(ctx, hostSystem.Reference(), []string{"config.storageDevice"}, &hs)
		if err != nil || (hs.Config == nil && hs.Config.StorageDevice == nil) {
			return nil, fmt.Errorf("failed to get host system properties: %v", err)
		}
		hostStorageMap.Store(hostSystem.String(), hs.Config.StorageDevice)
		hostStorageDevice = hs.Config.StorageDevice
	}
	return hostStorageDevice, nil
}

// populateRDMDiskInfoFromAttributes processes VM annotations and custom attributes to populate RDM disk information
// RDM disk attributes in Vmware for migration - VJB_RDM:diskName:volumeRef:value
// eg:
//
//	VJB_RDM:Hard Disk:volumeRef:"source-id"="abac111"
func populateRDMDiskInfoFromAttributes(ctx context.Context, baseRDMDisk vjailbreakv1alpha1.RDMDisk, attributes []string) (vjailbreakv1alpha1.RDMDisk, error) {
	log := log.FromContext(ctx)
	// Process attributes for additional RDM information
	for _, attr := range attributes {
		if strings.Contains(attr, "VJB_RDM:") {
			parts := strings.Split(attr, ":")
			if len(parts) != 4 {
				continue
			}

			diskName := strings.TrimSpace(parts[1])
			key := parts[2]
			value := parts[3]

			// Update fields only if new value is provided
			if strings.TrimSpace(key) == "volumeRef" && value != "" {
				splotVolRef := strings.Split(value, "=")
				if len(splotVolRef) != 2 {
					return vjailbreakv1alpha1.RDMDisk{}, fmt.Errorf("invalid volume reference format: %s", baseRDMDisk.Spec.OpenstackVolumeRef)
				}
				mp := make(map[string]string)
				mp[splotVolRef[0]] = splotVolRef[1]
				log.Info("Setting OpenStack Volume Ref for RDM disk", "diskName", diskName, "volumeRef", mp)
				baseRDMDisk.Spec.OpenstackVolumeRef = vjailbreakv1alpha1.OpenstackVolumeRef{
					VolumeRef: mp,
				}
			}
		}
	}
	return baseRDMDisk, nil
}

// getClusterNameFromHost gets the cluster name from a host system
func getClusterNameFromHost(ctx context.Context, c *vim25.Client, host mo.HostSystem) string {
	if host.Parent == nil {
		return ""
	}

	// Determine parent type based on the object reference type
	parentType := host.Parent.Type
	// Get the parent name
	var parentEntity mo.ManagedEntity
	err := property.DefaultCollector(c).RetrieveOne(ctx, *host.Parent, []string{"name"}, &parentEntity)
	if err != nil {
		fmt.Printf("failed to get parent info for host %s: %v\n", host.Name, err)
		return ""
	}

	// Handle based on the parent's type
	switch parentType {
	case "ClusterComputeResource":
		var cluster mo.ClusterComputeResource
		err = property.DefaultCollector(c).RetrieveOne(ctx, *host.Parent, []string{"name"}, &cluster)
		if err != nil {
			fmt.Printf("failed to get cluster name for host %s: %v\n", host.Name, err)
			return ""
		}
		return cluster.Name
	case "ComputeResource":
		var compute mo.ComputeResource
		err = property.DefaultCollector(c).RetrieveOne(ctx, *host.Parent, []string{"name"}, &compute)
		if err != nil {
			fmt.Printf("failed to get compute resource name for host %s: %v\n", host.Name, err)
			return ""
		}
		return compute.Name
	default:
		fmt.Printf("unknown parent type for host %s: %s\n", host.Name, parentType)
		return ""
	}
}

// CreateOrUpdateRDMDisks creates or updates CreateOrUpdateRDMDisks objects for the given VMs
func CreateOrUpdateRDMDisks(ctx context.Context, client client.Client,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds, sm *sync.Map) error {
	logger := log.FromContext(ctx)
	var values []vjailbreakv1alpha1.RDMDisk
	sm.Range(func(_, value interface{}) bool {
		rdmDisk, ok := value.(vjailbreakv1alpha1.RDMDisk)
		if !ok {
			logger.Error(fmt.Errorf("unexpected type for RDM disk: %T", value), "Type assertion failed")
			return true
		}
		values = append(values, rdmDisk)
		return true
	})
	err := syncRDMDisks(ctx, client, vmwcreds, values)
	if err != nil {
		return err
	}
	return nil
}

// getCinderVolumeBackendPools retrieves the list of Cinder volume backend pools from OpenStack
func getCinderVolumeBackendPools(ctx context.Context, openstackClients *OpenStackClients) ([]string, error) {
	allStoragePoolPages, err := schedulerstats.List(openstackClients.BlockStorageClient, nil).AllPages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all storage backend pools")
	}
	pools, err := schedulerstats.ExtractStoragePools(allStoragePoolPages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract all storage backend pools")
	}
	volBackendPools := make([]string, 0, len(pools))
	for _, pool := range pools {
		volBackendPools = append(volBackendPools, pool.Name)
	}
	return volBackendPools, nil
}

// GetFinderForVMwareCreds creates a finder for VMware credentials
func GetFinderForVMwareCreds(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter string) (*vim25.Client, *find.Finder, error) {
	c, err := ValidateVMwareCreds(ctx, k3sclient, vmwcreds)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}
	if c != nil {
		defer c.CloseIdleConnections()
		defer func() {
			if err := LogoutVMwareClient(ctx, k3sclient, vmwcreds, c); err != nil {
				log.FromContext(ctx).Error(err, "Failed to logout VMware client")
			}
		}()
	}
	finder := find.NewFinder(c, false)

	if datacenter != "" {
		dc, err := finder.Datacenter(ctx, datacenter)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find datacenter: %w", err)
		}
		finder.SetDatacenter(dc)
	}
	return c, finder, nil
}

var hostStorageMap *sync.Map = &sync.Map{}
var rdmDiskMap *sync.Map = &sync.Map{}

func buildVMInfo(moVM mo.VirtualMachine, dsNames []string, disks []vjailbreakv1alpha1.Disk, netNames []string, rdmForVM []string, esxiName string, clusterName string, nicList []vjailbreakv1alpha1.NIC, guestNetworks []vjailbreakv1alpha1.GuestNetwork, osFamily string) vjailbreakv1alpha1.VMInfo {
	return vjailbreakv1alpha1.VMInfo{
		Name:              moVM.Config.Name,
		Datastores:        dsNames,
		Disks:             disks,
		Networks:          netNames,
		IPAddress:         moVM.Guest.IpAddress,
		VMState:           moVM.Guest.GuestState,
		OSFamily:          osFamily,
		CPU:               int(moVM.Config.Hardware.NumCPU),
		Memory:            int(moVM.Config.Hardware.MemoryMB),
		RDMDisks:          rdmForVM,
		ESXiName:          esxiName,
		ClusterName:       clusterName,
		NetworkInterfaces: nicList,
		GuestNetworks:     guestNetworks,
	}
}

func processGuestNetworks(ctx context.Context, moVM mo.VirtualMachine, nicList []vjailbreakv1alpha1.NIC, vmwareMachineMap map[string]*vjailbreakv1alpha1.VMwareMachine, log logr.Logger) ([]vjailbreakv1alpha1.GuestNetwork, string, []vjailbreakv1alpha1.NIC, error) {
	// Get the guest network info (already fetched in bulk via Property Collector)
	guestNetworksFromVmware, err := ExtractGuestNetworkInfo(&moVM)
	if err != nil {
		return nil, "", nicList, fmt.Errorf("failed to extract guest network info: %w", err)
	}

	var guestNetworks []vjailbreakv1alpha1.GuestNetwork
	var osFamily string

	// Lookup existing VMwareMachine from bulk-fetched map (no K8s API call)
	vmwvm, exists := vmwareMachineMap[moVM.Config.Name]

	if !exists {
		// First time creation – use whatever vCenter gave us (could be nil)
		guestNetworks = guestNetworksFromVmware
		osFamily = moVM.Guest.GuestFamily
	} else {
		// Object exists
		if len(guestNetworksFromVmware) > 0 {
			// Only update if we got fresh data from vCenter
			guestNetworks = guestNetworksFromVmware
		} else {
			// Use existing data because VM is switched off and we can't get the info from vCenter
			guestNetworks = vmwvm.Spec.VMInfo.GuestNetworks
		}
		if moVM.Guest.GuestFamily != "" {
			osFamily = moVM.Guest.GuestFamily
		} else {
			osFamily = vmwvm.Spec.VMInfo.OSFamily
		}
	}

	// Update NIC list with IP addresses
	updatedNicList := make([]vjailbreakv1alpha1.NIC, len(nicList))
	copy(updatedNicList, nicList)

	if len(guestNetworksFromVmware) > 0 {
		// Extract IP addresses from guest networks and set it in network interfaces
		for i, nic := range updatedNicList {
			for _, guestNet := range guestNetworksFromVmware {
				if nic.MAC == guestNet.MAC {
					// Check if IP is ipv4
					if !strings.Contains(guestNet.IP, ":") {
						updatedNicList[i].IPAddress = guestNet.IP
					}
					break
				}
			}
		}
	} else {
		log.Info("No guest network info available from VMware for vm", "VM NAME", moVM.Config.Name)
		// Check if network Interfaces have IP addresses from previous runs, if yes, retain them
		for _, nic := range vmwvm.Spec.VMInfo.NetworkInterfaces {
			for i, existingNic := range updatedNicList {
				if existingNic.MAC == nic.MAC {
					updatedNicList[i].IPAddress = nic.IPAddress
					break
				}
			}
		}
	}

	return guestNetworks, osFamily, updatedNicList, nil
}

func getHostClusterInfo(ctx context.Context, moVM mo.VirtualMachine, client *vim25.Client, collector *property.Collector) (string, string, error) {
	// Determine cluster name
	val, ok := hostSystemMap.Load(moVM.Runtime.Host.Value)
	if !ok {
		// Get the host name and parent (cluster) information
		host := mo.HostSystem{}
		hostSystemMap.Store(host.Reference().Value, host)

		err := collector.RetrieveOne(ctx, *moVM.Runtime.Host, []string{"name", "parent"}, &host)
		if err != nil {
			return "", "", fmt.Errorf("failed to get host name: %w", err)
		}
		clusterName := getClusterNameFromHost(ctx, client, host)
		if strings.TrimSpace(moVM.Config.Name) == "" {
			return "", "", fmt.Errorf("VM has no name")
		}
		esxiName := host.Name
		clusterDetails := clusterDetails{
			ESXIName:    esxiName,
			ClusterName: clusterName,
		}
		hostSystemMap.Store(moVM.Runtime.Host.Value, clusterDetails)
		return clusterName, esxiName, nil
	} else if val != nil {
		if details, ok := val.(clusterDetails); ok {
			return details.ClusterName, details.ESXIName, nil
		}
	}
	return "", "", fmt.Errorf("unable to get host cluster info")
}

func processRDMDisks(ctx context.Context, moVM mo.VirtualMachine, hostStorageInfo *types.HostStorageDeviceInfo, attributes []string, rdmDiskMap *sync.Map, log logr.Logger) []string {
	rdmForVM := make([]string, 0)

	for _, device := range moVM.Config.Hardware.Device {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}

		_, rdmInfo, err := processVMDisk(ctx, disk, hostStorageInfo, moVM.Config.Name)
		if err != nil {
			log.Error(err, "failed to process VM disk", "VM NAME", moVM.Config.Name)
			continue
		}

		// check if rdmInfo is empty
		if !reflect.DeepEqual(rdmInfo, vjailbreakv1alpha1.RDMDisk{}) {
			rdmForVM = append(rdmForVM, strings.TrimSpace(rdmInfo.Name))
			rdmInfo, err := populateRDMDiskInfoFromAttributes(ctx, rdmInfo, attributes)
			if err != nil {
				log.Error(err, "failed to populate RDM disk info from attributes for vm", "VM NAME", moVM.Config.Name)
				continue
			}

			if savedRDM, ok := rdmDiskMap.Load(rdmInfo.Name); ok && savedRDM != nil {
				savedRDMDetails, ok := savedRDM.(vjailbreakv1alpha1.RDMDisk)
				if !ok {
					log.Error(fmt.Errorf("invalid type for savedRDM"), "expected RDMDisk", "got", fmt.Sprintf("%T", savedRDM))
					continue
				}
				// Compare OpenstackVolumeRef details
				if savedRDMDetails.Spec.OpenstackVolumeRef.VolumeRef != nil && rdmInfo.Spec.OpenstackVolumeRef.VolumeRef != nil {
					if !reflect.DeepEqual(savedRDMDetails.Spec.OpenstackVolumeRef.VolumeRef, rdmInfo.Spec.OpenstackVolumeRef.VolumeRef) {
						log.Info("RDM VolumeRef doesn't match compared to other clustered VM's, skipping the VM", "diskName", rdmInfo.Spec.DiskName, "vmName", moVM.Config.Name, "otherVMs", savedRDMDetails.Spec.OwnerVMs)
						continue
					}
				}
				// Add owner VMs if not exists already and sort OwnerVMs alphabetically
				// Compare existing OwnerVMs with rdmInfos.Spec.OwnerVMs
				rdmInfo.Spec.OwnerVMs = AppendUnique(rdmInfo.Spec.OwnerVMs, moVM.Config.Name)
				slices.Sort(rdmInfo.Spec.OwnerVMs) // Sort OwnerVMs alphabetically
			}

			savedInfo, loaded := rdmDiskMap.LoadOrStore(strings.TrimSpace(rdmInfo.Name), rdmInfo)
			if loaded {
				savedInfoDetails, ok := savedInfo.(vjailbreakv1alpha1.RDMDisk)
				if ok && !reflect.DeepEqual(rdmInfo.Spec.OwnerVMs, savedInfoDetails.Spec.OwnerVMs) {
					rdmInfo.Spec.OwnerVMs = AppendUnique(rdmInfo.Spec.OwnerVMs, savedInfoDetails.Spec.OwnerVMs...)
					slices.Sort(rdmInfo.Spec.OwnerVMs) // Sort OwnerVMs alphabetically
					rdmDiskMap.Store(strings.TrimSpace(rdmInfo.Name), rdmInfo)
				}
			}
		}
	}

	return rdmForVM
}

func validateVM(moVM mo.VirtualMachine, vmMap map[string]*object.VirtualMachine, log logr.Logger) bool {
	switch {
	case moVM.Config == nil:
		log.Error(fmt.Errorf("vm config is nil"), "skipping VM", "VM NAME", moVM.Config.Name)
		return false
	case moVM.Config.Uuid == "":
		log.Error(fmt.Errorf("vm uuid is empty"), "skipping VM", "VM NAME", moVM.Config.Name)
		return false
	case vmMap[moVM.Config.Uuid] == nil:
		log.Error(fmt.Errorf("vm not found in vmMap"), "skipping VM", "VM NAME", moVM.Config.Name)
		return false
	}
	return true
}

// VMwareObject represents an interface for VMware objects that have Self and Name properties
type VMwareObject interface {
	mo.Datastore | mo.Network
}

func retrieveNames[T VMwareObject](ctx context.Context, collector *property.Collector, refs map[types.ManagedObjectReference]struct{}, logger logr.Logger, objectType string) map[types.ManagedObjectReference]string {
	mapObj := make(map[types.ManagedObjectReference]string)
	if len(refs) > 0 {
		var objectList []T
		refList := make([]types.ManagedObjectReference, 0, len(refs))
		for ref := range refs {
			refList = append(refList, ref)
		}
		if err := collector.Retrieve(ctx, refList, []string{"name"}, &objectList); err == nil {
			for _, obj := range objectList {
				switch v := any(obj).(type) {
				case mo.Datastore:
					mapObj[v.Self] = v.Name
				case mo.Network:
					mapObj[v.Self] = v.Name
				}
			}
		} else {
			logger.Error(err, fmt.Sprintf("failed to retrieve %s names", objectType))
		}
	}
	return mapObj
}

func collectVMReferences(fetchedVMs []mo.VirtualMachine) (map[types.ManagedObjectReference]struct{}, map[types.ManagedObjectReference]struct{}, map[string][]string) {
	dsRefs := make(map[types.ManagedObjectReference]struct{})
	netRefs := make(map[types.ManagedObjectReference]struct{})
	attributeMap := make(map[string][]string)

	for _, moVM := range fetchedVMs {
		for _, ref := range moVM.Datastore {
			dsRefs[ref] = struct{}{}
		}
		for _, ref := range moVM.Network {
			netRefs[ref] = struct{}{}
		}
		attributeMap[moVM.Reference().Value] = strings.Split(moVM.Summary.Config.Annotation, "\n")
	}

	return dsRefs, netRefs, attributeMap
}

// bulkFetchVMwareMachines fetches all VMwareMachine objects in one K8s List API call
// This eliminates N individual K8s Get calls during processGuestNetworks
func bulkFetchVMwareMachines(ctx context.Context, scope *scope.VMwareCredsScope, fetchedVMs []mo.VirtualMachine, log logr.Logger) map[string]*vjailbreakv1alpha1.VMwareMachine {
	vmMachineMap := make(map[string]*vjailbreakv1alpha1.VMwareMachine)

	// List all VMwareMachine objects for this credential
	vmList := &vjailbreakv1alpha1.VMwareMachineList{}
	if err := scope.Client.List(ctx, vmList,
		client.InNamespace(scope.Namespace()),
		client.MatchingLabels{constants.VMwareCredsLabel: scope.Name()}); err != nil {
		log.Error(err, "failed to bulk fetch VMwareMachine objects")
		return vmMachineMap
	}

	// Build map by VM name for quick lookup
	for i := range vmList.Items {
		vmMachineMap[vmList.Items[i].Spec.VMInfo.Name] = &vmList.Items[i]
	}

	return vmMachineMap
}

// bulkRetrieveHostInfo fetches all host information (name, parent cluster) in one bulk API call
// This eliminates N individual API calls and avoids vCenter throttling
func bulkRetrieveHostInfo(ctx context.Context, collector *property.Collector, hostRefs map[types.ManagedObjectReference]struct{}, client *vim25.Client, log logr.Logger) map[types.ManagedObjectReference]clusterDetails {
	hostInfoMap := make(map[types.ManagedObjectReference]clusterDetails)

	if len(hostRefs) == 0 {
		return hostInfoMap
	}

	// Convert map to slice for bulk retrieval
	refList := make([]types.ManagedObjectReference, 0, len(hostRefs))
	for ref := range hostRefs {
		refList = append(refList, ref)
	}

	// Bulk retrieve host properties
	var hosts []mo.HostSystem
	if err := collector.Retrieve(ctx, refList, []string{"name", "parent"}, &hosts); err != nil {
		log.Error(err, "failed to bulk retrieve host information")
		return hostInfoMap
	}

	// Process each host to get cluster name
	for _, host := range hosts {
		clusterName := getClusterNameFromHost(ctx, client, host)
		hostInfoMap[host.Reference()] = clusterDetails{
			ESXIName:    host.Name,
			ClusterName: clusterName,
		}
		// Cache in global map for backward compatibility
		hostSystemMap.Store(host.Reference().Value, clusterDetails{
			ESXIName:    host.Name,
			ClusterName: clusterName,
		})
	}

	return hostInfoMap
}

func buildVMPropertyFilter(vms []*object.VirtualMachine) types.PropertyFilterSpec {
	// Build ObjectSet for VM retrieval
	objectSet := make([]types.ObjectSpec, 0, len(vms))
	for _, vm := range vms {
		objectSet = append(objectSet, types.ObjectSpec{
			Obj:  vm.Reference(),
			Skip: types.NewBool(false),
		})
	}

	// Retrieve VM core properties - expanded for bulk fetching to avoid throttling
	return types.PropertyFilterSpec{
		ObjectSet: objectSet,
		PropSet: []types.PropertySpec{
			{
				Type: "VirtualMachine",
				PathSet: []string{
					"config.hardware.device", "config.name", "config.uuid", "config.instanceUuid",
					"guest.ipAddress", "guest.guestState", "guest.guestFamily", "guest.net",
					"config.hardware.numCPU", "config.hardware.memoryMB",
					"datastore", "network", "runtime", "runtime.host",
					"summary.config.annotation",
				},
			},
		},
	}
}

func getVMDetails(ctx context.Context, scope *scope.VMwareCredsScope, vms []*object.VirtualMachine, client *vim25.Client) ([]vjailbreakv1alpha1.VMInfo, *sync.Map, error) {
	vmMap := make(map[string]*object.VirtualMachine)
	for _, vm := range vms {
		vmMap[vm.UUID(ctx)] = vm
	}
	const batchSize = 50
	log := scope.Logger
	collector := property.DefaultCollector(client)

	// Build property filter for VM retrieval
	filterSpec := buildVMPropertyFilter(vms)
	req := types.RetrieveProperties{
		SpecSet: []types.PropertyFilterSpec{filterSpec},
	}

	res, err := collector.RetrieveProperties(ctx, req, batchSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve VM properties: %w", err)
	}

	// Decode into VMs
	var fetchedVMs []mo.VirtualMachine
	if err := mo.LoadObjectContent(res.Returnval, &fetchedVMs); err != nil {
		return nil, nil, fmt.Errorf("failed to decode ObjectContent into mo.VirtualMachine: %w", err)
	}
	// Fetch all annotations and collect references
	dsRefs, netRefs, attributeMap := collectVMReferences(fetchedVMs)

	// Collect unique host references from all VMs
	hostRefs := make(map[types.ManagedObjectReference]struct{})
	for _, moVM := range fetchedVMs {
		if moVM.Runtime.Host != nil {
			hostRefs[*moVM.Runtime.Host] = struct{}{}
		}
	}

	// Step 2: Bulk retrieve datastore and network names
	dsMap := retrieveNames[mo.Datastore](ctx, collector, dsRefs, log, "datastore")
	netMap := retrieveNames[mo.Network](ctx, collector, netRefs, log, "network")

	// Step 3: Bulk retrieve host information (name, parent cluster) to avoid per-VM calls
	hostInfoMap := bulkRetrieveHostInfo(ctx, collector, hostRefs, client, log)

	// Step 4: Bulk fetch all VMwareMachine objects to avoid N K8s API calls
	vmwareMachineMap := bulkFetchVMwareMachines(ctx, scope, fetchedVMs, log)

	// Step 5: Build final VMInfo list
	batchInfos := make([]vjailbreakv1alpha1.VMInfo, 0, len(fetchedVMs))
	for _, moVM := range fetchedVMs {
		if moVM.Config == nil || strings.HasPrefix(moVM.Config.Name, "vCLS-") {
			continue
		}

		// Datastore names
		var dsNames []string
		for _, ref := range moVM.Datastore {
			if name, ok := dsMap[ref]; ok {
				dsNames = append(dsNames, name)
			}
		}
		// NIC & Guest Network extraction
		nicList, err := ExtractVirtualNICs(&moVM)
		if err != nil {
			log.Error(err, "failed to extract NICs", "VM NAME", moVM.Config.Name)
			continue
		}
		if moVM.Config == nil || strings.HasPrefix(moVM.Config.Name, "vCLS-") {
			continue
		}
		var netNames []string
		// Retrieve Network on basis on NICs
		// Build networks list from NetworkInterfaces to match NIC count
		for _, nic := range nicList {
			netRef := types.ManagedObjectReference{Type: "Network", Value: nic.Network}
			if name, ok := netMap[netRef]; ok {
				netNames = append(netNames, name)
			} else {
				log.Error(fmt.Errorf("network not found in netMap"), "Network MOR not found", "networkMOR", nic.Network)
			}
		}

		// Process disk information
		disks := make([]vjailbreakv1alpha1.Disk, 0)
		controllers := make(map[int32]types.BaseVirtualSCSIController)
		for _, device := range moVM.Config.Hardware.Device {
			if scsiCtrl, ok := device.(types.BaseVirtualSCSIController); ok {
				controllers[device.GetVirtualDevice().Key] = scsiCtrl
			}
			if disk, ok := device.(*types.VirtualDisk); ok {
				// Skip RDM disks as they're handled separately
				if _, isRDM := disk.Backing.(*types.VirtualDiskRawDiskMappingVer1BackingInfo); isRDM {
					continue
				}
				diskInfo := vjailbreakv1alpha1.Disk{
					Name:       disk.DeviceInfo.GetDescription().Label,
					CapacityGB: int(disk.CapacityInBytes / (1024 * 1024 * 1024)),
				}
				// Get datastore information if available
				switch backing := disk.Backing.(type) {
				case *types.VirtualDiskFlatVer2BackingInfo:
					if backing.Datastore != nil {
						if dsName, ok := dsMap[*backing.Datastore]; ok {
							diskInfo.Datastore = dsName
						}
						diskInfo.DatastoreID = backing.Datastore.Value
					}
				case *types.VirtualDiskSparseVer2BackingInfo:
					if backing.Datastore != nil {
						if dsName, ok := dsMap[*backing.Datastore]; ok {
							diskInfo.Datastore = dsName
						}
						diskInfo.DatastoreID = backing.Datastore.Value
					}
				}
				disks = append(disks, diskInfo)
			}
		}
		// Get basic RDM disk info from VM properties
		if !validateVM(moVM, vmMap, log) {
			continue
		}
		hostStorageInfo, err := getHostStorageDeviceInfo(ctx, vmMap[moVM.Config.Uuid], hostStorageMap)
		if err != nil {
			log.Error(err, "failed to get host storage device info", "VM NAME", moVM.Config.Name)
			continue
		}
		attributes := attributeMap[moVM.Reference().Value]
		var guestNetworks []vjailbreakv1alpha1.GuestNetwork
		var osFamily string
		var clusterName string
		var esxiName string

		// Process RDM disks
		rdmForVM := processRDMDisks(ctx, moVM, hostStorageInfo, attributes, rdmDiskMap, log)

		// Get host and cluster information from bulk-fetched map (no API call)
		if moVM.Runtime.Host == nil {
			log.Error(fmt.Errorf("VM has no host reference"), "failed to get host info", "VM NAME", moVM.Config.Name)
			continue
		}
		hostInfo, ok := hostInfoMap[*moVM.Runtime.Host]
		if !ok {
			log.Error(fmt.Errorf("host info not found in map"), "failed to get host cluster info", "VM NAME", moVM.Config.Name)
			continue
		}
		clusterName = hostInfo.ClusterName
		esxiName = hostInfo.ESXIName

		// Process guest networks and update NICs with IP addresses (using bulk-fetched map, no K8s API call)
		guestNetworks, osFamily, nicList, err = processGuestNetworks(ctx, moVM, nicList, vmwareMachineMap, log)
		if err != nil {
			log.Error(err, "failed to process guest networks", "VM NAME", moVM.Config.Name)
			continue
		}

		// Build and append VMInfo
		vmInfo := buildVMInfo(moVM, dsNames, disks, netNames, rdmForVM, esxiName, clusterName, nicList, guestNetworks, osFamily)
		batchInfos = append(batchInfos, vmInfo)
	}
	return batchInfos, rdmDiskMap, nil
}

var hostSystemMap sync.Map

type clusterDetails struct {
	ClusterName string
	ESXIName    string
}

// FindHotplugBaseFlavor connects to OpenStack and finds a flavor with 0 vCPUs and 0 RAM
func FindHotplugBaseFlavor(computeClient *gophercloud.ServiceClient) (*flavors.Flavor, error) {
	allPages, err := flavors.ListDetail(computeClient, nil).AllPages(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to list flavors: %w", err)
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract flavors: %w", err)
	}

	for _, flavor := range allFlavors {
		if flavor.VCPUs == 0 && flavor.RAM == 0 {
			return &flavor, nil
		}
	}

	return nil, errors.New("no suitable base flavor found (0 vCPU, 0 RAM)")
}

// LogoutVMwareClient logs out from the VMware vCenter client session
func LogoutVMwareClient(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, vcentreClient *vim25.Client) error {
	vmwareCredsinfo, err := GetVMwareCredentialsFromSecret(ctx, k3sclient, vmwcreds.Spec.SecretRef.Name)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error getting vCenter credentials from secret")
		return err
	}

	host := vmwareCredsinfo.Host
	username := vmwareCredsinfo.Username
	password := vmwareCredsinfo.Password
	disableSSLVerification := vmwareCredsinfo.Insecure
	u, err := netutils.NormalizeVCenterURL(host)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error normalizing vCenter URL for logout")
		return err
	}
	u.User = url.UserPassword(username, password)
	// Connect and log in to ESX or vCenter
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
	}
	err = s.Logout(ctx, vcentreClient)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error logging out of vCenter")
		return err
	}
	return nil
}

// CleanupCachedVMwareClient removes the cached VMware client for the given credentials. It's a best effort approach to avoid stale clients.
func CleanupCachedVMwareClient(ctx context.Context, vmwcreds *vjailbreakv1alpha1.VMwareCreds) {
	ctxlog := log.FromContext(ctx)
	mapKey := string(vmwcreds.UID)
	if vmwareClientMap != nil {
		vmwareClientMap.Delete(mapKey)
		ctxlog.Info("Removed VMware client from cache", "uid", string(vmwcreds.UID))
	}
}

// GetBackendPools discovers and returns storage backend pools from OpenStack Cinder
func GetBackendPools(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (map[string]map[string]string, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Discovering backend pools from OpenStack Cinder")

	// Get OpenStack credentials to extract region
	openstackCredential, err := GetOpenstackCredentialsFromSecret(ctx, k3sclient, openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get OpenStack credentials from secret")
	}

	// Get OpenStack client
	providerClient, err := ValidateAndGetProviderClient(ctx, k3sclient, openstackcreds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get OpenStack provider client")
	}

	// Get Cinder client
	cinderClient, err := openstack.NewBlockStorageV3(providerClient, gophercloud.EndpointOpts{
		Region: openstackCredential.RegionName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Cinder client")
	}

	// Get pool backend info
	poolPages, err := schedulerstats.List(cinderClient, schedulerstats.ListOpts{Detail: true}).AllPages(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to list backend pools")
	}

	backendPools, err := schedulerstats.ExtractStoragePools(poolPages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract backend pools")
	}

	ctxlog.Info("Discovered backend pools", "count", len(backendPools))
	ctxlog.Info("Backend pools", "pools", backendPools)

	// Map backend name -> vendor/type info for quick lookup
	backendMap := make(map[string]map[string]string)
	for _, pool := range backendPools {
		vendor := pool.Capabilities.VendorName
		driver := pool.Capabilities.DriverVersion
		volumeType, backendName := parsePoolName(pool.Name)

		// Extract hostname@backend from pool.Name for Cinder manage API
		// pool.Name format: "hostname@backend#pool" or "hostname@backend"
		cinderHost := extractCinderHost(pool.Name)

		backendMap[backendName] = map[string]string{
			"vendor":     vendor,
			"driver":     driver,
			"pool":       pool.Name,
			"volumeType": volumeType,
			"cinderHost": cinderHost,
		}
	}

	return backendMap, nil
}

// parsePoolName extracts backendName and poolName from a full Cinder pool name.
// Example: "host@pure-iscsi-1#vt-pure-iscsi" → ("pure-iscsi-1", "vt-pure-iscsi")
func parsePoolName(fullPoolName string) (volumeType string, backendName string) {
	// Example input: "host@backend#pool"
	parts := strings.Split(fullPoolName, "@")
	if len(parts) < 2 {
		return "", ""
	}

	rest := parts[1]
	segments := strings.SplitN(rest, "#", 2)

	if len(segments) > 1 {
		volumeType = segments[1]
	} else {
		volumeType = "default"
	}

	return volumeType, segments[0]
}

// extractCinderHost extracts the hostname@backend part from full Cinder pool name for the manage API.
// Example: "pcd-ce@pure-iscsi-1#vt-pure-iscsi" → "pcd-ce@pure-iscsi-1"
// Example: "pcd-ce@pure-iscsi-1" → "pcd-ce@pure-iscsi-1"
func extractCinderHost(fullPoolName string) string {
	// Remove the pool part (#pool) if it exists
	parts := strings.Split(fullPoolName, "#")
	return parts[0]
}

// GetArrayVendor normalizes and returns the storage array vendor name from a vendor string
func GetArrayVendor(vendor string) string {
	// Convert vendor to lowercase
	vendor = strings.ToLower(vendor)

	if strings.Contains(vendor, "pure") {
		return "pure"
	}
	return "unsupported"
}

// Contains checks if a datastore is present in the datastores slice
func Contains(datastores []vjailbreakv1alpha1.DatastoreInfo, datastore vjailbreakv1alpha1.DatastoreInfo) bool {
	for _, ds := range datastores {
		if ds.Name == datastore.Name && ds.MoID == datastore.MoID {
			return true
		}
	}
	return false
}
