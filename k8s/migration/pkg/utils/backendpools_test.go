package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gophercloud "github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/schedulerstats"
)

// hitachiPoolsResponse mirrors what Hitachi Vantara's Cinder driver actually
// returns for GET /scheduler-stats/get_pools?detail=True: capabilities.location_info
// is a nested JSON object (ldev_range/pool_id/snap_pool_id/storage_id), not the
// plain string gophercloud's schedulerstats.Capabilities expects.
const hitachiPoolsResponse = `
{
  "pools": [
    {
      "name": "bf179d55-abee-4c75-8a33-6dd5c9be3338@hitachi-primary#hitach-primary",
      "capabilities": {
        "vendor_name": "Hitachi",
        "driver_version": "2.8.0",
        "volume_backend_name": "hitach-primary",
        "storage_protocol": "FC",
        "location_info": {
          "storage_id": "A34000810034",
          "pool_id": "0",
          "snap_pool_id": "0",
          "ldev_range": []
        }
      }
    },
    {
      "name": "pcd-ce@pure-iscsi-1#vt-pure-iscsi",
      "capabilities": {
        "vendor_name": "Pure Storage",
        "driver_version": "1.0.0",
        "volume_backend_name": "vt-pure-iscsi",
        "storage_protocol": "iSCSI",
        "location_info": "pure-01|iscsi"
      }
    }
  ]
}
`

func newTestBlockStorageClient(t *testing.T, body string) *gophercloud.ServiceClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)

	return &gophercloud.ServiceClient{
		ProviderClient: &gophercloud.ProviderClient{},
		Endpoint:       server.URL + "/",
	}
}

func TestExtractStoragePools_ToleratesObjectLocationInfo(t *testing.T) {
	client := newTestBlockStorageClient(t, hitachiPoolsResponse)

	pages, err := schedulerstats.List(client, schedulerstats.ListOpts{Detail: true}).AllPages(context.Background())
	if err != nil {
		t.Fatalf("List().AllPages() returned error: %v", err)
	}

	// Sanity check: gophercloud's own typed extraction is expected to fail on
	// this payload, since Hitachi's location_info is an object, not a string.
	if _, err := schedulerstats.ExtractStoragePools(pages); err == nil {
		t.Fatal("expected gophercloud's typed ExtractStoragePools to fail on object location_info, but it succeeded")
	}

	pools, err := extractStoragePools(pages)
	if err != nil {
		t.Fatalf("extractStoragePools() returned error: %v", err)
	}

	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}

	hitachi := pools[0]
	if hitachi.Name != "bf179d55-abee-4c75-8a33-6dd5c9be3338@hitachi-primary#hitach-primary" {
		t.Errorf("unexpected hitachi pool name: %q", hitachi.Name)
	}
	if hitachi.Capabilities.VendorName != "Hitachi" {
		t.Errorf("expected vendor Hitachi, got %q", hitachi.Capabilities.VendorName)
	}
	if hitachi.Capabilities.DriverVersion != "2.8.0" {
		t.Errorf("expected driver version 2.8.0, got %q", hitachi.Capabilities.DriverVersion)
	}

	pure := pools[1]
	if pure.Capabilities.VendorName != "Pure Storage" {
		t.Errorf("expected vendor Pure Storage, got %q", pure.Capabilities.VendorName)
	}
}
