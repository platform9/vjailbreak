package constants

import vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"

const (
	VjailbreakNodeControllerName = "vjailbreaknode-controller"
	K8sMasterNodeAnnotation      = "node-role.kubernetes.io/master"
	NodeRoleMaster               = "master"
	InternalIPAnnotation         = "k3s.io/internal-ip"

	VjailbreakNodePhaseCreated = vjailbreakv1alpha1.VjailbreakNodePhase("Created")
)
