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

## Scaling Up in L2-Only Networks

In L2-only network environments (networks without DHCP), agent nodes follow a similar startup process to the primary vJailbreak VM. The agent VM boots, waits for network configuration, and then joins the master node once an IP is assigned.

### Agent Startup Sequence in L2-Only Networks

#### Step 1: Agent VM Boot and Wait State

When you scale up agents in an L2-only network, each new agent VM will:

1. Boot and initialize basic services
2. Set the default password for the `ubuntu` user
3. Enter a waiting state for network availability

The agent console will display messages similar to:
```
[2026-03-17 10:07:59] IS_MASTER: false
[2026-03-17 10:07:59] MASTER_IP: 10.96.9.100
[2026-03-17 10:07:59] K3S_TOKEN: <token>
[2026-03-17 10:07:59] Setting default password for ubuntu user...
[2026-03-17 10:07:59] Default password set for ubuntu user. User will need to change it on first login
[2026-03-17 10:07:59] Waiting for network availability...
[2026-03-17 10:07:59] Waiting for network: missing default route and global IPv4 address...
[2026-03-17 10:08:59] Waiting for network: missing default route and global IPv4 address...
```

The agent waits until both an IP address and default route are configured before proceeding.

#### Step 2: Assign IP Address to Agent

Access the agent VM console and configure networking using one of these methods:

**Option A: Using DHCP Client**
```bash
# Request IP via DHCP
dhclient ens3

# Verify IP assignment
ip a
```

**Option B: Static IP Configuration**
```bash
# Assign static IP (use appropriate values for your network)
sudo ip addr add 192.168.1.101/24 dev ens3

# Configure default gateway
sudo ip route add default via 192.168.1.1

# Verify configuration
ip a
ip route
```

:::caution
Ensure each agent VM receives a **unique IP address** on the same network as the primary vJailbreak VM. The agent must be able to reach the master node's IP address.
:::

#### Step 3: Agent Joins Master Node

Once the network is configured, the agent automatically:

1. Detects the network availability
2. Connects to the master vJailbreak VM using the pre-configured `MASTER_IP` and `K3S_TOKEN`
3. Joins the K3s cluster as a worker node
4. Syncs VDDK libraries from the master node
5. Becomes available for scheduling migrations

Monitor the agent's progress:
```bash
tail -f /var/log/pf9-install.log
```

Expected output after IP assignment:
```
[2026-03-17 11:34:03] Network detected. Default route and global IPv4 address available.
[2026-03-17 11:34:03] K3S_URL: https://<master-ip>:6443
[2026-03-17 11:34:03] K3S_TOKEN: 
[2026-03-17 11:34:03] INSTALL_K3S_EXEC: 
[2026-03-17 11:34:27] K3s worker node is ready (containerd is responsive).
[2026-03-17 11:34:27] Loading all the images in /etc/pf9/images...
[2026-03-17 11:35:35] K3s worker setup completed.
[2026-03-17 11:35:55] removing the cron job
```

#### Step 4: Verify Agent Status

After the agent joins, verify its status from the primary vJailbreak VM:

```bash
# SSH to the primary vJailbreak VM
kubectl get vjailbreaknodes -n migration-system 
NAME                      PHASE   VMIP
vjailbreak-agent-tj158j   Ready   <ip>
vjailbreak-master         Ready   <ip>

# Expected output shows the new agent
NAME                    STATUS   ROLES                  AGE   VERSION
vjailbreak-master       Ready    control-plane,master   1d    v1.28.x
vjailbreak-agent-001    Ready    <none>                 5m    v1.28.x
```

The agent will also appear in the vJailbreak dashboard under the **Agents** tab with a "Ready" status.

### Scaling Multiple Agents in L2-Only Networks

When scaling up multiple agents simultaneously in L2-only networks:

1. **Scale up from the dashboard** - Create the desired number of agent VMs
2. **Configure each agent sequentially** - Access each agent's console and assign unique IP addresses
3. **Agents join independently** - Each agent joins the master as soon as its network is configured

:::tip
For efficiency, prepare a list of IP addresses before scaling up. This allows you to quickly configure each agent without delays.
:::

### Troubleshooting Agent Scale-Up in L2-Only Networks

| Issue | Solution |
|-------|----------|
| Agent not joining master | Verify agent can ping the master IP; check firewall rules |
| "Connection refused" errors | Ensure K3s is running on master; check port 6443 accessibility |
| Agent stuck in "NotReady" state | Check `/var/log/pf9-install.log` on the agent for errors |
| VDDK sync failed | Verify rsync daemon is running on master (`systemctl status rsyncd`) |

**Verify Network Connectivity from Agent:**
```bash
# Test connectivity to master
ping <master-ip>

# Test K3s API port
nc -zv <master-ip> 6443
```

## Logging into Agent VMs

Agent VMs use the same login process as the primary vJailbreak VM:
- **Username**: `ubuntu`
- **Default Password**: `password`
- On first login, you will be prompted to change the password immediately. 

:::note
VDDK libraries are automatically synced from the primary vJailbreak VM to all agent nodes. You only need to upload VDDK to the primary vJailbreak VM.
:::
