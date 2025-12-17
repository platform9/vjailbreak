// Package utils provides utility functions for handling migration-related operations.
// It includes functions for working with VMware environments, ESXi hosts, VM management,
// and integration with Platform9 components for migration workflows.
package utils

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetVMwareClustersAndHosts retrieves a list of all available VMware clusters and their hosts
func GetVMwareClustersAndHosts(ctx context.Context, scope *scope.VMwareCredsScope) ([]VMwareClusterInfo, error) {
	// Pre-allocate clusters slice with initial capacity
	clusters := make([]VMwareClusterInfo, 0, 4)
	vmwarecreds, err := GetVMwareCredentialsFromSecret(ctx, scope.Client, scope.VMwareCreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vCenter credentials")
	}
	_, finder, err := getFinderForVMwareCreds(ctx, scope.Client, scope.VMwareCreds, vmwarecreds.Datacenter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get finder for vCenter credentials")
	}
	clusterList, err := finder.ClusterComputeResourceList(ctx, "*")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, errors.Wrap(err, "failed to get cluster list")
	}

	for _, cluster := range clusterList {
		var clusterProperties mo.ClusterComputeResource
		err := cluster.Properties(ctx, cluster.Reference(), []string{"name"}, &clusterProperties)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get cluster properties")
		}

		hosts, err := cluster.Hosts(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get hosts")
		}
		var vmHosts []VMwareHostInfo
		for _, host := range hosts {
			hostSummary, err := GetESXiSummary(ctx, scope.Client, host.Name(), scope.VMwareCreds)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get ESXi summary")
			}
			vmHosts = append(vmHosts, VMwareHostInfo{Name: host.Name(), HardwareUUID: hostSummary.Summary.Hardware.Uuid})
		}
		clusters = append(clusters, VMwareClusterInfo{
			Name:  clusterProperties.Name,
			Hosts: vmHosts,
		})
	}
	return clusters, nil
}

// createVMwareHost creates a VMware host resource in Kubernetes
func createVMwareHost(ctx context.Context, scope *scope.VMwareCredsScope, host VMwareHostInfo, credName, clusterName, namespace string) (string, error) {
	hostk8sName, err := GetK8sCompatibleVMWareObjectName(host.Name, credName)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert host name to k8s name")
	}
	clusterk8sName, err := GetK8sCompatibleVMWareObjectName(clusterName, credName)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert cluster name to k8s name")
	}

	vmwareHost := vjailbreakv1alpha1.VMwareHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostk8sName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.VMwareClusterLabel: clusterk8sName,
				constants.VMwareCredsLabel:   credName,
			},
		},
		Spec: vjailbreakv1alpha1.VMwareHostSpec{
			Name:         host.Name,
			HardwareUUID: host.HardwareUUID,
			ClusterName:  clusterName,
		},
	}
	existingHost := vjailbreakv1alpha1.VMwareHost{}
	if err := scope.Client.Get(ctx, client.ObjectKey{Name: hostk8sName, Namespace: namespace}, &existingHost); err == nil {
		if existingHost.Spec.Name != host.Name || existingHost.Spec.HardwareUUID != host.HardwareUUID || existingHost.Spec.ClusterName != clusterName {
			existingHost.Spec = vmwareHost.Spec
			updateErr := scope.Client.Update(ctx, &existingHost)
			if updateErr != nil {
				return "", errors.Wrap(updateErr, "failed to update vmware host")
			}
		}
	} else {
		err = scope.Client.Create(ctx, &vmwareHost)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return "", errors.Wrap(err, "failed to create vmware host")
		}
	}

	return hostk8sName, nil
}

// createVMwareCluster creates a VMware cluster resource in Kubernetes
func createVMwareCluster(ctx context.Context, scope *scope.VMwareCredsScope, cluster VMwareClusterInfo) error {
	log := scope.Logger

	clusterk8sName, err := GetK8sCompatibleVMWareObjectName(cluster.Name, scope.Name())
	if err != nil {
		return errors.Wrap(err, "failed to convert cluster name to k8s name")
	}

	vmwareCluster := vjailbreakv1alpha1.VMwareCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterk8sName,
			Namespace: scope.Namespace(),
			Labels: map[string]string{
				constants.VMwareCredsLabel: scope.Name(),
			},
		},
		Spec: vjailbreakv1alpha1.VMwareClusterSpec{
			Name: cluster.Name,
		},
	}

	// Create hosts and collect their k8s names
	for _, host := range cluster.Hosts {
		log.Info("Processing VMware host", "host", host.Name)
		hostk8sName, err := createVMwareHost(ctx, scope, host, scope.Name(), cluster.Name, scope.Namespace())
		if err != nil {
			return err
		}
		vmwareCluster.Spec.Hosts = append(vmwareCluster.Spec.Hosts, hostk8sName)
	}

	// Create the cluster
	existingCluster := vjailbreakv1alpha1.VMwareCluster{}
	if err := scope.Client.Get(ctx, client.ObjectKey{Name: clusterk8sName, Namespace: scope.Namespace()}, &existingCluster); err == nil {
		if existingCluster.Spec.Name != cluster.Name {
			existingCluster.Spec = vmwareCluster.Spec
			updateErr := scope.Client.Update(ctx, &existingCluster)
			if updateErr != nil {
				return errors.Wrap(updateErr, "failed to update vmware cluster")
			}
		}
	} else {
		createErr := scope.Client.Create(ctx, &vmwareCluster)
		if createErr != nil && !apierrors.IsAlreadyExists(createErr) {
			return errors.Wrap(createErr, "failed to create vmware cluster")
		}
	}

	return nil
}

