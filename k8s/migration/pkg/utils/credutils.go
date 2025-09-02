// Package utils provides utility functions for handling credentials and other operations
package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	gophercloud "github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/schedulerstats"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	migrationutils "github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	trueString = "true" // Define at package level
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
	if datacenter == "" {
		return vjailbreakv1alpha1.VMwareCredsInfo{}, errors.Errorf("VCENTER_DATACENTER is missing in secret '%s'", secretName)
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
			ctrllog.Log.Info("Error closing connection", "error", err)
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
	allPages, err := networks.List(openstackClients.NetworkingClient, nil).AllPages()
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

	allPages, err := ports.List(openstackClients.NetworkingClient, nil).AllPages()
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
	allPages, err := volumetypes.List(openstackClients.BlockStorageClient, nil).AllPages()
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

	allVolumeTypePages, err := volumetypes.List(openstackClients.BlockStorageClient, nil).AllPages()
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

	allNetworkPages, err := networks.List(openstackClients.NetworkingClient, nil).AllPages()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list networks")
	}

	allNetworks, err := networks.ExtractNetworks(allNetworkPages)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract all networks")
	}
	volumeBackendPools, err := getCinderVolumeBackendPools(openstackClients)
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
	allPages, err := projects.List(identityClient, listOpts).AllPages()
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
	}).AllPages()
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

	return &vjailbreakv1alpha1.OpenstackInfo{
		VolumeTypes:    openstackvoltypes,
		Networks:       openstacknetworks,
		VolumeBackends: volumeBackendPools,
		SecurityGroups: openstacksecuritygroups,
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
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if openstackCredential.Insecure {
		tlsConfig.InsecureSkipVerify = true
	} else {
		// Get the certificate for the Openstack endpoint
		caCert, certerr := GetCert(openstackCredential.AuthURL)
		if certerr != nil {
			return nil, errors.Wrap(certerr, "failed to get certificate for openstack")
		}
		// Trying to fetch the system cert pool and add the Openstack certificate to it
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to get system cert pool: %w", err)
		}
		if caCertPool == nil {
			caCertPool = x509.NewCertPool()
		}
		caCertPool.AddCert(caCert)
		tlsConfig.RootCAs = caCertPool
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	providerClient.HTTPClient = http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: openstackCredential.AuthURL,
		Username:         openstackCredential.Username,
		Password:         openstackCredential.Password,
		DomainName:       openstackCredential.DomainName,
		TenantName:       openstackCredential.TenantName,
	}
	if err := openstack.Authenticate(providerClient, authOpts); err != nil {
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
	if host[:4] != "http" {
		host = "https://" + host
	}
	if host[len(host)-4:] != "/sdk" {
		host += "/sdk"
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	u.User = url.UserPassword(username, password)
	// Connect and log in to ESX or vCenter
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
		Reauth:   true,
	}

	c := new(vim25.Client)
	err = s.Login(context.Background(), c, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to login to vSphere: %w", err)
	}

	// Check if the datacenter exists
	finder := find.NewFinder(c, false)
	_, err = finder.Datacenter(context.Background(), vmwareCredsinfo.Datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter: %w", err)
	}

	return c, nil
}

