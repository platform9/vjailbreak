package resmgr

type Extensions struct {
	Interfaces InterfaceInfo `json:"interfaces"`
}

type InterfaceInfo struct {
	Status string           `json:"status"`
	Data   InterfaceDetails `json:"data"`
}

type InterfaceDetails struct {
	InterfaceIP map[string]string `json:"iface_ip"`
}

type NICDetails struct {
	MAC    string             `json:"mac"`
	Ifaces []InterfaceAddress `json:"ifaces"`
}

type InterfaceAddress struct {
	Addr      string `json:"addr"`
	Netmask   string `json:"netmask"`
	Broadcast string `json:"broadcast"`
}
