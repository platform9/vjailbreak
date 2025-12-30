---
title: "Use vJailbreak Settings"
description: "How to modify global settings for vJailbreak using the vjailbreak-settings ConfigMap"
---

The vjailbreak-settings ConfigMap provides a centralized way to customize and override default settings in the vJailbreak system. This guide shows you how to use this ConfigMap effectively.

## Overview

The vjailbreak-settings ConfigMap allows you to:
- Override system-wide default values
- Enable or disable optional features
- Configure global system behaviors
- Set resource limits and operational parameters

## Checking for the ConfigMap

In vJailbreak v0.3.0 and above, the `vjailbreak-settings` ConfigMap should already exist in your cluster. If it doesn't exist, you're likely using an older version and should upgrade to v0.3.0 or above.

To check if the ConfigMap exists:

```bash
kubectl get configmap vjailbreak-settings -n migration-system
```

If you receive an error that the ConfigMap doesn't exist, please upgrade your vJailbreak installation to the latest version.

## Available Settings

The vjailbreak-settings ConfigMap supports the following settings:

| Setting | Description | Default Value | Example Values |
|---------|-------------|---------------|---------------|
| `AUTO_FSTAB_UPDATE` | Automatically update fstab during migration | `false` | `true`, `false` |
| `AUTO_PXE_BOOT_ON_CONVERSION` | Automatically configure PXE boot during conversion | `false` | `true`, `false` |
| `CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD` | Number of iterations to copy changed blocks during hot migration | `20` | Any positive integer |
| `CLEANUP_PORTS_AFTER_MIGRATION_FAILURE` | Automatically cleanup OpenStack ports after migration failure | `false` | `true`, `false` |
| `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE` | Automatically cleanup OpenStack volumes after conversion failure | `false` | `true`, `false` |
| `DEFAULT_MIGRATION_METHOD` | Default method for VM migration | `cold` | `hot` (migrate while VM is running), `cold` (power off VM before migration) |
| `DEPLOYMENT_NAME` | Name of the vJailbreak deployment | `vJailbreak` | Any string |
| `OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES` | Interval in minutes to requeue OpenStack credentials validation | `60` | Any positive integer |
| `PERIODIC_SYNC_INTERVAL` | Interval for periodic sync during admin cutover | `1h` | Duration format (e.g., `30m`, `1h`, `2h`) |
| `PERIODIC_SYNC_MAX_RETRIES` | Maximum number of retries for periodic sync | `3` | Any positive integer |
| `PERIODIC_SYNC_RETRY_CAP` | Maximum duration to retry periodic sync | `3h` | Duration format (e.g., `1h`, `3h`, `6h`) |
| `POPULATE_VMWARE_MACHINE_FLAVORS` | Automatically populate flavor recommendations for VMware machines | `true` | `true`, `false` |
| `VALIDATE_RDM_OWNER_VMS` | Validates that all VMs linked to an RDM disk are migrated in a single migration plan | `true` | `true`, `false` |
| `VCENTER_LOGIN_RETRY_LIMIT` | Number of retries for vCenter login attempts | `5` | Any positive integer |
| `VCENTER_SCAN_CONCURRENCY_LIMIT` | Maximum number of vCenter VMs to scan concurrently | `10` | Any positive integer |
| `VM_ACTIVE_WAIT_INTERVAL_SECONDS` | Interval to wait for VM to become active (in seconds) | `20` | Any positive integer |
| `VM_ACTIVE_WAIT_RETRY_LIMIT` | Number of retries to wait for VM to become active | `15` | Any positive integer |
| `VMWARE_CREDS_REQUEUE_AFTER_MINUTES` | Interval in minutes to requeue VMware credentials validation | `60` | Any positive integer |
| `VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS` | Interval to wait for volume to become available (in seconds) | `10` | Any positive integer |
| `VOLUME_AVAILABLE_WAIT_RETRY_LIMIT` | Number of retries to wait for volume to become available | `15` | Any positive integer |

## Modifying Settings

You can modify the ConfigMap to change settings using one of the following methods:

### Method 1: Edit with kubectl

```bash
kubectl edit configmap vjailbreak-settings -n migration-system
```

