---
title: Hot-Add Proxy Migration
description: High-performance VM migration using a Proxy VM for direct disk attachment and NBD-based data transfer
---

Hot-Add Proxy is an advanced data copy method that attaches source VM disks directly to a dedicated Proxy VM and streams the data over NBD (Network Block Device) to the destination. Instead of copying data over the NFC protocol from ESXi, this method leverages vCenter's disk-attach capability to transfer data at near-disk speeds without requiring shared storage arrays.

## Overview

### How It Works

Traditional vJailbreak migrations copy VM disk data from VMware ESXi hosts to PCD Cinder volumes over the network using the NFC (Network File Copy) protocol, limited to approximately **1 Gbps per VMDK**.

Hot-Add Proxy bypasses this limitation by:

1. Powering off the source VM and then taking a snapshot
2. Attaching the frozen snapshot disks directly to a Proxy VM running in vCenter
3. Identifying each disk as a block device inside the Proxy VM using disk UUID matching
4. Exposing each disk as an NBD resource on the Proxy VM via `qemu-nbd`
5. Running `nbdcopy` on the vJailbreak VM to pull data from the Proxy VM directly to the destination Cinder volume

### Benefits

- **Faster migrations**: Direct block-device access avoids NFC protocol overhead
- **No shared storage required**: Works with any datastore — NFS, VMFS, vSAN
- **Lower ESXi host load**: Data is streamed from the Proxy VM, not the ESXi NFC daemon

### Requirements

- **Proxy VM**: A Linux VM running in the same vCenter with `qemu-nbd` and `openssh-server` installed
- **SSH access**: vJailbreak must be able to SSH into the Proxy VM as root
- **disk.EnableUUID**: Must be set to `TRUE` on the Proxy VM in vCenter
- **vCenter permissions**: Sufficient permissions to snapshot VMs and attach/detach disks

## Prerequisites

### 1. Proxy VM Requirements

The Proxy VM must have the following utilities installed and running:

| Utility | Purpose |
|---------|---------|
| `openssh-server` | SSH connectivity for vJailbreak to control the Proxy VM |
| `qemu-nbd` | Expose attached block devices as NBD resources |

The Proxy VM must be a **Linux-based OS** (recommended: Ubuntu, Alpine, or Debian) with **root SSH access** enabled.

### 2. vCenter Requirements

- The Proxy VM must have **disk.EnableUUID = TRUE** set in vCenter VM settings
- vCenter must allow disk attach/detach operations on the Proxy VM
- The Proxy VM must be powered on and reachable over SSH


## Setting Up the Proxy VM

### Option A: Deploy from the vJailbreak UI (Easiest)

vJailbreak can deploy and register the Proxy VM in a single step directly from the UI:

1. Navigate to **Hot-Add Proxy** in the left sidebar
2. Click **Add Proxy VM** and select **Deploy a new vJailbreak Proxy VM**
3. Select your VMware credentials and fill in the deployment target (datacenter, datastore, network, and optionally a cluster or host)
4. Enter a unique VM name and click **Deploy & Register VM**

vJailbreak will import the pre-configured OVA into vCenter, power the VM on, generate and inject an SSH key pair automatically, and register the Proxy VM — no manual key setup required. The VM appears in the list with status **Deploying** and transitions to **Ready** once verification completes (typically 3–5 minutes).

:::caution
The OVA image uses default credentials (`root` / `password`) for the initial SSH key injection step. Change the root password on the VM after deployment in production environments.
:::

:::caution[ESXi/vCenter version requirement]
The bundled OVA uses virtual hardware version **vmx-21**, which requires **ESXi 8.0 U2 (vCenter 8.0 U2) or newer**. Deploying it to an older host fails with an "unsupported hardware family" error. On ESXi/vCenter 7.x, use **Option B** to register a manually created Linux VM instead.
:::

### Option B: Register an Existing Linux VM

Any Linux VM can serve as the Proxy VM provided it meets the requirements. Install the necessary utilities:

**Ubuntu / Debian:**
```bash
sudo apt update
sudo apt install -y openssh-server qemu-utils
```

**Alpine:**
```bash
apk update
apk add openssh qemu-nbd
```

:::note
Root access is required for SSH and for running `qemu-nbd` commands on the Proxy VM. Ensure `PermitRootLogin yes` is set in `/etc/ssh/sshd_config` if root SSH is not already enabled.
:::

### Configure disk.EnableUUID on the Proxy VM

This setting is required for vJailbreak to match attached disks to their block devices inside the Proxy VM:

1. In vSphere Client, right-click the Proxy VM and select **Edit Settings**
2. Click **VM Options** → **Advanced** → **Edit Configuration**
3. Find or add the key `disk.EnableUUID` and set the value to `TRUE`
4. Click **OK** and restart the VM if it was already running

