---
title: Scale vJailbreak 
description: You can scale up vJailbreak to perform more parallel migrations
---

vJailbreak can be scaled to perform multiple migrations in parallel by deploying additional `agents`, enabling greater efficiency and workload distribution.

Additional agents can be created in the Agents tab of the vJailbreak dashboard using the "Scale Up" button. You will need to choose the destination OpenStack credentials, the size of the agent VM(s), and the number of agent nodes up to a maximum of 5 per scale up. Additional agent nodes can be scaled up in batches of 5, providing the flexibility to change agent VM sizes to help with throttling network traffic.

:::caution
It is entirely possible to fully saturate a 10Gb network with many parallel migrations!
:::

## Agent Node Sizing and Migration Capacity

:::caution
The sizing recommendations below apply to **agent nodes only**. The primary vJailbreak VM hosts additional services (controller, UI, Prometheus, Grafana, etc.) in addition to running migrations, and therefore requires separate capacity planning with additional overhead. Agent nodes are dedicated worker nodes that primarily run migration pods.
:::

Each migration running on an agent node consumes the following resources:

| Resource | Request | Limit |
|----------|---------|-------|
| CPU | 1 core | 2 cores |
| Memory | 1 GiB | 3 GiB |
| Ephemeral Storage | 3 GiB | 3 GiB |

### Calculating Concurrent Migrations per Agent

The number of concurrent migrations an agent node can handle depends on its available resources. While Kubernetes uses **resource requests** for scheduling decisions, the actual resource consumption during migration is closer to the **limits**. Therefore, consider both when planning capacity:

**Scheduling Capacity (based on requests):**
- Maximum Concurrent Migrations = min(Available CPU / 1 core, Available Memory / 1 GiB, Available Storage / 3 GiB)

**Actual Runtime Capacity (based on limits):**
- Maximum Concurrent Migrations = min(Available CPU / 2 cores, Available Memory / 3 GiB, Available Storage / 3 GiB)

For safe capacity planning, use the **limits-based calculation** to ensure migrations have sufficient resources during peak usage.

### Recommended Agent Flavors

Below are recommended OpenStack flavors for agent nodes based on desired migration capacity. Reserve approximately **20-25% of resources** for system overhead (OS, K3s, monitoring, etc.):

| Agent Flavor | vCPUs | RAM | Storage | Concurrent Migrations (per agent) | Use Case |
|--------------|-------|-----|---------|-----------------------------------|----------|
| **Small** | 8 | 16 GiB | 60 GiB | 2-3 | Small-scale migrations, testing |
| **Medium** | 16 | 32 GiB | 100 GiB | 5-7 | Standard production workloads |
| **Large** | 32 | 64 GiB | 200 GiB | 10-14 | High-throughput migrations |
| **X-Large** | 48 | 96 GiB | 300 GiB | 15-21 | Maximum parallel migrations |

:::note
Agent nodes require a **minimum of 60 GiB disk storage**. Flavors with less than 60 GiB are not supported.
:::

**Example Calculation for Medium Flavor (16 vCPU, 32 GiB RAM):**
- Available CPU after overhead: ~12 cores → 12 / 2 = 6 migrations
- Available Memory after overhead: ~24 GiB → 24 / 3 = 8 migrations
- **Effective capacity: 6 concurrent migrations** (limited by CPU)

### Best Practices

- **Network bandwidth** is often the bottleneck. Monitor network utilization and adjust agent count/size accordingly.
- **Storage I/O** on the agent node should be sufficient for temporary disk operations during migration.
- Start with **Medium** flavors and scale up based on observed resource utilization and network capacity.
- Distribute migrations across multiple smaller agents rather than one large agent for better fault tolerance.
- Monitor agent resource usage via the vJailbreak dashboard or Prometheus metrics to optimize sizing.

Agent nodes can be scaled down by selecting the agent and using the "Scale Down" button.

## Logging into Agent VMs

Agent VMs use the same login process as the primary vJailbreak VM:
- **Username**: `ubuntu`
- **Default Password**: `password`
- On first login, you will be prompted to change the password immediately. 

:::note
VDDK libraries are automatically synced from the primary vJailbreak VM to all agent nodes. You only need to upload VDDK to the primary vJailbreak VM.
:::

:::note 
The following instructions apply to versions of vJailbreak older than v0.1.7
:::

Each agent must also have a copy of the VMware VDDK libraries in their `/home/ubuntu` directories.
- Copy the latest version of the [VDDK libraries](https://developer.broadcom.com/sdks/vmware-virtual-disk-development-kit-vddk/8.0) for Linux into `/home/ubuntu` of the new agents. Untar it to a folder name `vmware-vix-disklib-distrib` in `/home/ubuntu` directory.