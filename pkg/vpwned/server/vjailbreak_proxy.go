package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	gophercloud "github.com/gophercloud/gophercloud"
	openstack "github.com/gophercloud/gophercloud/openstack"
	ports "github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	errors "github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	trueString = "true"
	RevalidateAnnotationKey = "vjailbreak.k8s.pf9.io/revalidate-timestamp"
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

func (p *vjailbreakProxy) RevalidateCredentials(ctx context.Context, in *api.RevalidateCredentialsRequest) (*api.RevalidateCredentialsResponse, error) {
	var obj client.Object
	kind := in.GetKind()

	switch kind {
	case "VmwareCreds":
		obj = &vjailbreakv1alpha1.VMwareCreds{}
	case "OpenstackCreds":
		obj = &vjailbreakv1alpha1.OpenstackCreds{}
	default:
		return nil, fmt.Errorf("unknown resource kind: %s", kind)
	}

	err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: in.GetName(), Namespace: in.GetNamespace()}, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource %s/%s: %w", in.GetNamespace(), in.GetName(), err)
	}

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[RevalidateAnnotationKey] = time.Now().Format(time.RFC3339)
	obj.SetAnnotations(annotations)

	if err := p.K8sClient.Patch(ctx, obj, patch); err != nil {
		return nil, fmt.Errorf("failed to patch resource with annotation: %w", err)
	}

	return &api.RevalidateCredentialsResponse{
		Message: fmt.Sprintf("Re-validation triggered for %s %s/%s", kind, in.GetNamespace(), in.GetName()),
	}, nil
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

	openstackClients, err := GetOpenStackClients(ctx, p.K8sClient, openstackAccessInfo)
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
func GetOpenStackClients(ctx context.Context, k8sclient client.Client, openstackAccessInfo *api.OpenstackAccessInfo) (*OpenStackClients, error) {

	if openstackAccessInfo == nil {
		return nil, fmt.Errorf("openstackAccessInfo cannot be nil")
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
