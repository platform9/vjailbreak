---
title: Storage Accelerated Copy
description: High-performance VM migration using storage array level XCOPY operations
---

Storage Accelerated Copy is an advanced data copy method that leverages storage array level XCOPY operations to dramatically improve migration performance. Instead of copying data over the network via the traditional NBD/NFC protocol, this method offloads the data copy to the storage array itself, achieving significantly faster transfer speeds.

## Overview

### How It Works

Traditional vJailbreak migrations copy VM disk data from VMware ESXi hosts to PCD Cinder volumes over the network using the NFC (Network File Copy) protocol. This approach is limited to approximately **1 Gbps per VMDK** due to VMware's NFC protocol constraints.

Storage-Accelerated Copy bypasses this limitation by:

1. Creating a target volume directly on the storage array
2. Importing the volume into PCD Cinder
3. Mapping the volume to the ESXi host
4. Using ESXi's `vmkfstools` to perform an XCOPY clone operation directly on the storage array
5. The storage array handles the data copy internally.

### Benefits

- **Dramatically faster migrations**: Array-level copy operations are significantly faster than network-based transfers
- **Reduced network load**: Data doesn't traverse the network between VMware and PCD.
- **Lower ESXi host CPU usage**: The storage array handles the heavy lifting
- **Ideal for large VMs**: Especially beneficial for VMs with large disks (hundreds of GB to TB)

### Requirements

- **Supported storage arrays**: Pure Storage FlashArray or NetApp ONTAP
- **Shared storage**: Both VMware datastores and PCD must be backed by the same storage array.
- **ESXi SSH access**: SSH access to ESXi hosts with root privileges
- **iSCSI connectivity**: ESXi hosts must have iSCSI initiators configured to the storage array

## Supported Storage Arrays

| Vendor | Product | 
|--------|---------|
| Pure Storage | FlashArray | 
| NetApp   | ONTAP    | 

:::note
Additional storage vendors may be added in future releases. The storage SDK is designed to be extensible.
:::

## Prerequisites

Before using Storage-Accelerated Copy, ensure the following prerequisites are met:

### 1. Storage Array Configuration

- Storage array must be accessible from both VMware ESXi hosts and PCD compute nodes
- VMware datastores must be VMFS volumes backed by LUNs on the supported storage array
- PCD Cinder must be configured with a backend driver for the same storage array
- Cinder volume types must be created and mapped to the storage array backend

### 2. ESXi SSH Access

Storage-Accelerated Copy requires SSH access to ESXi hosts to execute `vmkfstools` commands. Follow these steps to set up SSH access:

#### Step 1: Enable SSH on ESXi Hosts

**Option A: Using vSphere Client (GUI)**

1. Log in to vSphere Client
2. Navigate to the ESXi host
3. Click on the **Configure** tab
4. Under **System**, select **Services**
5. Find **SSH** in the list of services
6. Right-click on **SSH** and select **Start**
7. (Optional) Right-click again and select **Policy** → **Start and stop with host** to enable SSH automatically on boot

**Option B: Using ESXi Host Client (Direct)**

1. Log in to the ESXi host directly via web browser: `https://<esxi-host-ip>`
2. Navigate to **Host** → **Actions** → **Services** → **Enable Secure Shell (SSH)**

**Option C: Using ESXi Shell (Console)**

1. Access the ESXi host console (physical or via iLO/iDRAC)
2. Press `F2` to customize system/view logs
3. Log in with root credentials
4. Navigate to **Troubleshooting Options**
5. Select **Enable SSH**
6. Press `Enter` to confirm

#### Step 2: Generate SSH Key Pair

On your workstation or the vJailbreak VM, generate an SSH key pair:

```bash
# Generate RSA key pair (recommended for ESXi compatibility)
ssh-keygen -t rsa -b 4096 -f ~/.ssh/esxi_migration_key -C "vjailbreak-migration"

# When prompted:
# - Enter passphrase: Leave empty (press Enter) for passwordless authentication
# - Confirm passphrase: Press Enter again
```

This will create two files:
- `~/.ssh/esxi_migration_key` - Private key (keep this secure)
- `~/.ssh/esxi_migration_key.pub` - Public key (to be copied to ESXi)

:::tip
**Alternative: Ed25519 Keys (Modern ESXi versions)**

