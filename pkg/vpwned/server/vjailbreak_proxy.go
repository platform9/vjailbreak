package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	gophercloud "github.com/gophercloud/gophercloud"
	openstack "github.com/gophercloud/gophercloud/openstack"
	ports "github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	errors "github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	openstackvalidation "github.com/platform9/vjailbreak/pkg/validation/openstack"
	vmwarevalidation "github.com/platform9/vjailbreak/pkg/validation/vmware"
	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	logrus.Info("Starting ValidateOpenstackIp request")

	retVal := &api.ValidateOpenstackIpResponse{}

	ips := in.GetIp()
	openstackAccessInfo := in.AccessInfo

	logrus.WithFields(logrus.Fields{
		"ip_count": len(ips),
		"ips":      ips,
	}).Info("Validating OpenStack IPs")

	// Add a check if there are same ips present in the list
	ipMap := make(map[string]bool)
	for _, ip := range ips {
		if _, ok := ipMap[ip]; ok {
			ipMap[ip] = false
			logrus.WithField("ip", ip).Warn("Duplicate IP detected")
		} else {
			ipMap[ip] = true
		}
	}

	logrus.Info("Creating OpenStack clients")
	openstackClients, err := GetOpenStackClients(ctx, openstackAccessInfo)
	if err != nil {
		logrus.WithError(err).Error("Failed to create OpenStack clients")
		return nil, err
	}
	logrus.Info("Successfully created OpenStack clients")

	for idx, ip := range ips {
		logrus.WithFields(logrus.Fields{
			"ip":    ip,
			"index": idx,
		}).Debug("Validating IP")

		if ipMap[ip] == false {
			logrus.WithField("ip", ip).Warn("IP marked as duplicate, skipping validation")
			retVal.IsValid = append(retVal.IsValid, false)
			retVal.Reason = append(retVal.Reason, "Duplicate IP")
			continue
		}

		isInUse, reason, err := isIPInUse(openstackClients.NetworkingClient, ip)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"ip":    ip,
				"error": err,
			}).Error("Failed to check if IP is in use")
			return nil, err
		}

		logrus.WithFields(logrus.Fields{
			"ip":        ip,
			"is_in_use": isInUse,
			"reason":    reason,
		}).Info("IP validation result")

		retVal.IsValid = append(retVal.IsValid, !isInUse)
		retVal.Reason = append(retVal.Reason, reason)
	}

	logrus.WithFields(logrus.Fields{
		"total_ips":     len(ips),
		"valid_count":   countValid(retVal.IsValid),
		"invalid_count": len(ips) - countValid(retVal.IsValid),
	}).Info("Completed ValidateOpenstackIp request")

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
	logrus.Debug("GetOpenStackClients: Starting client creation")

	if openstackAccessInfo == nil {
		logrus.Error("GetOpenStackClients: openstackAccessInfo is nil")
		return nil, fmt.Errorf("openstackAccessInfo cannot be nil")
	}

	logrus.WithFields(logrus.Fields{
		"secret_name":      openstackAccessInfo.SecretName,
		"secret_namespace": openstackAccessInfo.SecretNamespace,
	}).Debug("Creating in-cluster k8s client")

	k8sclient, err := CreateInClusterClient()
	if err != nil {
		logrus.WithError(err).Error("Failed to create in-cluster k8s client")
		return nil, err
	}
	logrus.Info("Successfully created in-cluster k8s client")

	logrus.WithFields(logrus.Fields{
		"secret_name":      openstackAccessInfo.SecretName,
		"secret_namespace": openstackAccessInfo.SecretNamespace,
	}).Info("Retrieving OpenStack credentials from secret")

	openstackCreds, err := GetOpenstackCredentialsFromSecret(ctx, k8sclient, openstackAccessInfo.SecretName, openstackAccessInfo.SecretNamespace)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"secret_name":      openstackAccessInfo.SecretName,
			"secret_namespace": openstackAccessInfo.SecretNamespace,
		}).Error("Failed to get OpenStack credentials from secret")
		return nil, err
	}
	logrus.WithFields(logrus.Fields{
		"auth_url": openstackCreds.AuthURL,
		"region":   openstackCreds.RegionName,
		"username": openstackCreds.Username,
		"insecure": openstackCreds.Insecure,
	}).Info("Successfully retrieved OpenStack credentials")

	endpoint := gophercloud.EndpointOpts{
		Region: openstackCreds.RegionName,
	}

	logrus.WithField("region", openstackCreds.RegionName).Info("Creating OpenStack provider client")
	providerClient, err := ValidateAndGetProviderClient(&openstackCreds)
	if err != nil {
		logrus.WithError(err).WithField("region", openstackCreds.RegionName).Error("Failed to create provider client")
		return nil, err
	}
	if providerClient == nil {
		logrus.WithField("region", openstackCreds.RegionName).Error("Provider client is nil")
		return nil, fmt.Errorf("failed to get provider client for region '%s'", openstackCreds.RegionName)
	}
	logrus.Info("Successfully created and authenticated provider client")

	logrus.WithField("region", openstackCreds.RegionName).Debug("Creating compute client")
	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		logrus.WithError(err).WithField("region", openstackCreds.RegionName).Error("Failed to create compute client")
		return nil, fmt.Errorf("failed to create openstack compute client for region '%s'", openstackCreds.RegionName)
	}

	logrus.WithField("region", openstackCreds.RegionName).Debug("Creating block storage client")
	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		logrus.WithError(err).WithField("region", openstackCreds.RegionName).Error("Failed to create block storage client")
		return nil, fmt.Errorf("failed to create openstack block storage client for region '%s'",
			openstackCreds.RegionName)
	}

	logrus.WithField("region", openstackCreds.RegionName).Debug("Creating networking client")
	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		logrus.WithError(err).WithField("region", openstackCreds.RegionName).Error("Failed to create networking client")
		return nil, fmt.Errorf("failed to create openstack networking client for region '%s'",
			openstackCreds.RegionName)
	}

	logrus.WithField("region", openstackCreds.RegionName).Info("Successfully created all OpenStack clients")
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
		Proxy:           http.ProxyFromEnvironment,
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

