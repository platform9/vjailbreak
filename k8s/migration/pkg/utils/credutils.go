// Package utils provides utility functions for handling credentials and other operations
package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	govmitypes "github.com/vmware/govmomi/vim25/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// OpenStackClients holds clients for interacting with OpenStack services
type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
}

// VMwareCredentials holds the actual credentials after decoding
type VMwareCredentials struct {
	Host       string
	Username   string
	Password   string
	Datacenter string
	Insecure   bool
}

// OpenStackCredentials holds the actual credentials after decoding
type OpenStackCredentials struct {
	AuthURL    string
	Username   string
	Password   string
	RegionName string
	TenantName string
	Insecure   bool
	DomainName string
}

// GetVMwareCredentials retrieves vCenter credentials from a secret
func GetVMwareCredentials(ctx context.Context, k3sclient client.Client, secretName string) (VMwareCredentials, error) {
	secret := &corev1.Secret{}

	// Get In cluster client
	if err := k3sclient.Get(ctx, types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return VMwareCredentials{}, errors.Wrapf(err, "failed to get secret '%s'", secretName)
	}

	if secret.Data == nil {
		return VMwareCredentials{}, fmt.Errorf("no data in secret '%s'", secretName)
	}

	host := string(secret.Data["VCENTER_HOST"])
	username := string(secret.Data["VCENTER_USERNAME"])
	password := string(secret.Data["VCENTER_PASSWORD"])
	insecureStr := string(secret.Data["VCENTER_INSECURE"])
	datacenter := string(secret.Data["VCENTER_DATACENTER"])

	if host == "" {
		return VMwareCredentials{}, errors.Errorf("VCENTER_HOST is missing in secret '%s'", secretName)
	}
	if username == "" {
		return VMwareCredentials{}, errors.Errorf("VCENTER_USERNAME is missing in secret '%s'", secretName)
	}
	if password == "" {
		return VMwareCredentials{}, errors.Errorf("VCENTER_PASSWORD is missing in secret '%s'", secretName)
	}
	if datacenter == "" {
		return VMwareCredentials{}, errors.Errorf("VCENTER_DATACENTER is missing in secret '%s'", secretName)
	}

	insecure := strings.TrimSpace(insecureStr) == "true"

	return VMwareCredentials{
		Host:       host,
		Username:   username,
		Password:   password,
		Datacenter: datacenter,
		Insecure:   insecure,
	}, nil
}

// GetOpenstackCredentials retrieves and checks the secret
func GetOpenstackCredentials(ctx context.Context, k3sclient client.Client, secretName string) (OpenStackCredentials, error) {
	secret := &corev1.Secret{}
	if err := k3sclient.Get(ctx, types.NamespacedName{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
		return OpenStackCredentials{}, errors.Wrap(err, "failed to get secret")
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
			return OpenStackCredentials{}, errors.Errorf("%s is missing in secret '%s'", key, secretName)
		}
	}

	insecureStr := string(secret.Data["OS_INSECURE"])
	insecure := strings.TrimSpace(insecureStr) == "true"

	return OpenStackCredentials{
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

	for i := 0; i < len(allNetworks); i++ {
		openstacknetworks = append(openstacknetworks, allNetworks[i].Name)
	}

	return &vjailbreakv1alpha1.OpenstackInfo{
		VolumeTypes: openstackvoltypes,
		Networks:    openstacknetworks,
	}, nil
}

// GetOpenStackClients is a function to create openstack clients
func GetOpenStackClients(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*OpenStackClients, error) {
	if openstackcreds == nil {
		return nil, errors.New("openstackcreds cannot be nil")
	}

	openstackCredential, err := GetOpenstackCredentials(ctx, k3sclient, openstackcreds.Spec.SecretRef.Name)
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
		return nil, errors.New(fmt.Sprintf("failed to get provider client for region '%s'", openstackCredential.RegionName)) //nolint:revive // preferred over revive
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
	openstackCredential, err := GetOpenstackCredentials(ctx, k3sclient, openstackcreds.Spec.SecretRef.Name)
	ctxlog := ctrllog.Log
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
		ctxlog.Info("Insecure flag is set, skipping certificate verification")
		tlsConfig.InsecureSkipVerify = true
	} else {
		// Get the certificate for the Openstack endpoint
		caCert, certerr := GetCert(openstackCredential.AuthURL)
		if certerr != nil {
			return nil, errors.Wrap(certerr, "failed to get certificate for openstack")
		}
		// Logging the certificate
		ctxlog.Info(fmt.Sprintf("Trusting certificate for '%s'", openstackCredential.AuthURL))
		ctxlog.Info(string(pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caCert.Raw,
		})))
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
	}
	err = openstack.Authenticate(providerClient, gophercloud.AuthOptions{
		IdentityEndpoint: openstackCredential.AuthURL,
		Username:         openstackCredential.Username,
		Password:         openstackCredential.Password,
		DomainName:       openstackCredential.DomainName,
		TenantName:       openstackCredential.TenantName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to authenticate to openstack")
	}

	return providerClient, nil
}

