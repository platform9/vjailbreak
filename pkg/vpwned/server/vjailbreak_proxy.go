package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"

	gophercloud "github.com/gophercloud/gophercloud"
	openstack "github.com/gophercloud/gophercloud/openstack"
	ports "github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	errors "github.com/pkg/errors"
	openstackvalidation "github.com/platform9/vjailbreak/pkg/validation/openstack"
	vmwarevalidation "github.com/platform9/vjailbreak/pkg/validation/vmware"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type vjailbreakProxy struct {
	api.UnimplementedVailbreakProxyServer
	K8sClient client.Client
}

type OpenstackCredsinfo struct {
	AuthUrl    string `json:"auth_url"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	DomainName string `json:"domain_name"`
	TenantName string `json:"tenant_name"`
	RegionName string `json:"region_name"`
	Insecure   bool   `json:"insecure"`
}

func (p *vjailbreakProxy) ValidateOpenstackIp(ctx context.Context, in *api.ValidateOpenstackIpRequest) (*api.ValidateOpenstackIpResponse, error) {

	retVal := &api.ValidateOpenstackIpResponse{}

	ips := in.GetIp()
	openstackAccessInfo := in.AccessInfo

	// Add a check if there are same ips present in the list
	ipMap := make(map[string]bool)
	for _, ip := range ips {
		if _, ok := ipMap[ip]; ok {
			ipMap[ip] = false
		} else {
			ipMap[ip] = true
		}
	}

	openstackClients, err := GetOpenStackClients(ctx, openstackAccessInfo)
	if err != nil {
		return nil, err
	}

	for _, ip := range ips {
		if ipMap[ip] == false {
			retVal.IsValid = append(retVal.IsValid, false)
			retVal.Reason = append(retVal.Reason, "Duplicate IP")
			continue
		}
		isInUse, reason, err := isIPInUse(openstackClients.NetworkingClient, ip)
		if err != nil {
			return nil, err
		}
		retVal.IsValid = append(retVal.IsValid, !isInUse)
		retVal.Reason = append(retVal.Reason, reason)
	}

	return retVal, nil
}

type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
}

// Helper: checks if IP is used by any port or floating IP in OpenStack
func isIPInUse(client *gophercloud.ServiceClient, ip string) (bool, string, error) {
	// Check fixed IPs on ports
	portList, err := ports.List(client, ports.ListOpts{
		FixedIPs: []ports.FixedIPOpts{{IPAddress: ip}},
	}).AllPages()
	if err != nil {
		return false, "", err
	}
	allPorts, _ := ports.ExtractPorts(portList)
	if len(allPorts) > 0 {
		return true, "Already in use (port)", nil
	}

	return false, "Available", nil
}

// GetOpenstackCredentialsFromSecret retrieves and checks the secret
func GetOpenstackCredentialsFromSecret(ctx context.Context, k3sclient client.Client, secretName string, secretNamespace string) (vjailbreakv1alpha1.OpenStackCredsInfo, error) {
	secret := &corev1.Secret{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret); err != nil {
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
	insecure := strings.EqualFold(strings.TrimSpace(insecureStr), "true")

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

// GetOpenStackClients is a function to create openstack clients
func GetOpenStackClients(ctx context.Context, openstackAccessInfo *api.OpenstackAccessInfo) (*OpenStackClients, error) {

	if openstackAccessInfo == nil {
		return nil, fmt.Errorf("openstackAccessInfo cannot be nil")
	}

	k8sclient, err := CreateInClusterClient()
	if err != nil {
		return nil, err
	}

	openstackCreds, err := GetOpenstackCredentialsFromSecret(ctx, k8sclient, openstackAccessInfo.SecretName, openstackAccessInfo.SecretNamespace)
	if err != nil {
		return nil, err
	}

	endpoint := gophercloud.EndpointOpts{
		Region: openstackCreds.RegionName,
	}

	providerClient, err := ValidateAndGetProviderClient(&openstackCreds)
	if err != nil {
		return nil, err
	}
	if providerClient == nil {
		return nil, fmt.Errorf("failed to get provider client for region '%s'", openstackCreds.RegionName)
	}
	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack compute client for region '%s'", openstackCreds.RegionName)
	}
	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack block storage client for region '%s'",
			openstackCreds.RegionName)
	}
	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack networking client for region '%s'",
			openstackCreds.RegionName)
	}
	return &OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
	}, nil
}

func ValidateAndGetProviderClient(openstackAccessInfo *vjailbreakv1alpha1.OpenStackCredsInfo) (*gophercloud.ProviderClient, error) {
	providerClient, err := openstack.NewClient(openstackAccessInfo.AuthURL)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if openstackAccessInfo.Insecure {
		tlsConfig.InsecureSkipVerify = true
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	providerClient.HTTPClient = http.Client{
		Transport: transport,
	}
	err = openstack.Authenticate(providerClient, gophercloud.AuthOptions{
		IdentityEndpoint: openstackAccessInfo.AuthURL,
		Username:         openstackAccessInfo.Username,
		Password:         openstackAccessInfo.Password,
		DomainName:       openstackAccessInfo.DomainName,
		TenantName:       openstackAccessInfo.TenantName,
	})
	if err != nil {
		return nil, err
	}

	return providerClient, nil
}

// Create in cluster k8s client
func CreateInClusterClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vjailbreakv1alpha1.AddToScheme(scheme))

	clientset, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func (p *vjailbreakProxy) RevalidateCredentials(ctx context.Context, in *api.RevalidateCredentialsRequest) (*api.RevalidateCredentialsResponse, error) {
	kind := in.GetKind()
	name := in.GetName()
	namespace := in.GetNamespace()
	log.Printf("Revalidating credentials: name=%s", name)

	switch kind {

	case "VmwareCreds":
		vmwcreds := &vjailbreakv1alpha1.VMwareCreds{}
		if err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, vmwcreds); err != nil {
			log.Printf("Failed to get VMwareCreds %s: %v", name, err)
			return nil, fmt.Errorf("failed to get VMwareCreds %s: %w", name, err)
		}

		log.Printf("Starting VMware validation for %s", name)
		result := vmwarevalidation.Validate(ctx, p.K8sClient, vmwcreds)

		// Update status immediately
		if result.Valid {
			vmwcreds.Status.VMwareValidationStatus = "Succeeded"
			vmwcreds.Status.VMwareValidationMessage = result.Message
			log.Printf("VMware validation succeeded for %s", name)
		} else {
			vmwcreds.Status.VMwareValidationStatus = "Failed"
			vmwcreds.Status.VMwareValidationMessage = result.Message
			log.Printf("VMware validation failed for %s: %s", name, result.Message)
		}

		if err := p.K8sClient.Status().Update(ctx, vmwcreds); err != nil {
			log.Printf("Failed to update VMwareCreds status: %v", err)
			return nil, fmt.Errorf("failed to update VMwareCreds status: %w", err)
		}

		responseMsg := fmt.Sprintf("Validation completed for %s: %s", name, result.Message)
		return &api.RevalidateCredentialsResponse{Message: responseMsg}, nil

	case "OpenstackCreds":
		oscreds := &vjailbreakv1alpha1.OpenstackCreds{}
		if err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, oscreds); err != nil {
			log.Printf("Failed to get OpenstackCreds %s: %v", name, err)
			return nil, fmt.Errorf("failed to get OpenstackCreds %s: %w", name, err)
		}

		log.Printf("Starting OpenStack validation for %s", name)
		result := openstackvalidation.Validate(ctx, p.K8sClient, oscreds)

		if result.Valid {
			oscreds.Status.OpenStackValidationStatus = "Succeeded"
			oscreds.Status.OpenStackValidationMessage = result.Message
			log.Printf("OpenStack validation succeeded for %s", name)
		} else {
			oscreds.Status.OpenStackValidationStatus = "Failed"
			oscreds.Status.OpenStackValidationMessage = result.Message
			log.Printf("OpenStack validation failed for %s: %s", name, result.Message)
		}

		if err := p.K8sClient.Status().Update(ctx, oscreds); err != nil {
			log.Printf("Failed to update OpenstackCreds status: %v", err)
			return nil, fmt.Errorf("failed to update OpenstackCreds status: %w", err)
		}

		responseMsg := fmt.Sprintf("Validation completed for %s: %s", name, result.Message)
		return &api.RevalidateCredentialsResponse{Message: responseMsg}, nil

	default:
		log.Printf("Unknown credentials kind: %s", kind)
		return nil, fmt.Errorf("unknown credentials kind: %s", kind)
	}
}