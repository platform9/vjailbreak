// Package constants provides constant values used throughout the migration system
package constants

import (
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// TerminationPeriod defines the grace period for pod termination in seconds
const (
	TerminationPeriod = int64(120)

	// NameMaxLength defines the maximum length of a name
	NameMaxLength = 242

	// VjailbreakNodeControllerName is the name of the vjailbreak node controller
	VjailbreakNodeControllerName = "vjailbreaknode-controller"

	// OpenstackCredsControllerName is the name of the openstack credentials controller
	OpenstackCredsControllerName = "openstackcreds-controller" //nolint:gosec // not a password string

	// VMwareCredsControllerName is the name of the vmware credentials controller
	VMwareCredsControllerName = "vmwarecreds-controller" //nolint:gosec // not a password string

	// MigrationControllerName is the name of the migration controller
	MigrationControllerName = "migration-controller"

	// RollingMigrationPlanControllerName is the name of the rolling migration plan controller
	RollingMigrationPlanControllerName = "rollingmigrationplan-controller"

	// ESXIMigrationControllerName is the name of the ESXi migration controller
	ESXIMigrationControllerName = "esximigration-controller"

	// ClusterMigrationControllerName is the name of the cluster migration controller
	ClusterMigrationControllerName = "clustermigration-controller"

	// K8sMasterNodeAnnotation is the annotation for k8s master node
	K8sMasterNodeAnnotation = "node-role.kubernetes.io/control-plane"

	// VMwareCredsLabel is the label for vmware credentials
	VMwareCredsLabel = "vmwarecreds.k8s.pf9.io" //nolint:gosec // not a password string

	// RollingMigrationPlanFinalizer is the finalizer for rolling migration plan
	RollingMigrationPlanFinalizer = "rollingmigrationplan.k8s.pf9.io/finalizer"

	// VMwareClusterLabel is the label for vmware cluster
	VMwareClusterLabel = "vjailbreak.k8s.pf9.io/vmware-cluster"

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

	// VjailbreakNodePhaseVMCreating is the phase for creating VM
	VjailbreakNodePhaseVMCreating = vjailbreakv1alpha1.VjailbreakNodePhase("CreatingVM")

	// VjailbreakNodePhaseVMCreated is the phase for VM created
	VjailbreakNodePhaseVMCreated = vjailbreakv1alpha1.VjailbreakNodePhase("VMCreated")

	// VjailbreakNodePhaseDeleting is the phase for deleting
	VjailbreakNodePhaseDeleting = vjailbreakv1alpha1.VjailbreakNodePhase("Deleting")

	// VjailbreakNodePhaseNodeReady is the phase for node ready
	VjailbreakNodePhaseNodeReady = vjailbreakv1alpha1.VjailbreakNodePhase("Ready")

	// NamespaceMigrationSystem is the namespace for migration system
	NamespaceMigrationSystem = "migration-system"

	// VjailbreakMasterNodeName is the name of the vjailbreak master node
	VjailbreakMasterNodeName = "vjailbreak-master"

	// VjailbreakNodeFinalizer is the finalizer for vjailbreak node
	VjailbreakNodeFinalizer = "vjailbreak.k8s.pf9.io/finalizer"

	// K3sTokenFileLocation is the location of the k3s token file
	K3sTokenFileLocation = "/etc/pf9/k3s/token" //nolint:gosec // not a password string

	// OpenstackCredsRequeueAfter is the requeue after time for openstack credentials
	OpenstackCredsRequeueAfter = 1 * time.Minute

	// VMwareCredsRequeueAfter is the requeue after time for vmware credentials
	VMwareCredsRequeueAfter = 1 * time.Minute

	// ENVFileLocation is the location of the env file
	ENVFileLocation = "/etc/pf9/k3s.env"

	// MigrationTriggerDelay is the delay for migration trigger
	MigrationTriggerDelay = 5 * time.Second

	// MigrationReason is the reason for migration
	MigrationReason = "Migration"

	// StartCutOverYes is the value for start cut over yes
	StartCutOverYes = "yes"

	// MaxVCPUs is the maximum number of vCPUs
	MaxVCPUs = 99999

	// MaxRAM is the maximum amount of RAM
	MaxRAM = 99999

	// StartCutOverNo is the value for start cut over no
	StartCutOverNo = "no"

	// RollingMigrationPlanPhaseWaiting is the phase for waiting
	RollingMigrationPlanPhaseWaiting vjailbreakv1alpha1.RollingMigrationPlanPhase = "Waiting"
	// RollingMigrationPlanPhaseRunning is the phase for running
	RollingMigrationPlanPhaseRunning vjailbreakv1alpha1.RollingMigrationPlanPhase = "Running"
	// RollingMigrationPlanPhaseFailed is the phase for failed
	RollingMigrationPlanPhaseFailed vjailbreakv1alpha1.RollingMigrationPlanPhase = "Failed"
	// RollingMigrationPlanPhaseSucceeded is the phase for succeeded
	RollingMigrationPlanPhaseSucceeded vjailbreakv1alpha1.RollingMigrationPlanPhase = "Succeeded"

	// ClusterMigrationPhaseWaiting is the phase for waiting
	ClusterMigrationPhaseWaiting vjailbreakv1alpha1.ClusterMigrationPhase = "Waiting"
	// ClusterMigrationPhaseRunning is the phase for running
	ClusterMigrationPhaseRunning vjailbreakv1alpha1.ClusterMigrationPhase = "Running"
	// ClusterMigrationPhaseFailed is the phase for failed
	ClusterMigrationPhaseFailed vjailbreakv1alpha1.ClusterMigrationPhase = "Failed"
	// ClusterMigrationPhaseSucceeded is the phase for succeeded
	ClusterMigrationPhaseSucceeded vjailbreakv1alpha1.ClusterMigrationPhase = "Succeeded"
)

// CloudInitScript contains the cloud-init script for VM initialization
var (
	CloudInitScript = `#cloud-config
password: %s
chpasswd: { expire: False }
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

	// StatesEnum is a map of migration phase to state
	StatesEnum = map[vjailbreakv1alpha1.MigrationPhase]int{
		vjailbreakv1alpha1.MigrationPhasePending:                  0,
		vjailbreakv1alpha1.MigrationPhaseValidating:               1,
		vjailbreakv1alpha1.MigrationPhaseFailed:                   2,
		vjailbreakv1alpha1.MigrationPhaseAwaitingDataCopyStart:    3,
		vjailbreakv1alpha1.MigrationPhaseCopying:                  4,
		vjailbreakv1alpha1.MigrationPhaseCopyingChangedBlocks:     5,
		vjailbreakv1alpha1.MigrationPhaseConvertingDisk:           6,
		vjailbreakv1alpha1.MigrationPhaseAwaitingCutOverStartTime: 7,
		vjailbreakv1alpha1.MigrationPhaseAwaitingAdminCutOver:     8,
		vjailbreakv1alpha1.MigrationPhaseSucceeded:                9,
		vjailbreakv1alpha1.MigrationPhaseUnknown:                  10,
	}

	// MigrationJobTTL is the TTL for migration job
	MigrationJobTTL int32 = 300
)
