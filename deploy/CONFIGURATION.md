# vJailbreak Configuration

This document describes configurable settings available in vJailbreak through the `vjailbreak-settings` ConfigMap.

## ConfigMap: vjailbreak-settings

The `vjailbreak-settings` ConfigMap allows you to customize various aspects of vJailbreak behavior. It should be created in the `migration-system` namespace.

## Setting Categories

Settings are organized into the following categories:

- **Validation Settings**: Control validation behavior during migration planning
- **Performance Settings**: Tune migration performance and resource usage
- **Timing Settings**: Configure wait intervals and retry limits
- **vCenter Settings**: Control vCenter interaction behavior
- **Automation Settings**: Configure automatic behaviors and cleanup

### Available Settings

#### VALIDATE_RDM_OWNER_VMS

**Description**: Controls whether to validate RDM (Raw Device Mapping) disk owner VMs during migration planning.

**Default**: `"true"`

**Values**:
- `"true"`: (Default) Enable validation - The migration plan will verify that all owner VMs specified in RDM disks are included in the migration plan. If an owner VM is not found, an error will be logged.
- `"false"`: Disable validation - Skip the RDM disk owner VM validation, allowing migrations to proceed even if some owner VMs are not included in the migration plan.

**Use Case**: 
- Set to `"false"` when you want to migrate VMs with RDM disks but don't need all owner VMs to be included in the same migration plan
- Useful for partial migrations or when dealing with complex RDM disk configurations

#### CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD

**Description**: Controls the threshold for changed blocks copy iterations during hot migration.

**ConfigMap Key**: `CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD`

**Default**: `20`

**Values**: Integer value representing the maximum number of iterations for copying changed blocks.

**Use Case**: 
- Increase for VMs with high I/O activity to allow more iterations before cutover
- Decrease to reduce migration time but may require longer cutover windows

#### VM_ACTIVE_WAIT_INTERVAL_SECONDS

**Description**: The interval (in seconds) to wait between checks when waiting for a VM to become active in OpenStack.

**ConfigMap Key**: `VM_ACTIVE_WAIT_INTERVAL_SECONDS`

**Default**: `20`

**Values**: Integer value in seconds.

**Use Case**: 
- Increase for slower OpenStack environments
- Decrease for faster response times in high-performance environments

#### VM_ACTIVE_WAIT_RETRY_LIMIT

**Description**: The maximum number of retries when waiting for a VM to become active in OpenStack.

**ConfigMap Key**: `VM_ACTIVE_WAIT_RETRY_LIMIT`

**Default**: `15`

**Values**: Integer value representing maximum retry attempts.

**Use Case**: 
- Increase for environments where VM startup takes longer
- Decrease to fail faster in case of persistent issues

#### DEFAULT_MIGRATION_METHOD

**Description**: The default migration method to use when not explicitly specified.

**ConfigMap Key**: `DEFAULT_MIGRATION_METHOD`

**Default**: `"hot"`

**Values**:
- `"hot"`: Live migration with minimal downtime
- `"cold"`: Offline migration requiring VM shutdown

**Use Case**: 
- Set to `"cold"` for environments where hot migration is not supported or preferred
- Keep as `"hot"` for minimal downtime requirements

#### VCENTER_SCAN_CONCURRENCY_LIMIT

**Description**: The maximum number of concurrent vCenter scanning operations.

**ConfigMap Key**: `VCENTER_SCAN_CONCURRENCY_LIMIT`

**Default**: `100`

**Values**: Integer value representing maximum concurrent operations.

**Use Case**: 
- Reduce for vCenter environments with performance constraints
- Increase for high-performance vCenter deployments

#### CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE

**Description**: Whether to automatically cleanup OpenStack volumes after a conversion failure.

**ConfigMap Key**: `CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE`

**Default**: `"true"`

**Values**:
- `"true"`: (Default) Automatically cleanup volumes after conversion failures
- `"false"`: Preserve volumes for manual inspection and cleanup

**Use Case**: 
- Set to `"false"` for debugging conversion issues
- Keep as `"true"` for automatic cleanup and resource management

#### POPULATE_VMWARE_MACHINE_FLAVORS

**Description**: Whether to automatically populate VMwareMachine objects with matching OpenStack flavors.

**ConfigMap Key**: `POPULATE_VMWARE_MACHINE_FLAVORS`

**Default**: `"true"`

