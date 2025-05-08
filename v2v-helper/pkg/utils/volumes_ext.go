package utils

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// ToVolumeManageMap builds the request payload for manage volume.
func ToVolumeManageMap(rdmDisk vjailbreakv1alpha1.RDMDiskInfo) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"volume": map[string]interface{}{
			"host": rdmDisk.CinderBackendPool,
			"ref": map[string]string{
				"source-name": fmt.Sprintf("volume-%s", rdmDisk.UUID),
			},
			"name":              rdmDisk.DiskName,
			"volume_type":       rdmDisk.VolumeType,
			"description":       rdmDisk.Description,
			"bootable":          rdmDisk.Bootable,
			"availability_zone": nil,
		},
	}
	return payload, nil
}

// Manage triggers the volume manage request.
func (osclient *OpenStackClients) CinderManage(rdmDisk vjailbreakv1alpha1.RDMDiskInfo) (map[string]interface{}, error) {
	body, err := ToVolumeManageMap(rdmDisk)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	_, err = osclient.BlockStorageClient.Post(osclient.BlockStorageClient.ServiceURL("manageable_volumes"), body, &result, &gophercloud.RequestOpts{
		OkCodes:      []int{202},
		MoreHeaders:  map[string]string{"OpenStack-API-Version": "volume 3.8"},
		JSONResponse: &result,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
