package server

import (
	"encoding/json"
	"net/http"
	"path"
)

// VCenterResourcesResponse is the JSON response for GET /vpw/v1/vcenter-resources.
type VCenterResourcesResponse struct {
	Datacenters []string `json:"datacenters"`
	Clusters    []string `json:"clusters"`
	Datastores  []string `json:"datastores"`
	Networks    []string `json:"networks"`
}

// HandleVCenterResources lists available vCenter resources (datacenters, clusters,
// datastores, networks) for a given set of VMware credentials.
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

	vcHost, vcUsername, vcPassword, datacenter, err := credsFromVMwareCredsAll(ctx, credsRef)
	if err != nil {
		http.Error(w, "failed to get credentials: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := VCenterResourcesResponse{
		Datacenters: []string{},
		Clusters:    []string{},
		Datastores:  []string{},
		Networks:    []string{},
	}

	// Connect without a datacenter set so we can list all datacenters first.
	clientNoDC, finderNoDC, err := connectVCenterNoDC(ctx, vcHost, vcUsername, vcPassword)
	if err != nil {
		http.Error(w, "failed to connect to vCenter: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientNoDC.Logout(ctx)

	if dcs, listErr := finderNoDC.DatacenterList(ctx, "*"); listErr == nil {
		for _, dc := range dcs {
			resp.Datacenters = append(resp.Datacenters, path.Base(dc.InventoryPath))
		}
	}

	// Scope subsequent queries to the datacenter stored in the credentials (if any).
	if datacenter == "" && len(resp.Datacenters) > 0 {
		datacenter = resp.Datacenters[0]
	}
	clientNoDC.Logout(ctx) //nolint:errcheck

	if datacenter != "" {
		_, finder, connErr := connectVCenter(ctx, vcHost, vcUsername, vcPassword, datacenter)
		if connErr == nil {
			if clusters, clErr := finder.ClusterComputeResourceList(ctx, "*"); clErr == nil {
				for _, c := range clusters {
					if n := path.Base(c.InventoryPath); n != "" {
						resp.Clusters = append(resp.Clusters, n)
					}
				}
			}
			if hosts, hErr := finder.HostSystemList(ctx, "*"); hErr == nil {
				for _, h := range hosts {
					if n := path.Base(h.InventoryPath); n != "" {
						resp.Clusters = append(resp.Clusters, n)
					}
				}
			}
			if datastores, dsErr := finder.DatastoreList(ctx, "*"); dsErr == nil {
				for _, ds := range datastores {
					if n := path.Base(ds.InventoryPath); n != "" {
						resp.Datastores = append(resp.Datastores, n)
					}
				}
			}
			if networks, netErr := finder.NetworkList(ctx, "*"); netErr == nil {
				for _, n := range networks {
					if name := path.Base(n.GetInventoryPath()); name != "" {
						resp.Networks = append(resp.Networks, name)
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
