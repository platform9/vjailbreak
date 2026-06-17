package server

import (
	"encoding/json"
	"net/http"
	"path"

	"github.com/sirupsen/logrus"
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

	// Single connection — no reconnect, so the session stays alive for all listings.
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

	// List all datacenters.
	dcs, dcListErr := finder.DatacenterList(ctx, "*")
	if dcListErr != nil {
		logrus.Warnf("vcenter-resources: DatacenterList: %v", dcListErr)
	}
	for _, dc := range dcs {
		resp.Datacenters = append(resp.Datacenters, path.Base(dc.InventoryPath))
	}
	logrus.Infof("vcenter-resources: found %d datacenters", len(resp.Datacenters))

	// Pick which datacenter to scope to: prefer the one from credentials.
	scopeDC := preferredDC
	if scopeDC == "" && len(resp.Datacenters) > 0 {
		scopeDC = resp.Datacenters[0]
	}

	if scopeDC == "" {
		// Nothing more we can do without a datacenter.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Scope the SAME finder to the datacenter — no second login.
	dc, dcErr := finder.Datacenter(ctx, scopeDC)
	if dcErr != nil {
		logrus.Warnf("vcenter-resources: Datacenter(%q): %v", scopeDC, dcErr)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	finder.SetDatacenter(dc)
	logrus.Infof("vcenter-resources: scoped to datacenter %q", scopeDC)

	// Clusters.
	if clusters, clErr := finder.ClusterComputeResourceList(ctx, "*"); clErr == nil {
		for _, c := range clusters {
			if n := path.Base(c.InventoryPath); n != "" {
				resp.Clusters = append(resp.Clusters, n)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources: ClusterComputeResourceList: %v", clErr)
	}

	// Standalone hosts (added to Clusters so the UI can pick either).
	if hosts, hErr := finder.HostSystemList(ctx, "*"); hErr == nil {
		for _, h := range hosts {
			if n := path.Base(h.InventoryPath); n != "" {
				resp.Clusters = append(resp.Clusters, n)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources: HostSystemList: %v", hErr)
	}

	// Datastores.
	if datastores, dsErr := finder.DatastoreList(ctx, "*"); dsErr == nil {
		for _, ds := range datastores {
			if n := path.Base(ds.InventoryPath); n != "" {
				resp.Datastores = append(resp.Datastores, n)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources: DatastoreList: %v", dsErr)
	}
	logrus.Infof("vcenter-resources: found %d datastores", len(resp.Datastores))

	// Networks.
	if networks, netErr := finder.NetworkList(ctx, "*"); netErr == nil {
		for _, n := range networks {
			if name := path.Base(n.GetInventoryPath()); name != "" {
				resp.Networks = append(resp.Networks, name)
			}
		}
	} else {
		logrus.Warnf("vcenter-resources: NetworkList: %v", netErr)
	}
	logrus.Infof("vcenter-resources: found %d networks", len(resp.Networks))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
