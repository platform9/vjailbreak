package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gophercloud "github.com/gophercloud/gophercloud/v2"
	openstack "github.com/gophercloud/gophercloud/v2/openstack"
	ports "github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	errors "github.com/pkg/errors"
	netutils "github.com/platform9/vjailbreak/common/utils"
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

type uploadState struct {
	filename           string
	tempFilePath       string
	tempFile           *os.File
	totalBytesReceived int64
	totalChunks        int64
	receivedChunks     map[int64]bool
	mu                 sync.Mutex
}

type vjailbreakProxy struct {
	api.UnimplementedVailbreakProxyServer
	K8sClient      client.Client
	uploadStates   map[string]*uploadState
	uploadStatesMu sync.RWMutex
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
	const fn = "ValidateOpenstackIp"
	logrus.WithField("func", fn).Info("Starting ValidateOpenstackIp request")
	defer logrus.WithField("func", fn).Info("Completed ValidateOpenstackIp request")

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

	return retVal, nil
}

type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
}

// Helper: checks if IP is used by any port or floating IP in OpenStack
func isIPInUse(client *gophercloud.ServiceClient, ip string) (bool, string, error) {
	const fn = "isIPInUse"
	logrus.WithFields(logrus.Fields{"func": fn, "ip": ip}).Debug("Entering isIPInUse")
	defer logrus.WithFields(logrus.Fields{"func": fn, "ip": ip}).Debug("Exiting isIPInUse")
	// Check fixed IPs on ports
	portList, err := ports.List(client, ports.ListOpts{
		FixedIPs: []ports.FixedIPOpts{{IPAddress: ip}},
	}).AllPages(context.TODO())
	if err != nil {
		logrus.WithFields(logrus.Fields{"func": fn, "ip": ip}).WithError(err).Error("Failed to list ports for IP")
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
	const fn = "GetOpenstackCredentialsFromSecret"
	logrus.WithFields(logrus.Fields{"func": fn, "secret": secretName, "namespace": secretNamespace}).Debug("Entering GetOpenstackCredentialsFromSecret")
	defer logrus.WithFields(logrus.Fields{"func": fn, "secret": secretName, "namespace": secretNamespace}).Debug("Exiting GetOpenstackCredentialsFromSecret")
	secret := &corev1.Secret{}
	if err := k3sclient.Get(ctx, k8stypes.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret); err != nil {
		logrus.WithFields(logrus.Fields{"func": fn, "secret": secretName, "namespace": secretNamespace}).WithError(err).Error("Failed to get secret")
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
			logrus.WithFields(logrus.Fields{"func": fn, "missing_field": key, "secret": secretName}).Error("Missing field in OpenStack secret")
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
	const fn = "GetOpenStackClients"
	logrus.WithField("func", fn).Debug("Starting client creation")

	if openstackAccessInfo == nil {
		logrus.WithField("func", fn).Error("openstackAccessInfo is nil")
		return nil, fmt.Errorf("openstackAccessInfo cannot be nil")
	}

	logrus.WithField("func", fn).Debug("Creating in-cluster k8s client")

	k8sclient, err := CreateInClusterClient()
	if err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to create in-cluster k8s client")
		return nil, err
	}
	logrus.WithField("func", fn).Info("Successfully created in-cluster k8s client")

	logrus.WithField("func", fn).Info("Retrieving OpenStack credentials from secret")

	openstackCreds, err := GetOpenstackCredentialsFromSecret(ctx, k8sclient, openstackAccessInfo.SecretName, openstackAccessInfo.SecretNamespace)
	if err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to get OpenStack credentials from secret")
		return nil, err
	}
	logrus.WithFields(logrus.Fields{
		"auth_url": openstackCreds.AuthURL,
		"region":   openstackCreds.RegionName,
		"insecure": openstackCreds.Insecure,
		"func":     fn,
	}).Info("Successfully retrieved OpenStack credentials")

	endpoint := gophercloud.EndpointOpts{
		Region: openstackCreds.RegionName,
	}

	logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Info("Creating OpenStack provider client")
	providerClient, err := ValidateAndGetProviderClient(&openstackCreds)
	if err != nil {
		logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).WithError(err).Error("Failed to create provider client")
		return nil, err
	}
	if providerClient == nil {
		logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Error("Provider client is nil")
		return nil, fmt.Errorf("failed to get provider client for region '%s'", openstackCreds.RegionName)
	}
	logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Info("Successfully created and authenticated provider client")

	logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Debug("Creating compute client")
	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).WithError(err).Error("Failed to create compute client")
		return nil, fmt.Errorf("failed to create openstack compute client for region '%s'", openstackCreds.RegionName)
	}

	logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Debug("Creating block storage client")
	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).WithError(err).Error("Failed to create block storage client")
		return nil, fmt.Errorf("failed to create openstack block storage client for region '%s'",
			openstackCreds.RegionName)
	}

	logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Debug("Creating networking client")
	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).WithError(err).Error("Failed to create networking client")
		return nil, fmt.Errorf("failed to create openstack networking client for region '%s'",
			openstackCreds.RegionName)
	}

	logrus.WithFields(logrus.Fields{"region": openstackCreds.RegionName, "func": fn}).Info("Successfully created all OpenStack clients")
	return &OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
	}, nil
}