// GetVMwNetworks gets the networks of a VM
func GetVMwNetworks(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	// Pre-allocate networks slice to avoid append allocations
	networks := make([]string, 0)
	c, finder, err := getFinderForVMwareCreds(ctx, k3sclient, vmwcreds, datacenter)
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
	c, finder, err := getFinderForVMwareCreds(ctx, k3sclient, vmwcreds, datacenter)
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
				dsref = backing.Datastore.Reference()
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

// GetAllVMs gets all the VMs in a datacenter.
//
//nolint:gocyclo // GetAllVMs is complex but intentional due to VM discovery logic
func GetAllVMs(ctx context.Context, scope *scope.VMwareCredsScope, datacenter string) ([]vjailbreakv1alpha1.VMInfo, error) {
	log := scope.Logger
	vmErrors := []vmError{}
	errMu := sync.Mutex{}
	panicMu := sync.Mutex{}
	panicErrors := []interface{}{}
	vminfoMu := sync.Mutex{}
	var wg sync.WaitGroup

	vjailbreakSettings, err := migrationutils.GetVjailbreakSettings(ctx, scope.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get vjailbreak settings: %w", err)
	}
	log.Info("Fetched vjailbreak settings for vcenter scan concurrency limit", "vcenter_scan_concurrency_limit", vjailbreakSettings.VCenterScanConcurrencyLimit)

	c, finder, err := getFinderForVMwareCreds(ctx, scope.Client, scope.VMwareCreds, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to get finder: %w", err)
	}

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to get vms: %w", err)
	}
	// Pre-allocate vminfo slice with capacity of vms to avoid append allocations
	vminfo := make([]vjailbreakv1alpha1.VMInfo, 0, len(vms))

	// Create a semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, vjailbreakSettings.VCenterScanConcurrencyLimit)

	for i := range vms {
		// Acquire semaphore (blocks if 100 goroutines are already running)
		semaphore <- struct{}{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Release semaphore when done
			defer func() { <-semaphore }()
			// Don't panic on error
			defer func() {
				if r := recover(); r != nil {
					panicMu.Lock()
					panicErrors = append(panicErrors, r)
					panicMu.Unlock()
				}
			}()

			processSingleVM(ctx, scope, vms[i], &errMu, &vmErrors, &vminfoMu, &vminfo, c)
		}(i)
	}
	// Wait for all VMs to be processed
	wg.Wait()

	// Close the semaphore channel after all goroutines have completed
	close(semaphore)

	if len(vmErrors) > 0 {
		log.Error(fmt.Errorf("failed to get (%d) VMs", len(vmErrors)), "failed to get VMs")
		// Print individual VM errors for better debugging
		for _, e := range vmErrors {
			log.Error(e.err, "VM error details", "vmName", e.vmName)
		}
	}

	return vminfo, nil
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
func processVMDisk(ctx context.Context,
	disk *types.VirtualDisk,
	controllers map[int32]types.BaseVirtualSCSIController,
	hostStorageInfo *types.HostStorageDeviceInfo,
	vmName string) (dsref *types.ManagedObjectReference, rdmDiskInfos vjailbreakv1alpha1.RDMDiskInfo, skipVM bool, err error) {
	if controller, ok := controllers[disk.ControllerKey]; ok {
		if controller.GetVirtualSCSIController().SharedBus == types.VirtualSCSISharingPhysicalSharing {
			ctrllog.FromContext(ctx).Info("SKipping VM: VM has SCSI controller with shared bus, migration not supported",
				"vm", vmName)
			return nil, vjailbreakv1alpha1.RDMDiskInfo{}, true, nil
		}
	}
	switch backing := disk.Backing.(type) {
	case *types.VirtualDiskFlatVer2BackingInfo:
		ref := backing.Datastore.Reference()
		dsref = &ref
	case *types.VirtualDiskSparseVer2BackingInfo:
		ref := backing.Datastore.Reference()
		dsref = &ref
	case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
		ref := backing.Datastore.Reference()
		dsref = &ref
		if hostStorageInfo != nil {
			rdmDiskInfos = vjailbreakv1alpha1.RDMDiskInfo{
				DiskName: disk.DeviceInfo.GetDescription().Label,
				DiskSize: disk.CapacityInBytes,
			}
			for _, scsiDisk := range hostStorageInfo.ScsiLun {
				lunDetails := scsiDisk.GetScsiLun()
				if backing.LunUuid == lunDetails.Uuid {
					rdmDiskInfos.DisplayName = lunDetails.DisplayName
					rdmDiskInfos.UUID = lunDetails.Uuid
				}
			}
		}
	default:
		return nil, vjailbreakv1alpha1.RDMDiskInfo{}, false, fmt.Errorf("unsupported disk backing type: %T", disk.Backing)
	}

	return dsref, rdmDiskInfos, false, nil
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
	vmwcreds *vjailbreakv1alpha1.VMwareCreds, vminfo *vjailbreakv1alpha1.VMInfo) error {
	sanitizedVMName, err := GetK8sCompatibleVMWareObjectName(vminfo.Name, vmwcreds.Name)
	if err != nil {
		return fmt.Errorf("failed to get VM name: %w", err)
	}
	esxiK8sName, err := GetK8sCompatibleVMWareObjectName(vminfo.ESXiName, vmwcreds.Name)
	if err != nil {
		return errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	clusterK8sName, err := GetK8sCompatibleVMWareObjectName(vminfo.ClusterName, vmwcreds.Name)
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
			syncRDMDisks(vminfo, vmwvm)
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

// GetClosestFlavour gets the closest flavor for the given CPU and memory
func GetClosestFlavour(cpu, memory int, allFlavors []flavors.Flavor) (*flavors.Flavor, error) {
	// Check if the flavor slice is empty
	if len(allFlavors) == 0 {
		return nil, fmt.Errorf("no flavors available to select from")
	}

	bestFlavor := new(flavors.Flavor)
	bestFlavor.VCPUs = constants.MaxVCPUs
	bestFlavor.RAM = constants.MaxRAM

	// Find the smallest flavor that meets the requirements
	for _, flavor := range allFlavors {
		if flavor.VCPUs >= cpu && flavor.RAM >= memory {
			if flavor.VCPUs < bestFlavor.VCPUs ||
				(flavor.VCPUs == bestFlavor.VCPUs && flavor.RAM < bestFlavor.RAM) {
				bestFlavor = &flavor
			}
		}
	}

	if bestFlavor.VCPUs != constants.MaxVCPUs {
		return bestFlavor, nil
	}
	return nil, fmt.Errorf("no suitable flavor found for %d vCPUs and %d MB RAM", cpu, memory)
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

// syncRDMDisks handles synchronization of RDM disk information between VMInfo and VMwareMachine
func syncRDMDisks(vminfo *vjailbreakv1alpha1.VMInfo, vmwvm *vjailbreakv1alpha1.VMwareMachine) {
	// Both have RDM disks - preserve OpenStack related information
	if vminfo.RDMDisks != nil && vmwvm.Spec.VMInfo.RDMDisks != nil {
		// Create a map of existing VMware Machine RDM disks by disk name
		existingDisks := make(map[string]vjailbreakv1alpha1.RDMDiskInfo)
		for _, disk := range vmwvm.Spec.VMInfo.RDMDisks {
			existingDisks[disk.DiskName] = disk
		}

		// Update VMInfo RDM disks while preserving OpenStack information
		for i, disk := range vminfo.RDMDisks {
			if existingDisk, ok := existingDisks[disk.DiskName]; ok {
				// Preserve OpenStack volume reference if new one is nil
				if reflect.DeepEqual(vminfo.RDMDisks[i].OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) &&
					!reflect.DeepEqual(existingDisk.OpenstackVolumeRef, vjailbreakv1alpha1.OpenStackVolumeRefInfo{}) {
					vminfo.RDMDisks[i].OpenstackVolumeRef = existingDisk.OpenstackVolumeRef
				} else {
					// Preserve CinderBackendPool if new one is nil
					if vminfo.RDMDisks[i].OpenstackVolumeRef.CinderBackendPool == "" &&
						existingDisk.OpenstackVolumeRef.CinderBackendPool != "" {
						vminfo.RDMDisks[i].OpenstackVolumeRef.CinderBackendPool = existingDisk.OpenstackVolumeRef.CinderBackendPool
					}

					// Preserve VolumeType if new one is nil
					if vminfo.RDMDisks[i].OpenstackVolumeRef.VolumeType == "" &&
						existingDisk.OpenstackVolumeRef.VolumeType != "" {
						vminfo.RDMDisks[i].OpenstackVolumeRef.VolumeType = existingDisk.OpenstackVolumeRef.VolumeType
					}
				}
			} else {
				fmt.Printf("RDM attributes exist on VM but disk not found in  RDM disks\n")
			}
		}
	}
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
func populateRDMDiskInfoFromAttributes(ctx context.Context, baseRDMDisks []vjailbreakv1alpha1.RDMDiskInfo, attributes []string) ([]vjailbreakv1alpha1.RDMDiskInfo, error) {
	rdmMap := make(map[string]vjailbreakv1alpha1.RDMDiskInfo)
	log := ctrllog.FromContext(ctx)

	// Create copies of base RDM disks to preserve existing data
	for i := range baseRDMDisks {
		diskCopy := baseRDMDisks[i] // Make a copy
		rdmMap[strings.TrimSpace(diskCopy.DiskName)] = diskCopy
	}
	// Process attributes for additional RDM information
	for _, attr := range attributes {
		if strings.Contains(attr, "VJB_RDM:") {
			fmt.Println("Processing RDM attribute:", attr)
			parts := strings.Split(attr, ":")
			if len(parts) != 4 {
				continue
			}

			diskName := strings.TrimSpace(parts[1])
			key := parts[2]
			value := parts[3]

			// Get or create RDMDiskInfo
			rdmInfo, exists := rdmMap[diskName]
			if exists {
				// Update fields only if new value is provided
				if strings.TrimSpace(key) == "volumeRef" && value != "" {
					splotVolRef := strings.Split(value, "=")
					if len(splotVolRef) != 2 {
						return nil, fmt.Errorf("invalid volume reference format: %s", rdmInfo.OpenstackVolumeRef.VolumeRef)
					}
					mp := make(map[string]string)
					mp[splotVolRef[0]] = splotVolRef[1]
					log.Info("Setting OpenStack Volume Ref for RDM disk:", diskName, "to")
					rdmInfo.OpenstackVolumeRef = vjailbreakv1alpha1.OpenStackVolumeRefInfo{
						VolumeRef: mp,
					}
					rdmMap[diskName] = rdmInfo
				}
			} else {
				log.Info("RDM attributes exist on VM but disk not found in  RDM disks")
			}
		}
	}
	// Convert map back to slice while preserving all data
	rdmDisks := make([]vjailbreakv1alpha1.RDMDiskInfo, 0, len(rdmMap))
	for _, rdmInfo := range rdmMap {
		rdmDisks = append(rdmDisks, rdmInfo)
	}
	return rdmDisks, nil
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

// getCinderVolumeBackendPools retrieves the list of Cinder volume backend pools from OpenStack
func getCinderVolumeBackendPools(openstackClients *OpenStackClients) ([]string, error) {
	allStoragePoolPages, err := schedulerstats.List(openstackClients.BlockStorageClient, nil).AllPages()
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

func appendToVMErrorsThreadSafe(errMu *sync.Mutex, vmErrors *[]vmError, vmName string, err error) {
	errMu.Lock()
	*vmErrors = append(*vmErrors, vmError{vmName: vmName, err: err})
	errMu.Unlock()
}

func appendToVMInfoThreadSafe(vminfoMu *sync.Mutex, vminfo *[]vjailbreakv1alpha1.VMInfo, vmInfo vjailbreakv1alpha1.VMInfo) {
	vminfoMu.Lock()
	*vminfo = append(*vminfo, vmInfo)
	vminfoMu.Unlock()
}

func getFinderForVMwareCreds(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter string) (*vim25.Client, *find.Finder, error) {
	c, err := ValidateVMwareCreds(ctx, k3sclient, vmwcreds)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find datacenter: %w", err)
	}
	finder.SetDatacenter(dc)
	return c, finder, nil
}

//nolint:gocyclo
func processSingleVM(ctx context.Context, scope *scope.VMwareCredsScope, vm *object.VirtualMachine, errMu *sync.Mutex, vmErrors *[]vmError, vminfoMu *sync.Mutex, vminfo *[]vjailbreakv1alpha1.VMInfo, c *vim25.Client) {
	var vmProps mo.VirtualMachine
	var datastores []string
	networks := make([]string, 0, 4) // Pre-allocate with estimated capacity
	disks := make([]string, 0, 8)    // Pre-allocate with estimated capacity
	var clusterName string
	log := scope.Logger
	err := vm.Properties(ctx, vm.Reference(), []string{
		"config",
		"guest",
		"runtime",
		"network",
		"summary.config.annotation",
	}, &vmProps)
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get VM properties: %w", err))
		return
	}
	if vmProps.Config == nil {
		// VM is not powered on or is in creating state
		log.Info("VM properties not available for vm, skipping this VM", "VM NAME", vm.Name())
		return
	}
	// Fetch details required for RDM disks
	hostStorageMap := sync.Map{}
	controllers := make(map[int32]types.BaseVirtualSCSIController)
	// Collect all SCSI controller to find shared RDM disks
	for _, device := range vmProps.Config.Hardware.Device {
		if scsiController, ok := device.(types.BaseVirtualSCSIController); ok {
			controllers[device.GetVirtualDevice().Key] = scsiController
		}
	}
	// Get basic RDM disk info from VM properties
	rdmDiskInfos := make([]vjailbreakv1alpha1.RDMDiskInfo, 0)
	hostStorageInfo, err := getHostStorageDeviceInfo(ctx, vm, &hostStorageMap)
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get disk info for vm: %w", err))
		return
	}

	attributes := strings.Split(vmProps.Summary.Config.Annotation, "\n")
	pc := property.DefaultCollector(c)

	var skipVM bool
	for _, device := range vmProps.Config.Hardware.Device {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}
		dsref, rdmInfos, skip, err := processVMDisk(ctx, disk, controllers, hostStorageInfo, vm.Name())
		if err != nil {
			appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to process VM disk: %w", err))
			return
		}
		if skip {
			skipVM = true
			break
		}
		if !reflect.DeepEqual(rdmInfos, vjailbreakv1alpha1.RDMDiskInfo{}) {
			rdmDiskInfos = append(rdmDiskInfos, rdmInfos)
			continue
		}

		var ds mo.Datastore
		err = pc.RetrieveOne(ctx, *dsref, []string{"name"}, &ds)
		if err != nil {
			appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get datastore: %w", err))
			return
		}

		datastores = AppendUnique(datastores, ds.Name)
		disks = append(disks, disk.DeviceInfo.GetDescription().Label)
	}

	// Get the host name and parent (cluster) information
	host := mo.HostSystem{}
	err = property.DefaultCollector(c).RetrieveOne(ctx, *vmProps.Runtime.Host, []string{"name", "parent"}, &host)
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get host name: %w", err))
		return
	}

	clusterName = getClusterNameFromHost(ctx, c, host)

	if skipVM {
		return
	}
	if len(rdmDiskInfos) >= 1 && len(disks) == 0 {
		log.Info("Skipping VM: VM has RDM disks but no regular bootable disks found, migration not supported", "VM NAME", vm.Name())
		return
	}
	if len(rdmDiskInfos) > 0 {
		log.Info("VM has RDM disks, populating RDM disk info from attributes", "VM NAME", vm.Name())
		rdmDiskInfos, err = populateRDMDiskInfoFromAttributes(ctx, rdmDiskInfos, attributes)
		if err != nil {
			log.Error(err, "failed to populate RDM disk info from attributes for vm", "VM NAME", vm.Name())
			return
		}
	}

	// Get the virtual NICs
	nicList, err := ExtractVirtualNICs(&vmProps)
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get virtual NICs for vm %s: %w", vm.Name(), err))
	}

	// Build networks list from NetworkInterfaces to match NIC count
	for _, nic := range nicList {
		var netObj mo.Network
		netRef := types.ManagedObjectReference{Type: "Network", Value: nic.Network}
		err := pc.RetrieveOne(ctx, netRef, []string{"name"}, &netObj)
		if err != nil {
			appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to retrieve network name for %s: %w", nic.Network, err))
			return
		}
		networks = append(networks, netObj.Name)
	}

	// Get the guest network info
	guestNetworksFromVmware, err := ExtractGuestNetworkInfo(&vmProps)
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get guest network info for vm %s: %w", vm.Name(), err))
	}

	// Convert VM name to Kubernetes-safe name
	vmName, err := GetK8sCompatibleVMWareObjectName(vmProps.Config.Name, scope.Name())
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to convert vm name: %w", err))
	}

	vmwvm := &vjailbreakv1alpha1.VMwareMachine{}
	vmwvmKey := k8stypes.NamespacedName{Name: vmName, Namespace: scope.Namespace()}
	var guestNetworks []vjailbreakv1alpha1.GuestNetwork
	var osFamily string
	err = scope.Client.Get(ctx, vmwvmKey, vmwvm)
	switch {
	case apierrors.IsNotFound(err):
		// First time creation – use whatever vCenter gave us (could be nil)
		guestNetworks = guestNetworksFromVmware
		osFamily = vmProps.Guest.GuestFamily

	case err != nil:
		// Unexpected error
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to get VMwareMachine: %w", err))
		return

	default:
		// Object exists
		if len(guestNetworksFromVmware) > 0 {
			// Only update if we got fresh data from vCenter
			guestNetworks = guestNetworksFromVmware
		} else {
			// Use existing data because VM is switched off and we can't get the info from vCenter
			guestNetworks = vmwvm.Spec.VMInfo.GuestNetworks
		}
		if vmProps.Guest.GuestFamily != "" {
			osFamily = vmProps.Guest.GuestFamily
		} else {
			osFamily = vmwvm.Spec.VMInfo.OSFamily
		}
	}

	if len(guestNetworksFromVmware) > 0 {
		// Extract IP addresses from guest networks and set it in network interfaces
		for i, nic := range nicList {
			for _, guestNet := range guestNetworksFromVmware {
				if nic.MAC == guestNet.MAC {
					// Check if IP is ipv4
					if !strings.Contains(guestNet.IP, ":") {
						nicList[i].IPAddress = guestNet.IP
					}
					break
				}
			}
		}
	} else {
		log.Info("No guest network info available from VMware for vm", "VM NAME", vm.Name())
		// Check if network Interfaces have IP addresses from previous runs, if yes, retain them
		for _, nic := range vmwvm.Spec.VMInfo.NetworkInterfaces {
			for i, existingNic := range nicList {
				if existingNic.MAC == nic.MAC {
					nicList[i].IPAddress = nic.IPAddress
					break
				}
			}
		}
	}

	// exclude vCLS VMs
	if strings.HasPrefix(vmProps.Config.Name, "vCLS-") {
		return
	}
	currentVM := vjailbreakv1alpha1.VMInfo{
		Name:              vmProps.Config.Name,
		Datastores:        datastores,
		Disks:             disks,
		Networks:          networks,
		IPAddress:         vmProps.Guest.IpAddress,
		VMState:           vmProps.Guest.GuestState,
		OSFamily:          osFamily,
		CPU:               int(vmProps.Config.Hardware.NumCPU),
		Memory:            int(vmProps.Config.Hardware.MemoryMB),
		ESXiName:          host.Name,
		ClusterName:       clusterName,
		RDMDisks:          rdmDiskInfos,
		NetworkInterfaces: nicList,
		GuestNetworks:     guestNetworks,
	}
	appendToVMInfoThreadSafe(vminfoMu, vminfo, currentVM)
	err = CreateOrUpdateVMwareMachine(ctx, scope.Client, scope.VMwareCreds, &currentVM)
	if err != nil {
		appendToVMErrorsThreadSafe(errMu, vmErrors, vm.Name(), fmt.Errorf("failed to create or update VMwareMachine: %w", err))
	}
}

// FindHotplugBaseFlavor connects to OpenStack and finds a flavor with 0 vCPUs and 0 RAM
func FindHotplugBaseFlavor(computeClient *gophercloud.ServiceClient) (*flavors.Flavor, error) {
	allPages, err := flavors.ListDetail(computeClient, nil).AllPages()
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