// countValid is a helper function to count valid IPs
func countValid(validFlags []bool) int {
	count := 0
	for _, valid := range validFlags {
		if valid {
			count++
		}
	}
	return count
}

// Create in cluster k8s client
func CreateInClusterClient() (client.Client, error) {
	logrus.Debug("Creating in-cluster k8s client")
	config, err := rest.InClusterConfig()
	if err != nil {
		logrus.WithError(err).Error("Failed to get in-cluster config")
		return nil, err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vjailbreakv1alpha1.AddToScheme(scheme))

	clientset, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to create k8s client")
		return nil, err
	}
	logrus.Info("Successfully created in-cluster k8s client")
	return clientset, nil
}

func (p *vjailbreakProxy) RevalidateCredentials(ctx context.Context, in *api.RevalidateCredentialsRequest) (*api.RevalidateCredentialsResponse, error) {
	zapLogger := zap.New(zap.UseDevMode(true))
	ctx = ctrlLog.IntoContext(ctx, zapLogger)
	reqLogger := ctrlLog.FromContext(ctx)

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

			if err := p.K8sClient.Status().Update(ctx, vmwcreds); err != nil {
				return nil, fmt.Errorf("failed to update status: %w", err)
			}

			// Fetch resources
			log.Printf("Triggering resource fetch for %s", name)

			// Capture name and namespace
			credName := vmwcreds.Name
			credNamespace := vmwcreds.Namespace

			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				bgCtx = ctrlLog.IntoContext(bgCtx, reqLogger)

				freshVMCreds := &vjailbreakv1alpha1.VMwareCreds{}
				if err := p.K8sClient.Get(bgCtx, k8stypes.NamespacedName{
					Name:      credName,
					Namespace: credNamespace,
				}, freshVMCreds); err != nil {
					log.Printf("Failed to fetch VMwareCreds %s: %v", credName, err)
					return
				}

				log.Printf("Fetching VMware resources for %s", credName)
				resources, err := vmwarevalidation.FetchResourcesPostValidation(bgCtx, p.K8sClient, freshVMCreds)
				if err != nil {
					log.Printf("Warning: Failed to fetch VMware resources for %s: %v", credName, err)
				} else {
					log.Printf("Successfully fetched %d VMs for %s", len(resources.VMInfo), credName)
				}
			}()

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

			// Fetch resources post-validation
			log.Printf("Fetching OpenStack resources for %s", name)
			resources, err := openstackvalidation.FetchResourcesPostValidation(ctx, p.K8sClient, oscreds)
			if err != nil {
				log.Printf("Warning: Failed to fetch OpenStack resources for %s: %v", name, err)
			} else {
				oscreds.Spec.Flavors = resources.Flavors

				// Update the spec
				if err := p.K8sClient.Update(ctx, oscreds); err != nil {
					log.Printf("Warning: Failed to update OpenstackCreds spec: %v", err)
				} else {
					log.Printf("Updated OpenstackCreds spec with %d flavors for %s", len(resources.Flavors), name)
				}

				// Update status with OpenStack info
				if resources.OpenstackInfo != nil {
					oscreds.Status.Openstack = *resources.OpenstackInfo
				}

				log.Printf("Successfully fetched %d flavors for %s", len(resources.Flavors), name)
			}
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