func ValidateAndGetProviderClient(openstackAccessInfo *vjailbreakv1alpha1.OpenStackCredsInfo) (*gophercloud.ProviderClient, error) {
	const fn = "ValidateAndGetProviderClient"
	logrus.WithFields(logrus.Fields{"func": fn, "auth_url": openstackAccessInfo.AuthURL, "region": openstackAccessInfo.RegionName, "insecure": openstackAccessInfo.Insecure}).Debug("Entering ValidateAndGetProviderClient")
	defer logrus.WithFields(logrus.Fields{"func": fn, "auth_url": openstackAccessInfo.AuthURL, "region": openstackAccessInfo.RegionName, "insecure": openstackAccessInfo.Insecure}).Debug("Exiting ValidateAndGetProviderClient")

	providerClient, err := openstack.NewClient(openstackAccessInfo.AuthURL)
	if err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to create OpenStack provider client")
		return nil, err
	}
	vjbNet := netutils.NewVjbNet()
	if openstackAccessInfo.Insecure {
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

	err = openstack.Authenticate(context.TODO(), providerClient, gophercloud.AuthOptions{
		IdentityEndpoint: openstackAccessInfo.AuthURL,
		Username:         openstackAccessInfo.Username,
		Password:         openstackAccessInfo.Password,
		DomainName:       openstackAccessInfo.DomainName,
		TenantName:       openstackAccessInfo.TenantName,
	})
	if err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to authenticate OpenStack provider client")
		return nil, err
	}

	return providerClient, nil
}

// countValid is a helper function to count valid IPs
func countValid(validFlags []bool) int {
	const fn = "countValid"
	logrus.WithField("func", fn).Debug("Entering countValid")
	count := 0
	for _, valid := range validFlags {
		if valid {
			count++
		}
	}
	logrus.WithFields(logrus.Fields{"func": fn, "count": count}).Debug("Exiting countValid")
	return count
}

// Create in cluster k8s client
func CreateInClusterClient() (client.Client, error) {
	const fn = "CreateInClusterClient"
	logrus.WithField("func", fn).Debug("Creating in-cluster k8s client")
	config, err := rest.InClusterConfig()
	if err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to get in-cluster config")
		return nil, err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vjailbreakv1alpha1.AddToScheme(scheme))

	clientset, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to create k8s client")
		return nil, err
	}
	logrus.WithField("func", fn).Info("Successfully created in-cluster k8s client")
	return clientset, nil
}

