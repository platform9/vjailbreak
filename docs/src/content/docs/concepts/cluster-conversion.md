---
title: Cluster Conversion
description: Overview of the Cluster Conversion Process
---

vJailbreak offers two primary migration options:

## VM Migration
This is the default option, where a user selects a set of VMs within VMware, across clusters, and migrates them. This keeps
the source VMs intact and creates a new copy of them in OpenStack/PCD.

## Cluster Conversion
This is the second option, where a user selects a cluster within VMware and migrates it to a PCD cluster. This converts not only the VMs within the cluster but also the individual ESXi hosts into PCD hypervisors.
This is done through 'evacuating' the ESXi host and converting it into a PCD hypervisor through IPMI/PXE. In the current version of vJailbreak, Canonical MAAS is used to achieve that.

See below the animation of the cluster conversion process.

![Cluster conversion](/vjailbreak/images/vjb-cluster-conversion.gif)

Choose a VMware cluster and select the destination cluster in PCD. Once the clusters are selected, you will be shown the VMware ESXi hosts and the corresponding VMs. The VM portion of the wizard is similar to, or the same as, that in the [VM Migration](#vm-migration) section.

The ESXi portion of the wizard is different from the VM portion and deals with options on how each server will be configured as a hypervisor in the PCD cluster. The most important aspect is the Host configuration; the information is pulled from the PCD cluster blueprint. The host config determines what NICs are associated with what networks. See [PCD cluster blueprint docs](https://platform9.com/docs/private-cloud-director/private-cloud-director/virtualized-cluster-blueprint) for more details.

The process of converting the ESXi hosts into PCD hypervisors is simple: each ESXi host is put into maintenance mode, which migrates all the running VMs into other ESXi hosts. Then the ESXi host is converted into a PCD hypervisor and the VMs are migrated into the PCD hypervisor. This process is repeated for all the ESXi hosts in the cluster.

The detailed configuration steps are described in the [cluster conversion guide](../../guides/cluster-conversion/cluster-conversion/).

See below the diagram of the cluster conversion setup.

![Cluster conversion](/vjailbreak/images/cluster-conversion-2.png)


:::note
The conversion of ESXi into a PCD hypervisor is a one-time operation and cannot be undone. All the VMs are converted into PCD VMs. This requires a fully automated DRS cluster configuration on the VMware side.
:::