// ValidateVMwareCreds validates the VMware credentials
func ValidateVMwareCreds(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vim25.Client, error) {
	VMwareCredentials, err := GetVMwareCredentials(ctx, k3sclient, vmwcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get vCenter credentials from secret: %w", err)
	}

	host := VMwareCredentials.Host
	username := VMwareCredentials.Username
	password := VMwareCredentials.Password
	disableSSLVerification := VMwareCredentials.Insecure
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
	return c, nil
}

// GetVMwNetworks gets the networks of a VM
func GetVMwNetworks(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	var networks []string
	c, err := ValidateVMwareCreds(ctx, k3sclient, vmwcreds)
	if err != nil {
		return nil, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	// Get the vm
	vm, err := finder.VirtualMachine(ctx, vmname)
	if err != nil {
		return nil, fmt.Errorf("failed to find vm: %w", err)
	}

	// Get the network name of the VM
	var o mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config"}, &o)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	for _, device := range o.Config.Hardware.Device {
		switch dev := device.(type) {
		case *govmitypes.VirtualE1000e:
			networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
		case *govmitypes.VirtualVmxnet3:
			networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
		}
	}

	return networks, nil
}

// GetVMwDatastore gets the datastores of a VM
func GetVMwDatastore(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	c, err := ValidateVMwareCreds(ctx, k3sclient, vmwcreds)
	if err != nil {
		return nil, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

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
	var dsref govmitypes.ManagedObjectReference
	for _, device := range vmProps.Config.Hardware.Device {
		if _, ok := device.(*govmitypes.VirtualDisk); ok {
			switch backing := device.GetVirtualDevice().Backing.(type) {
			case *govmitypes.VirtualDiskFlatVer2BackingInfo:
				dsref = backing.Datastore.Reference()
			case *govmitypes.VirtualDiskSparseVer2BackingInfo:
				dsref = backing.Datastore.Reference()
			case *govmitypes.VirtualDiskRawDiskMappingVer1BackingInfo:
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

// GetAllVMs gets all the VMs in a datacenter
func GetAllVMs(ctx context.Context, k3sclient client.Client, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter string) ([]vjailbreakv1alpha1.VMInfo, error) {
	c, err := ValidateVMwareCreds(ctx, k3sclient, vmwcreds)
	if err != nil {
		return nil, fmt.Errorf("failed to validate vCenter connection: %w", err)
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	// Get all the vms
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to get vms: %w", err)
	}
	vminfo := make([]vjailbreakv1alpha1.VMInfo, 0, len(vms))
	for _, vm := range vms {
		var vmProps mo.VirtualMachine
		err = vm.Properties(ctx, vm.Reference(), []string{"config", "guest"}, &vmProps)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM properties: %w", err)
		}
		var datastores []string
		var networks []string
		var disks []string
		var ds mo.Datastore
		var dsref govmitypes.ManagedObjectReference
		if vmProps.Config == nil {
			// VM is not powered on or is in creating state
			fmt.Printf("VM properties not available for vm (%s), skipping this VM", vm.Name())
			continue
		}
		for _, device := range vmProps.Config.Hardware.Device {
			switch dev := device.(type) {
			case *govmitypes.VirtualE1000e:
				networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
			case *govmitypes.VirtualVmxnet3:
				networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
			case *govmitypes.VirtualDisk:
				switch backing := device.GetVirtualDevice().Backing.(type) {
				case *govmitypes.VirtualDiskFlatVer2BackingInfo:
					dsref = backing.Datastore.Reference()
				case *govmitypes.VirtualDiskSparseVer2BackingInfo:
					dsref = backing.Datastore.Reference()
				case *govmitypes.VirtualDiskRawDiskMappingVer1BackingInfo:
					dsref = backing.Datastore.Reference()
				default:
					return nil, fmt.Errorf("unsupported disk backing type: %T", device.GetVirtualDevice().Backing)
				}
				err := property.DefaultCollector(c).RetrieveOne(ctx, dsref, []string{"name"}, &ds)
				if err != nil {
					return nil, fmt.Errorf("failed to get datastore: %w", err)
				}
				datastores = AppendUnique(datastores, ds.Name)
				disks = append(disks, device.GetVirtualDevice().DeviceInfo.GetDescription().Label)
			}
		}

		vminfo = append(vminfo, vjailbreakv1alpha1.VMInfo{
			Name:       vmProps.Config.Name,
			Datastores: datastores,
			Disks:      disks,
			Networks:   networks,
			IPAddress:  vmProps.Guest.IpAddress,
			VMState:    vmProps.Guest.GuestState,
			OSType:     vmProps.Guest.GuestFamily,
		})
	}

	return vminfo, nil
}

// AppendUnique appends unique values to a slice
func AppendUnique(slice []string, values ...string) []string {
	for _, value := range values {
		if !containsString(slice, value) {
			slice = append(slice, value)
		}
	}
	return slice
}

// containsString checks if a string exists in a slice of strings.
// It is used internally by the package for string slice operations.
func containsString(slice []string, target string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}