Then add or modify the data values as needed:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vjailbreak-settings
  namespace: migration-system
data:
  AUTO_FSTAB_UPDATE: "false"
  AUTO_PXE_BOOT_ON_CONVERSION: "false"
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: "30"
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE: "false"
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: "false"
  DEFAULT_MIGRATION_METHOD: "cold"
  DEPLOYMENT_NAME: "vJailbreak"
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: "60"
  PERIODIC_SYNC_INTERVAL: "1h"
  PERIODIC_SYNC_MAX_RETRIES: "3"
  PERIODIC_SYNC_RETRY_CAP: "3h"
  POPULATE_VMWARE_MACHINE_FLAVORS: "true"
  VALIDATE_RDM_OWNER_VMS: "true"
  VCENTER_LOGIN_RETRY_LIMIT: "5"
  VCENTER_SCAN_CONCURRENCY_LIMIT: "150"
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: "30"
  VM_ACTIVE_WAIT_RETRY_LIMIT: "20"
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: "60"
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: "10"
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: "15"
```

### Method 2: Using kubectl patch

You can modify individual settings without editing the entire ConfigMap:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"VM_ACTIVE_WAIT_INTERVAL_SECONDS":"30","VM_ACTIVE_WAIT_RETRY_LIMIT":"20"}}'
```

### Method 3: From a file

Create a file with your settings:

```bash
cat > vjailbreak-settings.yaml << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: vjailbreak-settings
  namespace: migration-system
data:
  AUTO_FSTAB_UPDATE: "false"
  AUTO_PXE_BOOT_ON_CONVERSION: "false"
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: "30"
  CLEANUP_PORTS_AFTER_MIGRATION_FAILURE: "false"
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: "false"
  DEFAULT_MIGRATION_METHOD: "cold"
  DEPLOYMENT_NAME: "vJailbreak"
  OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES: "60"
  PERIODIC_SYNC_INTERVAL: "1h"
  PERIODIC_SYNC_MAX_RETRIES: "3"
  PERIODIC_SYNC_RETRY_CAP: "3h"
  POPULATE_VMWARE_MACHINE_FLAVORS: "true"
  VALIDATE_RDM_OWNER_VMS: "true"
  VCENTER_LOGIN_RETRY_LIMIT: "5"
  VCENTER_SCAN_CONCURRENCY_LIMIT: "150"
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: "30"
  VM_ACTIVE_WAIT_RETRY_LIMIT: "20"
  VMWARE_CREDS_REQUEUE_AFTER_MINUTES: "60"
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: "10"
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: "15"
EOF

kubectl apply -f vjailbreak-settings.yaml
```

## Settings in Action

### Optimizing Block Copy Operations

To increase the number of iterations for copying changed blocks during hot migrations:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD":"30"}}'
```

### Adjusting VM Activation Parameters

To increase wait time and retry attempts for VM activation:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"VM_ACTIVE_WAIT_INTERVAL_SECONDS":"30","VM_ACTIVE_WAIT_RETRY_LIMIT":"20"}}'
```

### Optimizing Scan Performance

To increase the number of concurrent vCenter scan pods:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"VCENTER_SCAN_CONCURRENCY_LIMIT":"150"}}'
```

### Configuring Periodic Sync for Admin Cutover

To adjust the periodic sync settings for admin cutover migrations:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"PERIODIC_SYNC_INTERVAL":"30m","PERIODIC_SYNC_MAX_RETRIES":"5","PERIODIC_SYNC_RETRY_CAP":"6h"}}'
```

This configures the system to sync every 30 minutes, with a maximum of 5 retries, and a total retry window of 6 hours.

### Enabling Automatic Cleanup After Failures

To automatically cleanup resources after migration failures:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"CLEANUP_PORTS_AFTER_MIGRATION_FAILURE":"true","CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE":"true"}}'
```

> **Note:** Enabling automatic cleanup helps prevent resource accumulation after failed migrations, but ensure you have proper logging and monitoring in place to track what gets cleaned up.

### Adjusting Volume Availability Wait Parameters

To increase wait time and retry attempts for volumes to become available:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS":"15","VOLUME_AVAILABLE_WAIT_RETRY_LIMIT":"20"}}'
```

