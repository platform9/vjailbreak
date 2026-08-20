// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/pkg/errors"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/virtv2v"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// disconnects the source VM's network interfaces
func (migobj *Migrate) DisconnectSourceNetworkIfRequested() error {
	if !migobj.DisconnectSourceNetwork {
		return nil
	}

	migobj.logMessage(fmt.Sprintf("Disconnecting source VM network interfaces (DisconnectSourceNetwork=%v)", migobj.DisconnectSourceNetwork))

	if err := migobj.VMops.DisconnectNetworkInterfaces(); err != nil {
		errMsg := fmt.Sprintf("Failed to disconnect source VM network interfaces: %v", err)
		migobj.logMessage("ERROR: " + errMsg)
		return fmt.Errorf("failed to disconnect network interfaces: %w", err)
	}

	migobj.logMessage("Successfully disconnected source VM network interfaces")
	return nil
}

// ReservePortsForVM reserves ports for every VM NIC: reuseExistingPorts if the
// user pre-created ports, otherwise createPortsForNetworks makes new ones.
func (migobj *Migrate) ReservePortsForVM(ctx context.Context, vminfo *vm.VMInfo) ([]string, []string, []string, error) {
	migobj.isSimpleNetwork = false

	securityGroupIDs, err := migobj.Openstackclients.GetSecurityGroupIDs(ctx, migobj.SecurityGroups, migobj.TenantName)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to resolve security group names to IDs")
	}
	utils.PrintLog(fmt.Sprintf("Using provided security group IDs %v", securityGroupIDs))

	if migobj.ServerGroup != "" {
		utils.PrintLog(fmt.Sprintf("Server group ID for VM placement: %s", migobj.ServerGroup))
	}

	if len(migobj.Networkports) != 0 {
		return migobj.reuseExistingPorts(ctx)
	}
	return migobj.createPortsForNetworks(ctx, vminfo, securityGroupIDs)
}

// reuseExistingPorts handles the pre-created-ports flow: migobj.Networkports
// holds one OpenStack port ID per NIC, created outside this migration.
func (migobj *Migrate) reuseExistingPorts(ctx context.Context) ([]string, []string, []string, error) {
	if len(migobj.Networkports) != len(migobj.Networknames) {
		return nil, nil, nil, errors.Errorf("number of network ports does not match number of network names")
	}

	networkids := []string{}
	portids := []string{}
	ipaddresses := []string{}
	for _, portID := range migobj.Networkports {
		retrPort, err := migobj.Openstackclients.GetPort(ctx, portID)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to get port")
		}
		networkids = append(networkids, retrPort.NetworkID)
		portids = append(portids, retrPort.ID)
		for _, fixedIP := range retrPort.FixedIPs {
			ipaddresses = append(ipaddresses, fixedIP.IPAddress)
		}
	}
	return networkids, portids, ipaddresses, nil
}

// createPortsForNetworks handles the create-new-ports flow: one port per
// entry in migobj.Networknames, in NIC order.
func (migobj *Migrate) createPortsForNetworks(ctx context.Context, vminfo *vm.VMInfo, securityGroupIDs []string) ([]string, []string, []string, error) {
	networkids := []string{}
	portids := []string{}
	ipaddresses := []string{}

	// subnetPortIndex is shared across every NIC of this VM: it's how
	// distinct port names get assigned when two or more NICs land on the same
	// subnet (see GetCreateOpts/buildPortName), so it must persist across the
	// whole loop, not be recreated per NIC.
	subnetPortIndex := make(map[string]int)

	for idx, networkname := range migobj.Networknames {
		network, err := migobj.Openstackclients.GetNetwork(ctx, networkname)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to get network")
		}
		if network == nil {
			return nil, nil, nil, errors.Errorf("network not found")
		}

		isSimpleNetwork, err := migobj.Openstackclients.GetIsSimpleNetwork(ctx, network.ID)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to check if network is L2")
		}
		migobj.isSimpleNetwork = migobj.isSimpleNetwork || isSimpleNetwork

		override, err := resolveNICOverride(migobj.NetworkOverrides, idx)
		if err != nil {
			return nil, nil, nil, err
		}

		mac := vminfo.Mac[idx]
		detectedIPs := logAndCollectDetectedIPs(vminfo, mac)
		loggedIPs := applyPreserveIPOverride(vminfo, idx, mac, override, detectedIPs, migobj.FallbackToDHCP)
		mac = applyPreserveMACOverride(vminfo, idx, mac, override.preserveMAC)
		utils.PrintLog(fmt.Sprintf("Using IPs for MAC %s: %v", vminfo.Mac[idx], loggedIPs))

		port, err := migobj.createPort(ctx, network, mac, vminfo, securityGroupIDs, subnetPortIndex)
		if err != nil {
			return nil, nil, nil, err
		}
		syncIPperMacFromPort(vminfo, mac, port)

		networkids = append(networkids, network.ID)
		portids = append(portids, port.ID)
		for _, fixedIP := range port.FixedIPs {
			ipaddresses = append(ipaddresses, fixedIP.IPAddress)
		}
	}
	utils.PrintLog(fmt.Sprintf("Gateways : %v", vminfo.GatewayIP))
	return networkids, portids, ipaddresses, nil
}

