---
title: Credential Management
description: Overview of Credential Management
---

Before you start using vJailbreak, you need to provide credentials for both the VMware vCenter and OpenStack/PCD environments.

### VMware vCenter Credentials
VMware vCenter credentials are required to connect to the vCenter server and retrieve information about the virtual machines you want to migrate.

The credentials should have enough permissions to retrieve information about the virtual machines you want to migrate and if you are looking to the cluster conversion, the credentials should have enough permissions to retrieve information about the cluster, put host into maintenance mode, etc (see [Cluster Conversion](../../guides/cluster-conversion/) for more details).

The VMware credentials needs vCenter Server IP address or vCenter Server name, username and password.
The credentials also take the Datacenter name and the VMs, Hosts being worked on would be restricted to the Datacenter specified in the credentials.

Some VMware environments may be using self signed certificates, in such cases, you would need to "Allow insecure connection" option in the credentials.

### OpenStack/PCD Credentials
OpenStack/PCD credentials are required to create VMs inside the OpenStack/PCD environment. The credentials are supplied via the `openstack.rc` file that is available in the PCD environment.

To copy the content of the `openstack.rc` file, you should navigate to Settings > API Access > pcdctl RC section.

If using PCD we recommend toggling the "Is PCD credentials" option. This will automatically indicate to vJailbreak that the credentials are for PCD and would use PCD Cluster as a destination for different migrations.

For non PCD environment the `openstack.rc` file will be available as part of various distribution and documentation. The `openstack.rc` file is typically used for any automation with the OpenStack CLI.

#### Required Variables

vJailbreak requires the following environment variables to be present in your admin RC file. **All of these variables are mandatory** and the migration will fail if any are missing:

| Variable | Description | Example |
|----------|-------------|---------|
| `OS_AUTH_URL` | OpenStack Keystone authentication URL | `https://keystone.example.com:5000/v3` |
| `OS_USERNAME` | OpenStack username with admin privileges | `admin` |
| `OS_PASSWORD` | Password for the OpenStack user | `your-secure-password` |
| `OS_REGION_NAME` | OpenStack region where VMs will be created | `RegionOne` |
| `OS_PROJECT_NAME` | OpenStack project name for VM deployment | `service` |
| `OS_PROJECT_DOMAIN_NAME` | OpenStack project domain name | `Default` |
| `OS_AUTH_TYPE` | OpenStack authentication type | `password` |
| `OS_IDENTITY_API_VERSION` | OpenStack identity API version | `3` |
| `OS_USER_DOMAIN_NAME` | OpenStack user domain name | `Default` |
| `OS_INTERFACE` | OpenStack API interface type | `public` |

#### Optional Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OS_INSECURE` | Skip SSL certificate verification | `true` or `false` |

#### User Permissions

The user specified in `OS_USERNAME` must have administrative privileges in OpenStack to:
- Create and manage virtual machines
- Access network and storage resources  
- Create and manage volumes
- Access compute, network, and storage services

The project specified in `OS_PROJECT_NAME` must exist and have sufficient quotas for the VMs being migrated.

#### Example Admin RC File

Here is an example of the `openstack.rc` file with all variables:

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
export OS_INSECURE=false
```

:::note
The `openstack.rc` must contain both the `Domain` and the `Project`/`Tenant` information. When using the OpenStack credentials, the `Domain` and `Project`/`Tenant` information is used as the destination `domain` and `project`/`tenant` for the OpenStack/PCD environment.
:::
