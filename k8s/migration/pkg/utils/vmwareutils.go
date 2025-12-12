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
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetVMwareClustersAndHosts retrieves a list of all available VMware clusters and their hosts
func GetVMwareClustersAndHosts(ctx context.Context, scope *scope.VMwareCredsScope) ([]VMwareClusterInfo, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Getting VMware clusters and hosts", "vmwarecreds", scope.VMwareCreds.Name)

	_, finder, err := getFinderForVMwareCreds(ctx, scope.Client, scope.VMwareCreds, scope.VMwareCreds.Spec.DataCenter)
	if err != nil {
		ctxlog.Error(err, "Failed to get finder for VCenter credentials")
		return nil, errors.Wrap(err, "failed to get VMware finder")
	}

	// If no datacenter is specified, search across all datacenters
	if scope.VMwareCreds.Spec.DataCenter == "" {
		return getClustersFromAllDatacenters(ctx, finder)
	}

	clusterList, err := finder.ClusterComputeResourceList(ctx, "*")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, errors.Wrap(err, "failed to get cluster list")
	}

	// Pre-allocate clusters slice with initial capacity
	clusters := make([]VMwareClusterInfo, 0, len(clusterList))

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
			Name:       clusterProperties.Name,
			Hosts:      vmHosts,
			Datacenter: scope.VMwareCreds.Spec.DataCenter,
		})
	}
	return clusters, nil
}