func (p *vjailbreakProxy) RevalidateCredentials(ctx context.Context, in *api.RevalidateCredentialsRequest) (*api.RevalidateCredentialsResponse, error) {
	const fn = "RevalidateCredentials"
	logrus.WithFields(logrus.Fields{"func": fn, "kind": in.GetKind(), "name": in.GetName(), "namespace": in.GetNamespace()}).Info("Entering RevalidateCredentials")
	defer logrus.WithFields(logrus.Fields{"func": fn, "kind": in.GetKind(), "name": in.GetName(), "namespace": in.GetNamespace()}).Info("Exiting RevalidateCredentials")

	zapLogger := zap.New(zap.UseDevMode(true))
	ctx = ctrlLog.IntoContext(ctx, zapLogger)
	reqLogger := ctrlLog.FromContext(ctx)

	kind := in.GetKind()
	name := in.GetName()
	namespace := in.GetNamespace()
	logrus.WithFields(logrus.Fields{"func": fn, "kind": kind, "name": name, "namespace": namespace}).Info("Revalidating credentials")

	switch kind {

	case "VmwareCreds":
		vmwcreds := &vjailbreakv1alpha1.VMwareCreds{}
		if err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, vmwcreds); err != nil {
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Error("Failed to get VMwareCreds")
			return nil, fmt.Errorf("failed to get VMwareCreds %s: %w", name, err)
		}

		logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).Info("Starting VMware validation")
		result := vmwarevalidation.Validate(ctx, p.K8sClient, vmwcreds)

		// Update status immediately
		if result.Valid {
			vmwcreds.Status.VMwareValidationStatus = "Succeeded"
			vmwcreds.Status.VMwareValidationMessage = result.Message
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).Info("VMware validation succeeded")

			if err := p.K8sClient.Status().Update(ctx, vmwcreds); err != nil {
				logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Error("Failed to update VMwareCreds status after success")
				return nil, fmt.Errorf("failed to update status: %w", err)
			}

			// Fetch resources
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).Info("Triggering VMware resource fetch")

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
					logrus.WithFields(logrus.Fields{"func": fn, "name": credName, "namespace": credNamespace}).WithError(err).Error("Failed to fetch VMwareCreds in background")
					return
				}

				logrus.WithFields(logrus.Fields{"func": fn, "name": credName, "namespace": credNamespace}).Info("Fetching VMware resources")
				resources, err := vmwarevalidation.FetchResourcesPostValidation(bgCtx, p.K8sClient, freshVMCreds)
				if err != nil {
					logrus.WithFields(logrus.Fields{"func": fn, "name": credName, "namespace": credNamespace}).WithError(err).Warn("Failed to fetch VMware resources")
				} else {
					logrus.WithFields(logrus.Fields{"func": fn, "name": credName, "namespace": credNamespace, "vm_count": len(resources.VMInfo)}).Info("Successfully fetched VMware resources")
				}
			}()

		} else {
			vmwcreds.Status.VMwareValidationStatus = "Failed"
			vmwcreds.Status.VMwareValidationMessage = result.Message
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace, "message": result.Message}).Warn("VMware validation failed")

			if err := p.K8sClient.Status().Update(ctx, vmwcreds); err != nil {
				logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Error("Failed to update VMwareCreds status")
				return nil, fmt.Errorf("failed to update VMwareCreds status: %w", err)
			}
		}

		responseMsg := fmt.Sprintf("Validation completed for %s: %s", name, result.Message)
		return &api.RevalidateCredentialsResponse{Message: responseMsg}, nil

	case "OpenstackCreds":
		oscreds := &vjailbreakv1alpha1.OpenstackCreds{}
		if err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, oscreds); err != nil {
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Error("Failed to get OpenstackCreds")
			return nil, fmt.Errorf("failed to get OpenstackCreds %s: %w", name, err)
		}

		logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).Info("Starting OpenStack validation")
		result := openstackvalidation.Validate(ctx, p.K8sClient, oscreds)

		if result.Valid {
			oscreds.Status.OpenStackValidationStatus = "Succeeded"
			oscreds.Status.OpenStackValidationMessage = result.Message
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).Info("OpenStack validation succeeded")

			// Update status immediately after validation succeeds
			if err := p.K8sClient.Status().Update(ctx, oscreds); err != nil {
				logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Error("Failed to update OpenstackCreds status after success")
				return nil, fmt.Errorf("failed to update status: %w", err)
			}

			// Fetch resources post-validation
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).Info("Fetching OpenStack resources")
			resources, err := openstackvalidation.FetchResourcesPostValidation(ctx, p.K8sClient, oscreds)
			if err != nil {
				logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Warn("Failed to fetch OpenStack resources")
			} else {
				// Refetch the latest version before updating spec
				freshOSCreds := &vjailbreakv1alpha1.OpenstackCreds{}
				if err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, freshOSCreds); err != nil {
					logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Warn("Failed to refetch OpenstackCreds for spec update")
				} else {
					freshOSCreds.Spec.Flavors = resources.Flavors

					// Update the spec
					if err := p.K8sClient.Update(ctx, freshOSCreds); err != nil {
						logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Warn("Failed to update OpenstackCreds spec")
					} else {
						logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace, "flavor_count": len(resources.Flavors)}).Info("Updated OpenstackCreds spec with flavors")
					}

					// Refetch again before updating status with OpenStack info
					if err := p.K8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, freshOSCreds); err != nil {
						logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Warn("Failed to refetch OpenstackCreds for status update")
					} else {
						// Update status with OpenStack info
						if resources.OpenstackInfo != nil {
							freshOSCreds.Status.Openstack = *resources.OpenstackInfo
						}

						if err := p.K8sClient.Status().Update(ctx, freshOSCreds); err != nil {
							logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Warn("Failed to update OpenstackCreds status with OpenStack info")
						} else {
							logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace, "flavor_count": len(resources.Flavors)}).Info("Successfully fetched and updated OpenStack resources")
						}
					}
				}
			}
		} else {
			oscreds.Status.OpenStackValidationStatus = "Failed"
			oscreds.Status.OpenStackValidationMessage = result.Message
			logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace, "message": result.Message}).Warn("OpenStack validation failed")

			if err := p.K8sClient.Status().Update(ctx, oscreds); err != nil {
				logrus.WithFields(logrus.Fields{"func": fn, "name": name, "namespace": namespace}).WithError(err).Error("Failed to update OpenstackCreds status")
				return nil, fmt.Errorf("failed to update OpenstackCreds status: %w", err)
			}
		}

		responseMsg := fmt.Sprintf("Validation completed for %s: %s", name, result.Message)
		return &api.RevalidateCredentialsResponse{Message: responseMsg}, nil

	default:
		logrus.WithFields(logrus.Fields{"func": fn, "kind": kind, "name": name, "namespace": namespace}).Error("Unknown credentials kind")
		return nil, fmt.Errorf("unknown credentials kind: %s", kind)
	}
}

