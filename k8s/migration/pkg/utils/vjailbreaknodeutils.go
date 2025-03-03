package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	openstackutils "github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
)

type Network struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Link      string `json:"link"`
	NetworkID string `json:"network_id"`
}

type OpenStackMetadata struct {
	Networks []Network `json:"networks"`
}

func CheckAndCreateMasterNodeEntry(ctx context.Context) error {
	k3sclient, err := GetInclusterClient()
	if err != nil {
		return errors.Wrap(err, "failed to get client")
	}

	masterNode, err := GetMasterK8sNode(ctx, k3sclient)
	if err != nil {
		return errors.Wrap(err, "failed to get master node")
	}

	err = k3sclient.Get(ctx, client.ObjectKey{Name: masterNode.Name}, &vjailbreakv1alpha1.VjailbreakNode{})
	if err == nil {
		// VjailbreakNode already exists
		return nil
	}

	// Controller manager is always on the master node due to pod affinity
	openstackuuid, err := openstackutils.GetCurrentInstanceUUID()
	if err != nil {
		return errors.Wrap(err, "failed to get current instance uuid")
	}

	vjNode := vjailbreakv1alpha1.VjailbreakNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.MasterVjailbreakNodeName,
			Namespace: constants.NamespaceMigrationSystem,
		},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeRole: constants.NodeRoleMaster,
		},
	}

	err = k3sclient.Create(ctx, &vjNode)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create vjailbreak node")
	}

	err = k3sclient.Get(ctx, types.NamespacedName{
		Namespace: constants.NamespaceMigrationSystem,
		Name:      constants.MasterVjailbreakNodeName,
	}, &vjNode)
	if err != nil {
		return errors.Wrap(err, "failed to get vjailbreak node")
	}

	vjNode.Status.VMIP = GetNodeInternalIP(masterNode)
	vjNode.Status.Phase = constants.VjailbreakNodePhaseNodeReady
	vjNode.Status.OpenstackUUID = openstackuuid

	err = k3sclient.Status().Update(ctx, &vjNode)
	if err != nil {
		return errors.Wrap(err, "failed to update vjailbreak node status")
	}

	return nil
}

func UpdateMasterNodeImageID(ctx context.Context, k3sclient client.Client, openstackcreds *vjailbreakv1alpha1.OpenstackCreds) error {
	vjNode := vjailbreakv1alpha1.VjailbreakNode{}
	err := k3sclient.Get(ctx, types.NamespacedName{
		Namespace: constants.NamespaceMigrationSystem,
		Name:      constants.MasterVjailbreakNodeName,
	}, &vjNode)
	if err != nil {
		return errors.Wrap(err, "failed to get vjailbreak node")
	}

	// Controller manager is always on the master node due to pod affinity
	openstackuuid, err := openstackutils.GetCurrentInstanceUUID()
	if err != nil {
		return errors.Wrap(err, "failed to get current instance uuid")
	}

	imageID, err := GetImageIDFromVM(openstackuuid, openstackcreds)
	if err != nil {
		return errors.Wrap(err, "failed to get image id of master node")
	}

	vjNode.Spec.ImageID = imageID
	vjNode.Spec.OpenstackCreds = corev1.ObjectReference{
		Name:      openstackcreds.Name,
		Namespace: openstackcreds.Namespace,
		Kind:      openstackcreds.Kind,
	}

	flavors, err := ListAllFlavors(openstackcreds)
	if err != nil {
		return errors.Wrap(err, "failed to get flavors")
	}

	vjNode.Spec.AvailableFlavors = flavors

	err = k3sclient.Update(ctx, &vjNode)
	if err != nil {
		return errors.Wrap(err, "failed to update vjailbreak node")
	}
	return nil
}

func IsMasterNode(node *corev1.Node) bool {
	_, ok := node.Labels[constants.K8sMasterNodeAnnotation]
	return ok
}

func GetAllk8sNodes(ctx context.Context, k3sclient client.Client) (corev1.NodeList, error) {
	nodeList := corev1.NodeList{}
	err := k3sclient.List(ctx, &nodeList)
	if err != nil {
		return corev1.NodeList{}, err
	}
	return nodeList, nil
}

func GetNodeInternalIP(node *corev1.Node) string {
	return node.Annotations[constants.InternalIPAnnotation]
}

func GetMasterK8sNode(ctx context.Context, k3sclient client.Client) (*corev1.Node, error) {
	nodeList, err := GetAllk8sNodes(ctx, k3sclient)
	if err != nil {
		return nil, err
	}

	var masterNode *corev1.Node

	for i := range nodeList.Items {
		if IsMasterNode(&nodeList.Items[i]) {
			masterNode = &nodeList.Items[i]
			break
		}
	}
	if masterNode == nil {
		return nil, fmt.Errorf("node with required annotation not found")
	}
	return masterNode, nil
}

