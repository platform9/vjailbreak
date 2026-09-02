---
title: vJailbreak Accelerated Copy
description: High-performance VM migration using a Proxy VM for direct disk attachment and NBD-based data transfer
---

vJailbreak Accelerated Copy is an advanced data copy method that attaches source VM disks directly to a dedicated Proxy VM and streams the data over NBD (Network Block Device) to the destination. Instead of copying data over the NFC protocol from ESXi, this method leverages vCenter's disk-attach capability to transfer data at near-disk speeds without requiring shared storage arrays.

> **Underlying feature:** vJailbreak Accelerated Copy is powered by VMware's **hot-add** disk transport mechanism to attach source disks to the Proxy VM.

:::danger[Cold migration only: hot data copy is not supported]
vJailbreak Accelerated Copy **does not support live (hot) migration**. The source VM is powered off
before its disks are attached to the Proxy VM. You must select **"Power off VMs, then copy"** as
the Data Copy Method when using vJailbreak Accelerated Copy. Attempting to use it with
**"Copy live VMs, then power off"** is not supported.
:::

:::note[VDDK not required]
As of v0.4.10, vJailbreak Accelerated Copy does not require VDDK at any stage. Data is transferred
directly via the hot-add disk transport and NBD streaming. VDDK is never used. This makes it the
recommended copy method when VMware's VDDK download pages are unavailable.
:::

## Overview

### How It Works

Traditional vJailbreak migrations copy VM disk data from VMware ESXi hosts to PCD Cinder volumes over the network using the NFC (Network File Copy) protocol, limited to approximately **1 Gbps per VMDK**.

vJailbreak Accelerated Copy bypasses this limitation by:

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
- **Open ports**: The Proxy VM must accept inbound TCP from the vJailbreak VM on **22** (SSH) and **10809–11808** (`qemu-nbd`, one port per disk copied in parallel)
- **disk.EnableUUID**: Must be set to `TRUE` on the Proxy VM in vCenter
- **PVSCSI controller**: The Proxy VM's first SCSI controller (**SCSI controller 0**) must be of type **VMware Paravirtual (PVSCSI)**
- **Datastore accessibility**: The HotAdd proxy must have access to the same datastore as the target virtual machine, and the VMFS version and data block sizes for the target VM must be the same as the datastore where the HotAdd proxy resides.
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
- The Proxy VM's **SCSI controller 0** must be of type **VMware Paravirtual (PVSCSI)**
- vCenter must allow disk attach/detach operations on the Proxy VM
- The Proxy VM must be powered on and reachable over SSH
- The Proxy VM must be on the same datastore as the source VM's disks, with a matching VMFS version and block size (see **Datastore accessibility** under [Requirements](#requirements))


## Setting Up the Proxy VM

### Option A: Deploy from the vJailbreak UI (Easiest)

vJailbreak can deploy and register the Proxy VM in a single step directly from the UI:

1. Navigate to **vJailbreak Accelerated Copy** in the left sidebar
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

### Configure the SCSI Controller Type on the Proxy VM

Source disks are attached to the Proxy VM's first SCSI controller, and vJailbreak can only match them to block devices when that controller is **VMware Paravirtual**:

1. Power off the Proxy VM
2. In vSphere Client, right-click the Proxy VM and select **Edit Settings**
3. Under **Virtual Hardware**, locate **SCSI controller 0**
4. Set **Change Type** to **VMware Paravirtual**
5. Click **OK** and power the VM back on

:::caution
Other controller types — including **LSI Logic SAS**, **LSI Logic Parallel**, and **BusLogic Parallel** — are not supported. Migrations using a Proxy VM without PVSCSI on SCSI controller 0 fail with `could not identify block device for disk <uuid>`. See [Proxy VM Must Use a PVSCSI Controller](../../reference/known-limitations/#proxy-vm-must-use-a-pvscsi-controller).
:::

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

1. In the vJailbreak UI, navigate to **vJailbreak Accelerated Copy** in the left sidebar
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

## Using vJailbreak Accelerated Copy in a Migration

### Step 1: Create a Migration

1. Navigate to the **Migrations** page and click **New Migration**
2. Fill out the migration form with source VM and target configuration
3. For the **Data Copy Method**, select **vJailbreak Accelerated Copy**

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

When vJailbreak Accelerated Copy is selected, the migration follows this workflow:

```
┌─────────────────────────────────────────────────────────────────┐
│                vJailbreak Accelerated Copy Flow                 │
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
- **PVSCSI controller only**: The Proxy VM's first SCSI controller (**SCSI controller 0**) must be **VMware Paravirtual (PVSCSI)**. Disk UUID matching does not work on other controller types, and migrations fail with `could not identify block device for disk <uuid>`. See [Configure the SCSI Controller Type](#configure-the-scsi-controller-type-on-the-proxy-vm).
- **Concurrent disk attach can fail**: When several migrations reach the disk-attach step at the same time on the same Proxy VM, vCenter may reject some of the simultaneous reconfigure tasks and those migrations fail. This is a transient race — the migrations that attached first continue normally, and the failed ones succeed on retry. Stagger migration start times or spread migrations across multiple Proxy VMs to reduce the chance of it happening.
- **Maximum 60 disks per Proxy VM (including its own boot disk)**: vSphere allows at most **60** virtual disks per VM (4 SCSI controllers × 15 disks). The Proxy VM's own boot disk counts toward this total, so the constraint is **Proxy VM boot disk + attached source disks ≤ 60** — a Proxy VM with a single boot disk can have up to **59** source disks attached at any one time. This is a shared ceiling across **all** migrations using the same Proxy VM concurrently, not a per-migration limit. To migrate more disks in parallel, register additional Proxy VMs and distribute migrations across them.

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
3. Ensure the NBD ports (TCP **10809–11808**, one per disk copied in parallel) are not blocked by a firewall between the Proxy VM and vJailbreak
4. Confirm the Proxy VM's guest IP is correct (VMware Tools must be running)

### Block Device Not Found in Proxy VM

```
Error: could not identify block device for disk <uuid>
```

**Resolution:**
1. Verify `disk.EnableUUID = TRUE` is set on the Proxy VM (this is the most common cause)
2. Verify the Proxy VM's **SCSI controller 0** is of type **VMware Paravirtual** — no other controller type is supported. See [Configure the SCSI Controller Type](#configure-the-scsi-controller-type-on-the-proxy-vm)
3. Confirm the disk was actually attached — check vCenter → Proxy VM → Edit Settings → Hard Disks
4. SSH into the Proxy VM and run `lsblk` to list visible block devices
5. Check vCenter events for disk attach errors on the Proxy VM

### Disk Attach Fails When Several Migrations Start Together

**Symptoms:** A batch of migrations is started at once and some of them fail early with a vCenter error while attaching disks to the Proxy VM. The remaining migrations proceed into the copy phase normally.

**Cause:** vCenter does not always handle simultaneous VM reconfigure (disk attach) tasks on the same Proxy VM gracefully, so some attach requests are rejected. This is a transient race condition, not a misconfiguration.

**Resolution:**
1. Wait until the surviving migrations have entered the copy phase
2. [Retry](../../guides/how-to/retry_failed_migration/) the failed migrations — they normally succeed on the second attempt
3. To reduce the chance of the race, stagger migration start times instead of starting a large batch at once, or register additional Proxy VMs and distribute migrations across them

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
