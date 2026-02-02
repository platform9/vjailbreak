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

**Option A: Using the UI (Recommended)**

1. Navigate to **Storage Management** page
2. At the top, find the **ESXi SSH Key** section
![SSH Key configuration](../../assets/sshkey.png)
3. Click **Configure**
4. Paste the contents of your private key:
   ```bash
   cat ~/.ssh/esxi_migration_key
   ```
5. Copy the entire output (including `-----BEGIN OPENSSH PRIVATE KEY-----` and `-----END OPENSSH PRIVATE KEY-----`)
6. Paste into the UI textarea
7. Click **Save**

**Option B: Using kubectl**

```bash
# Create the secret with the private key
kubectl create secret generic esxi-ssh-key \
  --from-file=ssh-privatekey=~/.ssh/esxi_migration_key \
  -n migration-system

# Verify the secret was created
kubectl get secret esxi-ssh-key -n migration-system
```

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
1. Verify SSH service is running: `ssh root@<esxi-host> "/etc/init.d/SSH status"`
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

![Storage Management Page](../../assets/storagemanagementpage.png)

#### Example: PCD with Multiple Storage Backends

In PCD, storage backends are configured in the **Cluster Blueprint** under **Persistent Storage Connectivity**. Each volume backend can have multiple configurations, and each configuration represents a connection to a storage array.

![PCD Cluster Blueprint - Storage Configuration](../../assets/clusterblueprint.png)

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
Only storage backends with supported vendors (Pure Storage, NetApp) can be used for Storage Accelerated Copy. NFS and other backends will be auto-discovered but cannot be configured for array-level XCOPY operations.
:::

### Step 1: Create ArrayCreds

ArrayCreds stores the storage array credentials and PCD mapping information.

#### Option A: Update Auto-Discovered ArrayCreds (Recommended)

If you've already added PCD credentials, ArrayCreds entries are automatically created. Simply update them with storage array credentials:

1. Navigate to **Storage Management** page
2. Find the auto-discovered entry for your storage array (status shows "Pending")
3. Click the **Edit** icon
4. Fill in the storage array credentials:
   - **Hostname/IP**: Storage array management IP
   - **Username**: Array admin username
   - **Password**: Array admin password
   - **Skip SSL Verification**: Enable for testing (disable in production)
5. Click **Save**
6. The system will validate credentials and discover VMware datastores

#### Option B: Manually Create ArrayCreds

If you need to manually create ArrayCreds (e.g., for testing or custom configurations):

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: ArrayCreds
metadata:
  name: pure-array-01
  namespace: vjailbreak
spec:
  vendorType: pure  # or "netapp"
  secretRef:
    name: pure-array-secret
    namespace: vjailbreak
  PCDMapping:
    volumeType: "pure-iscsi"
    cinderBackendName: "pure-iscsi-backend"
    # cinderHost is auto-discovered if not specified
```

Create the corresponding secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pure-array-secret
  namespace: vjailbreak
type: Opaque
stringData:
  hostname: "192.168.1.100"
  username: "pureuser"
  password: "your-password"
  skipSSLVerification: "true"  # Set to "false" in production
```

#### ArrayCreds Fields

| Field | Description | Required |
|-------|-------------|----------|
| `vendorType` | Storage array vendor: `pure` or `netapp` | Yes |
| `secretRef` | Reference to Kubernetes secret with array credentials | Yes |
| `PCDMapping.volumeType` | Cinder volume type for this array | Yes |
| `PCDMapping.cinderBackendName` | Cinder backend name (from cinder.conf) | Yes |
| `PCDMapping.cinderHost` | Full Cinder host string (auto-discovered if empty) | No |

### Step 2: Create ArrayCredsMapping

ArrayCredsMapping maps VMware datastores to their corresponding ArrayCreds resources.

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: ArrayCredsMapping
metadata:
  name: datacenter-mapping
  namespace: vjailbreak
spec:
  mappings:
    - source: "datastore-pure-01"  # VMware datastore name
      target: "pure-array-01"       # ArrayCreds name
    - source: "datastore-pure-02"
      target: "pure-array-01"
```

:::tip
After creating ArrayCreds, the controller automatically discovers which VMware datastores are backed by the storage array. Check the ArrayCreds status to see discovered datastores.
:::

### Step 3: Configure MigrationTemplate

Update your MigrationTemplate to use Storage-Accelerated Copy:

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: MigrationTemplate
metadata:
  name: migration-template-xcopy
  namespace: vjailbreak
spec:
  source:
    vmwareRef: vmware-creds
  destination:
    PCDRef: PCD-creds
  networkMapping: network-mapping
  storageCopyMethod: StorageAcceleratedCopy  # Enable Storage-Accelerated Copy
  arrayCredsMapping: datacenter-mapping       # Reference to ArrayCredsMapping
  # storageMapping is not needed when using StorageAcceleratedCopy
```