// CreateVMwareClustersAndHosts creates VMware clusters and hosts
func CreateVMwareClustersAndHosts(ctx context.Context, scope *scope.VMwareCredsScope) error {
	log := scope.Logger

	clusters, err := GetVMwareClustersAndHosts(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "failed to get clusters and hosts")
	}

	// Create a dummy cluster for standalone ESX
	if err := CreateDummyClusterForStandAloneESX(ctx, scope, clusters); err != nil {
		return errors.Wrap(err, "failed to create dummy cluster for standalone ESX")
	}

	for _, cluster := range clusters {
		log.Info("Processing VMware cluster", "cluster", cluster.Name)
		if err := createVMwareCluster(ctx, scope, cluster); err != nil {
			return err
		}
	}

	return nil
}

// DeleteStaleVMwareClustersAndHosts removes VMware cluster and host resources that no longer exist in vCenter.
// This helps maintain synchronization between Kubernetes resources and the actual infrastructure state.
func DeleteStaleVMwareClustersAndHosts(ctx context.Context, scope *scope.VMwareCredsScope) error {
	clusters, err := GetVMwareClustersAndHosts(ctx, scope)
	if err != nil {
		return errors.Wrap(err, "failed to get clusters and hosts")
	}

	// add entry for dummy cluster so save it from cleanup
	standAloneHosts, err := FetchStandAloneESXHostsFromVcenter(ctx, scope, clusters)
	if err != nil {
		return errors.Wrap(err, "failed to fetch standalone ESX hosts")
	}
	vmHosts := make([]VMwareHostInfo, 0, len(standAloneHosts))
	for _, host := range standAloneHosts {
		vmHosts = append(vmHosts, VMwareHostInfo{
			Name: host.Name(),
		})
	}
	clusters = append(clusters, VMwareClusterInfo{
		Name:  constants.VMwareClusterNameStandAloneESX,
		Hosts: vmHosts,
	})

	hosts := []VMwareHostInfo{}
	for _, cluster := range clusters {
		hosts = append(hosts, cluster.Hosts...)
	}

	existingClusters := vjailbreakv1alpha1.VMwareClusterList{}
	if err := scope.Client.List(ctx, &existingClusters, client.MatchingLabels{constants.VMwareCredsLabel: scope.Name()}); err != nil {
		return errors.Wrap(err, "failed to list vmware clusters")
	}

	existingHosts := vjailbreakv1alpha1.VMwareHostList{}
	if err := scope.Client.List(ctx, &existingHosts, client.MatchingLabels{constants.VMwareCredsLabel: scope.Name()}); err != nil {
		return errors.Wrap(err, "failed to list vmware hosts")
	}

	// Create a map of valid cluster names for O(1) lookups
	clusterNames := make(map[string]bool)
	for _, cluster := range clusters {
		cname, err := GetK8sCompatibleVMWareObjectName(cluster.Name, scope.Name())
		if err != nil {
			return errors.Wrap(err, "failed to convert cluster name to k8s name")
		}
		clusterNames[cname] = true
	}

	// Delete only clusters that don't exist in vSphere anymore
	for _, existingCluster := range existingClusters.Items {
		if !clusterNames[existingCluster.Name] {
			if err := scope.Client.Delete(ctx, &existingCluster); err != nil {
				return errors.Wrap(err, "failed to delete stale vmware cluster")
			}
		}
	}

	// Create a map of valid host names for O(1) lookups
	hostNames := make(map[string]bool)
	for _, host := range hosts {
		hname, err := GetK8sCompatibleVMWareObjectName(host.Name, scope.Name())
		if err != nil {
			return errors.Wrap(err, "failed to convert host name to k8s name")
		}
		hostNames[hname] = true
	}

	// Delete only hosts that don't exist in vSphere anymore
	for _, existingHost := range existingHosts.Items {
		if !hostNames[existingHost.Name] {
			if err := scope.Client.Delete(ctx, &existingHost); err != nil {
				return errors.Wrap(err, "failed to delete stale vmware host")
			}
		}
	}
	return nil
}

