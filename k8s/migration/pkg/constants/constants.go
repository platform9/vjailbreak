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
)

var (
	// Cloud-Init Script (User Data)
	CloudInitScript = `#cloud-config
write_files:
  - path: /home/ubuntu/test.txt
    content: |
		export MASTER_IP=%s
		export IS_MASTER=%s
runcmd:
  - echo "Cloud-Init Worked!" >> /home/ubuntu/cloud-init.log
`
)