// createPort creates a single OpenStack port on network for the given
// (possibly overridden) MAC, using vminfo.IPperMac to determine fixed IPs.
// subnetPortIndex must be the same map across every NIC of this VM (see
// createPortsForNetworks) so NICs sharing a subnet get distinct port names.
func (migobj *Migrate) createPort(ctx context.Context, network *networks.Network, mac string, vminfo *vm.VMInfo, securityGroupIDs []string, subnetPortIndex map[string]int) (*ports.Port, error) {
	port, err := migobj.Openstackclients.ValidateAndCreatePort(ctx, network, mac, vminfo.IPperMac, vminfo.Name, securityGroupIDs, migobj.FallbackToDHCP, vminfo.GatewayIP, subnetPortIndex)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create port group")
	}
	addressesOfPort := []string{}
	for _, fixedIP := range port.FixedIPs {
		addressesOfPort = append(addressesOfPort, fixedIP.IPAddress)
	}
	utils.PrintLog(fmt.Sprintf("Port created successfully: MAC:%s IP:%s and Security Groups:%v\n", port.MACAddress, addressesOfPort, securityGroupIDs))
	return port, nil
}

// nicOverride is the resolved per-NIC configuration for preserveIP/preserveMAC
// and any user-assigned replacement IP, defaulting to "preserve everything".
type nicOverride struct {
	preserveIP     bool
	preserveMAC    bool
	userAssignedIP []string
}

// resolveNICOverride resolves the NICOverride (if any) for interface idx
// against the "preserve everything" default; a nil field means "not set".
func resolveNICOverride(overrides []NICOverride, idx int) (nicOverride, error) {
	result := nicOverride{preserveIP: true, preserveMAC: true}

	for _, override := range overrides {
		if override.InterfaceIndex != idx {
			continue
		}
		if override.PreserveIP != nil {
			result.preserveIP = *override.PreserveIP
		}
		if override.PreserveMAC != nil {
			result.preserveMAC = *override.PreserveMAC
		}
		if override.UserAssignedIP != "" {
			for _, ip := range strings.Split(override.UserAssignedIP, ",") {
				if trimmedIP := strings.TrimSpace(ip); trimmedIP != "" {
					result.userAssignedIP = append(result.userAssignedIP, trimmedIP)
				}
			}
		}
		break
	}

	if len(result.userAssignedIP) > 1 {
		return nicOverride{}, errors.Errorf("multiple user assigned IPs not supported for an interface")
	}
	return result, nil
}

// logAndCollectDetectedIPs logs and returns the IPs VMware Tools reported for
// mac, or nil if none were detected.
func logAndCollectDetectedIPs(vminfo *vm.VMInfo, mac string) []string {
	detectedIPs := vminfo.IPperMac[mac]
	if len(detectedIPs) == 0 {
		return nil
	}
	ippm := make([]string, 0, len(detectedIPs))
	for _, detectedIP := range detectedIPs {
		ippm = append(ippm, detectedIP.IP)
	}
	utils.PrintLog(fmt.Sprintf("Detected IPs from VMware Tools for MAC %s: %v", mac, detectedIPs))
	return ippm
}