func CreateOpenstackVMForWorkerNode(ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (string, error) {
	vjNode := scope.VjailbreakNode
	log := scope.Logger

	// Update the VjailbreakNode status
	err := k3sclient.Status().Update(ctx, vjNode)
	if err != nil {
		return "", errors.Wrap(err, "failed to update vjailbreak node status")
	}
	imageID, err := GetImageID(ctx, k3sclient)
	if err != nil {
		return "", errors.Wrap(err, "failed to get image id")
	}

	token, err := os.ReadFile(constants.K3sTokenFileLocation)
	if err != nil {
		return "", errors.Wrap(err, "failed to read k3s token file")
	}

	masterNode, err := GetMasterK8sNode(ctx, k3sclient)
	if err != nil {
		return "", errors.Wrap(err, "failed to get master node")
	}

	creds, err := GetOpenstackCreds(ctx, k3sclient, scope)
	if err != nil {
		return "", errors.Wrap(err, "failed to get openstack creds")
	}
	computeClient, err := GetOpenstackComputeClient(creds)
	if err != nil {
		return "", errors.Wrap(err, "failed to get compute client")
	}

	networkIDs, err := GetCurrentInstanceNetworkInfo()
	if err != nil {
		return "", errors.Wrap(err, "failed to get network info")
	}

	// Define server creation parameters
	serverCreateOpts := servers.CreateOpts{
		Name:      vjNode.Name,
		FlavorRef: vjNode.Spec.OpenstackFlavorID,
		ImageRef:  imageID,
		Networks:  networkIDs,
		UserData: []byte(fmt.Sprintf(constants.CloudInitScript,
			token[:12], constants.ENVFileLocation,
			"false", GetNodeInternalIP(masterNode),
			token)),
	}

	// Create the VM
	server, err := servers.Create(computeClient, serverCreateOpts).Extract()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create server")
	}

	log.Info("Server created", "ID", server.ID)
	return server.ID, nil
}

func GetOpenstackCreds(ctx context.Context, k3sclient client.Client,
	scope *scope.VjailbreakNodeScope) (*vjailbreakv1alpha1.OpenstackCreds, error) {
	vjNode := scope.VjailbreakNode

	oscreds := &vjailbreakv1alpha1.OpenstackCreds{}
	err := k3sclient.Get(ctx, client.ObjectKey{
		Name:      vjNode.Spec.OpenstackCreds.Name,
		Namespace: vjNode.Spec.OpenstackCreds.Namespace,
	}, oscreds)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	return oscreds, nil
}

func GetCurrentInstanceNetworkInfo() ([]servers.Network, error) {
	client := &http.Client{}
	networks := []servers.Network{}
	req, err := http.NewRequestWithContext(context.Background(), "GET",
		"http://169.254.169.254/openstack/latest/network_data.json", http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	var metadata OpenStackMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	for _, Network := range metadata.Networks {
		networks = append(networks, servers.Network{
			UUID: Network.NetworkID,
		})
	}
	return networks, nil
}

func GetOpenstackVMIP(uuid string, ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (string, error) {
	creds, err := GetOpenstackCreds(ctx, k3sclient, scope)
	if err != nil {
		return "", errors.Wrap(err, "failed to get openstack creds")
	}
	computeClient, err := GetOpenstackComputeClient(creds)
	if err != nil {
		return "", errors.Wrap(err, "failed to get compute client")
	}

	// Fetch the VM details
	server, err := servers.Get(computeClient, uuid).Extract()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get server details")
	}

	// Extract IP addresses
	for _, addresses := range server.Addresses {
		for _, addr := range addresses.([]any) {
			ipInfo := addr.(map[string]any)
			return ipInfo["addr"].(string), nil
		}
	}
	return "", errors.New("failed to get vm ip")
}

func GetImageIDFromVM(uuid string,
	openstackcreds *vjailbreakv1alpha1.OpenstackCreds) (string, error) {
	computeClient, err := GetOpenstackComputeClient(openstackcreds)
	if err != nil {
		return "", errors.Wrap(err, "failed to get compute client")
	}

	// Fetch the VM details
	server, err := servers.Get(computeClient, uuid).Extract()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get server details")
	}

	if server.Image["id"] != nil {
		fmt.Println("Image ID found", "Image ID", server.Image["id"])
	} else {
		return "", fmt.Errorf("instance was booted from a volume, no image ID available")
	}

	if imageID, ok := server.Image["id"].(string); ok {
		return imageID, nil
	}
	return "", fmt.Errorf("failed to assert image ID as string")
}

