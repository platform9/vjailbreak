package utils

import (
	"context"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VMwareHostInfo represents a host in a VMware cluster.
// It contains essential information about a VMware ESXi host.
type VMwareHostInfo struct {
	// Name is the fully qualified domain name or IP address of the host
	Name string
	// HardwareUUID is the unique identifier of the host
	HardwareUUID string
}

// VMwareClusterInfo represents a cluster in a VMware environment.
// It contains information about a VMware cluster and its associated hosts.
type VMwareClusterInfo struct {
	// Name is the unique identifier of the cluster
	Name string
	// Hosts is a list of ESXi hosts that are part of this cluster
	Hosts []VMwareHostInfo
}

// GetVMwareClustersAndHosts retrieves a list of all available VMware clusters and their hosts
func GetVMwareClustersAndHosts(ctx context.Context, k3sclient client.Client, scope *scope.VMwareCredsScope) ([]VMwareClusterInfo, error) {
	// Pre-allocate clusters slice with initial capacity
	clusters := make([]VMwareClusterInfo, 0, 4)
	vmwarecreds, err := GetVMwareCredentialsFromSecret(ctx, k3sclient, scope.VMwareCreds.Spec.SecretRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get vCenter credentials")
	}
	c, err := ValidateVMwareCreds(ctx, k3sclient, scope.VMwareCreds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate vCenter connection")
	}
	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, vmwarecreds.Datacenter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find datacenter")
	}
	finder.SetDatacenter(dc)
	clusterList, err := finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
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
			hostSummary, err := GetESXiSummary(ctx, k3sclient, host.Name(), scope.VMwareCreds)
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
func createVMwareHost(ctx context.Context, k3sclient client.Client, host VMwareHostInfo, credName, clusterName, namespace string) (string, error) {
	hostk8sName, err := ConvertToK8sName(host.Name)
	if err != nil {
		return "", errors.Wrap(err, "failed to convert host name to k8s name")
	}

	vmwareHost := vjailbreakv1alpha1.VMwareHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostk8sName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.VMwareClusterLabel: clusterName,
				constants.VMwareCredsLabel:   credName,
			},
		},
		Spec: vjailbreakv1alpha1.VMwareHostSpec{
			Name:         host.Name,
			HardwareUUID: host.HardwareUUID,
		},
	}

	err = k3sclient.Create(ctx, &vmwareHost)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", errors.Wrap(err, "failed to create vmware host")
	}

	return hostk8sName, nil
}

// createVMwareCluster creates a VMware cluster resource in Kubernetes
func createVMwareCluster(ctx context.Context, k3sclient client.Client, cluster VMwareClusterInfo, scope *scope.VMwareCredsScope) error {
	clusterk8sName, err := ConvertToK8sName(cluster.Name)
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
			Name:  cluster.Name,
			Hosts: []string{},
		},
	}

	// Create hosts and collect their k8s names
	for _, host := range cluster.Hosts {
		hostk8sName, err := createVMwareHost(ctx, k3sclient, host, scope.Name(), cluster.Name, scope.Namespace())
		if err != nil {
			return err
		}
		vmwareCluster.Spec.Hosts = append(vmwareCluster.Spec.Hosts, hostk8sName)
	}

	// Create the cluster
	err = k3sclient.Create(ctx, &vmwareCluster)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create vmware cluster")
	}

	return nil
}

// CreateVMwareClustersAndHosts creates VMware clusters and hosts
func CreateVMwareClustersAndHosts(ctx context.Context, k3sclient client.Client, scope *scope.VMwareCredsScope) error {
	clusters, err := GetVMwareClustersAndHosts(ctx, k3sclient, scope)
	if err != nil {
		return errors.Wrap(err, "failed to get clusters and hosts")
	}

	for _, cluster := range clusters {
		if err := createVMwareCluster(ctx, k3sclient, cluster, scope); err != nil {
			return err
		}
	}
	return nil
}