// applyPreserveIPOverride replaces vminfo.IPperMac[mac] with the user-assigned
// IP, or clears it, when preserveIP is false; returns the IPs for logging.
// If no custom IP is given, fallbackToDHCP picks nil (auto-allocate) vs an
// empty slice (no fixed IPs) for GetCreateOpts.
func applyPreserveIPOverride(vminfo *vm.VMInfo, idx int, mac string, override nicOverride, detectedIPs []string, fallbackToDHCP bool) []string {
	if override.preserveIP {
		return detectedIPs
	}

	if len(override.userAssignedIP) > 0 {
		entries := make([]vm.IpEntry, 0, len(override.userAssignedIP))
		for _, ip := range override.userAssignedIP {
			entries = append(entries, vm.IpEntry{IP: ip, Prefix: 0})
		}
		vminfo.IPperMac[mac] = entries
		utils.PrintLog(fmt.Sprintf("NIC[%d]: preserveIP=false, using user-assigned custom IP for MAC %s", idx, mac))
		return override.userAssignedIP
	}

	if fallbackToDHCP {
		utils.PrintLog(fmt.Sprintf("NIC[%d]: preserveIP=false, no custom IP for MAC %s, fallbackToDHCP=true — port will auto-allocate an IP", idx, mac))
		vminfo.IPperMac[mac] = nil
		return detectedIPs
	}

	utils.PrintLog(fmt.Sprintf("NIC[%d]: preserveIP=false, no custom IP for MAC %s, fallbackToDHCP=false — port will have no fixed IPs", idx, mac))
	vminfo.IPperMac[mac] = []vm.IpEntry{}
	return detectedIPs
}

// applyPreserveMACOverride moves mac's IPs to the "" key and returns "" so
// OpenStack generates a new MAC, when preserveMAC is false. The real MAC
// OpenStack assigns isn't known yet at this point in the flow — see
// syncIPperMacFromPort, which re-keys "" to the real MAC once the port is
// created.
func applyPreserveMACOverride(vminfo *vm.VMInfo, idx int, mac string, preserveMAC bool) string {
	if preserveMAC {
		return mac
	}
	utils.PrintLog(fmt.Sprintf("NIC[%d]: preserveMAC=false for MAC %s — OpenStack will generate a new MAC", idx, mac))
	vminfo.IPperMac[""] = vminfo.IPperMac[mac]
	delete(vminfo.IPperMac, mac)
	return ""
}

// syncIPperMacFromPort reconciles vminfo.IPperMac with the port OpenStack
// actually created, keyed by placeholderMAC (the MAC passed to createPort:
// the original MAC if preserveMAC=true, or "" if preserveMAC=false). Two
// situations need reconciling once the real port exists:
//
//   - preserveMAC=false: OpenStack assigns a new MAC, but the entry is still
//     sitting under the "" placeholder key. It's moved as-is to the real
//     MAC (port.MACAddress) so MAC-keyed guest-config code (AddWildcardNetplan,
//     the network-persistence script) can match it against the real NIC.
//   - preserveIP=false + fallbackToDHCP=true: applyPreserveIPOverride left
//     the entry nil so OpenStack would auto-allocate an IP instead of
//     getting an explicit empty port. That nil is filled in with whatever
//     IP(s) the port actually ended up with (possibly none, e.g. on an
//     L2 network with no subnets).
//
// A NIC whose entries were already populated (preserveIP=true, or a custom
// IP) is left untouched by the fill step: the port was created with exactly
// that IP, so rebuilding from port.FixedIPs would risk losing a preserved
// subnet prefix for no benefit.
func syncIPperMacFromPort(vminfo *vm.VMInfo, placeholderMAC string, port *ports.Port) {
	realMAC := port.MACAddress
	if realMAC == "" {
		// Defensive: don't lose the entry if a backend ever fails to echo a MAC.
		realMAC = placeholderMAC
	}

	if realMAC != placeholderMAC {
		vminfo.IPperMac[realMAC] = vminfo.IPperMac[placeholderMAC]
		delete(vminfo.IPperMac, placeholderMAC)
	}

	if vminfo.IPperMac[realMAC] == nil {
		// These IPs came from a live OpenStack auto-allocation, not a
		// preserved/custom static IP — mark them so guest-config writers
		// configure a real DHCP client instead of pinning the IP statically.
		entries := make([]vm.IpEntry, 0, len(port.FixedIPs))
		for _, fixedIP := range port.FixedIPs {
			entries = append(entries, vm.IpEntry{IP: fixedIP.IPAddress, Prefix: 0, DHCP: true})
		}
		vminfo.IPperMac[realMAC] = entries
	}
}

