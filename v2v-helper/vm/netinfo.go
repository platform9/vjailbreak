package vm

import (
	"strings"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// CanonicalMAC lowercases a MAC address so it can be used as a map key.
// vCenter preserves the case of manually-assigned MACs while the
// VMwareMachine CR (populated from VMware Tools) may report a different
// case, so every IPperMac key and lookup must go through this.
func CanonicalMAC(mac string) string {
	return strings.ToLower(strings.TrimSpace(mac))
}

// CollectIPsPerMac maps each NIC MAC (canonical lowercase) to the IPv4
// addresses recorded in the VMwareMachine CR. GuestNetworks (VMware Tools
// data) takes precedence; NetworkInterfaces is the fallback. IPv6 addresses
// are skipped.
func CollectIPsPerMac(macs []string, guestNetworks []vjailbreakv1alpha1.GuestNetwork, networkInterfaces []vjailbreakv1alpha1.NIC) map[string][]IpEntry {
	ipPerMac := make(map[string][]IpEntry)
	for _, macAddress := range macs {
		key := CanonicalMAC(macAddress)
		if guestNetworks != nil {
			for _, guestNetwork := range guestNetworks {
				if !strings.EqualFold(guestNetwork.MAC, macAddress) {
					continue
				}
				if _, ok := ipPerMac[key]; !ok {
					ipPerMac[key] = []IpEntry{}
				}
				if !strings.Contains(guestNetwork.IP, ":") {
					ipPerMac[key] = append(ipPerMac[key], IpEntry{
						IP:     guestNetwork.IP,
						Prefix: guestNetwork.PrefixLength,
					})
				}
			}
			continue
		}
		for _, networkInterface := range networkInterfaces {
			if !strings.EqualFold(networkInterface.MAC, macAddress) {
				continue
			}
			if _, ok := ipPerMac[key]; !ok {
				ipPerMac[key] = []IpEntry{}
			}
			for _, ipAddress := range networkInterface.IPAddress {
				if strings.Contains(ipAddress, ":") {
					continue
				}
				ipPerMac[key] = append(ipPerMac[key], IpEntry{
					IP:     ipAddress,
					Prefix: 0,
				})
			}
		}
	}
	return ipPerMac
}