For ESXi 7.0 and later, you can use Ed25519 keys which are more secure and faster:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/esxi_migration_key -C "vjailbreak-migration"
```
:::

#### Step 3: Copy Public Key to ESXi Hosts

**Option A: Using ssh-copy-id (Easiest)**

```bash
# Copy public key to ESXi host
ssh-copy-id -i ~/.ssh/esxi_migration_key.pub root@<esxi-host-ip>

# Enter the root password when prompted
```

**Option B: Manual Copy**

If `ssh-copy-id` is not available:

```bash
# Display the public key
cat ~/.ssh/esxi_migration_key.pub

# SSH into the ESXi host
ssh root@<esxi-host-ip>

# On the ESXi host, add the public key to authorized_keys
cat >> /etc/ssh/keys-root/authorized_keys << 'EOF'
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC... vjailbreak-migration
EOF

# Set correct permissions
chmod 600 /etc/ssh/keys-root/authorized_keys
```

#### Step 4: Test SSH Connection

Verify passwordless SSH access works:

```bash
# Test SSH connection (should not prompt for password)
ssh -i ~/.ssh/esxi_migration_key root@<esxi-host-ip> 

```

If successful, you should be able to login to esxi.
#### Step 5: Configure SSH Key in vJailbreak

1. Navigate to **Storage Management** page
2. At the top, find the **ESXi SSH Key** section
3. Click **Configure**
4. Get your private key contents:
   ```bash
   cat ~/.ssh/esxi_migration_key
   ```
5. Copy the entire output (including `-----BEGIN OPENSSH PRIVATE KEY-----` and `-----END OPENSSH PRIVATE KEY-----`)
6. Paste into the UI textarea
7. Click **Save**

#### Step 6: Repeat for All ESXi Hosts

Repeat Steps 3-4 for **all ESXi hosts** in your vCenter cluster that host VMs you plan to migrate. The same SSH key pair can be used for all hosts.

```bash
# Example: Copy to multiple hosts
for host in esxi-host1.example.com esxi-host2.example.com esxi-host3.example.com; do
  echo "Configuring $host..."
  ssh-copy-id -i ~/.ssh/esxi_migration_key.pub root@$host