func (migobj *Migrate) configureWindowsNetwork(ctx context.Context, vminfo vm.VMInfo, bootVolumeIndex int, osRelease string) error {
	persistNetwork := utils.GetNetworkPersistance(ctx, migobj.K8sClient)
	if persistNetwork {
		osType := strings.ToLower(vminfo.OSType)
		if err := virtv2v.InjectMacToIps(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path, vminfo.GuestNetworks, vminfo.GatewayIP, vminfo.IPperMac, osType); err != nil {
			return errors.Wrap(err, "failed to inject mac to ips")
		}
		utils.PrintLog("Mac to IP mapping injected successfully")
		if err := virtv2v.InjectRestorationScript(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path); err != nil {
			return errors.Wrap(err, "failed to inject restoration script")
		}
		utils.PrintLog("Restoration script injected successfully")
	}
	return nil
}

// configureLinuxNetwork handles network configuration for Linux systems
func (migobj *Migrate) configureLinuxNetwork(ctx context.Context, vminfo vm.VMInfo, bootVolumeIndex int, osRelease string) error {
	persisNetwork := utils.GetNetworkPersistance(ctx, migobj.K8sClient)
	if persisNetwork {
		osType := strings.ToLower(vminfo.OSType)
		if err := virtv2v.InjectMacToIps(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path, vminfo.GuestNetworks, vminfo.GatewayIP, vminfo.IPperMac, osType); err != nil {
			return errors.Wrap(err, "failed to inject mac to ips")
		}
		utils.PrintLog("Mac to ips injection completed successfully")
		versionID := parseVersionID(osRelease)
		if versionID == "" {
			return errors.Errorf("failed to get version ID")
		}
		isNetplan := isNetplanSupported(versionID) && strings.Contains(osRelease, "ubuntu")
		utils.PrintLog(fmt.Sprintf("Is netplan: %v", isNetplan))
		utils.PrintLog("Running network persistence script")
		if err := virtv2v.RunNetworkPersistence(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path, vminfo.OSType, isNetplan); err != nil {
			utils.PrintLog(fmt.Sprintf("Warning: Network persistence script failed: %v", err))
		} else {
			utils.PrintLog("Network persistence script executed successfully")
		}
	} else {
		if strings.Contains(osRelease, "ubuntu") {
			return migobj.configureUbuntuNetwork(vminfo, bootVolumeIndex, osRelease)
		}

		if virtv2v.IsRHELFamily(osRelease) {
			return migobj.configureRHELNetwork(vminfo, bootVolumeIndex, osRelease)
		}
	}

	return nil
}

// configureUbuntuNetwork handles Ubuntu-specific network configuration
func (migobj *Migrate) configureUbuntuNetwork(vminfo vm.VMInfo, bootVolumeIndex int, osRelease string) error {
	versionID := parseVersionID(osRelease)
	utils.PrintLog(fmt.Sprintf("Version ID: %s", versionID))

	if versionID == "" {
		return errors.Errorf("failed to get version ID")
	}

	if isNetplanSupported(versionID) {
		utils.PrintLog("Adding wildcard netplan")
		if migobj.isSimpleNetwork {
			utils.PrintLog("L2 network detected adding l2 wildcard")
			if err := virtv2v.AddWildcardNetplanForL2(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path); err != nil {
				return errors.Wrap(err, "failed to add l2 wildcard netplan")
			}
		} else {
			if err := virtv2v.AddWildcardNetplan(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path, vminfo.GuestNetworks, vminfo.GatewayIP, vminfo.IPperMac); err != nil {
				return errors.Wrap(err, "failed to add wildcard netplan")
			}
		}
		utils.PrintLog("Wildcard netplan added successfully")
		return nil
	}

	return migobj.addUdevRulesForUbuntu(vminfo, bootVolumeIndex)
}

// addUdevRulesForUbuntu adds udev rules for older Ubuntu versions
func (migobj *Migrate) addUdevRulesForUbuntu(vminfo vm.VMInfo, bootVolumeIndex int) error {
	utils.PrintLog("Ubuntu version does not support netplan, going to use udev rules")

	interfaces, err := virtv2v.GetNetworkInterfaceNames(vminfo.VMDisks[bootVolumeIndex].Path)
	if err != nil {
		return errors.Wrap(err, "failed to get network interface names")
	}

	if len(interfaces) == 0 {
		log.Printf("Failed to get network interface names, cannot add udev rules, network might not come up post migration, please check the network configuration post migration")
		return nil
	}

	utils.PrintLog("Adding udev rules")
	utils.PrintLog(fmt.Sprintf("Interfaces: %v", interfaces))

	macs := []string{}
	for _, nic := range vminfo.NetworkInterfaces {
		macs = append(macs, nic.MAC)
	}
	utils.PrintLog(fmt.Sprintf("MACs: %v", macs))

	if err := virtv2v.AddUdevRules(vminfo.VMDisks, vminfo.VMDisks[bootVolumeIndex].Path, interfaces, macs); err != nil {
		log.Printf(`Warning Failed to add udev rules: %s, incase of interface name mismatch,
                        network might not come up post migration, please check the network configuration post migration`, err)
		log.Println("Continuing with migration")
	}

	return nil
}

