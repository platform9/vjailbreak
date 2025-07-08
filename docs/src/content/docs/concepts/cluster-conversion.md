---
title: Cluster Conversion
description: Overview of Cluster Conversion Process
---

vJailbreak offers two primary migration options

## VM Migration
The default option, where a user selects a set of VMs within VMware, across clusters and migrates them. This keeps
the source VM intact and creates a new copy of them into OpenStack/PCD.

## Cluster Conversion
The second option, where a user selects a cluster within VMware and migrates it to a PCD cluster. This converts not only the VMs within the cluster but also the individual ESXi into PCD Hypervisor.

![Cluster Conversion](/vjailbreak/images/cluster-conversion-1.png)

As indicated in the diagram, the cluster conversion requires you to setup a Metal As a Service (MAAS from Canonical) as a way to convert the ESXi into PCD Hypervisor using IPMI and PXE/iPXE.

The configuration steps are described in [cluster conversion guide](../../guides/how-to/cluster-conversion/).

Choose a VMware cluster and select the destination cluster in PCD. Once the clusters are selected, you will be shown the VMware ESXi hosts and the corresponding VMs. The VM portion of the wizard is similar/same as that in the [VM Migration](#vm-migration) section.

The ESXi portion of the wizard is different from the VM portion and deals with options on how each server will be configured as a hypervisor in the PCD cluster. The most important aspect is the Host configuration, the information is pulled from the PCD cluster blueprint. The host config determines what NICs are associated with what networks. See [PCD cluster blueprint docs](https://platform9.com/docs/private-cloud-director/private-cloud-director/virtualized-cluster-blueprint) for more details.

The process of converting the ESXi into PCD hypervisor is simple, each ESXi host is put into maintenance mode which migrates all the running VMs into other ESXi hosts. Then the ESXi host is converted into PCD hypervisor and the VMs are migrated into the PCD hypervisor. This process is repeated for all the ESXi hosts in the cluster.

:::note
The conversion of ESXi into PCD hypervisor is a one time operation and cannot be undone. All the VMs are converted into PCD VMs and requires a fully DRS cluster configuration on the VMware side.
:::