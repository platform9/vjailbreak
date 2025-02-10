package constants

import vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"

const (
	VjailbreakNodeControllerName = "vjailbreaknode-controller"
	K8sMasterNodeAnnotation      = "node-role.kubernetes.io/master"
	NodeRoleMaster               = "master"
	InternalIPAnnotation         = "k3s.io/internal-ip"

	VjailbreakNodePhaseVMCreating  = vjailbreakv1alpha1.VjailbreakNodePhase("CreatingVM")
	VjailbreakNodePhaseVMCreated   = vjailbreakv1alpha1.VjailbreakNodePhase("VMCreated")
	VjailbreakNodePhaseNodeCreated = vjailbreakv1alpha1.VjailbreakNodePhase("NodeCreated")

	NamespaceMigrationSystem = "migration-system"
	MasterVjailbreakNodeName = "vjailbreak-master"
	VjailbreakNodeFinalizer  = "vjailbreak.k8s.pf9.io/finalizer"

	K3sTokenFileLocation = "/etc/pf9/k3s/token" //nolint:gosec // not a password string
	ENVFileLocation      = "/etc/pf9/k3s.env"
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
)
