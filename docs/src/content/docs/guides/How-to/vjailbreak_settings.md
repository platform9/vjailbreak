---
title: "Using vjailbreak-settings ConfigMap"
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
| `CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD` | Number of iterations to copy changed blocks | `20` | Any positive integer |
| `VM_ACTIVE_WAIT_INTERVAL_SECONDS` | Interval to wait for VM to become active (in seconds) | `20` | Any positive integer |
| `VM_ACTIVE_WAIT_RETRY_LIMIT` | Number of retries to wait for VM to become active | `15` | Any positive integer |
| `DEFAULT_MIGRATION_METHOD` | Default method for VM migration *(placeholder for future releases, not currently used)* | `hot` | `hot`, `cold` |
| `VCENTER_SCAN_CONCURRENCY_LIMIT` | Maximum number of vCenter VMs to scan concurrently | `100` | Any positive integer |

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
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: "30"
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: "30"
  VM_ACTIVE_WAIT_RETRY_LIMIT: "20"
  DEFAULT_MIGRATION_METHOD: "hot"
  VCENTER_SCAN_CONCURRENCY_LIMIT: "150"
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
  LOG_LEVEL: "debug"
  MAX_CONCURRENT_MIGRATIONS: "5"
  RESOURCE_QUOTA_CPU: "2000m"
  RESOURCE_QUOTA_MEMORY: "4Gi"
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

### Setting Default Migration Method (Future Feature)

> **Note:** This setting is currently a placeholder and does not affect the system. It will be implemented in future releases.

In upcoming releases, you'll be able to set the default migration method for VMs:

```bash
kubectl patch configmap -n migration-system vjailbreak-settings --type merge -p '{"data":{"DEFAULT_MIGRATION_METHOD":"cold"}}'
```

## Verification

To verify your settings have been applied correctly:

```bash
kubectl get configmap -n migration-system vjailbreak-settings -o yaml
```

## Applying Changes

After modifying settings in the ConfigMap, no restart is required. The ConfigMap is not mounted as a volume; instead, its values are fetched at runtime by the system components. Changes to the settings take effect immediately for new operations.

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
