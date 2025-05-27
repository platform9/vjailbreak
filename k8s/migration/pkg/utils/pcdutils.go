package utils

import (
	"context"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	"github.com/platform9/vjailbreak/k8s/migration/pkg/sdk/resmgr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncPCDInfo syncs PCD info from resmgr
func SyncPCDInfo(ctx context.Context, k8sClient client.Client, openstackCreds vjailbreakv1alpha1.OpenstackCreds) error {
	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCreds.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return errors.Wrap(err, "failed to get resmgr client")
	}

	// Get PCDHostConfig from openstackCreds
	pcdHostConfig, err := resmgrClient.ListHostConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list host configs")
	}
	openstackCreds.Spec.PCDHostConfig = pcdHostConfig

	if err := k8sClient.Update(ctx, &openstackCreds); err != nil {
		return errors.Wrap(err, "failed to update openstack creds")
	}

	clusterList, err := resmgrClient.ListClusters(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list clusters")
	}
	for _, cluster := range clusterList {
		err := CreatePCDClusterFromResmgrCluster(ctx, k8sClient, cluster, &openstackCreds)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				updateErr := UpdatePCDClusterFromResmgrCluster(ctx, k8sClient, cluster, &openstackCreds)
				if updateErr != nil {
					return errors.Wrap(updateErr, "failed to update PCD cluster")
				}
				continue
			}
			return errors.Wrap(err, "failed to create PCD cluster")
		}
	}
	hostList, err := resmgrClient.ListHosts(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list hosts")
	}
	for _, host := range hostList {
		err := CreatePCDHostFromResmgrHost(ctx, k8sClient, host, &openstackCreds)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				updateErr := UpdatePCDHostFromResmgrHost(ctx, k8sClient, host, &openstackCreds)
				if updateErr != nil {
					return errors.Wrap(updateErr, "failed to update PCD host")
				}
				continue
			}
			return errors.Wrap(err, "failed to create PCD host")
		}
	}
	err = DeleteStalePCDHosts(ctx, k8sClient, openstackCreds)
	if err != nil {
		return errors.Wrap(err, "failed to delete stale PCD hosts")
	}
	err = DeleteStalePCDClusters(ctx, k8sClient, openstackCreds)
	if err != nil {
		return errors.Wrap(err, "failed to delete stale PCD clusters")
	}
	return nil
}

// CreatePCDHostFromResmgrHost creates a PCDHost from resmgr Host
func CreatePCDHostFromResmgrHost(ctx context.Context, k8sClient client.Client, host resmgr.Host, openstackCreds *vjailbreakv1alpha1.OpenstackCreds) error {
	pcdHost := generatePCDHostFromResmgrHost(openstackCreds, host)
	if err := k8sClient.Create(ctx, &pcdHost); err != nil {
		return errors.Wrap(err, "failed to create PCD host")
	}
	return nil
}

// CreatePCDClusterFromResmgrCluster creates a PCDCluster from resmgr Cluster
func CreatePCDClusterFromResmgrCluster(ctx context.Context, k8sClient client.Client, cluster resmgr.Cluster, openstackCreds *vjailbreakv1alpha1.OpenstackCreds) error {
	pcdCluster := generatePCDClusterFromResmgrCluster(openstackCreds, cluster)
	if err := k8sClient.Create(ctx, &pcdCluster); err != nil {
		return errors.Wrap(err, "failed to create PCD cluster")
	}
	return nil
}

func UpdatePCDHostFromResmgrHost(ctx context.Context, k8sClient client.Client, host resmgr.Host, openstackCreds *vjailbreakv1alpha1.OpenstackCreds) error {
	pcdHost := generatePCDHostFromResmgrHost(openstackCreds, host)
	oldPCDHost := vjailbreakv1alpha1.PCDHost{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: host.ID, Namespace: constants.NamespaceMigrationSystem}, &oldPCDHost); err != nil {
		return errors.Wrap(err, "failed to get PCD host")
	}
	oldPCDHost.Spec = pcdHost.Spec
	oldPCDHost.Status = pcdHost.Status
	if err := k8sClient.Update(ctx, &oldPCDHost); err != nil {
		return errors.Wrap(err, "failed to update PCD host")
	}
	if err := k8sClient.Status().Update(ctx, &oldPCDHost); err != nil {
		return errors.Wrap(err, "failed to update PCD host status")
	}
	return nil
}