// getClustersFromAllDatacenters fetches clusters from all datacenters when no specific datacenter is provided
func getClustersFromAllDatacenters(ctx context.Context, finder *find.Finder) ([]VMwareClusterInfo, error) {
	ctxlog := log.FromContext(ctx)
	ctxlog.Info("Fetching clusters from all datacenters")

	var allClusters []VMwareClusterInfo

	// Get all datacenters
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to list datacenters")
	}

	// Iterate through each datacenter and get clusters
	for _, dc := range datacenters {
		finder.SetDatacenter(dc)
		
		clusterList, err := finder.ClusterComputeResourceList(ctx, "*")
		if err != nil && !strings.Contains(err.Error(), "not found") {
			ctxlog.Error(err, "Failed to get clusters from datacenter", "datacenter", dc.Name())
			continue
		}

		for _, cluster := range clusterList {
			var clusterProperties mo.ClusterComputeResource
			err := cluster.Properties(ctx, cluster.Reference(), []string{"name"}, &clusterProperties)
			if err != nil {
				ctxlog.Error(err, "Failed to get cluster properties", "cluster", cluster.Name())
				continue
			}

			hosts, err := cluster.Hosts(ctx)
			if err != nil {
				ctxlog.Error(err, "Failed to get hosts for cluster", "cluster", cluster.Name())
				continue
			}

			var vmHosts []VMwareHostInfo
			for _, host := range hosts {
				vmHosts = append(vmHosts, VMwareHostInfo{Name: host.Name()})
			}

			allClusters = append(allClusters, VMwareClusterInfo{
				Name:       clusterProperties.Name,
				Hosts:      vmHosts,
				Datacenter: dc.Name(),
			})
		}
	}

	return allClusters, nil
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

	labels := map[string]string{
		constants.VMwareCredsLabel: scope.Name(),
	}
	annotations := map[string]string{}
	if cluster.Datacenter != "" {
		annotations[constants.VMwareDatacenterLabel] = cluster.Datacenter
	}

	vmwareCluster := vjailbreakv1alpha1.VMwareCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clusterk8sName,
			Namespace:   scope.Namespace(),
			Labels:      labels,
			Annotations: annotations,
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

	// No need to add dummy clusters to the list - they are preserved by checking spec.name

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
	// Preserve all NO CLUSTER dummy clusters (they have spec.name == VMwareClusterNameStandAloneESX)
	for _, existingCluster := range existingClusters.Items {
		// Skip deletion of NO CLUSTER dummies
		if existingCluster.Spec.Name == constants.VMwareClusterNameStandAloneESX {
			continue
		}
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

	var hostList []*object.HostSystem

	// If no specific datacenter is provided, collect standalone hosts from all datacenters
	if vmwareCredsInfo.Datacenter == "" {
		datacenters, err := finder.DatacenterList(ctx, "*")
		if err != nil {
			return nil, errors.Wrap(err, "failed to list datacenters for standalone ESX discovery")
		}
		for _, dc := range datacenters {
			finder.SetDatacenter(dc)
			dcHostList, err := finder.HostSystemList(ctx, "*")
			if err != nil && !strings.Contains(err.Error(), "not found") {
				return nil, errors.Wrap(err, "failed to get host list")
			}
			hostList = append(hostList, dcHostList...)
		}
	} else {
		hostList, err = finder.HostSystemList(ctx, "*")
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return nil, errors.Wrap(err, "failed to get host list")
		}
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

// CreateDummyClusterForStandAloneESX creates VMware cluster(s) for standalone ESX hosts
// If datacenter is specified in creds, creates one NO CLUSTER for that datacenter
// If datacenter is empty, creates one NO CLUSTER per datacenter with standalone hosts
func CreateDummyClusterForStandAloneESX(ctx context.Context, scope *scope.VMwareCredsScope, existingClusters []VMwareClusterInfo) error {
	log := scope.Logger

	// Get VMware credentials to check if datacenter is specified
	vmwareCredsInfo, err := GetVMwareCredentialsFromSecret(ctx, scope.Client, scope.VMwareCreds.Spec.SecretRef.Name)
	if err != nil {
		return errors.Wrap(err, "failed to get vCenter credentials")
	}

	standAloneHosts, err := FetchStandAloneESXHostsFromVcenter(ctx, scope, existingClusters)
	if err != nil {
		return errors.Wrap(err, "failed to fetch standalone ESX hosts")
	}

	// If no standalone hosts, still create empty NO CLUSTER for consistency
	if len(standAloneHosts) == 0 {
		// Determine datacenter(s) to create NO CLUSTER for
		if vmwareCredsInfo.Datacenter != "" {
			// Single datacenter case
			return createDummyClusterForDatacenter(ctx, scope, vmwareCredsInfo.Datacenter, nil, log)
		}
		// Multi-datacenter case: create NO CLUSTER for each datacenter that has real clusters
		datacenters := make(map[string]bool)
		for _, cluster := range existingClusters {
			if cluster.Datacenter != "" {
				datacenters[cluster.Datacenter] = true
			}
		}
		for dc := range datacenters {
			if err := createDummyClusterForDatacenter(ctx, scope, dc, nil, log); err != nil {
				return err
			}
		}
		return nil
	}

	// Group standalone hosts by datacenter
	hostsByDatacenter := make(map[string][]*object.HostSystem)
	if vmwareCredsInfo.Datacenter != "" {
		// Single datacenter: all hosts belong to it
		hostsByDatacenter[vmwareCredsInfo.Datacenter] = standAloneHosts
	} else {
		// Multi-datacenter: need to determine each host's datacenter
		// Get finder to query host datacenter
		_, finder, err := getFinderForVMwareCreds(ctx, scope.Client, scope.VMwareCreds, "")
		if err != nil {
			return errors.Wrap(err, "failed to get finder for vCenter credentials")
		}

		datacenters, err := finder.DatacenterList(ctx, "*")
		if err != nil {
			return errors.Wrap(err, "failed to list datacenters")
		}

		// For each host, find which datacenter it belongs to
		for _, host := range standAloneHosts {
			hostName := host.Name()
			found := false
			for _, dc := range datacenters {
				finder.SetDatacenter(dc)
				dcHosts, err := finder.HostSystemList(ctx, "*")
				if err != nil {
					continue
				}
				for _, dcHost := range dcHosts {
					if dcHost.Name() == hostName {
						dcName := dc.Name()
						hostsByDatacenter[dcName] = append(hostsByDatacenter[dcName], host)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}

	// Create one NO CLUSTER per datacenter
	for datacenter, hosts := range hostsByDatacenter {
		if err := createDummyClusterForDatacenter(ctx, scope, datacenter, hosts, log); err != nil {
			return err
		}
	}

	return nil
}

// createDummyClusterForDatacenter creates a single NO CLUSTER for a specific datacenter
func createDummyClusterForDatacenter(ctx context.Context, scope *scope.VMwareCredsScope, datacenter string, hosts []*object.HostSystem, log logr.Logger) error {
	// Create unique k8s name for this NO CLUSTER by including datacenter
	clusterNameWithDC := constants.VMwareClusterNameStandAloneESX
	if datacenter != "" {
		clusterNameWithDC = fmt.Sprintf("%s-%s", constants.VMwareClusterNameStandAloneESX, datacenter)
	}
	k8sClusterName, err := GetK8sCompatibleVMWareObjectName(clusterNameWithDC, scope.Name())
	if err != nil {
		return errors.Wrap(err, "failed to convert cluster name to k8s name")
	}

	labels := map[string]string{
		constants.VMwareCredsLabel: scope.Name(),
	}
	annotations := map[string]string{}
	if datacenter != "" {
		annotations[constants.VMwareDatacenterLabel] = datacenter
	}

	vmwareCluster := vjailbreakv1alpha1.VMwareCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        k8sClusterName,
			Namespace:   constants.NamespaceMigrationSystem,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: vjailbreakv1alpha1.VMwareClusterSpec{
			Name: constants.VMwareClusterNameStandAloneESX,
		},
	}

	// Create hosts and collect their k8s names
	for _, host := range hosts {
		log.Info("Processing VMware host", "host", host.Name, "datacenter", datacenter)
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
