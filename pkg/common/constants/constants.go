// Package constants provides constant values used throughout the vjailbreak system
package constants

import (
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ============================================================================
// RESOLVED CONFLICTS:
// The following constants had conflicts that have been resolved based on usage:
// - OSFamily: Using v2v-helper values ("windowsguest"/"linuxguest") as they match VMware GuestFamily types
// - MaxVCPUs/MaxRAM: Using k8s/migration values (99999) - more reasonable limits
// - DefaultMigrationMethod: Using k8s/migration value ("hot") - safer default
// - VCenterLoginRetryLimit: Using k8s/migration value (5) - more resilient
// ============================================================================

// OS Family Constants
// These values match VMware's VirtualMachineGuestOsFamily types
const (
	OSFamilyWindows = "windowsguest"
	OSFamilyLinux   = "linuxguest"
)

// Post-migration script constants
const (
	NextScriptDelimiterLine = "### NEXT SCRIPT ###"
	LinuxTag                = "LINUX-SCRIPT:"
	WindowsTag              = "WINDOWS-SCRIPT:"
)

// Max CPU/RAM Constants
const (
	MaxVCPUs = 9999999
	MaxRAM   = 9999999
)

// Default Migration Method
const (
	DefaultMigrationMethod = "cold"
)

// VCenter Login Retry Limit
const (
	VCenterLoginRetryLimit = 5
)

// TerminationPeriod defines the grace period for pod termination in seconds
const (
	TerminationPeriod = int64(120)

	// NameMaxLength defines the maximum length of a name
	K8sNameMaxLength = 63

	// HashSuffixLength defines the length of the hash suffix
	HashSuffixLength = 5

	// VMNameMaxLength defines the maximum length of a VM name excluding the hash suffix
	VMNameMaxLength = 57

	// MaxJobNameLength defines the maximum length of a job name
	MaxJobNameLength = 46 // 63 - 11 (prefix v2v-helper-) - 1 (hyphen) - 5 (hash)

	// VjailbreakNodeControllerName is the name of the vjailbreak node controller
	VjailbreakNodeControllerName = "vjailbreaknode-controller"

	// OpenstackCredsControllerName is the name of the openstack credentials controller
	OpenstackCredsControllerName = "openstackcreds-controller" //nolint:gosec // not a password string

	// VMwareCredsControllerName is the name of the vmware credentials controller
	VMwareCredsControllerName = "vmwarecreds-controller" //nolint:gosec // not a password string

	// ArrayCredsControllerName is the name of the storage array credentials controller
	ArrayCredsControllerName = "arraycreds-controller" //nolint:gosec // not a password string

	// ESXiSSHCredsControllerName is the name of the ESXi SSH credentials controller
	ESXiSSHCredsControllerName = "esxisshcreds-controller" //nolint:gosec // not a password string

	// MigrationControllerName is the name of the migration controller
	MigrationControllerName = "migration-controller"

	// RollingMigrationPlanControllerName is the name of the rolling migration plan controller
	RollingMigrationPlanControllerName = "rollingmigrationplan-controller"

	// ESXIMigrationControllerName is the name of the ESXi migration controller
	ESXIMigrationControllerName = "esximigration-controller"

	// ClusterMigrationControllerName is the name of the cluster migration controller
	ClusterMigrationControllerName = "clustermigration-controller"

	// BMConfigControllerName is the name of the BMConfig controller
	BMConfigControllerName = "bmconfig-controller"

	// K8sMasterNodeAnnotation is the annotation for k8s master node
	K8sMasterNodeAnnotation = "node-role.kubernetes.io/control-plane"

	// VMwareNetworkTypeNetwork is the VMware network type for standard port groups
	VMwareNetworkTypeNetwork = "Network"

	// VMwareNetworkTypeDistributedVirtualPortgroup is the VMware network type for dvPort groups
	VMwareNetworkTypeDistributedVirtualPortgroup = "DistributedVirtualPortgroup"

	// VMwareNetworkTypeOpaqueNetwork is the VMware network type for NSX-T opaque networks
	VMwareNetworkTypeOpaqueNetwork = "OpaqueNetwork"

	// VMwareCredsLabel is the label for vmware credentials
	VMwareCredsLabel = "vjailbreak.k8s.pf9.io/vmwarecreds" //nolint:gosec // not a password string

	// VMwareDatacenterLabel is the label for the vSphere datacenter
	VMwareDatacenterLabel = "vjailbreak.k8s.pf9.io/datacenter" //nolint:gosec // not a password string

	// OpenstackCredsLabel is the label for openstack credentials
	OpenstackCredsLabel = "vjailbreak.k8s.pf9.io/openstackcreds" //nolint:gosec // not a password string

	// IsPCDCredsLabel is the label for pcd credentials
	IsPCDCredsLabel = "vjailbreak.k8s.pf9.io/is-pcd" //nolint:gosec // not a password string

	// VMNameLabel is the label for vm name
	VMNameLabel = "vjailbreak.k8s.pf9.io/vm-name"

	// MigrationVMKeyLabel stores the sanitized name-<moid> VM key on Migration objects as a label.
	// Label values forbid spaces; use OriginalVMNameAnnotation to retrieve the unsanitized key.
	MigrationVMKeyLabel = "vjailbreak.k8s.pf9.io/vm-key"

	// OriginalVMNameAnnotation stores the original (unsanitized) name-<moid> VM key on Migration
	// objects. VM display names may contain spaces, which are invalid in label values; this
	// annotation preserves the raw key needed for VMwareMachine lookups and retry detection.
	OriginalVMNameAnnotation = "vjailbreak.k8s.pf9.io/original-vm-name"

	// RollingMigrationPlanFinalizer is the finalizer for rolling migration plan
	RollingMigrationPlanFinalizer = "rollingmigrationplan.k8s.pf9.io/finalizer"

	// BMConfigFinalizer is the finalizer for BMConfig
	BMConfigFinalizer = "bmconfig.k8s.pf9.io/finalizer"

	// VMwareClusterLabel is the label for vmware cluster
	VMwareClusterLabel = "vjailbreak.k8s.pf9.io/vmware-cluster"

	// ESXiNameLabel is the label for ESXi name
	ESXiNameLabel = "vjailbreak.k8s.pf9.io/esxi-name"

	// ClusterMigrationLabel is the label for cluster migration
	ClusterMigrationLabel = "vjailbreak.k8s.pf9.io/clustermigration"

	// RollingMigrationPlanLabel is the label for rolling migration plan
	RollingMigrationPlanLabel = "vjailbreak.k8s.pf9.io/rollingmigrationplan"

	// PostMigrationCompleteAnnotation is the annotation for tracking post-migration completion
	PostMigrationCompleteAnnotation = "vjailbreak.k8s.pf9.io/post-migration-complete"

	// PauseMigrationLabel is the label for pausing rolling migration plan
	PauseMigrationLabel = "vjailbreak.k8s.pf9.io/pause"

	// PauseMigrationValue is the value for pausing migration
	PauseMigrationValue = "true"

	// UserDataSecretKey is the key for user data secret
	UserDataSecretKey = "user-data"

	// CloudInitConfigKey is the key for cloud init config
	CloudInitConfigKey = "cloud-init-config"

	// RollingMigrationPlanValidationConfigKey is the key for rolling migration plan validation config
	RollingMigrationPlanValidationConfigKey = "validation-config"

	// NodeRoleMaster is the role of the master node
	NodeRoleMaster = "master"

	// InternalIPAnnotation is the annotation for internal IP
	InternalIPAnnotation = "k3s.io/internal-ip"

	// NumberOfDisksLabel is the label for number of disks
	NumberOfDisksLabel = "vjailbreak.k8s.pf9.io/disk-count"

	// OpenstackCredsFinalizer is the finalizer for openstack credentials
	OpenstackCredsFinalizer = "openstackcreds.k8s.pf9.io/finalizer" //nolint:gosec // not a password string

	// ClusterMigrationFinalizer is the finalizer for cluster migration
	ClusterMigrationFinalizer = "clustermigration.k8s.pf9.io/finalizer"

	// ESXIMigrationFinalizer is the finalizer for ESXi migration
	ESXIMigrationFinalizer = "esximigration.k8s.pf9.io/finalizer"

	// VMwareCredsFinalizer is the finalizer for vmware credentials
	VMwareCredsFinalizer = "vmwarecreds.k8s.pf9.io/finalizer" //nolint:gosec // not a password string

	// ArrayCredsFinalizer is the finalizer for storage array credentials
	ArrayCredsFinalizer = "arraycreds.k8s.pf9.io/finalizer" //nolint:gosec // not a password string

	// ESXiSSHCredsFinalizer is the finalizer for ESXi SSH credentials
	ESXiSSHCredsFinalizer = "esxisshcreds.k8s.pf9.io/finalizer" //nolint:gosec // not a password string

	// ESXiSSHCreds validation statuses
	ESXiSSHCredsStatusPending            = "Pending"
	ESXiSSHCredsStatusValidating         = "Validating"
	ESXiSSHCredsStatusSucceeded          = "Succeeded"
	ESXiSSHCredsStatusPartiallySucceeded = "PartiallySucceeded"
	ESXiSSHCredsStatusFailed             = "Failed"

	// ESXiSSHValidationConcurrency is the number of concurrent ESXi SSH validations
	ESXiSSHValidationConcurrency = 10

	// ArrayCreds phases
	ArrayCredsPhaseDiscovered            = "Discovered"
	ArrayCredsPhaseConfigured            = "Configured"
	ArrayCredsPhaseValidated             = "Validated"
	ArrayCredsPhaseFailed                = "Failed"
	ArrayCredsPhaseNeedsBackendSelection = "NeedsBackendSelection"

	// ArrayCreds validation statuses
	ArrayCredsStatusPending             = "Pending"
	ArrayCredsStatusSucceeded           = "Succeeded"
	ArrayCredsStatusFailed              = "Failed"
	ArrayCredsStatusAwaitingCredentials = "AwaitingCredentials"

	// VjailbreakNodePhaseVMCreating is the phase for creating VM
	VjailbreakNodePhaseVMCreating = vjailbreakv1alpha1.VjailbreakNodePhase("CreatingVM")

	// VjailbreakNodePhaseVMCreated is the phase for VM created
	VjailbreakNodePhaseVMCreated = vjailbreakv1alpha1.VjailbreakNodePhase("VMCreated")

	// VjailbreakNodePhaseDeleting is the phase for deleting
	VjailbreakNodePhaseDeleting = vjailbreakv1alpha1.VjailbreakNodePhase("Deleting")

	// VjailbreakNodePhaseNodeReady is the phase for node ready
	VjailbreakNodePhaseNodeReady = vjailbreakv1alpha1.VjailbreakNodePhase("Ready")

	// VjailbreakNodePhaseError is the phase for node in error state
	VjailbreakNodePhaseError = vjailbreakv1alpha1.VjailbreakNodePhase("Error")

	// NamespaceMigrationSystem is the namespace for migration system
	NamespaceMigrationSystem = "migration-system"

	// VjailbreakMasterNodeName is the name of the vjailbreak master node
	VjailbreakMasterNodeName = "vjailbreak-master"

	// VjailbreakNodeFinalizer is the finalizer for vjailbreak node
	VjailbreakNodeFinalizer = "vjailbreak.k8s.pf9.io/finalizer"

	// K3sTokenFileLocation is the location of the k3s token file
	K3sTokenFileLocation = "/etc/pf9/k3s/token" //nolint:gosec // not a password string

	// CredsRequeueAfter is the time to requeue after
	CredsRequeueAfter = 1 * time.Minute

	// OpenstackCredsRequeueAfter is the time to requeue after.
	OpenstackCredsRequeueAfterMinutes = 60

	// VMwareCredsRequeueAfter is the time to requeue after.
	VMwareCredsRequeueAfterMinutes = 60

	// ENVFileLocation is the location of the env file
	ENVFileLocation = "/etc/pf9/k3s.env"

	// MigrationTriggerDelay is the delay for migration trigger
	MigrationTriggerDelay = 5 * time.Second

	// MigrationReason is the reason for migration
	MigrationReason = "Migration"

	// StartCutOverYes is the value for start cut over yes
	StartCutOverYes = "yes"

	// StartCutOverNo is the value for start cut over no
	StartCutOverNo = "no"

	// PCDClusterNameNoCluster is the name of the PCD cluster when there is no cluster
	PCDClusterNameNoCluster = "NO CLUSTER"

	// RDMDiskControllerName is the name of the RDM disk controller
	RDMDiskControllerName = "rdmdisk-controller"

	// VCenterVMScanConcurrencyLimit is the limit for concurrency while scanning vCenter VMs
	VCenterVMScanConcurrencyLimit = 100

	// VMwareClusterNameStandAloneESX is the name of the VMware cluster when there is no cluster
	VMwareClusterNameStandAloneESX = "NO CLUSTER"

	// ConfigMap default values
	ChangedBlocksCopyIterationThreshold = 20

	PeriodicSyncInterval = "1h"

	// VMActiveWaitIntervalSeconds is the interval to wait for vm to become active
	VMActiveWaitIntervalSeconds = 20

	// VMActiveWaitRetryLimit is the number of retries to wait for vm to become active
	VMActiveWaitRetryLimit = 15

	// VolumeAvailableWaitIntervalSeconds is the interval to wait for volume to become available
	VolumeAvailableWaitIntervalSeconds = 5

	// VolumeAvailableWaitRetryLimit is the number of retries to wait for volume to become available
	VolumeAvailableWaitRetryLimit = 15

	// VCenterScanConcurrencyLimit is the max number of vcenter scan pods
	VCenterScanConcurrencyLimit = 100

	// CleanupVolumesAfterConvertFailure is the default value for cleanup volumes after convert failure
	CleanupVolumesAfterConvertFailure = true

	// CleanupPortsAfterMigrationFailure is the default value for cleanup ports after migration failure
	CleanupPortsAfterMigrationFailure = false

	// PopulateVMwareMachineFlavors is the default value for populate vmware machine flavors
	PopulateVMwareMachineFlavors = true

	// ValidateRDMOwnerVMs is the default value for RDM owner VM validation
	ValidateRDMOwnerVMs = true

	// MigrationPlan status message prefix
	MigrationPlanValidationFailedPrefix = "Migration plan validation failed"

	// ValidationStatusFailed is the status value for failed validation
	ValidationStatusFailed = "Failed"
	// ValidationStatusRevalidating is the status value while credential revalidation is in progress
	ValidationStatusRevalidating = "Revalidating"

	// VjailbreakSettingsConfigMapName is the name of the vjailbreak settings configmap
	VjailbreakSettingsConfigMapName = "vjailbreak-settings"

	MaxRetries = 3
	RetryCap   = "3h"

	// HTTPTimeoutSeconds is the default HTTP timeout in seconds
	HTTPTimeoutSeconds = 30
	// HTTPTimeoutSecondsKey is the configmap/env key for HTTP timeout
	HTTPTimeoutSecondsKey = "HTTP_TIMEOUT_SECONDS"

	// ConfigMap settings keys
	// ValidateRDMOwnerVMsKey is the key for enabling/disabling RDM owner VM validation
	ValidateRDMOwnerVMsKey = "VALIDATE_RDM_OWNER_VMS"

	// AgentHostEntriesKey is the ConfigMap key for custom host entries injected into agent node VMs
	AgentHostEntriesKey = "AGENT_HOST_ENTRIES"

	// AutoPXEBootOnConversionDefault is the default value for automatic PXE boot during cluster conversion
	AutoPXEBootOnConversionDefault = false
	// AutoPXEBootOnConversionKey is the key for enabling/disabling automatic PXE boot during cluster conversion
	AutoPXEBootOnConversionKey = "AUTO_PXE_BOOT_ON_CONVERSION"

	// AnnotationValueTrue is the string value "true" used for annotations
	AnnotationValueTrue = "true"

	// v2v-helper specific constants
	HotplugCPUKey       = "HOTPLUG_CPU"
	HotplugMemoryKey    = "HOTPLUG_MEMORY"
	HotplugCPUMaxKey    = "HOTPLUG_CPU_MAX"
	HotplugMemoryMaxKey = "HOTPLUG_MEMORY_MAX"

	// Number of intervals to wait for the volume to become available
	MaxIntervalCount = 60

	// Retry attempts for delete operations (port, volume) during cleanup.
	// Kept small — cleanup should not block indefinitely, but must survive transient API errors.
	DeleteOperationRetryCount           = 5
	DeleteOperationRetryIntervalSeconds = 5

	InspectOSCommand      = "inspect-os"
	LSBootCommand         = "ls /boot"
	XMLFileName           = "libxml.xml"
	MigrationSnapshotName = "migration-snap"
	MaxHTTPRetryCount     = 5
	MaxVMActiveCheckCount = 15
	VMActiveCheckInterval = 20 * time.Second
	TrueString            = "true"

	LogsDir = "/var/log/pf9"

	EventMessageConvertingDisk                    = "Converting disk"
	EventMessageWaitingForCutOverStart            = "Waiting for VM Cutover start time"
	EventMessageCopyingChangedBlocksWithIteration = "Copying changed blocks"
	EventMessageWaitingForDataCopyStart           = "Waiting for data copy start time"
	EventMessageDataCopyStart                     = "Data copy start time reached"
	EventMessageWaitingForAdminCutOver            = "Waiting for Admin Cutover conditions to be met"
	EventMessagePeriodicSyncWarning               = "Periodic Sync: In WARNING state - manual intervention required"
	EventMessageMigrationSucessful                = "VM created successfully"
	EventMessageMigrationFailed                   = "Trying to perform cleanup"
	EventMessageCopyingDisk                       = "Copying disk"
	EventMessageFailed                            = "Failed to"
	EventDisconnect                               = "Disconnected network interfaces"

	// StorageAcceleratedCopy specific event messages
	EventMessageEsxiSSHConnect                       = "Connecting to ESXi"
	EventMessageEsxiSSHTest                          = "Testing ESXi connection"
	EventMessageEsxiConnected                        = "Connected to ESXi"
	EventMessageInitiatorGroup                       = "Creating/updating initiator group"
	EventMessageStorageAcceleratedCopyCreatingVolume = "Creating target volume"
	EventMessageStorageAcceleratedCopyCinderManage   = "Cinder managing the volume"
	EventMessageStorageAcceleratedCopyMappingVolume  = "Mapping target volume"
	EventMessageStorageAcceleratedCopyRescanStorage  = "Waiting for target volume"
	EventMessageStorageAcceleratedCopyTargetDevice   = "Target device is visible:"

	// PeriodicSyncMaxRetries is the max number of retries for CBT sync
	PeriodicSyncMaxRetries = 3

	// PeriodicSyncRetryCap is the max retry interval for CBT sync
	PeriodicSyncRetryCap = "3h"

	// ESXiSSHSecretName is the name of the Kubernetes secret containing ESXi SSH private key
	ESXiSSHSecretName = "esxi-ssh-key"

	// AutoFstabUpdate is the default value for automatic fstab update
	AutoFstabUpdate = false
	// AutoFstabUpdateKey is the key for enabling/disabling automatic fstab update
	AutoFstabUpdateKey = "AUTO_FSTAB_UPDATE"

	// StorageCopyMethod is the default value for storage copy method
	StorageCopyMethod = "StorageAcceleratedCopy"

	// MaxPowerOffRetryLimit is the max number of retries for power off status check
	MaxPowerOffRetryLimit = 3

	// PowerOffRetryCap is the max retry interval for power off status check
	PowerOffRetryCap = 5 * time.Minute

	// V2VHelperPodCPURequest is the default CPU request for v2v-helper pod
	V2VHelperPodCPURequest = "1000m"
	// V2VHelperPodCPURequestKey is the key for v2v-helper pod CPU request
	V2VHelperPodCPURequestKey = "V2V_HELPER_POD_CPU_REQUEST"

	// V2VHelperPodMemoryRequest is the default memory request for v2v-helper pod
	V2VHelperPodMemoryRequest = "1Gi"
	// V2VHelperPodMemoryRequestKey is the key for v2v-helper pod memory request
	V2VHelperPodMemoryRequestKey = "V2V_HELPER_POD_MEMORY_REQUEST"

	// V2VHelperPodCPULimit is the default CPU limit for v2v-helper pod
	V2VHelperPodCPULimit = "2000m"
	// V2VHelperPodCPULimitKey is the key for v2v-helper pod CPU limit
	V2VHelperPodCPULimitKey = "V2V_HELPER_POD_CPU_LIMIT"

	// V2VHelperPodMemoryLimit is the default memory limit for v2v-helper pod
	V2VHelperPodMemoryLimit = "3Gi"
	// V2VHelperPodMemoryLimitKey is the key for v2v-helper pod memory limit
	V2VHelperPodMemoryLimitKey = "V2V_HELPER_POD_MEMORY_LIMIT"

	// V2VHelperPodEphemeralStorageRequest is the default ephemeral storage request for v2v-helper pod
	V2VHelperPodEphemeralStorageRequest = "3Gi"
	// V2VHelperPodEphemeralStorageRequestKey is the key for v2v-helper pod ephemeral storage request
	V2VHelperPodEphemeralStorageRequestKey = "V2V_HELPER_POD_EPHEMERAL_STORAGE_REQUEST"

	// V2VHelperPodEphemeralStorageLimit is the default ephemeral storage limit for v2v-helper pod
	V2VHelperPodEphemeralStorageLimit = "3Gi"
	// V2VHelperPodEphemeralStorageLimitKey is the key for v2v-helper pod ephemeral storage limit
	V2VHelperPodEphemeralStorageLimitKey = "V2V_HELPER_POD_EPHEMERAL_STORAGE_LIMIT"

	RhelFirstBootScript = `#!/bin/bash
set -e
LOG_FILE="/var/log/network_fix.log"
echo "$(date '+%Y-%m-%d %H:%M:%S') - Starting network fix script" >> "$LOG_FILE"

if ! systemctl is-active NetworkManager >/dev/null 2>&1; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - NetworkManager not active, attempting to start" >> "$LOG_FILE"
    systemctl start NetworkManager
    if ! systemctl is-active NetworkManager >/dev/null 2>&1; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Failed to start NetworkManager, exiting" >> "$LOG_FILE"
        exit 1
    fi
fi
echo "$(date '+%Y-%m-%d %H:%M:%S') - NetworkManager is active" >> "$LOG_FILE"

nmcli con reload || {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Reload failed, restarting NM" >> "$LOG_FILE"
    systemctl restart NetworkManager
    sleep 5
    nmcli con reload || echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Reload still failed" >> "$LOG_FILE"
}

OLD_CONNS=$(nmcli -t -f NAME,TYPE connection show | grep -v ':loopback' | cut -d: -f1)
if [ -z "$OLD_CONNS" ]; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - No existing connections found" >> "$LOG_FILE"
else
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Found connections: $OLD_CONNS" >> "$LOG_FILE"
fi

for conn in $OLD_CONNS; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Processing connection: $conn" >> "$LOG_FILE"
    nmcli con mod "$conn" ipv4.method auto ipv4.addresses "" ipv4.gateway "" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to modify IPv4 for $conn" >> "$LOG_FILE"
    nmcli con mod "$conn" ipv6.method auto ipv6.addresses "" ipv6.gateway "" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to modify IPv6 for $conn" >> "$LOG_FILE"
    nmcli con up "$conn" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Failed to activate $conn" >> "$LOG_FILE"
done

NEW_IFACES=$(ip link show | grep -o '^[0-9]\+: [a-zA-Z0-9]\+:' | cut -d ' ' -f2 | cut -d ':' -f1 | grep -v lo)
if [ -z "$NEW_IFACES" ]; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - No new interfaces detected" >> "$LOG_FILE"
else
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Detected interfaces: $NEW_IFACES" >> "$LOG_FILE"
fi

for iface in $NEW_IFACES; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Processing new interface: $iface" >> "$LOG_FILE"
    conn_name="$iface"
    if ! nmcli con show "$conn_name" >/dev/null 2>&1; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Creating new connection for $iface" >> "$LOG_FILE"
        nmcli con add type ethernet con-name "$conn_name" ifname "$iface" ipv4.method auto ipv6.method auto 2>>"$LOG_FILE" || \
            echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to create connection for $iface" >> "$LOG_FILE"
    fi
    nmcli con up "$conn_name" 2>>"$LOG_FILE" || \
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to activate $iface" >> "$LOG_FILE"
done
echo "$(date '+%Y-%m-%d %H:%M:%S') - Network fix script completed" >> "$LOG_FILE"`

	// LinuxVMwareToolsCleanupScript is the firstboot script that removes VMware Tools from Linux guests.
	// Supports: apt (Debian/Ubuntu), zypper (SUSE/openSUSE), dnf (RHEL 8+), yum (RHEL 6/7).
	// Supports: systemd and SysV init (pre-systemd distros like SUSE 11, RHEL 5/6).
	// Compatible with Bash 3.1+ (no mapfile, no process substitution).
	LinuxVMwareToolsCleanupScript = `#!/bin/bash
# VMware Tools Cleanup Script for Linux
# This script removes leftover VMware Tools files after VM migration to OpenStack
# Run as root during firstboot
#
# Compatibility: Bash 3.1+, systemd and SysV init, apt/dnf/yum/zypper

set -e

if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: This script must be run as root"
    exit 1
fi

LOG_FILE="/var/log/vmware-tools-cleanup.log"

if [ ! -d "$(dirname "$LOG_FILE")" ]; then
    mkdir -p "$(dirname "$LOG_FILE")" || {
        echo "ERROR: Cannot create log directory $(dirname "$LOG_FILE")"
        exit 1
    }
fi

if [ ! -w "$(dirname "$LOG_FILE")" ]; then
    echo "ERROR: No write permission for log directory $(dirname "$LOG_FILE")"
    exit 1
fi

log() {
    local level="$1"
    local message="$2"
    local timestamp
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

# Detect init system once
HAS_SYSTEMD=false
if command -v systemctl > /dev/null 2>&1 && systemctl --version > /dev/null 2>&1; then
    HAS_SYSTEMD=true
fi

stop_and_disable_service() {
    local service="$1"
    if $HAS_SYSTEMD; then
        if systemctl is-active --quiet "$service" 2>> "$LOG_FILE"; then
            log "INFO" "Stopping service: $service"
            if systemctl stop "$service" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully stopped: $service"
            else
                log "WARNING" "Failed to stop: $service"
            fi
        else
            log "INFO" "Service not running (skipping): $service"
        fi
        if systemctl list-unit-files 2>> "$LOG_FILE" | grep -q "$service"; then
            if systemctl disable "$service" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully disabled: $service"
            else
                log "WARNING" "Failed to disable: $service"
            fi
        fi
    elif [ -f "/etc/init.d/$service" ]; then
        if "/etc/init.d/$service" status > /dev/null 2>&1; then
            log "INFO" "Stopping service (SysV): $service"
            if "/etc/init.d/$service" stop 2>> "$LOG_FILE"; then
                log "INFO" "Successfully stopped: $service"
            else
                log "WARNING" "Failed to stop: $service"
            fi
        else
            log "INFO" "Service not running (skipping): $service"
        fi
        if command -v chkconfig > /dev/null 2>&1; then
            chkconfig "$service" off 2>> "$LOG_FILE" && log "INFO" "Disabled (chkconfig): $service" || log "WARNING" "Failed to disable: $service"
        elif command -v update-rc.d > /dev/null 2>&1; then
            update-rc.d "$service" disable 2>> "$LOG_FILE" && log "INFO" "Disabled (update-rc.d): $service" || log "WARNING" "Failed to disable: $service"
        fi
    else
        log "INFO" "Service not found (skipping): $service"
    fi
}

log "INFO" "=== VMware Tools Cleanup Started ==="

log "INFO" "Stopping VMware services..."
for service in vmware vmware-tools vmtoolsd open-vm-tools; do
    stop_and_disable_service "$service"
done

log "INFO" "Removing VMware packages..."
VMWARE_PACKAGES="open-vm-tools vmware-tools-core vmware-tools"

if command -v apt-get > /dev/null 2>&1 && apt-get --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
        if dpkg -l "$pkg" 2>> "$LOG_FILE" | grep -q "^ii"; then
            log "INFO" "Purging package: $pkg"
            if apt-get purge -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully purged package: $pkg"
            else
                log "WARNING" "Failed to purge package: $pkg"
            fi
        fi
    done
elif command -v zypper > /dev/null 2>&1 && zypper --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
        if rpm -q "$pkg" > /dev/null 2>&1; then
            log "INFO" "Removing package (zypper): $pkg"
            if zypper --non-interactive remove "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
elif command -v dnf > /dev/null 2>&1 && dnf --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
        if rpm -q "$pkg" > /dev/null 2>&1; then
            log "INFO" "Removing package (dnf): $pkg"
            if dnf remove -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
elif command -v yum > /dev/null 2>&1 && yum --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
        if rpm -q "$pkg" > /dev/null 2>&1; then
            log "INFO" "Removing package (yum): $pkg"
            if yum remove -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
else
    log "INFO" "No supported package manager found, skipping package removal"
fi

log "INFO" "Removing VMware directories..."
for dir in /etc/vmware-tools /var/lib/vmware /usr/lib/vmware-tools /usr/lib/open-vm-tools; do
    if [ -d "$dir" ]; then
        log "INFO" "Removing directory: $dir"
        if rm -rf "$dir"; then
            log "INFO" "Successfully removed: $dir"
        else
            log "WARNING" "Failed to remove: $dir"
        fi
    else
        log "INFO" "Directory not found (skipping): $dir"
    fi
done

log "INFO" "Removing VMware log files from /var/log/..."
_tmpfile=$(mktemp /tmp/vmware-cleanup-XXXXXX)
find /var/log -maxdepth 1 -type f \( -name "vmware-*" -o -name "*vmtools*" -o -name "*vm-tools*" \) > "$_tmpfile" 2>> "$LOG_FILE" || true
if [ -s "$_tmpfile" ]; then
    while IFS= read -r logfile; do
        log "INFO" "Removing log file: $logfile"
        if rm -f "$logfile"; then
            log "INFO" "Successfully removed: $logfile"
        else
            log "WARNING" "Failed to remove: $logfile"
        fi
    done < "$_tmpfile"
else
    log "INFO" "No VMware log files found in /var/log/"
fi
rm -f "$_tmpfile"

log "INFO" "=== VMware Tools Cleanup Completed ==="
log "INFO" "Log file saved to: $LOG_FILE"

exit 0`

	WindowsPersistFirstBootScript = `
	@echo off
setlocal EnableDelayedExpansion

:: ────────────────────────────────────────────────
::  Configuration
:: ────────────────────────────────────────────────
set "PS_SCRIPT=C:\NIC-Recovery\Orchestrate-NICRecovery.ps1"
set "LOGDIR=C:\NIC-Recovery"
set "LOGFILE=%LOGDIR%\NIC-Recovery-Orchestrate_%DATE:~-4%%DATE:~3,2%%DATE:~0,2%_%TIME:~0,2%%TIME:~3,2%.log"

:: Replace space in time with zero if hour < 10
set "LOGFILE=%LOGFILE: =0%"

:: ────────────────────────────────────────────────
::  Create log directory if missing
:: ────────────────────────────────────────────────
if not exist "%LOGDIR%\" (
    mkdir "%LOGDIR%" 2>nul
    if errorlevel 1 (
        echo ERROR: Cannot create log directory %LOGDIR%
        pause
        exit /b 1
    )
)

:: ────────────────────────────────────────────────
::  Header in log
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] ============================================== >> "%LOGFILE%"
echo [%DATE% %TIME%] Starting NIC Recovery Orchestration           >> "%LOGFILE%"
echo [%DATE% %TIME%] Script: %PS_SCRIPT%                           >> "%LOGFILE%"
echo [%DATE% %TIME%] Computer: %COMPUTERNAME%                      >> "%LOGFILE%"
echo [%DATE% %TIME%] User:     %USERNAME%                          >> "%LOGFILE%"
echo [%DATE% %TIME%] ============================================== >> "%LOGFILE%"

:: ────────────────────────────────────────────────
::  Check if PowerShell script exists
:: ────────────────────────────────────────────────
if not exist "%PS_SCRIPT%" (
    echo [%DATE% %TIME%] ERROR: PowerShell script not found at:     >> "%LOGFILE%"
    echo [%DATE% %TIME%]        %PS_SCRIPT%                         >> "%LOGFILE%"
    echo.
    echo ERROR: Script not found: %PS_SCRIPT%
    echo        Check path and try again.
    echo.
    pause
    exit /b 1
)

:: ────────────────────────────────────────────────
::  Self-elevate to Administrator if not already
:: ────────────────────────────────────────────────
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [%DATE% %TIME%] Requesting administrator rights...         >> "%LOGFILE%"
    echo.
    echo Requesting admin rights ─ please accept the UAC prompt...
    echo.

    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "Start-Process cmd -ArgumentList '/c %~f0' -Verb RunAs" 2>nul

    exit /b
)

:: ────────────────────────────────────────────────
::  Now we are elevated ─ run the real PowerShell script
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] Running PowerShell script as Administrator...  >> "%LOGFILE%"
echo.                                                            >> "%LOGFILE%"

powershell.exe -NoProfile -ExecutionPolicy Bypass ^
    -Command "& '%PS_SCRIPT%' *>> '%LOGFILE%' 2>&1"

set PS_EXITCODE=%errorlevel%

echo.                                                            >> "%LOGFILE%"
echo [%DATE% %TIME%] PowerShell script finished.                  >> "%LOGFILE%"
echo [%DATE% %TIME%] Exit code: !PS_EXITCODE!                     >> "%LOGFILE%"

if !PS_EXITCODE! equ 0 (
    echo [%DATE% %TIME%] Result: SUCCESS                              >> "%LOGFILE%"
    echo.
    echo NIC Recovery orchestration completed.
    echo Log saved to:
    echo   %LOGFILE%
) else (
    echo [%DATE% %TIME%] Result: FAILED (exit code !PS_EXITCODE!)     >> "%LOGFILE%"
    echo.
    echo NIC Recovery script FAILED (exit code !PS_EXITCODE!).
    echo Check the log for details:
    echo   %LOGFILE%
)

echo.
echo [%DATE% %TIME%] Finished. Press any key to exit...           >> "%LOGFILE%"
pause >nul
exit /b !PS_EXITCODE!
	`
)

// CloudInitScript contains the cloud-init script for VM initialization
var (
	K3sCloudInitScript = `#cloud-config
write_files:
- path: %s
  content: |
    export IS_MASTER=%s
    export MASTER_IP=%s
    export K3S_TOKEN=%s
runcmd:
  - echo "Created k3s env variables!" > /home/ubuntu/cloud-init.log
`

	// MigrationConditionTypeDataCopy represents the condition type for data copy phase
	MigrationConditionTypeDataCopy corev1.PodConditionType = "DataCopy"

	// MigrationConditionTypeMigrating represents the condition type for migrating phase
	MigrationConditionTypeMigrating corev1.PodConditionType = "Migrating"

	// MigrationConditionTypeValidated represents the condition type for validated phase
	MigrationConditionTypeValidated corev1.PodConditionType = "Validated"
	MigrationConditionTypeFailed    corev1.PodConditionType = "Failed"

	// MigrationConditionTypeStorageAcceleratedCopy represents the condition type for StorageAcceleratedCopy phases
	MigrationConditionTypeStorageAcceleratedCopy corev1.PodConditionType = "StorageAcceleratedCopy"

	// MigrationConditionTypeMigrated represents the condition type for successful completion
	MigrationConditionTypeMigrated corev1.PodConditionType = "Migrated"

	// VMMigrationStatesEnum is a map of migration phase to state
	VMMigrationStatesEnum = map[vjailbreakv1alpha1.VMMigrationPhase]int{
		vjailbreakv1alpha1.VMMigrationPhasePending:               0,
		vjailbreakv1alpha1.VMMigrationPhaseValidating:            1,
		vjailbreakv1alpha1.VMMigrationPhaseValidationFailed:      2,
		vjailbreakv1alpha1.VMMigrationPhaseFailed:                3,
		vjailbreakv1alpha1.VMMigrationPhaseAwaitingDataCopyStart: 4,
		// StorageAcceleratedCopy XCOPY specific phases (numbered to fit between AwaitingDataCopyStart and Copying)
		vjailbreakv1alpha1.VMMigrationPhaseConnectingToESXi:       5,
		vjailbreakv1alpha1.VMMigrationPhaseCreatingInitiatorGroup: 6,
		vjailbreakv1alpha1.VMMigrationPhaseCreatingVolume:         7,
		vjailbreakv1alpha1.VMMigrationPhaseImportingToCinder:      8,
		vjailbreakv1alpha1.VMMigrationPhaseMappingVolume:          9,
		vjailbreakv1alpha1.VMMigrationPhaseRescanningStorage:      10,
		// Common phases to both the copy methods.
		vjailbreakv1alpha1.VMMigrationPhaseCopying:                  11,
		vjailbreakv1alpha1.VMMigrationPhaseAwaitingCutOverStartTime: 12,
		vjailbreakv1alpha1.VMMigrationPhaseAwaitingAdminCutOver:     13,
		// Post-cutover phases: these happen after admin triggers cutover
		vjailbreakv1alpha1.VMMigrationPhaseCopyingChangedBlocks: 14,
		vjailbreakv1alpha1.VMMigrationPhaseConvertingDisk:       15,
		vjailbreakv1alpha1.VMMigrationPhaseSucceeded:            16,
		vjailbreakv1alpha1.VMMigrationPhaseUnknown:              17,
	}

	// MigrationJobTTL is the TTL for migration job
	MigrationJobTTL int32 = 300

	// PCDCloudInitScript contains the cloud-init script for PCD onboarding
	PCDCloudInitScript = `#cloud-config

# Run the cloud-init script on boot
runcmd:
  - echo "Validating prerequisites..."
  - for cmd in curl cloud-ctl; do
      if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "Error: Required command '$cmd' is not installed. Please install it and retry." >&2
        exit 1
      fi
    done
  - echo "All prerequisites met. Proceeding with setup."
  
  - echo "Downloading and executing cloud-ctl setup script..."
  - curl -s https://cloud-ctl.s3.us-west-1.amazonaws.com/cloud-ctl-setup | bash
  - echo "Cloud-ctl setup script executed successfully."

  - echo "Configuring cloud-ctl..."
  - cloud-ctl config set \
      -u https://cloud-region1.platform9.io \
      -e admin@airctl.localnet \
      -r Region1 \
      -t service \
      -p 'xyz'
  - echo "Cloud-ctl configuration set successfully."

  - echo "Preparing the node..."
  - cloud-ctl prep-node
  - echo "Node preparation complete. Setup finished successfully."`
)

var (
	// RDMPhaseManaging is the phase for RDMDisk when it is being managed
	RDMPhaseManaging = "Managing"
	// RDMPhaseManaged is the phase for RDMDisk when it has been successfully managed
	RDMPhaseManaged = "Managed"
	// RDMPhaseError is the phase for RDMDisk when there is an error
	RDMPhaseError = "Error"
)
