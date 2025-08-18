---
title: Credential Management
description: Overview of Credential Management
---

Before you start using vJailbreak, you need to provide credentials for both the VMware vCenter and OpenStack/PCD environments.

### VMware vCenter Credentials
VMware vCenter credentials are required to connect to the vCenter server and retrieve information about the virtual machines you want to migrate.

The credentials should have enough permissions to retrieve information about the virtual machines you want to migrate and if you are looking for cluster conversion, the credentials should have enough permissions to retrieve information about the cluster, put host into maintenance mode, etc (see [Cluster Conversion](../../guides/cluster-conversion/) for more details).

The VMware credentials needs vCenter Server IP address or vCenter Server name, username and password.
The credentials also take the Datacenter name and the VMs, Hosts being worked on would be restricted to the Datacenter specified in the credentials.

Some VMware environments may be using self signed certificates, in such cases, you would need to "Allow insecure connection" option in the credentials.

### OpenStack/PCD Credentials
OpenStack/PCD credentials are required to create VMs inside the OpenStack/PCD environment. The credentials are supplied via the `openstack.rc` file that is available in the PCD environment.

To copy the content of the `openstack.rc` file, you should navigate to Settings > API Access > pcdctl RC section.

If using PCD we recommend toggling the "Is PCD credentials" option. This will automatically indicate to vJailbreak that the credentials are for PCD and would use PCD Cluster as a destination for different migrations.

For non PCD environment the `openstack.rc` file will be available as part of various distribution and documentation. The `openstack.rc` file is typically used for any automation with the OpenStack CLI.

Here is an example of the `openstack.rc` file:

```bash
export OS_USERNAME=<your-username>
export OS_PASSWORD=<your-password>
export OS_AUTH_URL=https://<fqdn of the openstack/pcd>/keystone/v3
export OS_AUTH_TYPE=password
export OS_IDENTITY_API_VERSION=3
export OS_REGION_NAME=region-1
export OS_USER_DOMAIN_NAME=Default
export OS_PROJECT_DOMAIN_NAME=Default
export OS_PROJECT_NAME=service
export OS_INTERFACE=public
```

:::note
The `openstack.rc` must contain both the `Domain` and the `Project`/`Tenant` information. When using the OpenStack credentials, the `Domain` and `Project`/`Tenant` information is used as the destination `domain` and `project`/`tenant` for the OpenStack/PCD environment.
:::