// FilterVMwareHostsForCluster returns a list of VMwareHost resources associated with the specified cluster
// It filters the hosts by the VMwareClusterLabel matching the provided cluster name
func FilterVMwareHostsForCluster(ctx context.Context, k8sClient client.Client, clusterName string) ([]vjailbreakv1alpha1.VMwareHost, error) {
	// List all VMwareHost resources
	vmwareHosts := &vjailbreakv1alpha1.VMwareHostList{}

	// Filter VMwareHost resources by cluster name
	if err := k8sClient.List(ctx, vmwareHosts, client.MatchingLabels{constants.VMwareClusterLabel: clusterName}); err != nil {
		return nil, errors.Wrap(err, "failed to list VMwareHost resources")
	}

	return vmwareHosts.Items, nil
}

// FetchStandAloneESXHostsFromVcenter fetches standalone ESX hosts from vCenter
func FetchStandAloneESXHostsFromVcenter(ctx context.Context, scope *scope.VMwareCredsScope, clusters []VMwareClusterInfo) ([]*object.HostSystem, error) {
	// Create a map of all hosts that are part of clusters for O(1) lookups
	clusteredHosts := make(map[string]bool)
	for _, cluster := range clusters {
		for _, host := range cluster.Hosts {
			clusteredHosts[host.Name] = true
		}
	}

	// Get VMware credentials to connect to vCenter
	vmwareCredsInfo, err := GetVMwareCredentialsFromSecret(ctx, scope.Client, scope.VMwareCreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vCenter credentials")
	}

	// Get finder for vCenter
	_, finder, err := getFinderForVMwareCreds(ctx, scope.Client, scope.VMwareCreds, vmwareCredsInfo.Datacenter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get finder for vCenter credentials")
	}
	hostList, err := finder.HostSystemList(ctx, "*")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, errors.Wrap(err, "failed to get host list")
	}
	vmHosts := make([]*object.HostSystem, 0, len(hostList))
	for _, host := range hostList {
		// if part of a cluster, skip
		if clusteredHosts[host.Name()] {
			continue
		}
		vmHosts = append(vmHosts, host)
	}
	return vmHosts, nil
}

// CreateDummyClusterForStandAloneESX creates a VMware cluster for standalone ESX
func CreateDummyClusterForStandAloneESX(ctx context.Context, scope *scope.VMwareCredsScope, existingClusters []VMwareClusterInfo) error {
	log := scope.Logger
	k8sClusterName, err := GetK8sCompatibleVMWareObjectName(constants.VMwareClusterNameStandAloneESX, scope.Name())
	if err != nil {
		return errors.Wrap(err, "failed to convert cluster name to k8s name")
	}
	vmwareCluster := vjailbreakv1alpha1.VMwareCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sClusterName,
			Namespace: constants.NamespaceMigrationSystem,
			Labels: map[string]string{
				constants.VMwareCredsLabel: scope.Name(),
			},
		},
		Spec: vjailbreakv1alpha1.VMwareClusterSpec{
			Name: constants.VMwareClusterNameStandAloneESX,
		},
	}

	standAloneHosts, err := FetchStandAloneESXHostsFromVcenter(ctx, scope, existingClusters)
	if err != nil {
		return errors.Wrap(err, "failed to fetch standalone ESX hosts")
	}

	// Create hosts and collect their k8s names
	for _, host := range standAloneHosts {
		log.Info("Processing VMware host", "host", host.Name)
		hostSummary, err := GetESXiSummary(ctx, scope.Client, host.Name(), scope.VMwareCreds)
		if err != nil {
			return errors.Wrap(err, "failed to get ESXi summary")
		}
		hostInfo := VMwareHostInfo{
			Name:         host.Name(),
			HardwareUUID: hostSummary.Summary.Hardware.Uuid,
		}
		hostk8sName, err := createVMwareHost(ctx, scope, hostInfo, scope.Name(), constants.VMwareClusterNameStandAloneESX, constants.NamespaceMigrationSystem)
		if err != nil {
			return err
		}
		vmwareCluster.Spec.Hosts = append(vmwareCluster.Spec.Hosts, hostk8sName)
	}

	if err := scope.Client.Create(ctx, &vmwareCluster); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create VMware cluster")
	}
	return nil
}