func (p *vjailbreakProxy) UploadVDDK(stream api.VailbreakProxy_UploadVDDKServer) error {
	const fn = "UploadVDDK"
	logrus.WithField("func", fn).Info("Starting VDDK upload via gRPC stream")

	extractDir := "/home/ubuntu"
	tempDir := "/tmp/vddk-uploads"

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to create temp directory")
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	var uploadID string
	var state *uploadState
	var isComplete bool

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.WithField("func", fn).WithError(err).Error("Error receiving stream")
			if state != nil && state.tempFile != nil {
				state.tempFile.Close()
				os.Remove(state.tempFilePath)
			}
			return fmt.Errorf("failed to receive stream: %w", err)
		}

		uploadID = req.UploadId
		if uploadID == "" {
			uploadID = fmt.Sprintf("vddk_%d", time.Now().UnixNano())
		}

		p.uploadStatesMu.Lock()
		if p.uploadStates == nil {
			p.uploadStates = make(map[string]*uploadState)
		}
		state = p.uploadStates[uploadID]
		if state == nil {
			tempFilePath := filepath.Join(tempDir, req.Filename)
			tempFile, err := os.Create(tempFilePath)
			if err != nil {
				p.uploadStatesMu.Unlock()
				logrus.WithField("func", fn).WithError(err).Error("Failed to create temp file")
				return fmt.Errorf("failed to create temp file: %w", err)
			}

			state = &uploadState{
				filename:       req.Filename,
				tempFilePath:   tempFilePath,
				tempFile:       tempFile,
				totalChunks:    req.TotalChunks,
				receivedChunks: make(map[int64]bool),
			}
			p.uploadStates[uploadID] = state

			logrus.WithFields(logrus.Fields{
				"func":         fn,
				"upload_id":    uploadID,
				"filename":     req.Filename,
				"total_chunks": req.TotalChunks,
			}).Info("Initializing VDDK upload")
		}
		p.uploadStatesMu.Unlock()

		state.mu.Lock()
		chunk := req.FileChunk
		if len(chunk) > 0 {
			n, err := state.tempFile.Write(chunk)
			if err != nil {
				state.mu.Unlock()
				logrus.WithField("func", fn).WithError(err).Error("Failed to write chunk")
				state.tempFile.Close()
				os.Remove(state.tempFilePath)
				return fmt.Errorf("failed to write chunk: %w", err)
			}
			state.totalBytesReceived += int64(n)
			state.receivedChunks[req.ChunkIndex] = true

			logrus.WithFields(logrus.Fields{
				"func":          fn,
				"upload_id":     uploadID,
				"chunk_index":   req.ChunkIndex,
				"bytes_written": n,
				"total_bytes":   state.totalBytesReceived,
				"chunks_recv":   len(state.receivedChunks),
				"total_chunks":  state.totalChunks,
			}).Debug("Chunk written")

			if int64(len(state.receivedChunks)) >= state.totalChunks {
				isComplete = true
			}
		}
		state.mu.Unlock()
	}

	if state == nil {
		return fmt.Errorf("no upload state found")
	}

	if state.tempFile != nil {
		if err := state.tempFile.Sync(); err != nil {
			logrus.WithField("func", fn).WithError(err).Error("Failed to sync file")
			state.tempFile.Close()
			os.Remove(state.tempFilePath)
			return fmt.Errorf("failed to sync file: %w", err)
		}
		state.tempFile.Close()
	}

	if !isComplete {
		response := &api.UploadVDDKResponse{
			UploadId:           uploadID,
			Status:             "in_progress",
			Message:            fmt.Sprintf("Received %d/%d chunks", len(state.receivedChunks), state.totalChunks),
			BytesReceived:      state.totalBytesReceived,
			ProgressPercentage: float32(len(state.receivedChunks)) / float32(state.totalChunks) * 100,
		}
		if err := stream.SendAndClose(response); err != nil {
			logrus.WithField("func", fn).WithError(err).Error("Failed to send in-progress response")
			return fmt.Errorf("failed to send response: %w", err)
		}
		return nil
	}

	p.uploadStatesMu.Lock()
	delete(p.uploadStates, uploadID)
	p.uploadStatesMu.Unlock()

	logrus.WithFields(logrus.Fields{
		"func":        fn,
		"upload_id":   uploadID,
		"file_path":   state.tempFilePath,
		"total_bytes": state.totalBytesReceived,
	}).Info("VDDK tar file uploaded, starting extraction")

	if err := os.MkdirAll(extractDir, 0755); err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to create extraction directory")
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := extractTarFile(state.tempFilePath, extractDir); err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to extract tar file")
		return fmt.Errorf("failed to extract tar file: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"func":        fn,
		"upload_id":   uploadID,
		"extract_dir": extractDir,
	}).Info("VDDK tar file extracted successfully")

	response := &api.UploadVDDKResponse{
		UploadId:           uploadID,
		Status:             "success",
		Message:            "File uploaded and extracted successfully",
		ExtractDir:         extractDir,
		BytesReceived:      state.totalBytesReceived,
		ProgressPercentage: 100.0,
	}

	if err := stream.SendAndClose(response); err != nil {
		logrus.WithField("func", fn).WithError(err).Error("Failed to send response")
		return fmt.Errorf("failed to send response: %w", err)
	}

	return nil
}

