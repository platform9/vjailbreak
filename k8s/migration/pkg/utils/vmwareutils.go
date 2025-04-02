package utils

import (
	"context"

	"github.com/pkg/errors"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/k8s/migration/pkg/constants"
	scope "github.com/platform9/vjailbreak/k8s/migration/pkg/scope"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type VMwareHostInfo struct {
	Name string
}

type VMwareClusterInfo struct {
	Name  string
	Hosts []VMwareHostInfo
}

func GetVMwareClustersAndHosts(ctx context.Context, k3sclient client.Client, scope *scope.VMwareCredsScope) ([]VMwareClusterInfo, error) {
	var clusters []VMwareClusterInfo
	vmwarecreds, err := GetVMwareCredentials(ctx, k3sclient, scope.VMwareCreds.Spec.SecretRef.Name)
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
			vmHosts = append(vmHosts, VMwareHostInfo{Name: host.Name()})
		}
		clusters = append(clusters, VMwareClusterInfo{
			Name:  clusterProperties.Name,
			Hosts: vmHosts,
		})
	}
	return clusters, nil
}

func CreateVMwareClustersAndHosts(ctx context.Context, k3sclient client.Client, scope *scope.VMwareCredsScope) error {

	clusters, err := GetVMwareClustersAndHosts(ctx, k3sclient, scope)
	if err != nil {
		return errors.Wrap(err, "failed to get clusters and hosts")
	}
	for _, cluster := range clusters {
		clusterk8sName, err := ConvertToK8sName(cluster.Name)
		if err != nil {
			return errors.Wrap(err, "failed to convert cluster name to k8s name")
		}
		vmwareCluster := vjailbreakv1alpha1.VMwareCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterk8sName,
				Namespace: scope.Namespace(),
			},
			Spec: vjailbreakv1alpha1.VMwareClusterSpec{
				Name:  cluster.Name,
				Hosts: []string{},
			},
		}
		for _, host := range cluster.Hosts {
			hostk8sName, err := ConvertToK8sName(host.Name)
			if err != nil {
				return errors.Wrap(err, "failed to convert host name to k8s name")
			}
			vmwareHost := vjailbreakv1alpha1.VMwareHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hostk8sName,
					Namespace: scope.Namespace(),
					Labels: map[string]string{
						constants.VMwareClusterLabel: clusterk8sName,
					},
				},
				Spec: vjailbreakv1alpha1.VMwareHostSpec{
					Name: host.Name,
				},
			}
			vmwareCluster.Spec.Hosts = append(vmwareCluster.Spec.Hosts, vmwareHost.Name)
			_, err = controllerutil.CreateOrUpdate(ctx, scope.Client, &vmwareHost, func() error {
				return nil
			})
			if err != nil {
				return errors.Wrap(err, "failed to create vmware host")
			}
		}
		_, err = controllerutil.CreateOrUpdate(ctx, scope.Client, &vmwareCluster, func() error {
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "failed to create vmware cluster")
		}
	}
	return nil
}