func UpdatePCDClusterFromResmgrCluster(ctx context.Context, k8sClient client.Client, cluster resmgr.Cluster, openstackCreds *vjailbreakv1alpha1.OpenstackCreds) error {
	oldPCDCluster := vjailbreakv1alpha1.PCDCluster{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: constants.NamespaceMigrationSystem}, &oldPCDCluster); err != nil {
		return errors.Wrap(err, "failed to get PCD cluster")
	}

	pcdCluster := generatePCDClusterFromResmgrCluster(openstackCreds, cluster)
	oldPCDCluster.Spec = pcdCluster.Spec
	oldPCDCluster.Status = pcdCluster.Status
	if err := k8sClient.Update(ctx, &oldPCDCluster); err != nil {
		return errors.Wrap(err, "failed to update PCD cluster")
	}
	if err := k8sClient.Status().Update(ctx, &oldPCDCluster); err != nil {
		return errors.Wrap(err, "failed to update PCD cluster status")
	}
	return nil
}

func generatePCDHostFromResmgrHost(openstackCreds *vjailbreakv1alpha1.OpenstackCreds, host resmgr.Host) vjailbreakv1alpha1.PCDHost {
	// Create a new PCDHost
	interfaces := []vjailbreakv1alpha1.PCDHostInterface{}
	for name, itface := range host.Extensions.Interfaces.Data.IfaceInfo {
		// Collect all IP addresses from the interface
		ipAddresses := []string{}
		for _, iface := range itface.Ifaces {
			ipAddresses = append(ipAddresses, iface.Addr)
		}

		// Create the interface with all IPs and the MAC address
		interfaces = append(interfaces, vjailbreakv1alpha1.PCDHostInterface{
			IPAddresses: ipAddresses,
			MACAddress:  itface.MAC,
			Name:        name,
		})
	}
	pcdHost := vjailbreakv1alpha1.PCDHost{
		ObjectMeta: metav1.ObjectMeta{
			// Use the host ID as the name to ensure uniqueness
			Name:      host.ID,
			Namespace: constants.NamespaceMigrationSystem,
			// Add labels if needed
			Labels: map[string]string{
				constants.OpenstackCredsLabel: openstackCreds.Name,
			},
		},
		Spec: vjailbreakv1alpha1.PCDHostSpec{
			HostName:      host.Info.Hostname,
			HostID:        host.ID,
			HostState:     host.RoleStatus,
			RolesAssigned: host.Roles,
			OSFamily:      host.Info.OSFamily,
			Arch:          host.Info.Arch,
			OSInfo:        host.Info.OSInfo,
			Interfaces:    interfaces,
		},
		Status: vjailbreakv1alpha1.PCDHostStatus{
			Responding: host.Info.Responding,
			RoleStatus: host.RoleStatus,
		},
	}
	return pcdHost
}

func generatePCDClusterFromResmgrCluster(openstackCreds *vjailbreakv1alpha1.OpenstackCreds, cluster resmgr.Cluster) vjailbreakv1alpha1.PCDCluster {
	return vjailbreakv1alpha1.PCDCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.OpenstackCredsLabel: openstackCreds.Name,
			},
		},
		Spec: vjailbreakv1alpha1.PCDClusterSpec{
			ClusterName:                   cluster.Name,
			Description:                   cluster.Description,
			Hosts:                         cluster.Hostlist,
			VMHighAvailability:            cluster.VMHighAvailability.Enabled,
			EnableAutoResourceRebalancing: cluster.AutoResourceRebalancing.Enabled,
			RebalancingFrequencyMins:      cluster.AutoResourceRebalancing.RebalancingFrequencyMins,
		},
		Status: vjailbreakv1alpha1.PCDClusterStatus{
			AggregateID: cluster.AggregateID,
			CreatedAt:   cluster.CreatedAt,
			UpdatedAt:   cluster.UpdatedAt,
		},
	}
}

func DeleteStalePCDHosts(ctx context.Context, k8sClient client.Client, openstackCreds vjailbreakv1alpha1.OpenstackCreds) error {

	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCreds.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return errors.Wrap(err, "failed to get resmgr client")
	}
	upstreamHostList, err := resmgrClient.ListHosts(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list hosts")
	}
	upstreamHostNames := []string{}
	for _, host := range upstreamHostList {
		upstreamHostNames = append(upstreamHostNames, host.ID)
	}
	downstreamHostList, err := filterPCDHostsOnOpenstackCreds(ctx, k8sClient, openstackCreds)
	if err != nil {
		return errors.Wrap(err, "failed to filter PCD hosts")
	}
	for _, host := range downstreamHostList {
		if !containsString(upstreamHostNames, host.Spec.HostID) {
			if err := k8sClient.Delete(ctx, &vjailbreakv1alpha1.PCDHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      host.Name,
					Namespace: constants.NamespaceMigrationSystem,
				},
			}); err != nil {
				return errors.Wrap(err, "failed to delete stale PCD host")
			}
		}
	}
	return nil
}