func extractTarFile(tarPath, destDir string) error {
	const fn = "extractTarFile"
	logrus.WithFields(logrus.Fields{
		"func":     fn,
		"tar_path": tarPath,
		"dest_dir": destDir,
	}).Info("Starting tar extraction")

	tarFile, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarFile.Close()

	var tarReader *tar.Reader

	if strings.HasSuffix(tarPath, ".tar.gz") || strings.HasSuffix(tarPath, ".tgz") {
		gzReader, err := gzip.NewReader(tarFile)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else if strings.HasSuffix(tarPath, ".tar") {
		tarReader = tar.NewReader(tarFile)
	} else {
		cmd := exec.Command("tar", "-xzf", tarPath, "-C", destDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("tar command failed: %w, output: %s", err, string(output))
		}
		logrus.WithField("func", fn).Info("Extracted using tar command")
		return nil
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
			logrus.WithFields(logrus.Fields{"func": fn, "dir": target}).Debug("Created directory")

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", target, err)
			}

			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", target, err)
			}
			outFile.Close()
			logrus.WithFields(logrus.Fields{"func": fn, "file": target}).Debug("Extracted file")

		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}
			logrus.WithFields(logrus.Fields{"func": fn, "symlink": target}).Debug("Created symlink")

		default:
			logrus.WithFields(logrus.Fields{
				"func":     fn,
				"name":     header.Name,
				"typeflag": header.Typeflag,
			}).Warn("Unsupported tar entry type, skipping")
		}
	}

	logrus.WithFields(logrus.Fields{
		"func":     fn,
		"tar_path": tarPath,
		"dest_dir": destDir,
	}).Info("Tar extraction completed successfully")

	return nil
}