## Using the UI

Storage-Accelerated Copy can be configured through the vJailbreak UI with automatic backend discovery:

### Initial Setup

1. **Add PCD Credentials** (if not already done):
   - Navigate to **Credentials** → **PCD/OpenStack**
   - Add your PCD credentials
   - vJailbreak will automatically discover all storage volume backends configured in PCD

2. **Configure Storage Array Credentials**:
   - Navigate to **Storage Management** (Beta feature)
   - You'll see auto-discovered entries for each PCD storage backend:
     - **Name**: Auto-generated (e.g., `netapp-netapp`, `vt-pure-iscsi-pure-iscsi-1`)
     - **Vendor**: Auto-identified from driver type
     - **Volume Type**: Pre-populated from PCD configuration
     - **Backend Name**: Pre-populated from PCD configuration
     - **Source**: "Auto-discovered"
     - **Credentials**: "Pending" (requires your input)
   
3. **Update Array Credentials**:
   - Click the **Edit** icon for a storage array entry
   - Fill in the storage array credentials:
     - **Hostname/IP**: Storage array management IP address
     - **Username**: Array administrator username
     - **Password**: Array administrator password
     - **Skip SSL Verification**: Enable for testing environments (disable in production)
   - Click **Save**
   - The system will:
     - Validate the credentials
     - Connect to the storage array
     - Auto-discover VMware datastores backed by this array
     - Update the status to show validation results

4. **Configure ESXi SSH Key**:
   - At the top of the Storage Management page, you'll see "ESXi SSH Key" section
   - Click **Configure** if not already configured
   - Paste your ESXi SSH private key or upload the key file
   - Click **Save**

5. **Create Array Credentials Mapping**:
   - Navigate to **Mappings** → **Array Credentials Mapping**
   - Click **Add Mapping**
   - Map VMware datastores to the configured ArrayCreds
   - The system will show you which datastores were auto-discovered for each array

6. **Create Migration with Storage-Accelerated Copy**:
   - When creating a migration plan, select **Storage-Accelerated Copy** as the storage copy method
   - Select the ArrayCredsMapping you created
   - The migration will use array-level XCOPY for data transfer

:::note
**Auto-Discovery Benefits:**
- No manual typing of volume types, backend names, or Cinder host strings
- Automatic vendor identification from driver type
- Pre-populated PCD mapping configuration
- Reduced configuration errors
:::

### Manual Configuration (Alternative)

If you prefer to manually add storage arrays without auto-discovery:

1. Navigate to **Storage Management**
2. Click **Add Array Credentials**
3. Manually enter all details:
   - Array hostname/IP
   - Username and password
   - Vendor type (Pure Storage or NetApp)
   - PCD volume type mapping
   - Backend name
   - Cinder host (optional)
4. The system will validate credentials and auto-discover datastores

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

#### Storage Array Credential Validation Failed

```
Error: storage array credential validation failed
```

**Resolution**:
- Verify array hostname/IP is correct and reachable
- Check username and password
- Ensure the user has sufficient permissions on the storage array

#### Target Device Not Visible

```
Error: device naa.xxx not visible after timeout
```

**Resolution**:
- Verify iSCSI connectivity between ESXi and storage array
- Check that the initiator group is correctly configured
- Manually trigger a storage rescan on the ESXi host

#### XCOPY Clone Failed

```
Error: vmkfstools failed: ...
```

**Resolution**:
- Ensure the source VM is powered off
- Check that the source VMDK is not locked by another process
- Verify sufficient space on the target volume

### Checking ArrayCreds Status

```bash
kubectl get arraycreds -n vjailbreak -o wide
```

The status will show:
- `arrayValidationStatus`: Whether credentials are valid
- `dataStore`: List of discovered datastores backed by this array

## Performance Comparison

| Metric | Normal Copy (NFC) | Storage-Accelerated Copy |
|--------|-------------------|--------------------------|
| Transfer speed per disk | ~1 Gbps (125 MB/s) | Array-dependent (typically 5-50 Gbps) |
| Network utilization | High | Minimal |
| ESXi CPU usage | Moderate | Low |
| Best for | Small VMs, mixed storage | Large VMs, shared storage |

:::tip
For a 1 TB disk:
- **Normal copy**: ~2.5 hours
- **Storage-Accelerated Copy**: ~5-30 minutes (depending on array performance)
:::

## Best Practices

1. **Validate prerequisites first**: Ensure all connectivity and credentials are working before starting migrations
2. **Schedule during maintenance windows**: VMs must be powered off during copy
3. **Monitor array performance**: Large migrations can impact array performance
4. **Use for large VMs**: The setup overhead makes this most beneficial for VMs with large disks
5. **Batch similar VMs**: Group VMs on the same datastore for efficient migrations
