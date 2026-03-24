// Copyright © 2025 The vjailbreak authors

package migrate

import (
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
)

// hotAddGetVCenterClient extracts the VCenterClient from VMops using the same
// pattern as vaai_copy.go so we do not need to change the VMOperations interface.
func (migobj *Migrate) hotAddGetVCenterClient() *vcenter.VCenterClient {
	type vcenterClientGetter interface {
		GetVCenterClient() *vcenter.VCenterClient
	}
	if g, ok := migobj.VMops.(vcenterClientGetter); ok {
		return g.GetVCenterClient()
	}
	return nil
}