## SSH Key Configuration

:::note
This section applies to **Option B** (registering an existing VM). When using Option A (UI-based OVA deploy), SSH keys are generated and injected automatically — no manual key steps are needed.
:::

When registering an existing VM, vJailbreak needs SSH access to the Proxy VM. The UI offers two ways to provide the key pair:

### Sub-option 1: Let vJailbreak Generate the Key Pair

1. In the **Add Proxy VM** drawer, select **Register an existing VM**
2. Select your VMware credentials and the VM
3. Under **SSH Access**, choose **Generate Key Pair** and click **Generate**
4. The UI displays the public key — copy it and add it to the Proxy VM's `authorized_keys`:
   ```bash
   # On the Proxy VM (as root)
   echo "<paste public key here>" >> ~/.ssh/authorized_keys
   chmod 600 ~/.ssh/authorized_keys
   ```
5. Click **Register**

vJailbreak stores the generated private key as a Kubernetes secret automatically.

### Sub-option 2: Upload Your Own Private Key

If you have an existing key pair already configured on the VM:

1. Under **SSH Access**, choose **Upload Private Key**
2. Upload the private key file or paste its contents into the text area
3. Confirm the corresponding public key is already in the VM's `authorized_keys`
4. Click **Register**

vJailbreak stores the uploaded private key as a Kubernetes secret and uses it during verification and migration.

#### Generating a Key Pair Manually

If you prefer to generate the key pair yourself outside of vJailbreak:

```bash
ssh-keygen -t rsa -b 4096 -f proxy_vm_key -N ""
```

This produces two files:
- `proxy_vm_key` — private key (upload this into vJailbreak)
- `proxy_vm_key.pub` — public key (add this to the Proxy VM)

On the Proxy VM, append the public key to root's `authorized_keys`:

```bash
# On the Proxy VM (as root)
mkdir -p ~/.ssh
cat >> ~/.ssh/authorized_keys << 'EOF'
<contents of proxy_vm_key.pub>
EOF
chmod 600 ~/.ssh/authorized_keys
```

If you have temporary password SSH access, you can use `ssh-copy-id` from your workstation as a shortcut:

```bash
ssh-copy-id -i proxy_vm_key.pub root@<proxy-vm-ip>
```

:::note
**SSH key requirements:**
- No passphrase — vJailbreak uses the key non-interactively
- Any standard PEM format is accepted: RSA, EC, PKCS#8, or OpenSSH. PuTTY `.ppk` format is not supported
- `PermitRootLogin` must be `yes` or `prohibit-password` in `/etc/ssh/sshd_config` on the Proxy VM
:::

## Registering the Proxy VM in vJailbreak

Once the Proxy VM is set up and the SSH key is ready:

1. In the vJailbreak UI, navigate to **Hot-Add Proxy** in the left sidebar
2. Click **Add Proxy VM**
3. Fill in the form:
   - **Name**: A unique identifier for this Proxy VM
   - **VM Name**: The exact VM name as it appears in vCenter
   - **VMware Credentials**: Select the VMware credentials that can see this VM
   - **SSH Private Key**: Paste the contents of your private key file (e.g., `~/.ssh/proxy_vm_key`)
4. Click **Add**

vJailbreak will verify the Proxy VM by:
- Confirming the VM exists in vCenter
- Retrieving the guest IP via VMware Tools
- Checking `disk.EnableUUID = TRUE` — if not set, vJailbreak will automatically enable it and reboot the Proxy VM, so onboarding may take longer than usual
- Establishing an SSH connection
- Verifying `qemu-nbd` is available

The Proxy VM status will update to **Ready** once all checks pass. Any failed checks are reported with a specific error message in the UI.

:::tip
If verification fails, address the reported issue (e.g., install missing utilities, fix SSH access) and click **Retry** to re-run the validation without re-entering the form.
:::

## Using Hot-Add in a Migration

### Step 1: Create a Migration

1. Navigate to the **Migrations** page and click **New Migration**
2. Fill out the migration form with source VM and target configuration
3. For the **Data Copy Method**, select **Hot Add**

### Step 2: Select Proxy VM

4. A **Proxy VM** dropdown appears — select a Proxy VM in **Ready** state
5. The UI will only show Proxy VMs that are verified and ready

### Step 3: Start the Migration

6. Review **Advanced Options** if needed (network/storage mappings)
7. Click **Start Migration**

:::note
The selected Proxy VM must be in **Ready** state before the migration can proceed. If no Proxy VM is ready, register and verify one first.
:::

## Migration Workflow

When Hot-Add is selected, the migration follows this workflow:

