package constants

import (
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	TerminationPeriod = int64(120)
	NameMaxLength     = 242

	VjailbreakNodeControllerName = "vjailbreaknode-controller"
	K8sMasterNodeAnnotation      = "node-role.kubernetes.io/master"
	NodeRoleMaster               = "master"
	InternalIPAnnotation         = "k3s.io/internal-ip"
	NumberOfDisksLabel           = "vjailbreak.k8s.pf9.io/disk-count"

	VjailbreakNodePhaseVMCreating  = vjailbreakv1alpha1.VjailbreakNodePhase("CreatingVM")
	VjailbreakNodePhaseVMCreated   = vjailbreakv1alpha1.VjailbreakNodePhase("VMCreated")
	VjailbreakNodePhaseNodeCreated = vjailbreakv1alpha1.VjailbreakNodePhase("NodeCreated")

	NamespaceMigrationSystem = "migration-system"
	MasterVjailbreakNodeName = "vjailbreak-master"
	VjailbreakNodeFinalizer  = "vjailbreak.k8s.pf9.io/finalizer"

	K3sTokenFileLocation = "/etc/pf9/k3s/token" //nolint:gosec // not a password string
	ENVFileLocation      = "/etc/pf9/k3s.env"

	MigrationReason = "Migration"
	StartCutOverYes = "yes"
	StartCutOverNo  = "no"
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
	MigrationConditionTypeMigrated  corev1.PodConditionType = "Migrated"
	MigrationConditionTypeValidated corev1.PodConditionType = "validated"

	StatesEnum = map[vjailbreakv1alpha1.MigrationPhase]int{
		vjailbreakv1alpha1.MigrationPhasePending:                  0,
		vjailbreakv1alpha1.MigrationPhaseValidating:               1,
		vjailbreakv1alpha1.MigrationPhaseValidated:                2,
		vjailbreakv1alpha1.MigrationPhaseAwaitingDataCopyStart:    3,
		vjailbreakv1alpha1.MigrationPhaseCopying:                  4,
		vjailbreakv1alpha1.MigrationPhaseCopyingChangedBlocks:     5,
		vjailbreakv1alpha1.MigrationPhaseConvertingDisk:           6,
		vjailbreakv1alpha1.MigrationPhaseAwaitingCutOverStartTime: 7,
		vjailbreakv1alpha1.MigrationPhaseAwaitingAdminCutOver:     8,
		vjailbreakv1alpha1.MigrationPhaseSucceeded:                9,
		vjailbreakv1alpha1.MigrationPhaseFailed:                   10,
		vjailbreakv1alpha1.MigrationPhaseUnknown:                  11,
	}
)
