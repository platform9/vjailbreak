package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	Insecure   bool
	Datacenter string
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

const (
	trueString = "true" // Define at package level
)

// GetVMwareCredentials retrieves vCenter credentials from a secret
func GetVMwareCredentials(ctx context.Context, secretName string) (VMwareCredentials, error) {
	secret := &corev1.Secret{}

	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return VMwareCredentials{}, errors.Wrap(err, "failed to get in cluster client")
	}

	if err := c.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
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

	insecure := strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

	return VMwareCredentials{
		Host:       host,
		Username:   username,
		Password:   password,
		Insecure:   insecure,
		Datacenter: datacenter,
	}, nil
}

// GetOpenstackCredentials retrieves and checks the secret
func GetOpenstackCredentials(ctx context.Context, secretName string) (OpenStackCredentials, error) {
	secret := &corev1.Secret{}
	// Get In cluster client
	c, err := GetInclusterClient()
	if err != nil {
		return OpenStackCredentials{}, errors.Wrap(err, "failed to get in cluster client")
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: constants.NamespaceMigrationSystem, Name: secretName}, secret); err != nil {
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
	insecure := strings.EqualFold(strings.TrimSpace(insecureStr), trueString)

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
	defer conn.Close()
	cert := conn.ConnectionState().PeerCertificates[0]
	return cert, nil
}

