package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
			Name: masterNode.Name,
		},
		Spec: vjailbreakv1alpha1.VjailbreakNodeSpec{
			NodeName: masterNode.Name,
			NodeRole: constants.NodeRoleMaster,
		},
		Status: vjailbreakv1alpha1.VjailbreakNodeStatus{
			VMIP:          GetNodeInternalIp(masterNode),
			Phase:         constants.VjailbreakNodePhaseCreated,
			OpenstackUUID: openstackuuid,
		},
	}

	err = k3sclient.Create(ctx, &vjNode)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create vjailbreak node")
	}

	return nil
}

func IsMasterNode(node *corev1.Node) bool {
	_, ok := node.Annotations[constants.K8sMasterNodeAnnotation]
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

	creds, err := GetOpenstackCreds(ctx, k3sclient, scope)
	if err != nil {
		return "", err
	}

	// Authenticate with OpenStack
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: creds.Spec.OsAuthURL,
		Username:         creds.Spec.OsUsername,
		Password:         creds.Spec.OsPassword,
		DomainName:       creds.Spec.OsDomainName,
		TenantID:         creds.Spec.OsTenantName,
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		log.Error(err, "Failed to authenticate")
	}

	// Get the compute client
	computeClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		log.Error(err, "Failed to create compute client")
	}

	networkID, err := GetCurrentInstanceNetworkInfo()
	if err != nil {
		return "", err
	}

	// Define server creation parameters
	serverCreateOpts := servers.CreateOpts{
		Name:      vjNode.Name,
		FlavorRef: vjNode.Spec.OpenstackFlavorId,
		ImageRef:  vjNode.Spec.ImageID,
		Networks:  []servers.Network{{UUID: networkID}},
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

func GetCurrentInstanceNetworkInfo() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://169.254.169.254/openstack/latest/meta_data.json", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get response: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %s", err)
	}

	var metadata OpenStackMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return "", fmt.Errorf("failed to unmarshal metadata: %s", err)
	}

	return metadata.Networks[0].NetworkId, nil
}