func ListAllFlavors(openstackcreds *vjailbreakv1alpha1.OpenstackCreds) ([]flavors.Flavor, error) {
	computeClient, err := GetOpenstackComputeClient(openstackcreds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get compute client")
	}

	// List flavors
	allPages, err := flavors.ListDetail(computeClient, nil).AllPages()
	if err != nil {
		log.Fatalf("Failed to list flavors: %v", err)
	}

	return flavors.ExtractFlavors(allPages)
}

func DeleteOpenstackVM(uuid string, ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) error {
	creds, err := GetOpenstackCreds(ctx, k3sclient, scope)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack creds")
	}
	computeClient, err := GetOpenstackComputeClient(creds)
	if err != nil {
		return errors.Wrap(err, "failed to get compute client")
	}

	// delete the VM
	err = servers.Delete(computeClient, uuid).ExtractErr()
	if err != nil && !strings.Contains(err.Error(), "404") {
		return errors.Wrap(err, "Failed to delete server")
	}
	return nil
}

func GetOpenstackComputeClient(creds *vjailbreakv1alpha1.OpenstackCreds) (*gophercloud.ServiceClient, error) {
	// Authenticate with OpenStack

	openstackCredential, err := GetOpenstackCredentials(context.TODO(), creds.Spec.SecretRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get vCenter credentials from secret: %w", err)
	}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: openstackCredential.AuthURL,
		Username:         openstackCredential.Username,
		Password:         openstackCredential.Password,
		DomainName:       openstackCredential.DomainName,
		TenantName:       openstackCredential.TenantName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to authenticate")
	}

	// Get the compute client
	computeClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: openstackCredential.RegionName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create compute client")
	}

	return computeClient, nil
}

func GetImageID(ctx context.Context, k3sclient client.Client) (string, error) {
	vjNode := vjailbreakv1alpha1.VjailbreakNode{}
	// Get the image ID from the vjailbreak master node
	err := k3sclient.Get(ctx, types.NamespacedName{
		Namespace: constants.NamespaceMigrationSystem,
		Name:      constants.MasterVjailbreakNodeName,
	}, &vjNode)
	if err != nil {
		return "", errors.Wrap(err, "failed to get vjailbreak node")
	}
	return vjNode.Spec.ImageID, nil
}

func GetOpenstackVMByName(name string, ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (string, error) {
	creds, err := GetOpenstackCreds(ctx, k3sclient, scope)
	if err != nil {
		return "", errors.Wrap(err, "failed to get openstack creds")
	}
	computeClient, err := GetOpenstackComputeClient(creds)
	if err != nil {
		return "", errors.Wrap(err, "failed to get compute client")
	}

	listOpts := servers.ListOpts{Name: name}
	allPages, err := servers.List(computeClient, listOpts).AllPages()
	if err != nil {
		return "", errors.Wrap(err, "failed to list servers")
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil || len(allServers) == 0 {
		return "", errors.Wrap(err, "failed to extract servers")
	}

	vmID := allServers[0].ID
	return vmID, nil
}

func ReadFileContent(filePath string) (string, error) {
	// Read entire file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read file")
	}

	return string(data), nil
}

func GetActiveMigrations(nodeName string, ctx context.Context, k3sclient client.Client) ([]string, error) {
	migrationList := &vjailbreakv1alpha1.MigrationList{}
	err := k3sclient.List(ctx, migrationList)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list migrations")
	}

	ignorePhases := []vjailbreakv1alpha1.MigrationPhase{vjailbreakv1alpha1.MigrationPhasePending,
		vjailbreakv1alpha1.MigrationPhaseFailed,
		vjailbreakv1alpha1.MigrationPhaseSucceeded,
		vjailbreakv1alpha1.MigrationPhaseUnknown,
	}

	var activeMigrations []string
	for i := range migrationList.Items {
		migration := &migrationList.Items[i]
		if migration.Status.AgentName == nodeName && !slices.Contains(ignorePhases, migration.Status.Phase) {
			activeMigrations = append(activeMigrations,
				migration.Name)
		}
	}
	return activeMigrations, nil
}

func GetInclusterClient() (client.Client, error) {
	// Create a direct Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get in-cluster config")
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vjailbreakv1alpha1.AddToScheme(scheme))
	clientset, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get in-cluster config")
	}

	return clientset, err
}

// GetNodeByName returns the node object by name
func GetNodeByName(ctx context.Context, k3sclient client.Client, nodeName string) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := k3sclient.Get(ctx, client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}
	return node, nil
}

func DeleteNodeByName(ctx context.Context, k3sclient client.Client, nodeName string) error {
	node := &corev1.Node{}
	err := k3sclient.Get(ctx, client.ObjectKey{
		Name: nodeName,
	}, node)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get node")
	}
	err = k3sclient.Delete(ctx, node)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete node")
	}
	return nil
}