//nolint:dupl // This function is similar to VerifyNetworks, excluding from linting to keep it readable
func VerifyNetworks(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetnetworks []string) error {
	openstackClients, err := GetOpenStackClients(ctx, openstackcreds)
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

//nolint:dupl // This function is similar to VerifyNetworks, excluding from linting to keep it readable
func VerifyPorts(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetports []string) error {
	openstackClients, err := GetOpenStackClients(ctx, openstackcreds)
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

func VerifyStorage(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetstorages []string) error {
	openstackClients, err := GetOpenStackClients(ctx, openstackcreds)
	if err != nil {
		return err
	}
	allPages, err := volumetypes.List(openstackClients.BlockStorageClient, nil).AllPages()
	if err != nil {
		return fmt.Errorf("failed to list volume types: %w", err)
	}

	allvoltypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil {
		return fmt.Errorf("failed to extract all volume types: %w", err)
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
			return fmt.Errorf("volume type '%s' not found in OpenStack", targetstorage)
		}
	}
	return nil
}

func GetOpenstackInfo(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*vjailbreakv1alpha1.OpenstackInfo, error) {
	var openstackvoltypes []string
	var openstacknetworks []string
	openstackClients, err := GetOpenStackClients(ctx, openstackcreds)
	if err != nil {
		return nil, err
	}
	allVolumeTypePages, err := volumetypes.List(openstackClients.BlockStorageClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list volume types: %w", err)
	}

	allvoltypes, err := volumetypes.ExtractVolumeTypes(allVolumeTypePages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all volume types: %w", err)
	}

	for i := 0; i < len(allvoltypes); i++ {
		openstackvoltypes = append(openstackvoltypes, allvoltypes[i].Name)
	}

	allNetworkPages, err := networks.List(openstackClients.NetworkingClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	allNetworks, err := networks.ExtractNetworks(allNetworkPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all networks: %w", err)
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
func GetOpenStackClients(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*OpenStackClients, error) {
	if openstackcreds == nil {
		return nil, fmt.Errorf("openstackcreds cannot be nil")
	}

	openstackCredential, err := GetOpenstackCredentials(ctx, openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get openstack credentials from secret")
	}

	endpoint := gophercloud.EndpointOpts{
		Region: openstackCredential.RegionName,
	}
	providerClient, err := ValidateAndGetProviderClient(ctx, openstackcreds)
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
func ValidateAndGetProviderClient(ctx context.Context,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*gophercloud.ProviderClient, error) {
	ctxlog := log.FromContext(ctx)
	openstackCredential, err := GetOpenstackCredentials(ctx, openstackcreds.Spec.SecretRef.Name)
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
		ctxlog.Info("Trusting certificate for OpenStack endpoint", "authURL", openstackCredential.AuthURL)
		// Trying to fetch the system cert pool and add the Openstack certificate to it
		caCertPool, _ := x509.SystemCertPool()
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
func ValidateVMwareCreds(vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vim25.Client, error) {
	VMwareCredentials, err := GetVMwareCredentials(context.TODO(), vmwcreds.Spec.SecretRef.Name)
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
	// fmt.Println(u)
	// Connect and log in to ESX or vCenter
	// Share govc's session cache
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
		Reauth:   true,
	}

	c := new(vim25.Client)
	err = s.Login(context.Background(), c, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	// Check if the datacenter exists
	finder := find.NewFinder(c, false)
	_, err = finder.Datacenter(context.Background(), VMwareCredentials.Datacenter)
	if err != nil {
		return nil, fmt.Errorf("failed to find datacenter: %w", err)
	}

	return c, nil
}

// GetVMwNetworks gets the networks of a VM
func GetVMwNetworks(ctx context.Context, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	var networks []string
	c, err := ValidateVMwareCreds(vmwcreds)
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
		case *types.VirtualE1000e:
			networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
		case *types.VirtualVmxnet3:
			networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
		}
	}

	return networks, nil
}

// GetVMwDatastore gets the datastores of a VM
func GetVMwDatastore(ctx context.Context, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter, vmname string) ([]string, error) {
	c, err := ValidateVMwareCreds(vmwcreds)
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

// GetAllVMs gets all the VMs in a datacenter
func GetAllVMs(ctx context.Context, vmwcreds *vjailbreakv1alpha1.VMwareCreds, datacenter string) ([]vjailbreakv1alpha1.VMInfo, error) {
	c, err := ValidateVMwareCreds(vmwcreds)
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
	var vminfo []vjailbreakv1alpha1.VMInfo
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
		var dsref types.ManagedObjectReference
		if vmProps.Config == nil {
			// VM is not powered on or is in creating state
			fmt.Printf("VM properties not available for vm (%s), skipping this VM", vm.Name())
			continue
		}
		for _, device := range vmProps.Config.Hardware.Device {
			switch dev := device.(type) {
			case *types.VirtualE1000e:
				networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
			case *types.VirtualVmxnet3:
				networks = append(networks, dev.DeviceInfo.GetDescription().Summary)
			case *types.VirtualDisk:
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
			CPU:        int(vmProps.Config.Hardware.NumCPU),
			Memory:     int(vmProps.Config.Hardware.MemoryMB),
		})
	}

	return vminfo, nil
}

// AppendUnique appends unique values to a slice
func AppendUnique(slice []string, values ...string) []string {
	for _, v := range values {
		if !slices.Contains(slice, v) {
			slice = append(slice, v)
		}
	}
	return slice
}

func CreateOrUpdateVMwareMachines(ctx context.Context, client client.Client,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds, vminfo []vjailbreakv1alpha1.VMInfo) error {
	var wg sync.WaitGroup
	for i := range vminfo {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Don't panic on error
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic: %v\n", r)
				}
			}()
			vm := &vminfo[i] // Use a pointer
			err := CreateOrUpdateVMwareMachine(ctx, client, vmwcreds, vm)
			if err != nil {
				fmt.Printf("Error creating or updating VM '%s': %v\n", vm.Name, err)
			}
		}(i)
	}
	// Wait for all vms to be created or updated
	wg.Wait()
	return nil
}

func CreateOrUpdateVMwareMachine(ctx context.Context, client client.Client,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds, vminfo *vjailbreakv1alpha1.VMInfo) error {
	ctxlog := log.FromContext(ctx)
	sanitizedVMName, err := ConvertToK8sName(vminfo.Name)
	if err != nil {
		return fmt.Errorf("failed to convert VM name: %w", err)
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
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get VMwareMachine: %w", err)
	}

	// Check if the object is present or not if not present create a new object and set init to true.
	if k8serrors.IsNotFound(err) {
		// If not found, create a new object
		label := fmt.Sprintf("%s-%s", constants.VMwareCredsLabel, vmwcreds.Name)
		vmwvm = &vjailbreakv1alpha1.VMwareMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmwvmKey.Name,
				Namespace: vmwcreds.Namespace,
				Labels:    map[string]string{label: "true"},
			},
			Spec: vjailbreakv1alpha1.VMwareMachineSpec{
				VMs: *vminfo,
			},
		}
		ctxlog.Info("Creating new VMwareMachine", "Name", vmwvmKey.Name)
		init = true
	} else {
		label := fmt.Sprintf("%s-%s", constants.VMwareCredsLabel, vmwcreds.Name)

		// Check if label already exists with same value
		if vmwvm.Labels == nil || vmwvm.Labels[label] != "true" {
			ctxlog.Info("Adding new label to VMwareMachine",
				"Name", vmwvmKey.Name,
				"Label", label)

			// Initialize labels map if needed
			if vmwvm.Labels == nil {
				vmwvm.Labels = make(map[string]string)
			}

			// Set the new label
			vmwvm.Labels[label] = "true"

			// Update only if we made changes
			if err = client.Update(ctx, vmwvm); err != nil {
				return fmt.Errorf("failed to update VMwareMachine labels: %w", err)
			}
		}
	}

	_, err = controllerutil.CreateOrUpdate(ctx, client, vmwvm, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update VMwareMachine: %w", err)
	}

	// Assumption is if init is true, the object is new and it is not migrated hence mark migrated to false.
	if init {
		vmwvm.Status = vjailbreakv1alpha1.VMwareMachineStatus{
			PowerState: vminfo.VMState,
			Migrated:   false,
		}
		ctxlog.Info("Creating new VMwareMachine status", "Name", vmwvmKey.Name, "migrated", vmwvm.Status.Migrated)
	} else {
		// If the object is not new, update the status and persist migrated status.
		currentMigratedStatus := vmwvm.Status.Migrated
		if vmwvm.Status.PowerState != vminfo.VMState {
			vmwvm.Status.PowerState = vminfo.VMState
			ctxlog.Info("Updating VMwareMachine status", "Name", vmwvmKey.Name)
		}
		vmwvm.Status.Migrated = currentMigratedStatus
		ctxlog.Info("Updating VMwareMachine status", "Name", vmwvmKey.Name, "migrated", vmwvm.Status.Migrated)
	}

	// Update the status
	if err := client.Status().Update(ctx, vmwvm); err != nil {
		return fmt.Errorf("failed to update VMwareMachine status: %w", err)
	}
	return nil
}

func GetClosestFlavour(ctx context.Context, cpu, memory int, computeClient *gophercloud.ServiceClient) (*flavors.Flavor, error) {
	ctxlog := log.FromContext(ctx)

	// Fixed logging with proper string keys
	ctxlog.Info("Checking flavor requirements",
		"CPU", cpu,
		"MemoryMB", memory)

	allPages, err := flavors.ListDetail(computeClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list flavors: %w", err)
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all flavors: %w", err)
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
		// Fixed logging with proper string keys and descriptive field names
		ctxlog.Info("Found matching OpenStack flavor",
			"flavorName", bestFlavor.Name,
			"vCPUs", bestFlavor.VCPUs,
			"RAM_MB", bestFlavor.RAM,
			"diskGB", bestFlavor.Disk)
		return bestFlavor, nil
	}

	ctxlog.Info("No suitable flavor found matching requirements",
		"required_vCPUs", cpu,
		"required_RAM_MB", memory)
	return nil, fmt.Errorf("no suitable flavor found for %d vCPUs and %d MB RAM", cpu, memory)
}