func (migobj *Migrate) configureRHELNetwork(vminfo vm.VMInfo, bootVolumeIndex int, osRelease string) error {
	versionID := parseVersionID(osRelease)
	majorVersion, err := strconv.Atoi(strings.Split(versionID, ".")[0])
	if err != nil {
		return fmt.Errorf("failed to parse major version: %v", err)
	}

	if majorVersion < 7 {
		diskPath := vminfo.VMDisks[bootVolumeIndex].Path
		if err := DetectAndHandleNetwork(diskPath, osRelease, vminfo); err != nil {
			utils.PrintLog(fmt.Sprintf(`Warning: Failed to handle network: %v,Continuing with migration,
                    network might not come up post migration, please check the network configuration post migration`, err))
		}
	}

	return nil
}

// DetectAndHandleNetwork: Checks if RHEL family, then detects NM presence offline.
// If NM (nmcli exists), injects first-boot nmcli script for DHCP force.
// If not, adds udev rules to pin names without forcing DHCP.
func DetectAndHandleNetwork(diskPath string, osRelease string, vmInfo vm.VMInfo) error {

	// No NM: Add udev rules to pin names
	interfaces, err := virtv2v.GetInterfaceNames(diskPath)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Warning: Failed to get interfaces: %v", err))
	}
	if len(interfaces) == 0 {
		utils.PrintLog(`No network interfaces found, cannot add udev rules, network might not
            come up post migration, please check the network configuration post migration`)
		return nil
	}
	macs := []string{}
	// By default the network interfaces macs are in the same order as the interfaces
	for _, nic := range vmInfo.NetworkInterfaces {
		macs = append(macs, nic.MAC)
	}
	utils.PrintLog(fmt.Sprintf("Interfaces: %v", interfaces))
	utils.PrintLog(fmt.Sprintf("MACs: %v", macs))
	if len(interfaces) != len(macs) {
		utils.PrintLog("Mismatch between number of interfaces and MACs")
		return fmt.Errorf("mismatch between number of interfaces and MACs")
	}
	// Add udev rules to pin names without forcing DHCP
	utils.PrintLog("Adding udev rules to pin interface names")

	// This will ensure that the network interfaces are named consistently after migration
	// and they get the correct IP address.
	// This is important because RHEL family uses NetworkManager by default and it does not
	// automatically configure the network interfaces to use DHCP after migration.
	// So we need to add udev rules to pin the names of the network interfaces
	// to the MAC addresses so that they are consistent after migration.
	// This will ensure that the network interfaces are named consistently after migration
	// and they get the correct IP address.
	err = virtv2v.AddUdevRules([]vm.VMDisk{{Path: diskPath}}, diskPath, interfaces, macs)
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Warning: Failed to add udev: %v", err))
	}
	return nil
}

func (migobj *Migrate) DeleteAllPorts(ctx context.Context, portids []string) error {
	migobj.logMessage("Deleting all ports")
	openstackops := migobj.Openstackclients
	var deletionErrors []error
	successCount := 0

	for _, portID := range portids {
		err := openstackops.DeletePort(ctx, portID)
		if err != nil {
			utils.PrintLog(fmt.Sprintf("Failed to delete port %s: %s\n", portID, err))
			deletionErrors = append(deletionErrors, errors.Wrapf(err, "failed to delete port %s", portID))
		} else {
			utils.PrintLog(fmt.Sprintf("Successfully deleted port %s\n", portID))
			successCount++
		}
	}

	if len(deletionErrors) > 0 {
		migobj.logMessage(fmt.Sprintf("Port deletion completed with errors: %d succeeded, %d failed out of %d total", successCount, len(deletionErrors), len(portids)))
		// Combine all errors into a single error message
		errMsg := fmt.Sprintf("failed to delete %d port(s):", len(deletionErrors))
		for _, err := range deletionErrors {
			errMsg += fmt.Sprintf("\n  - %s", err.Error())
		}
		return errors.New(errMsg)
	}

	migobj.logMessage(fmt.Sprintf("Successfully deleted all %d ports", successCount))
	return nil
}
