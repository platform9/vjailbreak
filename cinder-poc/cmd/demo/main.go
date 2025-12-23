package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/platform9/vjailbreak/cinder-poc/pkg/cinder"
	"github.com/platform9/vjailbreak/cinder-poc/pkg/storage"
	"k8s.io/klog/v2"
)

var provider *cinder.CinderStorageProvider

func main() {
	klog.InitFlags(nil)

	// Load credentials from environment (sourced from openstack.rc)
	info := storage.StorageAccessInfo{
		Hostname:   os.Getenv("OS_AUTH_URL"),
		Username:   os.Getenv("OS_USERNAME"),
		Password:   os.Getenv("OS_PASSWORD"),
		TenantName: os.Getenv("OS_PROJECT_NAME"),
		DomainName: os.Getenv("OS_USER_DOMAIN_NAME"),
		Region:     os.Getenv("OS_REGION_NAME"),
		Insecure:   os.Getenv("OS_INSECURE") == "true",
	}

	provider = cinder.New(info)

	http.HandleFunc("/connect", connectHandler)
	http.HandleFunc("/create-volume", createVolumeHandler)
	http.HandleFunc("/map-volume", mapVolumeHandler)
	http.HandleFunc("/unmap-volume", unmapVolumeHandler)
	http.HandleFunc("/get-volume", getVolumeHandler)
	http.HandleFunc("/list-volumes", listVolumesHandler)
	http.HandleFunc("/whoami", whoamiHandler)

	klog.Infof("Starting vjailbreak-cinder-demo on :8080")
	klog.Infof("OpenStack endpoint: %s", info.Hostname)
	klog.Infof("Region: %s, Project: %s", info.Region, info.TenantName)
	klog.Infof("Insecure mode: %v", info.Insecure)
	http.ListenAndServe(":8080", nil)
}

// POST /connect - simplified, just connects using env vars
func connectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	if err := provider.Connect(ctx); err != nil {
		http.Error(w, fmt.Sprintf("connect failed: %v", err), http.StatusInternalServerError)
		return
	}
	if err := provider.ValidateCredentials(ctx); err != nil {
		http.Error(w, fmt.Sprintf("validate failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "connected to %s (region: %s)\n", provider.WhoAmI(), os.Getenv("OS_REGION_NAME"))
}

// POST /create-volume
// body: { "name": "vol1", "sizeBytes": 1073741824, "volumeType": "pure-sc" }
func createVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var req struct {
		Name       string `json:"name"`
		SizeBytes  int64  `json:"sizeBytes"`
		VolumeType string `json:"volumeType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	vol, err := provider.CreateVolume(ctx, req.Name, req.SizeBytes, req.VolumeType)
	if err != nil {
		http.Error(w, fmt.Sprintf("create failed: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(vol)
}

// POST /map-volume
// body: { "initiatorGroupName":"esx-group", "volumeName":"vol1", "iqns":["iqn...."] }
func mapVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var req struct {
		InitiatorGroupName string   `json:"initiatorGroupName"`
		VolumeName         string   `json:"volumeName"`
		IQNs               []string `json:"iqns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// create mapping context (no precreate)
	ctxMap, err := provider.CreateOrUpdateInitiatorGroup(ctx, req.InitiatorGroupName, req.IQNs, false)
	if err != nil {
		http.Error(w, fmt.Sprintf("create initiator group failed: %v", err), http.StatusInternalServerError)
		return
	}
	vol := storage.Volume{Name: req.VolumeName}
	mapped, err := provider.MapVolumeToGroup(ctx, req.InitiatorGroupName, vol, ctxMap)
	if err != nil {
		http.Error(w, fmt.Sprintf("map failed: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(mapped)
}

// POST /unmap-volume
// body: { "initiatorGroupName":"esx-group", "volumeName":"vol1", "iqns":["iqn..."] }
func unmapVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var req struct {
		InitiatorGroupName string   `json:"initiatorGroupName"`
		VolumeName         string   `json:"volumeName"`
		IQNs               []string `json:"iqns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctxMap := storage.MappingContext{
		"initiator_group_name": req.InitiatorGroupName,
		"iqns":                 req.IQNs,
	}
	vol := storage.Volume{Name: req.VolumeName}
	if err := provider.UnmapVolumeFromGroup(ctx, req.InitiatorGroupName, vol, ctxMap); err != nil {
		http.Error(w, fmt.Sprintf("unmap failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "unmapped")
}

func getVolumeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name query", http.StatusBadRequest)
		return
	}
	v, err := provider.GetVolumeInfo(ctx, name)
	if err != nil {
		http.Error(w, fmt.Sprintf("get failed: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(v)
}

func listVolumesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	vols, err := provider.ListAllVolumes(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("list failed: %v", err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(vols)
}

func whoamiHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, provider.WhoAmI())
}
