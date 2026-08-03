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

## Scaling in L2-Only Networks

Agent scale-up **is supported** in L2-only network environments — networks that have no OpenStack-managed DHCP/IPAM and instead rely on an external DHCP server on the segment.

### How it works

1. **L2 detection.** vJailbreak inspects the networks attached to the primary vJailbreak VM and treats any network tagged `simple_network` in Neutron as an L2-only network.
2. **Port pre-creation.** For each L2 network, vJailbreak pre-creates a Neutron port with no fixed IP and port security disabled, and attaches the new agent VM to that port instead of requesting an address from OpenStack.
3. **Config drive.** The agent VM is booted with a config drive, because the OpenStack metadata service at `169.254.169.254` is not reachable before the guest has an IP. The join configuration (master IP and cluster token) is delivered through the config drive to `/etc/pf9/k3s.env`.
4. **Wait for the guest to get an IP.** On first boot, the agent's provisioning script waits until the guest has **both** a global IPv4 address and a default route — normally handed out by the external DHCP server on the L2 segment. The check is retried every 60 seconds and does not time out, so the agent will keep waiting until the lease is granted.
5. **Agent addition and join.** Once the guest has an IP and a default route, the agent-addition sequence runs and the node joins the vJailbreak master at `https://<master-ip>:6443`. Pre-baked container images are then imported locally, so no registry access is required.
6. **Status convergence.** The agent's `VjailbreakNode` progresses `CreatingVM` → `VMCreated` → `Ready`. Because OpenStack reports no address for a port without a fixed IP, the agent's IP is blank in the dashboard while it is provisioning and is populated once the node reports `Ready`.

### Requirements

- The L2 network must be **tagged `simple_network`** in Neutron. This tag is what triggers the L2 code path.
- An **external DHCP server (or equivalent address source) must be reachable on the L2 segment**, and it must provide an IPv4 address, a **default route**, and DNS/routing such that the vJailbreak master is reachable on port `6443`. An address alone is not sufficient — the default route is also required.
- **Do not select security groups** when scaling up on an L2 network. L2 ports are created with port security disabled and any security group selection is ignored; the dashboard disables the selector for these networks.
- The hypervisor and image must support **config drive**.
- **IPv4 only.** IPv6-only L2 segments are not supported.

:::note
If an agent stays in `VMCreated` and never becomes `Ready`, the guest most likely never received a DHCP lease or a default route. Open the agent VM console and check `/var/log/pf9-install.log` — the wait loop logs which of the two conditions is still missing.
:::

## Logging into Agent VMs

Agent VMs use the same login process as the primary vJailbreak VM:
- **Username**: `ubuntu`
- **Default Password**: `password`
- On first login, you will be prompted to change the password immediately. 

:::note
VDDK libraries are automatically synced from the primary vJailbreak VM to all agent nodes. You only need to upload VDDK to the primary vJailbreak VM.
:::
