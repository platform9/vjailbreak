package types

type ProxyRequest struct {
	CredentialName string                 `json:"credentialName"`
	Endpoint       string                 `json:"endpoint"`
	Method         string                 `json:"method,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`
}

type OpenStackSecretData struct {
	OSAuthURL    string
	OSUsername   string
	OSPassword   string
	OSDomainName string
	OSInsecure   string
	OSRegionName string
	OSTenantName string
}

type VsphereSecretData struct {
	VcenterURL        string
	VcenterUsername   string
	VcenterPassword   string
	VcenterDatacenter string
}
