package utils

import (
	"github.com/gophercloud/gophercloud"
)

// ManageOpts defines the options for managing an existing backend volume.
type ManageOpts struct {
	Host       string
	Ref        map[string]string
	Name       string
	VolumeType *string // Optional
}

// ToVolumeManageMap builds the request payload for manage volume.
func (opts ManageOpts) ToVolumeManageMap() (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"volume": map[string]interface{}{
			"host": opts.Host,
			"ref":  opts.Ref,
			"name": opts.Name,
		},
	}
	if opts.VolumeType != nil {
		payload["volume"].(map[string]interface{})["volume_type"] = *opts.VolumeType
	}
	return payload, nil
}

// Manage triggers the volume manage request.
func Manage(client *gophercloud.ServiceClient, opts ManageOpts) (map[string]interface{}, error) {
	body, err := opts.ToVolumeManageMap()
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	_, err = client.Post(client.ServiceURL("manageable_volumes"), body, &result, &gophercloud.RequestOpts{
		OkCodes:      []int{202},
		MoreHeaders:  map[string]string{"OpenStack-API-Version": "volume 3.8"},
		JSONResponse: &result,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