```
┌─────────────────────────────────────────────────────────────────┐
│                      Hot-Add Proxy Copy Flow                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Validate Prerequisites                                      │
│     ├── Proxy VM is in Ready state                              │
│     ├── SSH connectivity to Proxy VM                            │
│     └── qemu-nbd available on Proxy VM                          │
│                                                                 │
│  2. Provision Destination Resources (standard workflow)         │
│     ├── Create Cinder volumes in PCD                            │
│     └── Attach destination disks to vJailbreak VM              │
│                                                                 │
│  3. Power Off Source VM, then Take Snapshot                     │
│                                                                 │
│  4. For Each Source Disk:                                       │
│     ├── Attach frozen snapshot disk to Proxy VM                 │
│     ├── Identify block device via disk UUID matching            │
│     ├── Find a free port on the Proxy VM                        │
│     ├── Expose disk as NBD via qemu-nbd on that port            │
│     ├── Run nbdcopy on vJailbreak VM to destination disk        │
│     └── Detach and clean up disk from Proxy VM                  │
│                                                                 │
│  5. Remove Source VM Snapshot                                   │
│                                                                 │
│  6. Disk Conversion (same as normal migration)                  │
│                                                                 │
│  7. Create Target VM in PCD (standard post-copy flow)           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Limitations

- **Cold copy only**: The source VM is powered off before disk attachment — live (hot) copy of the running VM's active disks is not supported
- **Same vCenter**: Proxy VM and source VM must be managed by the same vCenter instance
- **VMware Tools required**: The Proxy VM must have VMware Tools running so vJailbreak can retrieve its guest IP

## Troubleshooting

### Proxy VM Verification Failed

**Symptoms:** Proxy VM stuck in `Pending` or `Failed` state with a validation error.

**Resolution by error:**

| Error | Resolution |
|-------|-----------|
| VM not found in vCenter | Verify the VM name exactly matches the vCenter inventory name |
| Guest IP not available | Ensure VMware Tools is installed and running on the Proxy VM |
| `disk.EnableUUID` not set | Set `disk.EnableUUID = TRUE` in VM advanced settings and reboot |
| SSH connection refused | Verify `sshd` is running and port 22 is reachable from vJailbreak |
| `qemu-nbd` not found | Install `qemu-utils` (Ubuntu) or `qemu-nbd` (Alpine) on the Proxy VM |

### NBD Connection Failed During Copy

```
Error: failed to connect to NBD endpoint on proxy VM
```

**Resolution:**
1. Verify the Proxy VM is still running and SSH is accessible
2. Check that `qemu-nbd` started successfully — review v2v helper logs
3. Ensure the NBD port (typically in the 10809+ range) is not blocked by a firewall between the Proxy VM and vJailbreak
4. Confirm the Proxy VM's guest IP is correct (VMware Tools must be running)

### Block Device Not Found in Proxy VM

```
Error: could not identify block device for disk <uuid>
```

**Resolution:**
1. Verify `disk.EnableUUID = TRUE` is set on the Proxy VM (this is the most common cause)
2. Confirm the disk was actually attached — check vCenter → Proxy VM → Edit Settings → Hard Disks
3. SSH into the Proxy VM and run `lsblk` to list visible block devices
4. Check vCenter events for disk attach errors on the Proxy VM

### Snapshot Creation Failed

```
Error: failed to create snapshot on source VM
```

**Resolution:**
1. Verify the VMware credentials have snapshot creation permissions
2. Check if a snapshot with the same name already exists on the source VM — remove stale `vjailbreak-*` snapshots
3. Ensure the source VM's datastore has sufficient free space for the snapshot delta files

### Migration Stuck After Snapshot

If the migration is stuck after taking the snapshot and the source VM remains powered off:

1. Check v2v helper logs for the last successful phase
2. If the Proxy VM became unavailable, the migration will not auto-recover — clean up manually:
   ```bash
   # Remove the snapshot from vCenter
   govc snapshot.remove -vm "<source-vm>" "vjailbreak-hotadd-snap"
   ```
3. Detach any disks vJailbreak attached to the Proxy VM before retrying

## Best Practices

1. **Dedicate the Proxy VM**: Avoid running other workloads on the Proxy VM during migrations to ensure stable performance
2. **Match network placement**: Place the Proxy VM on a network with low latency to the vJailbreak VM for fast NBD transfers
3. **Verify before migrating**: Always confirm the Proxy VM shows **Ready** status before starting a migration
4. **Monitor disk space**: Snapshot delta files consume datastore space — ensure the source VM's datastore has at least 20% free space
5. **Use the recommended OVA**: The pre-built OVA is tested and configured correctly; custom VMs require manual validation of all prerequisites
6. **Rotate SSH keys**: Use a dedicated key pair for vJailbreak and rotate it periodically