**Values**:
- `"true"`: (Default) Automatically find and assign matching flavors
- `"false"`: Require manual flavor assignment

**Use Case**: 
- Set to `"false"` when you want full control over flavor assignment
- Keep as `"true"` for automatic flavor matching based on VM resources

#### VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS

**Description**: The interval (in seconds) to wait between checks when waiting for OpenStack volumes to become available.

**ConfigMap Key**: `VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS`

**Default**: `5`

**Values**: Integer value in seconds.

**Use Case**: 
- Increase for slower storage backends
- Decrease for high-performance storage systems

#### VOLUME_AVAILABLE_WAIT_RETRY_LIMIT

**Description**: The maximum number of retries when waiting for OpenStack volumes to become available.

**ConfigMap Key**: `VOLUME_AVAILABLE_WAIT_RETRY_LIMIT`

**Default**: `15`

**Values**: Integer value representing maximum retry attempts.

**Use Case**: 
- Increase for environments with slower volume provisioning
- Decrease to fail faster when volumes cannot be created

#### VCENTER_LOGIN_RETRY_LIMIT

**Description**: The maximum number of retries for vCenter login attempts.

**ConfigMap Key**: `VCENTER_LOGIN_RETRY_LIMIT`

**Default**: `1`

**Values**: Integer value representing maximum retry attempts.

**Use Case**: 
- Increase for unstable network connections to vCenter
- Keep low to avoid account lockouts due to repeated failed attempts

### Example Configuration

#### Basic Configuration
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vjailbreak-settings
  namespace: migration-system
data:
  validateRDMOwnerVMs: "false"  # Disable RDM owner VM validation
```

#### Comprehensive Configuration
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vjailbreak-settings
  namespace: migration-system
data:
  # RDM disk validation
  validateRDMOwnerVMs: "true"
  
  # Migration performance settings
  CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: "25"
  DEFAULT_MIGRATION_METHOD: "hot"
  
  # VM and volume wait settings
  VM_ACTIVE_WAIT_INTERVAL_SECONDS: "30"
  VM_ACTIVE_WAIT_RETRY_LIMIT: "20"
  VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: "10"
  VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: "20"
  
  # vCenter settings
  VCENTER_SCAN_CONCURRENCY_LIMIT: "50"
  VCENTER_LOGIN_RETRY_LIMIT: "3"
  
  # Automation settings
  POPULATE_VMWARE_MACHINE_FLAVORS: "true"
  CLEANUP_VOLUMES_AFTER_CONVERT_FAILURE: "true"
```

### Applying Configuration

1. Create or update the ConfigMap:
   ```bash
   kubectl apply -f 05vjailbreak-settings.yaml
   ```

2. The settings take effect immediately for new migration plans. No restart of the migration controller is required.

### Notes

- If the `vjailbreak-settings` ConfigMap is not found, default values will be used
- Invalid values will fall back to defaults with a warning logged
- Settings are read per migration plan, allowing for dynamic configuration changes
- All boolean settings use string values (`"true"` or `"false"`)
- All numeric settings use string representations of integers

### Troubleshooting

#### Checking Current Settings
```bash
kubectl get configmap vjailbreak-settings -n migration-system -o yaml
```

#### Viewing Controller Logs
```bash
kubectl logs -n migration-system deployment/migration-controller-manager -f
```

#### Common Issues
- **Settings not taking effect**: Ensure the ConfigMap is in the `migration-system` namespace
- **Invalid boolean values**: Use quoted strings (`"true"`/`"false"`) not boolean literals
- **Invalid numeric values**: Use quoted string numbers (`"20"` not `20`)
- **ConfigMap not found**: The system will use default values with a warning in the logs

### Performance Tuning Guidelines

#### For High I/O VMs
```yaml
CHANGED_BLOCKS_COPY_ITERATION_THRESHOLD: "30"
VM_ACTIVE_WAIT_RETRY_LIMIT: "25"
```

#### For Slow Storage Environments
```yaml
VOLUME_AVAILABLE_WAIT_INTERVAL_SECONDS: "15"
VOLUME_AVAILABLE_WAIT_RETRY_LIMIT: "25"
VM_ACTIVE_WAIT_INTERVAL_SECONDS: "30"
```

#### For Resource-Constrained vCenter
```yaml
VCENTER_SCAN_CONCURRENCY_LIMIT: "25"
VCENTER_LOGIN_RETRY_LIMIT: "5"
```
