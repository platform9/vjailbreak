package server

import (
	"encoding/json"
	"net/http"
	"sync"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	constants "github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OpenstackCredsDeletableResponse is the response for the openstackcreds-deletable check.
type OpenstackCredsDeletableResponse struct {
	CanDelete           bool     `json:"canDelete"`
	AgentNodeCount      int      `json:"agentNodeCount"`
	AgentNodeNames      []string `json:"agentNodeNames"`
	HasActiveMigrations bool     `json:"hasActiveMigrations"`
}

var (
	deletableOnce      sync.Once
	deletableClient    client.Client
	deletableClientErr error
)

// openstackCredsDeletableClientFunc returns the k8s client used by
// HandleCheckOpenstackCredsDeletable. It is a variable so tests can swap it
// without touching the sync.Once machinery.
var openstackCredsDeletableClientFunc = func() (client.Client, error) {
	deletableOnce.Do(func() {
		deletableClient, deletableClientErr = CreateInClusterClient()
	})
	return deletableClient, deletableClientErr
}

// HandleCheckOpenstackCredsDeletable checks whether an OpenstackCreds can be safely deleted
// based on the count of non-master VjailbreakNodes referencing it.
//
// GET /vpw/v1/openstackcreds-deletable?name=<credName>&namespace=<ns>
//
// Response: OpenstackCredsDeletableResponse
//   - canDelete: true when no non-master agent nodes reference this credential
//   - agentNodeCount: number of non-master nodes found
//   - agentNodeNames: names of those nodes
//   - hasActiveMigrations: true if any of those nodes have active migrations
func HandleCheckOpenstackCredsDeletable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	credName := r.URL.Query().Get("name")
	if credName == "" {
		http.Error(w, "name query parameter is required", http.StatusBadRequest)
		return
	}
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = constants.NamespaceMigrationSystem
	}

	k8sClient, err := openstackCredsDeletableClientFunc()
	if err != nil {
		logrus.WithError(err).Error("openstackcreds-deletable: failed to get k8s client")
		http.Error(w, "cluster unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	nodeList := &vjailbreakv1alpha1.VjailbreakNodeList{}
	if err := k8sClient.List(ctx, nodeList, client.InNamespace(namespace)); err != nil {
		logrus.WithError(err).Error("openstackcreds-deletable: failed to list VjailbreakNodes")
		http.Error(w, "failed to list nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	agentNodeNames := []string{}
	hasActiveMigrations := false

	for i := range nodeList.Items {
		node := nodeList.Items[i]
		if node.Spec.NodeRole == constants.NodeRoleMaster {
			continue
		}
		if node.Spec.OpenstackCreds.Name != credName {
			continue
		}
		agentNodeNames = append(agentNodeNames, node.Name)
		if len(node.Status.ActiveMigrations) > 0 {
			hasActiveMigrations = true
		}
	}

	resp := OpenstackCredsDeletableResponse{
		CanDelete:           len(agentNodeNames) == 0,
		AgentNodeCount:      len(agentNodeNames),
		AgentNodeNames:      agentNodeNames,
		HasActiveMigrations: hasActiveMigrations,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logrus.WithError(err).Error("openstackcreds-deletable: failed to encode response")
	}
}