done
```

:::caution
**Security Best Practices:**
- Use a dedicated SSH key pair for vJailbreak migrations (don't reuse existing keys)
- Store the private key securely and restrict access
- Consider disabling SSH on ESXi hosts after migrations are complete
- Use SSH key passphrases in production environments (requires additional configuration)
- Regularly rotate SSH keys
- Monitor SSH access logs on ESXi hosts
:::

:::note
**Troubleshooting SSH Issues:**

If SSH connection fails:
1. Verify SSH service is running: `ssh root@<esxi-host> "/etc/init.d/ssh status"`
2. Check firewall rules: ESXi firewall must allow SSH (port 22)
3. Verify authorized_keys permissions: Should be `600` or `400`
4. Check ESXi logs: `/var/log/auth.log` for authentication errors
5. Ensure the private key format is correct (OpenSSH format, not PuTTY format)
:::

### 3. Network Connectivity

| Source | Destination | Port | Protocol | Purpose |
|--------|-------------|------|----------|---------|
| vJailbreak | ESXi hosts | 22 | TCP | SSH for vmkfstools commands |
| ESXi hosts | Storage array | 3260 | TCP | iSCSI (if using iSCSI) |
| ESXi hosts | Storage array | Various | FC | Fibre Channel (if using FC) |
| vJailbreak | Storage array | 443 | TCP | Storage array API |

## Configuration

### Understanding Auto-Discovery

When you add PCD credentials to vJailbreak, the system automatically discovers all storage volume backends configured in your PCD environment. For each detected storage backend (NetApp, Pure Storage, etc.), vJailbreak creates a placeholder ArrayCreds entry with status "Auto-discovered" and credentials marked as "Pending".

#### How Auto-Discovery Works

1. **PCD Configuration**: In PCD, you configure multiple storage volume backends under "Persistent Storage Connectivity" (Cluster Blueprint → Storage). Each volume backend represents a storage array with its driver type (NetApp Data ONTAP, Pure Storage iSCSI, NFS, etc.).

2. **Backend Detection**: When PCD credentials are added to vJailbreak, the system queries the Cinder API to discover all configured volume backends and their properties:
   - Volume Type (e.g., `netapp`, `vt-pure-iscsi`)
   - Backend Name (e.g., `netapp`, `pure-iscsi-1`)
   - Driver Type (e.g., `NetApp Data ONTAP`, `Pure Storage iSCSI`)
   - Cinder Host string

3. **Placeholder Creation**: For each discovered backend, vJailbreak automatically creates an ArrayCreds resource with:
   - **Name**: Derived from the volume type and backend name (e.g., `netapp-netapp`, `vt-pure-iscsi-pure-iscsi-1`)
   - **Vendor**: Automatically identified from the driver type
   - **Source**: Marked as "Auto-discovered"
   - **Credentials**: Status shows "Pending" (requires user input)
   - **PCD Mapping**: Pre-populated with volume type, backend name, and Cinder host

4. **User Completion**: Users then update these auto-discovered entries with the actual storage array credentials (hostname, username, password) to enable Storage Accelerated Copy.

#### Storage Management Page

The Storage Management page displays all auto-discovered storage backends:

| Column | Description |
|--------|-------------|
| **Name** | Auto-generated name based on volume type and backend |
| **Vendor** | Storage array vendor (NetApp Storage, Pure Storage, N/A) |
| **Volume Type** | Cinder volume type name |
| **Backend Name** | Cinder backend name from configuration |
| **Source** | "Auto-discovered" for automatically detected backends |
| **Credentials** | "Pending" until user provides array credentials |
| **Actions** | Edit (to add credentials) and Delete |

![Storage Management Page](../../../assets/storagemanagementpage.png)

#### Example: PCD with Multiple Storage Backends

In PCD, storage backends are configured in the **Cluster Blueprint** under **Persistent Storage Connectivity**. Each volume backend can have multiple configurations, and each configuration represents a connection to a storage array.

![PCD Cluster Blueprint - Storage Configuration](../../../assets/clusterblueprint.png)

In the example above, PCD has three volume backends configured:
1. **nfs** - NFS backend with driver "NFS"
2. **netapp** - NetApp backend with driver "NetApp Data ONTAP"
3. **vt-pure-iscsi** - Pure Storage backend with driver "Pure Storage iSCSI"

Each backend can have multiple configurations (shown as "Volume Backend Configurations" with the + button). For example:
- The `nfs` volume backend might have one configuration named `nfs-backend`
- The `netapp` volume backend might have one configuration named `netapp`
- The `vt-pure-iscsi` volume backend might have multiple configurations: `pure-iscsi-1`, `pure-iscsi-2`, etc.

**Important**: For each volume type, you can configure multiple storage arrays. This is useful when you have multiple Pure Storage or NetApp arrays in your environment, each serving different datastores.

```
Storage Volume Backend Configuration (Example):
├── nfs (Volume Type)
│   └── nfs-backend (Backend Configuration)
│       ├── Driver: NFS
│       └── Backend Name: nfs-backend
├── netapp (Volume Type)
│   └── netapp (Backend Configuration)
│       ├── Driver: NetApp Data ONTAP
│       └── Backend Name: netapp
└── vt-pure-iscsi (Volume Type)
    ├── pure-iscsi-1 (Backend Configuration #1)
    │   ├── Driver: Pure Storage iSCSI
    │   └── Backend Name: pure-iscsi-1
    └── pure-iscsi-2 (Backend Configuration #2)
        ├── Driver: Pure Storage iSCSI
        └── Backend Name: pure-iscsi-2
```

After adding PCD credentials to vJailbreak, the system automatically creates ArrayCreds placeholders for each backend configuration:
- `nfs-nfs-backend` (Vendor: N/A, Credentials: Pending) - *Cannot be used for Storage Accelerated Copy*
- `netapp-netapp` (Vendor: NetApp Storage, Credentials: Pending)
- `vt-pure-iscsi-pure-iscsi-1` (Vendor: Pure Storage, Credentials: Pending)
- `vt-pure-iscsi-pure-iscsi-2` (Vendor: Pure Storage, Credentials: Pending)

:::tip
Only storage backends with supported vendors (Pure Storage, NetApp) For now we support only pure storage and netapp, ( As and when requirement arises we will keep adding support for other storage vendors ) nfs and other backends will be auto-discovered but cannot be configured for array-level XCOPY operations.
:::

## Using the UI

Storage-Accelerated Copy can be configured entirely through the vJailbreak UI with automatic backend discovery:

### Step 1: Add PCD Credentials

If not already done:
1. Navigate to **Credentials** → **PCD/OpenStack**
2. Add your PCD credentials
3. vJailbreak will automatically discover all storage volume backends configured in PCD

### Step 2: Configure Storage Array Credentials

1. Navigate to **Storage Management** (Beta feature)
2. You'll see auto-discovered entries for each PCD storage backend:
   - **Name**: Auto-generated (e.g., `netapp-netapp`, `vt-pure-iscsi-pure-iscsi-1`)
   - **Vendor**: Auto-identified from driver type
   - **Volume Type**: Pre-populated from PCD configuration
   - **Backend Name**: Pre-populated from PCD configuration
   - **Source**: "Auto-discovered"
   - **Credentials**: "Pending" (requires your input)

3. Click the **Edit** icon for a storage array entry

![Storage Array Credentials Edit](../../../assets/credsedit.png)

4. Fill in the storage array credentials:
   - **Hostname/IP**: Storage array management IP address
   - **Username**: Array administrator username
   - **Password**: Array administrator password
   - **Skip SSL Verification**: Enable for testing environments (disable in production)
5. Click **Save**
6. The system will:
   - Validate the credentials
   - Connect to the storage array
   - Auto-discover VMware datastores backed by this array
   - Update the status to show validation results

### Step 3: Configure ESXi SSH Key

1. At the top of the Storage Management page, find the **ESXi SSH Key** section
2. Click **Configure** if not already configured
3. Paste your ESXi SSH private key (see [ESXi SSH Access](#2-esxi-ssh-access) section for key generation steps)
4. Click **Save**

### Step 4: Create Migration with Storage-Accelerated Copy

1. When creating a migration plan, select **Storage-Accelerated Copy** as the storage copy method

![Storage Accelerated Copy](../../../assets/trigger-mig.png)

2. The UI will automatically map datastores to ArrayCreds if everything is configured correctly
3. Start the migration - it will use array-level XCOPY for data transfer

:::note
**Auto-Discovery Benefits:**
- No manual typing of volume types, backend names, or Cinder host strings
- Automatic vendor identification from driver type
- Pre-populated PCD mapping configuration
- Automatic datastore-to-array mapping
- Reduced configuration errors
:::

## Migration Workflow

When Storage-Accelerated Copy is enabled, the migration follows this workflow:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Storage-Accelerated Copy Flow                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Validate Prerequisites                                      │
│     ├── Storage provider credentials                            │
│     ├── ESXi SSH key                                            │
│     └── Array connectivity                                      │
│                                                                 │
│  2. Connect to ESXi Host via SSH                                │
│                                                                 │
│  3. Power Off Source VM (required for XCOPY)                    │
│                                                                 │
│  4. For Each Disk:                                              │
│     ├── Create target volume on storage array                   │
│     ├── Import volume to Cinder (manage existing)               │
│     ├── Create/update initiator group with ESXi IQN             │
│     ├── Map volume to ESXi host                                 │
│     ├── Rescan ESXi storage adapters                            │
│     ├── Wait for target device to appear                        │
│     ├── Execute vmkfstools XCOPY clone                          │
│     └── Monitor clone progress                                  │
│                                                                 │
│  5. Convert Volumes (same as normal migration)                  │
│                                                                 │
│  6. Create Target VM in PCD                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Migration Phases

When using Storage-Accelerated Copy, you'll see these additional migration phases:

| Phase | Description |
|-------|-------------|
| `ConnectingToESXi` | Establishing SSH connection to ESXi host |
| `CreatingInitiatorGroup` | Creating/updating initiator group on storage array |
| `CreatingVolume` | Creating target volume on storage array |
| `ImportingToCinder` | Importing volume to PCD Cinder |
| `MappingVolume` | Mapping volume to ESXi host |
| `RescanningStorage` | Rescanning ESXi storage adapters |
| `StorageAcceleratedCopyInProgress` | XCOPY clone operation in progress |

## Limitations

- **Cold migration only**: VMs must be powered off during the copy operation (no live migration support)
- **Shared storage required**: Source and destination must be on the same storage array
- **VMFS datastores only**: NFS datastores are not supported
- **No CBT support**: Change Block Tracking is not used; full disk copy is performed
- **Single array per datastore**: Each datastore can only be mapped to one ArrayCreds

## Troubleshooting

### Common Issues

#### ESXi SSH Connection Failed

```
Error: failed to connect to ESXi via SSH
```

**Resolution**:
- Verify SSH is enabled on the ESXi host
- Check that the SSH private key is correctly stored in the `esxi-ssh-key` secret
- Ensure network connectivity between vJailbreak and ESXi host on port 22

#### Storage Array Connection Failed

**Symptoms:**
```
Error: failed to connect to storage array: authentication failed
```

**Solutions:**
1. Verify credentials in the Storage Management UI:
   - Navigate to **Storage Management**
   - Check the array credentials are correct
   - Re-enter credentials if needed

2. Test connectivity from vJailbreak to storage array:
   - Ensure network connectivity on port 443 (HTTPS)
   - Check firewall rules between vJailbreak and storage array
   - Verify API access is enabled on the array

3. Check array-specific requirements:
   - **Pure Storage**: Ensure API token or username/password has sufficient permissions
   - **NetApp**: Verify ONTAP management interface is accessible

#### ESXi Device Not Found

**Symptoms:**
```
Error: timeout waiting for device naa.624a9370... to appear on ESXi host
```

**Solutions:**
1. Verify iSCSI initiator is configured on ESXi:
   - Log into ESXi via SSH
   - Run: `esxcli iscsi adapter list`
   - Ensure iSCSI software adapter is enabled

2. Check network connectivity from ESXi to storage array:
   - Verify ESXi can reach storage array management IP
   - Check iSCSI network configuration
   - Verify VLAN/network segmentation allows iSCSI traffic

3. Manually rescan storage:
   ```bash
   esxcli storage core adapter rescan --all
   esxcli storage core device list | grep naa.624a9370
   ```

4. Verify LUN masking/mapping on the storage array:
   - Check initiator group includes ESXi IQN
   - Verify volume is mapped to the correct initiator group
   - Check for iSCSI authentication issues (CHAP)

#### Cinder Manage Volume Failed

**Symptoms:**
```
Error: failed to manage existing volume: volume not found on backend
```

**Solutions:**
1. Verify volume exists on the storage array:
   - Check storage array management interface
   - Confirm volume was created successfully

2. Check Cinder backend configuration:
   - Verify Cinder services are running in PCD
   - Ensure volume backend name matches ArrayCreds configuration
   - Check that the Cinder host string is correct

3. Ensure volume naming matches backend expectations:
   - **Pure Storage**: Volume name must match exactly
   - **NetApp**: Full LUN path must be provided

4. Review Cinder logs for detailed errors:
   - Check PCD Cinder volume service logs
   - Look for backend connection issues
   - Verify the `cinderHost` field in ArrayCreds matches a running Cinder service

#### vmkfstools Clone Failed

**Symptoms:**
```
Error: vmkfstools clone failed: Unable to create raw disk
```

**Solutions:**
1. Check ESXi SSH connectivity and authentication:
   - Verify SSH key is configured correctly in vJailbreak
   - Test SSH connection manually
   - Check ESXi SSH service is running

2. Verify vmkfstools is available:
   ```bash
   vmkfstools --version
   ```

3. Check source VMDK accessibility:
   ```bash
   ls -lh /vmfs/volumes/<datastore>/<vm-name>/<disk>.vmdk
   ```

4. Verify target device is visible:
   ```bash
   ls -lh /vmfs/devices/disks/naa.*
   ```

5. Check ESXi host logs:
   ```bash
   tail -f /var/log/vmkernel.log
   ```

6. Ensure sufficient free space on ESXi:
   - RDM descriptor files require space on the datastore
   - Check datastore free space

#### Clone Progress Stalled

**Symptoms:**
```
Error: clone progress stalled - no update for 5 minutes
```

**Solutions:**
1. Check storage array performance and load:
   - Review array management interface for performance metrics
   - Check for high I/O load or resource contention
   - Verify no array-level issues or alerts

2. Verify ESXi storage adapter health:
   ```bash
   esxcli storage core adapter list
   ```

3. Review vmkfstools process on ESXi:
   ps | grep vmkfstools
   ps -c | grep vmkfstools
   ```
   - Check if process is still running
   - Look for any error indicators

4. Check for network issues:
   - Verify stable connectivity between ESXi and storage array
   - Check for packet loss or latency issues

5. Consider increasing timeout settings:
   - For very large disks, the operation may take longer than expected
   - Monitor array performance to ensure copy is progressing

### Checking ArrayCreds Status

You can check the status of your storage array credentials in the UI:

1. Navigate to **Storage Management**
2. Check the **Credentials** column - it should show "Valid" after successful configuration
3. The system will display discovered datastores for each array
4. Any validation errors will be shown in the status column

## Best Practices

1. **Validate prerequisites first**: Ensure all connectivity and credentials are working before starting migrations
2. **Schedule during maintenance windows**: VMs must be powered off during copy
3. **Monitor array performance**: Large migrations can impact array performance
4. **Use for large VMs**: The setup overhead makes this most beneficial for VMs with large disks
5. **Batch similar VMs**: Group VMs on the same datastore for efficient migrations
