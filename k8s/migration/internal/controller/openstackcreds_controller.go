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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	utils "github.com/platform9/vjailbreak/k8s/migration/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// OpenstackCredsReconciler reconciles a OpenstackCreds object
type OpenstackCredsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type OpenStackClients struct {
	BlockStorageClient *gophercloud.ServiceClient
	ComputeClient      *gophercloud.ServiceClient
	NetworkingClient   *gophercloud.ServiceClient
}

// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vjailbreak.k8s.pf9.io,resources=openstackcreds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenstackCreds object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *OpenstackCredsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxlog := log.FromContext(ctx)

	// Get the OpenstackCreds object
	openstackcreds := &vjailbreakv1alpha1.OpenstackCreds{}
	if err := r.Get(ctx, req.NamespacedName, openstackcreds); err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info("Received ignorable event for a recently deleted OpenstackCreds.")
			return ctrl.Result{}, nil
		}
		ctxlog.Error(err, fmt.Sprintf("Unexpected error reading OpenstackCreds '%s' object", openstackcreds.Name))
		return ctrl.Result{}, err
	}

	if openstackcreds.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check if speck matches with kubectl.kubernetes.io/last-applied-configuration
		ctxlog.Info(fmt.Sprintf("OpenstackCreds '%s' CR is being created or updated", openstackcreds.Name))
		ctxlog.Info(fmt.Sprintf("Validating OpenstackCreds '%s' object", openstackcreds.Name))
		if _, err := validateOpenstackCreds(ctxlog, openstackcreds); err != nil {
			// Update the status of the OpenstackCreds object
			ctxlog.Error(err, fmt.Sprintf("Error validating OpenstackCreds '%s'", openstackcreds.Name))
			openstackcreds.Status.OpenStackValidationStatus = "Failed"
			openstackcreds.Status.OpenStackValidationMessage = fmt.Sprintf("Error validating OpenstackCreds '%s'", openstackcreds.Name)
			if err := r.Status().Update(ctx, openstackcreds); err != nil {
				ctxlog.Error(err, fmt.Sprintf("Error updating status of OpenstackCreds '%s'", openstackcreds.Name))
				return ctrl.Result{}, err
			}
		} else {
			err := utils.UpdateMasterNodeImageID(ctx, r.Client, openstackcreds)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					ctxlog.Error(err, "failed to update master node image id and flavor list, skipping reconciliation")
				} else {
					return ctrl.Result{}, errors.Wrap(err, "failed to update master node image id")
				}
			}

			openstackCredential, err := utils.GetOpenstackCredentials(context.TODO(), openstackcreds.Spec.SecretRef.Name)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to get Openstack credentials from secret")
			}

			ctxlog.Info(fmt.Sprintf("Successfully authenticated to Openstack '%s'", openstackCredential.AuthURL))
			// Update the status of the OpenstackCreds object
			openstackcreds.Status.OpenStackValidationStatus = "Succeeded"
			openstackcreds.Status.OpenStackValidationMessage = "Successfully authenticated to Openstack"
			if err := r.Status().Update(ctx, openstackcreds); err != nil {
				ctxlog.Error(err, fmt.Sprintf("Error updating status of OpenstackCreds '%s'", openstackcreds.Name))
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

func getCert(endpoint string) (*x509.Certificate, error) {
	conf := &tls.Config{
		//nolint:gosec // This is required to skip certificate verification
		InsecureSkipVerify: true,
	}
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	hostname := parsedURL.Hostname()
	conn, err := tls.Dial("tcp", hostname+":443", conf)
	if err != nil {
		return nil, fmt.Errorf("error connecting to %s: %w", hostname, err)
	}
	defer conn.Close()
	cert := conn.ConnectionState().PeerCertificates[0]
	return cert, nil
}

func validateOpenstackCreds(ctxlog logr.Logger, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (*OpenStackClients, error) {
	openstackCredential, err := utils.GetOpenstackCredentials(context.TODO(), openstackcreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get openstack credentials from secret: %w", err)
	}

	providerClient, err := openstack.NewClient(openstackCredential.AuthURL)
	if err != nil {
		ctxlog.Error(err, fmt.Sprintf("Error creating Openstack Client'%s'", openstackCredential.AuthURL))
		return nil, err
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if openstackCredential.Insecure {
		ctxlog.Info("Insecure flag is set, skipping certificate verification")
		tlsConfig.InsecureSkipVerify = true
	} else {
		// Get the certificate for the Openstack endpoint
		caCert, certerr := getCert(openstackCredential.AuthURL)
		if certerr != nil {
			ctxlog.Error(err, fmt.Sprintf("Error getting certificate for '%s'", openstackCredential.AuthURL))
			return nil, err
		}
		// Logging the certificate
		ctxlog.Info(fmt.Sprintf("Trusting certificate for '%s'", openstackCredential.AuthURL))
		ctxlog.Info(string(pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caCert.Raw,
		})))
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
		ctxlog.Error(err, fmt.Sprintf("Error authenticating to Openstack '%s'", openstackCredential.AuthURL))
		return nil, err
	}
	endpoint := gophercloud.EndpointOpts{
		Region: openstackCredential.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(providerClient, endpoint)
	if err != nil {
		ctxlog.Error(err, fmt.Sprintf("Error validating region '%s' for '%s'",
			openstackCredential.RegionName, openstackCredential.AuthURL))
		return nil, err
	}
	blockStorageClient, err := openstack.NewBlockStorageV3(providerClient, endpoint)
	if err != nil {
		ctxlog.Error(err, fmt.Sprintf("Error validating region '%s' for '%s'",
			openstackCredential.RegionName, openstackCredential.AuthURL))
		return nil, err
	}
	networkingClient, err := openstack.NewNetworkV2(providerClient, endpoint)
	if err != nil {
		ctxlog.Error(err, fmt.Sprintf("Error validating region '%s' for '%s'",
			openstackCredential.RegionName, openstackCredential.AuthURL))
		return nil, err
	}
	return &OpenStackClients{
		BlockStorageClient: blockStorageClient,
		ComputeClient:      computeClient,
		NetworkingClient:   networkingClient,
	}, nil
}

//nolint:dupl // This function is similar to VerifyNetworks, excluding from linting to keep it readable
func VerifyNetworks(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetnetworks []string) error {
	openstackClients, err := validateOpenstackCreds(log.FromContext(ctx), openstackcreds)
	if err != nil {
		return err
	}
	allPages, err := networks.List(openstackClients.NetworkingClient, nil).AllPages()
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return fmt.Errorf("failed to extract all networks: %w", err)
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
	openstackClients, err := validateOpenstackCreds(log.FromContext(ctx), openstackcreds)
	if err != nil {
		return err
	}

	allPages, err := ports.List(openstackClients.NetworkingClient, nil).AllPages()
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	allPorts, err := ports.ExtractPorts(allPages)
	if err != nil {
		return fmt.Errorf("failed to extract all networks: %w", err)
	}

	// Build a map of all ports
	portMap := make(map[string]bool)
	for i := 0; i < len(allPorts); i++ {
		portMap[allPorts[i].ID] = true
	}

	// Verify that all port names in targetports exist in the openstack ports
	for _, targetPort := range targetports {
		if _, found := portMap[targetPort]; !found {
			return fmt.Errorf("port '%s' not found in OpenStack", targetPort)
		}
	}
	return nil
}

func VerifyStorage(ctx context.Context, openstackcreds *vjailbreakv1alpha1.OpenstackCreds, targetstorages []string) error {
	openstackClients, err := validateOpenstackCreds(log.FromContext(ctx), openstackcreds)
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
	openstackClients, err := validateOpenstackCreds(log.FromContext(ctx), openstackcreds)
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

// SetupWithManager sets up the controller with the Manager.
func (r *OpenstackCredsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vjailbreakv1alpha1.OpenstackCreds{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
