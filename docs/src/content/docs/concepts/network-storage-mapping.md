---
title: Network & Storage Mapping
description: Overview of Network and Storage Mapping
---

A large scale VMWare migration may require a large number of VMs to be migrated. In such cases, it is recommended to use network and storage mapping to optimize the migration process and keep both the environments running at the same time while migration is progressing. Network and Storage mapping are part of the migration wizard and are required for the migration to proceed.

## Network Mapping

vJailbreak recognizes the different types of networks in VMware and OpenStack/PCD environments.
We recommend to create the OpenStack/PCD networks in advance such that some multi-VM applications can continue to run while the migration is in progress.

:::note
If you enable **Persist source network interfaces**, network persistence may not work for cross network migration and will be blocked in such cases. Read more in [Migration Options](../migration-options/#persist-source-network-interfaces).
:::

### VMware Network Types
For VMware environment, the networks are typically of type  `vSphere Standard Port Group` or `vSphere Distributed Port Group`. Currently, vJailbreak supports only these two types of networks and not `NSX` created networks.

Typical VMware networks use VLAn configuration to define the network properties.

### OpenStack/PCD Network Types
For OpenStack/PCD environment, there are many more choices, refer to the PCD and OpenStack documentation on the various provider, physical and virtual networks.

vJailbreak expects the user to create the OpenStack/PCD networks in advance. In a typical environment for each VMware network an OpenStack/PCD physical network is created that use the corresponding VLAN of the Port Group or Distributed Port Group.

## Storage Mapping

Unlike network mapping, storage mapping is different. For networking, interconnectivity is the key, for storage it is not. During migration vJailbreak 'copies' the data over from VMware to OpenStack/PCD and this can be used to your advantage as needed.

### VMware Storage Types

For VMware environment, the storage is typically of type `vSphere Datastore` with either `VMFS` or `NFS` as the storage type. vJailbreak supports both of these types of storage.

There are a few exceptions and unsupported configurations.

* vJailbreak does not support the `vCenter Storage Policies` or 'vVols' at the time of writing this document.
* vJailbreak does not support the `RDM` (Raw Disk Mapping) feature of vCenter either. We plan to support this in the near future.
* vJailbreak doesn't support snapshot preservation during the copy.

### OpenStack/PCD Volume Types
For OpenStack/PCD environment, the volumes are created using 'Cinder' and can be of type `NFS` `iSCSI`, `FiberChannel`. vJailbreak supports all of these types of storage with the help of the corresponding OpenStack/PCD Cinder drivers. The Cinder and corresponding volume types must be precreated before the migration starts.

Since the migration involves copying the data from VMware to OpenStack/PCD, the storage types can be of different types, for example `NFS` datastore volume can be copied to `iSCSI` volume in OpenStack/PCD.

## Storage-Accelerated Copy

For environments where both VMware and OpenStack share the same storage array (Pure Storage or NetApp), vJailbreak supports **Storage-Accelerated Copy**. This method leverages storage array-level XCOPY operations to dramatically improve migration performance by offloading the data copy to the storage array itself.

Instead of copying data over the network (limited to ~1 Gbps per VMDK), Storage-Accelerated Copy performs the copy directly on the storage array at array speeds, which can be considerably faster than normal copy.


See [Storage-Accelerated Copy](../storage-accelerated-copy/) for detailed configuration and usage instructions.

:::note
Migration involves copying of the data from VMware to PCD, depending on the bandwidth and the network congestion, the migration can take a long time and needs more resources on the vJailbreak VM. See [scaling guide](../../guides/how-to/scaling/) for parallel migrations.
:::