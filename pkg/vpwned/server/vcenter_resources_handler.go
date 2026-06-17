package server

import (
	"encoding/json"
	"net/http"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi/object"
)

// VCenterResourcesResponse is the JSON response for GET /vpw/v1/vcenter-resources.
type VCenterResourcesResponse struct {
	Datacenters []string `json:"datacenters"`
	Clusters    []string `json:"clusters"`
	Datastores  []string `json:"datastores"`
	Networks    []string `json:"networks"`
}

// HandleVCenterResources lists available vCenter resources for a given set of
// VMware credentials.
//
// GET /vpw/v1/vcenter-resources?vmwareCredsRef=<name>
func HandleVCenterResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	credsRef := r.URL.Query().Get("vmwareCredsRef")
	if credsRef == "" {
		http.Error(w, "vmwareCredsRef query parameter is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	vcHost, vcUsername, vcPassword, preferredDC, err := credsFromVMwareCredsAll(ctx, credsRef)
	if err != nil {
		http.Error(w, "failed to get credentials: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Single connection throughout — no reconnect between listings.
	client, finder, err := connectVCenterNoDC(ctx, vcHost, vcUsername, vcPassword)
	if err != nil {
		http.Error(w, "failed to connect to vCenter: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Logout(ctx) //nolint:errcheck

	resp := VCenterResourcesResponse{
		Datacenters: []string{},
		Clusters:    []string{},
		Datastores:  []string{},
		Networks:    []string{},
	}

	// List all datacenters; keep the object we'll scope to.
	var selectedDC *object.Datacenter
	dcs, dcListErr := finder.DatacenterList(ctx, "*")
	if dcListErr != nil {
		logrus.Warnf("vcenter-resources[%s]: DatacenterList: %v", credsRef, dcListErr)
	}
	for _, dc := range dcs {
		name := path.Base(dc.InventoryPath)
		resp.Datacenters = append(resp.Datacenters, name)
		// Prefer the datacenter stored in credentials; otherwise use the first one.
		if selectedDC == nil || name == preferredDC {
			selectedDC = dc
		}
	}
	logrus.Infof("vcenter-resources[%s]: %d datacenter(s) found, scoping to %q",
		credsRef, len(resp.Datacenters), func() string {
			if selectedDC != nil {
				return path.Base(selectedDC.InventoryPath)
			}
			return "(none)"
		}())

	if selectedDC == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Scope the finder to the selected datacenter — same connection, no re-login.
	finder.SetDatacenter(selectedDC)

	// Clusters.
	if clusters, clErr := finder.ClusterComputeResourceList(ctx, "*"); clErr == nil {
		for _, c := range clusters {
			if n := path.Base(c.InventoryPath); n != "" {
				resp.Clusters = append(resp.Clusters, n)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources[%s]: ClusterComputeResourceList: %v", credsRef, clErr)
	}

	// Standalone hosts (merged into Clusters so the UI can pick either).
	if hosts, hErr := finder.HostSystemList(ctx, "*"); hErr == nil {
		for _, h := range hosts {
			if n := path.Base(h.InventoryPath); n != "" {
				resp.Clusters = append(resp.Clusters, n)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources[%s]: HostSystemList: %v", credsRef, hErr)
	}
	logrus.Infof("vcenter-resources[%s]: %d cluster/host(s)", credsRef, len(resp.Clusters))

	// Datastores.
	if datastores, dsErr := finder.DatastoreList(ctx, "*"); dsErr == nil {
		for _, ds := range datastores {
			if n := path.Base(ds.InventoryPath); n != "" {
				resp.Datastores = append(resp.Datastores, n)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources[%s]: DatastoreList: %v", credsRef, dsErr)
	}
	logrus.Infof("vcenter-resources[%s]: %d datastore(s)", credsRef, len(resp.Datastores))

	// Networks.
	if networks, netErr := finder.NetworkList(ctx, "*"); netErr == nil {
		for _, n := range networks {
			if name := path.Base(n.GetInventoryPath()); name != "" {
				resp.Networks = append(resp.Networks, name)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources[%s]: NetworkList: %v", credsRef, netErr)
	}
	logrus.Infof("vcenter-resources[%s]: %d network(s)", credsRef, len(resp.Networks))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
