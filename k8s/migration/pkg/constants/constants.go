package constants

import (
	"time"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	TerminationPeriod = int64(120)
	NameMaxLength     = 242

	VjailbreakNodeControllerName = "vjailbreaknode-controller"
	OpenstackCredsControllerName = "openstackcreds-controller" //nolint:gosec // not a password string
	VMwareCredsControllerName    = "vmwarecreds-controller"    //nolint:gosec // not a password string
	MigrationControllerName      = "migration-controller"

	K8sMasterNodeAnnotation = "node-role.kubernetes.io/control-plane"
	NodeRoleMaster          = "master"
	InternalIPAnnotation    = "k3s.io/internal-ip"

	NumberOfDisksLabel = "vjailbreak.k8s.pf9.io/disk-count"
	VMwareCredsLabel   = "vmwarecreds.k8s.pf9.io" //nolint:gosec // not a password string
	VMwareClusterLabel = "vjailbreak.k8s.pf9.io/vmware-cluster"

	OpenstackCredsFinalizer = "openstackcreds.k8s.pf9.io/finalizer" //nolint:gosec // not a password string
	VMwareCredsFinalizer    = "vmwarecreds.k8s.pf9.io/finalizer"    //nolint:gosec // not a password string

	VjailbreakNodePhaseVMCreating = vjailbreakv1alpha1.VjailbreakNodePhase("CreatingVM")
	VjailbreakNodePhaseVMCreated  = vjailbreakv1alpha1.VjailbreakNodePhase("VMCreated")
	VjailbreakNodePhaseDeleting   = vjailbreakv1alpha1.VjailbreakNodePhase("Deleting")
	VjailbreakNodePhaseNodeReady  = vjailbreakv1alpha1.VjailbreakNodePhase("Ready")

	NamespaceMigrationSystem = "migration-system"
	VjailbreakMasterNodeName = "vjailbreak-master"
	VjailbreakNodeFinalizer  = "vjailbreak.k8s.pf9.io/finalizer"

	K3sTokenFileLocation = "/etc/pf9/k3s/token" //nolint:gosec // not a password string
	ENVFileLocation      = "/etc/pf9/k3s.env"

	MigrationTriggerDelay      = 5 * time.Second
	VMwareCredsRequeueAfter    = 1 * time.Minute
	OpenstackCredsRequeueAfter = 1 * time.Minute

	MigrationReason = "Migration"
	StartCutOverYes = "yes"
	StartCutOverNo  = "no"

	MaxVCPUs = 99999
	MaxRAM   = 99999
)

var (
	// Cloud-Init Script (User Data)
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

	MigrationConditionTypeDataCopy  corev1.PodConditionType = "DataCopy"
	MigrationConditionTypeMigrating corev1.PodConditionType = "Migrating"
	MigrationConditionTypeValidated corev1.PodConditionType = "Validated"
	MigrationConditionTypeFailed    corev1.PodConditionType = "Failed"

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

	MigrationJobTTL int32 = 300
)
