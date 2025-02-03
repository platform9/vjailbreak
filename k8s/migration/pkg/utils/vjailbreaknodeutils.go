package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Network struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Link      string `json:"link"`
	NetworkId string `json:"network_id"`
}

type OpenStackMetadata struct {
	Networks []Network `json:"networks"`
}

func CheckAndCreateMasterNodeEntry(ctx context.Context, k3sclient client.Client) error {
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

	vjNode.Status.VMIP = GetNodeInternalIp(masterNode)
	vjNode.Status.Phase = constants.VjailbreakNodePhaseNodeCreated
	vjNode.Status.OpenstackUUID = openstackuuid

	err = k3sclient.Status().Update(ctx, &vjNode)
	if err != nil {
		return errors.Wrap(err, "failed to update vjailbreak node status")
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

func GetNodeInternalIp(node *corev1.Node) string {
	return node.Annotations[constants.InternalIPAnnotation]
}

func GetMasterK8sNode(ctx context.Context, k3sclient client.Client) (*corev1.Node, error) {
	nodeList, err := GetAllk8sNodes(ctx, k3sclient)
	if err != nil {
		return nil, err
	}

	var masterNode *corev1.Node

	for _, node := range nodeList.Items {
		if IsMasterNode(&node) {
			masterNode = &node
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

	token, err := os.ReadFile(constants.K3sTokenFileLocation)
	if err != nil {
		return "", errors.Wrap(err, "failed to read k3s token file")
	}

	masterNode, err := GetMasterK8sNode(ctx, k3sclient)
	if err != nil {
		return "", errors.Wrap(err, "failed to get master node")
	}

	computeClient, err := GetOpenstackComputeClient(ctx, k3sclient, scope)
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
		FlavorRef: vjNode.Spec.OpenstackFlavorId,
		ImageRef:  vjNode.Spec.ImageID,
		Networks:  networkIDs,
		UserData:  []byte(fmt.Sprintf(constants.CloudInitScript, constants.ENVFileLocation, "false", GetNodeInternalIp(masterNode), token)),
	}

	// Create the VM
	server, err := servers.Create(computeClient, serverCreateOpts).Extract()
	if err != nil {
		log.Error(err, "Failed to create server")
	}

	log.Info("Server created", "ID", server.ID)

	return server.ID, nil
}

func GetOpenstackCreds(ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (*vjailbreakv1alpha1.OpenstackCreds, error) {
	vjNode := scope.VjailbreakNode

	oscreds := &vjailbreakv1alpha1.OpenstackCreds{}
	err := k3sclient.Get(ctx, client.ObjectKey{
		Name:      vjNode.Spec.OpenstackCreds.Name,
		Namespace: vjNode.Spec.OpenstackCreds.Namespace,
	}, oscreds)
	if err != nil {
		return nil, err
	}
	return oscreds, nil

}

func GetCurrentInstanceNetworkInfo() ([]servers.Network, error) {
	client := &http.Client{}
	networks := []servers.Network{}
	req, err := http.NewRequest("GET", "http://169.254.169.254/openstack/latest/network_data.json", nil)
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
			UUID: Network.NetworkId,
		})
	}
	return networks, nil
}

func GetOpenstackVMIP(uuid string, ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (string, error) {
	computeClient, err := GetOpenstackComputeClient(ctx, k3sclient, scope)
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
		for _, addr := range addresses.([]interface{}) {
			ipInfo := addr.(map[string]interface{})
			return ipInfo["addr"].(string), nil
		}
	}
	return "", errors.New("failed to get vm ip")
}

func DeleteOpenstackVM(uuid string, ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) error {
	computeClient, err := GetOpenstackComputeClient(ctx, k3sclient, scope)
	if err != nil {
		return errors.Wrap(err, "failed to get compute client")
	}

	// delete the VM
	err = servers.Delete(computeClient, uuid).ExtractErr()
	if err != nil {
		return errors.Wrap(err, "Failed to delete server")
	}
	return nil

}

func GetOpenstackComputeClient(ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (*gophercloud.ServiceClient, error) {

	creds, err := GetOpenstackCreds(ctx, k3sclient, scope)
	if err != nil {
		return nil, err
	}

	// Authenticate with OpenStack
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: creds.Spec.OsAuthURL,
		Username:         creds.Spec.OsUsername,
		Password:         creds.Spec.OsPassword,
		DomainName:       creds.Spec.OsDomainName,
		TenantName:       creds.Spec.OsTenantName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to authenticate")
	}

	// Get the compute client
	computeClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: creds.Spec.OsRegionName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create compute client")
	}

	return computeClient, nil
}

func GetOpenstackVMByName(name string, ctx context.Context, k3sclient client.Client, scope *scope.VjailbreakNodeScope) (string, error) {
	computeClient, err := GetOpenstackComputeClient(ctx, k3sclient, scope)
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
		return "", nil
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