### Configuring Credentials Revalidation Intervals

To adjust how frequently credentials are revalidated:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES":"30","VMWARE_CREDS_REQUEUE_AFTER_MINUTES":"30"}}'
```

### Configuring RDM Disk Validation

To disable the validation that requires all VMs linked to an RDM disk to be migrated in a single migration plan:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"VALIDATE_RDM_OWNER_VMS":"false"}}'
```

> **Note:** When `VALIDATE_RDM_OWNER_VMS` is set to `true` (default), the system ensures that all VMs sharing an RDM disk are migrated together in the same migration plan. This prevents potential data consistency issues. Only disable this validation if you understand the implications for your RDM disk configuration.

### Setting Default Migration Method

To set the default migration method for VMs:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"DEFAULT_MIGRATION_METHOD":"hot"}}'
```

The system supports two migration methods:

- **Hot migration**: Migrates VMs while they are running, minimizing downtime but requiring more coordination and potentially multiple sync iterations to capture changed blocks
- **Cold migration** (default): Powers off the VM before migration, ensuring data consistency but causing downtime during the entire migration process

## Verification

To verify your settings have been applied correctly:

```bash
kubectl get configmap -n migration-system vjailbreak-settings -o yaml
```

## Applying Changes

After modifying settings in the ConfigMap, the behavior depends on which settings you changed:

### Settings That Require Controller Restart

The following settings are loaded at controller startup and require a restart of the migration controller pod to take effect:

- `OPENSTACK_CREDS_REQUEUE_AFTER_MINUTES`
- `VMWARE_CREDS_REQUEUE_AFTER_MINUTES`

To restart the controller after changing these settings:

```bash
kubectl rollout restart deployment migration-controller-manager -n migration-system
```

### Settings That Take Effect Immediately

All other settings are read dynamically at runtime and do not require a restart:

- `AUTO_FSTAB_UPDATE`
- `AUTO_PXE_BOOT_ON_CONVERSION`
- `CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD`
- `CLEANUP_PORTS_AFTER_MIGRATION_FAILURE`
- `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE`
- `DEFAULT_MIGRATION_METHOD`
- `DEPLOYMENT_NAME`
- `PERIODIC_SYNC_INTERVAL`
- `PERIODIC_SYNC_MAX_RETRIES`
- `PERIODIC_SYNC_RETRY_CAP`
- `POPULATE_VMWARE_MACHINE_FLAVORS`
- `VALIDATE_RDM_OWNER_VMS`
- `VCENTER_LOGIN_RETRY_LIMIT`
- `VCENTER_SCAN_CONCURRENCY_LIMIT`
- `VM_ACTIVE_WAIT_INTERVAL_SECONDS`
- `VM_ACTIVE_WAIT_RETRY_LIMIT`
- `VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS`
- `VOLUME_AVAILABLE_WAIT_RETRY_LIMIT`

### When Changes Take Effect

For settings that don't require restart:

- **On-demand access**: Values are read from the ConfigMap when they are needed for an operation
- **New operations**: Changes affect only new operations that start after the ConfigMap is updated
- **In-progress operations**: Running operations continue using the values they initially read
- **No caching**: The system does not cache these values for extended periods, ensuring relatively quick propagation of changes

Typically, your changes will be effective within seconds for any new operations initiated after updating the ConfigMap.

## Best Practices and Considerations

### Testing Recommendations
- Always test configuration changes in a test environment that closely matches your production setup before applying them to production.
- Validate each setting change independently to understand its impact on system behavior.
- Document any changes made to default settings for future reference and troubleshooting.

### Operational Considerations
- Setting changes take effect for new operations and do not affect in-progress tasks.
- The impact of settings varies based on your specific environment (hardware, network, storage configuration).
- Performance-related settings should be adjusted based on your specific infrastructure capabilities.

### Monitoring and Validation
- After changing settings, monitor system behavior to ensure the changes produce the expected results.
- Use vJailbreak logs to verify that settings are being correctly applied.

> **Important:** Always select configuration values appropriate for your specific environment. Incorrect settings may negatively impact system performance or stability.