func DeleteStalePCDClusters(ctx context.Context, k8sClient client.Client, openstackCreds vjailbreakv1alpha1.OpenstackCreds) error {
	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCreds.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return errors.Wrap(err, "failed to get resmgr client")
	}
	upstreamClusterList, err := resmgrClient.ListClusters(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list clusters")
	}
	upstreamClusterNames := []string{}
	for _, cluster := range upstreamClusterList {
		upstreamClusterNames = append(upstreamClusterNames, cluster.Name)
	}

	downstreamClusterList, err := filterPCDClustersOnOpenstackCreds(ctx, k8sClient, openstackCreds)
	if err != nil {
		return errors.Wrap(err, "failed to filter PCD clusters")
	}
	for _, cluster := range downstreamClusterList {
		if !containsString(upstreamClusterNames, cluster.Spec.ClusterName) {
			if err := k8sClient.Delete(ctx, &vjailbreakv1alpha1.PCDCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cluster.Name,
					Namespace: constants.NamespaceMigrationSystem,
				},
			}); err != nil {
				return errors.Wrap(err, "failed to delete stale PCD cluster")
			}
		}
	}
	return nil
}

func filterPCDClustersOnOpenstackCreds(ctx context.Context, k8sClient client.Client, openstackCreds vjailbreakv1alpha1.OpenstackCreds) ([]vjailbreakv1alpha1.PCDCluster, error) {
	err := k8sClient.List(ctx, &vjailbreakv1alpha1.PCDClusterList{}, &client.ListOptions{
		Namespace: constants.NamespaceMigrationSystem,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			constants.OpenstackCredsLabel: openstackCreds.Name,
		}),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list PCD clusters")
	}
	return nil, nil
}

func filterPCDHostsOnOpenstackCreds(ctx context.Context, k8sClient client.Client, openstackCreds vjailbreakv1alpha1.OpenstackCreds) ([]vjailbreakv1alpha1.PCDHost, error) {
	err := k8sClient.List(ctx, &vjailbreakv1alpha1.PCDHostList{}, &client.ListOptions{
		Namespace: constants.NamespaceMigrationSystem,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			constants.OpenstackCredsLabel: openstackCreds.Name,
		}),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list PCD hosts")
	}
	return nil, nil
}

func WaitForHostOnPCD(ctx context.Context, k8sClient client.Client, openstackCreds vjailbreakv1alpha1.OpenstackCreds, hostID string) (bool, error) {
	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCreds.Name)
	if err != nil {
		return false, errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return false, errors.Wrap(err, "failed to get resmgr client")
	}
	return resmgrClient.HostExists(ctx, hostID)
}

func GetVMwareHostFromESXiName(ctx context.Context, k8sClient client.Client, esxiName string) (*vjailbreakv1alpha1.VMwareHost, error) {
	vmwareHost := &vjailbreakv1alpha1.VMwareHost{}
	esxiK8sName, err := ConvertToK8sName(esxiName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ESXi name to k8s name")
	}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: esxiK8sName, Namespace: constants.NamespaceMigrationSystem}, vmwareHost)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VMwareHost")
	}
	return vmwareHost, nil
}

func AssignHypervisorRoleToHost(ctx context.Context, k8sClient client.Client, openstackCredsName, pcdHost, clusterName string) error {
	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCredsName)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return errors.Wrap(err, "failed to get resmgr client")
	}
	return resmgrClient.AssignHypervisor(ctx, pcdHost, clusterName)
}

func AssignHostConfigToHost(ctx context.Context, k8sClient client.Client, openstackCredsName string, pcdHost string, hostConfigID string) error {
	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCredsName)
	if err != nil {
		return errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return errors.Wrap(err, "failed to get resmgr client")
	}
	return resmgrClient.AssignHostConfig(ctx, pcdHost, hostConfigID)
}

func WaitForHypervisorRoleAssignment(ctx context.Context, k8sClient client.Client, openstackCredsName string, pcdHostID string) (bool, error) {
	OpenStackCredentials, err := GetOpenstackCredsInfo(ctx, k8sClient, openstackCredsName)
	if err != nil {
		return false, errors.Wrap(err, "failed to get openstack credentials")
	}
	resmgrClient, err := GetResmgrClient(OpenStackCredentials)
	if err != nil {
		return false, errors.Wrap(err, "failed to get resmgr client")
	}
	pcdHost, err := resmgrClient.GetHost(ctx, pcdHostID)
	if err != nil {
		return false, errors.Wrap(err, "failed to get host")
	}
	if !containsString(pcdHost.Roles, "pf9-ostackhost-neutron") || pcdHost.RoleStatus != "ok" {
		return false, nil
	}
	return true, nil
}
