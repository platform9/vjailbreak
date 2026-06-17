package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/sirupsen/logrus"
)

// CreateProxyVMRequest is the request body for POST /vpw/v1/create-proxy-vm.
type CreateProxyVMRequest struct {
	VMName         string `json:"vmName"`
	VMwareCredsRef string `json:"vmwareCredsRef"`
	Datacenter     string `json:"datacenter"`
	Datastore      string `json:"datastore"`
	Network        string `json:"network"`
	Cluster        string `json:"cluster,omitempty"`
}

// HandleCreateProxyVM accepts a deploy config, kicks off the full creation
// routine asynchronously, and returns 202 Accepted immediately.
func HandleCreateProxyVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateProxyVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.VMName == "" {
		http.Error(w, "vmName is required", http.StatusBadRequest)
		return
	}
	if req.VMwareCredsRef == "" {
		http.Error(w, "vmwareCredsRef is required", http.StatusBadRequest)
		return
	}
	if req.Datacenter == "" {
		http.Error(w, "datacenter is required", http.StatusBadRequest)
		return
	}
	if req.Datastore == "" {
		http.Error(w, "datastore is required", http.StatusBadRequest)
		return
	}
	if req.Network == "" {
		http.Error(w, "network is required", http.StatusBadRequest)
		return
	}

	ovaPath := filepath.Join(constants.ProxyVMOVADir, constants.ProxyVMOVAFileName)

	cfg := ProxyVMDeployConfig{
		VMName:         req.VMName,
		VMwareCredsRef: req.VMwareCredsRef,
		Datacenter:     req.Datacenter,
		Datastore:      req.Datastore,
		Network:        req.Network,
		Cluster:        req.Cluster,
	}

	go func() {
		logrus.Infof("create-proxy-vm: starting deploy for %q", req.VMName)
		deployProxyVMFromOVA(ovaPath, cfg)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "VM creation started — check ProxyVM resource for status",
	})
}
