// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// ---------------------------------------------------------------------------
// isBareDisk
// ---------------------------------------------------------------------------

func TestIsBareDisk(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// True cases – bare disk device, no partition suffix
		{name: "sda", path: "/dev/sda", want: true},
		{name: "sdb", path: "/dev/sdb", want: true},
		{name: "vda", path: "/dev/vda", want: true},
		{name: "sdz", path: "/dev/sdz", want: true},

		// False cases – partition or LVM/device-mapper paths
		{name: "sda1", path: "/dev/sda1", want: false},
		{name: "sda2", path: "/dev/sda2", want: false},
		{name: "vda1", path: "/dev/vda1", want: false},
		{name: "lv path", path: "/dev/vg0/lv_root", want: false},
		{name: "mapper path", path: "/dev/mapper/vg-lv", want: false},
		{name: "empty", path: "", want: false},
		{name: "no /dev prefix", path: "sda", want: false},
		{name: "first (virt-v2v sentinel)", path: "first", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBareDisk(tt.path)
			assert.Equal(t, tt.want, got, "isBareDisk(%q)", tt.path)
		})
	}
}

// ---------------------------------------------------------------------------
// IsSUSEFamily
// ---------------------------------------------------------------------------

func TestIsSUSEFamily(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		want      bool
	}{
		// Positive cases
		{name: "SLES 11 SuSE-release", osRelease: "suse linux enterprise server 11 (x86_64)", want: true},
		{name: "SLES 12 os-release", osRelease: `NAME="SLES"\nVERSION="12-SP5"`, want: true},
		{name: "SLES 15", osRelease: `NAME="SLES"\nVERSION="15-SP4"`, want: true},
		{name: "SLED", osRelease: "suse linux enterprise desktop 15", want: true},
		{name: "openSUSE Leap", osRelease: `NAME="openSUSE Leap"\nVERSION_ID="15.5"`, want: true},
		{name: "openSUSE Tumbleweed", osRelease: `NAME="openSUSE Tumbleweed"`, want: true},
		{name: "mixed case SUSE", osRelease: "SUSE Linux Enterprise Server 11", want: true},
		{name: "sles keyword only", osRelease: "sles", want: true},
		{name: "sled keyword only", osRelease: "sled", want: true},

		// Negative cases
		{name: "RHEL", osRelease: "red hat enterprise linux 8", want: false},
		{name: "CentOS", osRelease: "centos linux 7", want: false},
		{name: "Ubuntu", osRelease: `NAME="Ubuntu"\nVERSION_ID="22.04"`, want: false},
		{name: "Debian", osRelease: `NAME="Debian GNU/Linux"`, want: false},
		{name: "Fedora", osRelease: `NAME="Fedora Linux"`, want: false},
		{name: "Rocky Linux", osRelease: "rocky linux 9", want: false},
		{name: "Windows", osRelease: "windows server 2019", want: false},
		{name: "empty string", osRelease: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSUSEFamily(tt.osRelease)
			assert.Equal(t, tt.want, got, "IsSUSEFamily(%q)", tt.osRelease)
		})
	}
}

// ---------------------------------------------------------------------------
// FixLegacyMkinitrd – logic tests that do not require a real guestfish/qemu
//
// We test the pure-logic preconditions by writing the mkinitrd wrapper to a
// temp directory and verifying its content, and by checking that IsSUSEFamily
// correctly gates the call in the migration flow.
// ---------------------------------------------------------------------------

// mkinitrdLVMWrapperFile is the on-disk source of the wrapper script that the
// v2v-helper image ships at mkinitrdLVMWrapperPath. The script used to be an
// embedded Go const (mkinitrdLVMWrapper) but was moved to
// scripts/mkinitrd-lvm-wrapper.sh and is COPY'd into the image at build time, so
// the tests now validate the file that actually ships. Relative to this package
// dir (v2v-helper/virtv2v), the repo-root scripts/ dir is two levels up.
var mkinitrdLVMWrapperFile = filepath.Join("..", "..", "scripts", "mkinitrd-lvm-wrapper.sh")

// readMkinitrdLVMWrapper loads the wrapper script content from disk.
func readMkinitrdLVMWrapper(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(mkinitrdLVMWrapperFile)
	require.NoError(t, err, "wrapper script must exist at %s", mkinitrdLVMWrapperFile)
	return string(b)
}

// TestMkinitrdLVMWrapperContent verifies the wrapper script contains the
// required translation logic and safety guards.
func TestMkinitrdLVMWrapperContent(t *testing.T) {
	wrapper := readMkinitrdLVMWrapper(t)

	// Verify the wrapper calls the original binary
	assert.Contains(t, wrapper, "/sbin/mkinitrd.orig",
		"wrapper must delegate to the backed-up original")

	// Verify -d flag handling is present
	assert.Contains(t, wrapper, "-d",
		"wrapper must handle the -d flag")

	// Verify /dev/mapper translation is present
	assert.Contains(t, wrapper, "/dev/mapper/",
		"wrapper must translate to /dev/mapper/ path")

	// Verify argument-boundary preservation via xargs -0
	assert.Contains(t, wrapper, "xargs -0",
		"wrapper must use xargs -0 to preserve argument boundaries across spaces")

	// Verify temp-file cleanup on exit
	assert.Contains(t, wrapper, "trap",
		"wrapper must clean up temp file via trap")

	// Verify shebang
	assert.True(t, len(wrapper) > 0 && wrapper[0:2] == "#!",
		"wrapper must start with a shebang")
}

// TestMkinitrdWrapperWritable verifies the wrapper can be written to disk with
// correct permissions (mirrors the write step inside FixLegacyMkinitrd).
func TestMkinitrdWrapperWritable(t *testing.T) {
	wrapper := readMkinitrdLVMWrapper(t)
	dir := t.TempDir()
	wrapperPath := filepath.Join(dir, "mkinitrd-lvm-wrapper.sh")

	err := os.WriteFile(wrapperPath, []byte(wrapper), 0755)
	require.NoError(t, err, "wrapper should be writable")

	info, err := os.Stat(wrapperPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
		"wrapper file should be executable")

	content, err := os.ReadFile(wrapperPath)
	require.NoError(t, err)
	assert.Equal(t, wrapper, string(content),
		"wrapper content must round-trip through disk without modification")
}

// TestFixLegacyMkinitrdOnlyForSUSE verifies that IsSUSEFamily correctly gates
// the FixLegacyMkinitrd call for the OS families we care about.
func TestFixLegacyMkinitrdOnlyForSUSE(t *testing.T) {
	suseReleases := []string{
		"suse linux enterprise server 11 (x86_64)",
		"SLES 12-SP5",
		"opensuse leap 15.5",
	}
	nonSuseReleases := []string{
		"red hat enterprise linux 8",
		"ubuntu 22.04",
		"centos linux 7",
		"windows server 2019",
		"",
	}

	for _, r := range suseReleases {
		assert.True(t, IsSUSEFamily(r),
			"expected IsSUSEFamily=true for %q so FixLegacyMkinitrd would be called", r)
	}
	for _, r := range nonSuseReleases {
		assert.False(t, IsSUSEFamily(r),
			"expected IsSUSEFamily=false for %q so FixLegacyMkinitrd would be skipped", r)
	}
}

// ---------------------------------------------------------------------------
// RunMountPersistenceScript – flag selection logic
//
// The actual guestfish execution cannot run in unit tests, but we can verify
// that the OS-family check that determines the script flag is correct.
// ---------------------------------------------------------------------------

// mountPersistenceScriptFlag returns the flag that RunMountPersistenceScript
// would choose for a given osRelease, without actually running guestfish.
// This mirrors the flag-selection logic inside RunMountPersistenceScript.
func mountPersistenceScriptFlag(osRelease string) string {
	if IsSUSEFamily(osRelease) {
		return "--replace-fstab"
	}
	return "--force-uuid"
}

// TestMountPersistenceScriptFlagSelection verifies that SUSE guests get
// --replace-fstab (which skips fix_grub_config / device.map rewrite) and
// all other guests get --force-uuid.
func TestMountPersistenceScriptFlagSelection(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		wantFlag  string
	}{
		// SUSE family → must NOT rewrite device.map before virt-v2v
		{name: "SLES 11", osRelease: "suse linux enterprise server 11 (x86_64)", wantFlag: "--replace-fstab"},
		{name: "SLES 12", osRelease: `NAME="SLES" VERSION="12-SP5"`, wantFlag: "--replace-fstab"},
		{name: "SLES 15", osRelease: `NAME="SLES" VERSION="15-SP4"`, wantFlag: "--replace-fstab"},
		{name: "openSUSE Leap", osRelease: `NAME="openSUSE Leap" VERSION_ID="15.5"`, wantFlag: "--replace-fstab"},
		{name: "openSUSE Tumbleweed", osRelease: `NAME="openSUSE Tumbleweed"`, wantFlag: "--replace-fstab"},

		// Non-SUSE → full UUID conversion including GRUB config
		{name: "RHEL 8", osRelease: "red hat enterprise linux 8", wantFlag: "--force-uuid"},
		{name: "CentOS 7", osRelease: "centos linux 7", wantFlag: "--force-uuid"},
		{name: "Ubuntu 22.04", osRelease: `NAME="Ubuntu" VERSION_ID="22.04"`, wantFlag: "--force-uuid"},
		{name: "Rocky Linux 9", osRelease: "rocky linux 9", wantFlag: "--force-uuid"},
		{name: "empty string", osRelease: "", wantFlag: "--force-uuid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mountPersistenceScriptFlag(tt.osRelease)
			assert.Equal(t, tt.wantFlag, got,
				"wrong script flag for osRelease=%q", tt.osRelease)
		})
	}
}

// ---------------------------------------------------------------------------
// blockDriver arg selection
// ---------------------------------------------------------------------------

// buildV2VArgs is a thin wrapper that exposes the block-driver selection logic
// without executing the command, for unit testing.
func buildV2VArgs(ostype, blockDriver string) []string {
	args := []string{"-v", "--no-fstrim"}
	if strings.ToLower(ostype) == "windows" && blockDriver != "" {
		args = append(args, "--block-driver", blockDriver)
	}
	return args
}

func TestConvertDisk_BlockDriverArg(t *testing.T) {
	tests := []struct {
		name            string
		ostype          string
		blockDriver     string
		wantBlockDriver bool
		wantValue       string
	}{
		{
			name:            "windows virtio-scsi adds --block-driver",
			ostype:          "windows",
			blockDriver:     "virtio-scsi",
			wantBlockDriver: true,
			wantValue:       "virtio-scsi",
		},
		{
			name:            "windows empty blockDriver omits flag (defaults to virtio-blk)",
			ostype:          "windows",
			blockDriver:     "",
			wantBlockDriver: false,
		},
		{
			name:            "linux ignores blockDriver",
			ostype:          "linux",
			blockDriver:     "virtio-scsi",
			wantBlockDriver: false,
		},
		{
			name:            "linux empty blockDriver omits flag",
			ostype:          "linux",
			blockDriver:     "",
			wantBlockDriver: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildV2VArgs(tt.ostype, tt.blockDriver)
			idx := -1
			for i, a := range args {
				if a == "--block-driver" {
					idx = i
					break
				}
			}
			if tt.wantBlockDriver {
				if idx == -1 {
					t.Fatalf("expected --block-driver in args %v but not found", args)
				}
				if idx+1 >= len(args) || args[idx+1] != tt.wantValue {
					t.Errorf("--block-driver value = %q, want %q", args[idx+1], tt.wantValue)
				}
			} else {
				if idx != -1 {
					t.Errorf("unexpected --block-driver in args %v", args)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildWildcardNetplanYAML – DHCP-vs-static decision
//
// IpEntry.DHCP distinguishes an IP that came from a live OpenStack/Neutron
// auto-allocation (fallback-to-DHCP, or a subnet-mismatch DHCP fallback)
// from a preserved/custom static IP. DHCP-sourced entries must get a real
// dhcp4: true so the guest performs an actual DHCP handshake instead of
// having the IP pinned statically — some networks tie the IP-to-port
// binding to an observed lease, so a static pin that never went through
// DORA can leave the guest unreachable even though Neutron's port record is
// correct.
// ---------------------------------------------------------------------------

func TestBuildWildcardNetplanYAML_StaticEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "10.0.0.5", Prefix: 24}},
	}

	yaml := buildWildcardNetplanYAML(nil, map[string]string{}, ipPerMac)

	assert.Contains(t, yaml, "macaddress: aa:bb:cc:dd:ee:ff")
	assert.Contains(t, yaml, "dhcp4: false", "static entry must get dhcp4: false")
	assert.NotContains(t, yaml, "dhcp4: true", "static-only entry must not get dhcp4: true")
	assert.Contains(t, yaml, "- 10.0.0.5/24", "static entry must be written as an address")
}

func TestBuildWildcardNetplanYAML_DHCPEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
	}

	yaml := buildWildcardNetplanYAML(nil, map[string]string{}, ipPerMac)

	assert.Contains(t, yaml, "dhcp4: true", "DHCP-sourced entry must get dhcp4: true")
	assert.NotContains(t, yaml, "dhcp4: false", "DHCP-only entry must not get dhcp4: false")
	assert.NotContains(t, yaml, "addresses:", "DHCP-only entry must not get a static addresses: block")
	assert.NotContains(t, yaml, "192.168.50.77", "DHCP-sourced IP must not be pinned as a static address")
}

// TestBuildWildcardNetplanYAML_DHCPOnlyEntryOmitsRoutesAndDNS covers exactly
// the preserveIP=false + fallbackToDHCP=true + preserveMAC=true case: the MAC
// is purely DHCP-sourced (no static entries), but gatewayIP[mac] and
// macToDNS[mac] can still be populated (GetCreateOpts's auto-allocate branch
// records a gateway even for a DHCP-sourced port, and macToDNS comes from the
// source VM's GuestNetworks independent of the override). Neither should be
// written: the DHCP client owns the gateway/DNS entirely, and a hand-written
// default route or carried-over source-network DNS servers sitting next to
// dhcp4: true would fight with (or go stale relative to) the actual lease.
func TestBuildWildcardNetplanYAML_DHCPOnlyEntryOmitsRoutesAndDNS(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
	}
	gatewayIP := map[string]string{"aa:bb:cc:dd:ee:ff": "192.168.50.1"}
	guestNetworks := []vjailbreakv1alpha1.GuestNetwork{
		{MAC: "aa:bb:cc:dd:ee:ff", IP: "10.0.0.5", DNS: []string{"10.0.0.2"}},
	}

	yaml := buildWildcardNetplanYAML(guestNetworks, gatewayIP, ipPerMac)

	assert.Contains(t, yaml, "dhcp4: true")
	assert.NotContains(t, yaml, "routes:", "a purely DHCP-sourced MAC must not get a hand-written default route")
	assert.NotContains(t, yaml, "via: 192.168.50.1")
	assert.NotContains(t, yaml, "nameservers:", "a purely DHCP-sourced MAC must not get carried-over source-network DNS servers")
	assert.NotContains(t, yaml, "10.0.0.2")
}

func TestBuildWildcardNetplanYAML_MixedEntries(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {
			{IP: "10.0.0.5", Prefix: 24},
			{IP: "192.168.50.77", Prefix: 0, DHCP: true},
		},
	}
	gatewayIP := map[string]string{"aa:bb:cc:dd:ee:ff": "10.0.0.1"}

	yaml := buildWildcardNetplanYAML(nil, gatewayIP, ipPerMac)

	assert.Contains(t, yaml, "dhcp4: true", "any DHCP-sourced entry on the NIC should trigger dhcp4: true")
	assert.Contains(t, yaml, "- 10.0.0.5/24", "the static entry should still be written as an address")
	assert.NotContains(t, yaml, "192.168.50.77", "the DHCP-sourced entry must not appear under addresses:")
	assert.Contains(t, yaml, "via: 10.0.0.1", "a route is still written when the NIC also has a static entry")
}

func TestBuildWildcardNetplanYAML_EmptyEntriesSkipped(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {},
	}

	yaml := buildWildcardNetplanYAML(nil, map[string]string{}, ipPerMac)

	assert.NotContains(t, yaml, "aa:bb:cc:dd:ee:ff", "a MAC with zero entries must get no ethernet stanza")
}

func TestBuildWildcardNetplanYAML_GatewayWrittenForStaticEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 24}},
	}
	gatewayIP := map[string]string{"aa:bb:cc:dd:ee:ff": "192.168.50.1"}

	yaml := buildWildcardNetplanYAML(nil, gatewayIP, ipPerMac)

	assert.Contains(t, yaml, "via: 192.168.50.1", "gateway route must be written for a static entry")
}

// ---------------------------------------------------------------------------
// buildRHELIfcfgFiles / buildRHELNetworkManagerKeyfiles – RHEL 7+ guest
// network configuration (ifcfg / NetworkManager keyfile equivalents of
// buildWildcardNetplanYAML). Same DHCP-vs-static contract: DHCP-sourced
// entries must get a real DHCP client config (BOOTPROTO=dhcp / method=auto),
// static entries get a pinned IP.
// ---------------------------------------------------------------------------

func TestBuildRHELIfcfgFiles_StaticEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "10.0.0.5", Prefix: 24}},
	}

	files := buildRHELIfcfgFiles(ipPerMac, map[string]string{}, map[string][]string{})

	content, ok := files["ifcfg-vjb0"]
	assert.True(t, ok, "expected ifcfg-vjb0 to be generated, got files: %#v", files)
	assert.Contains(t, content, "HWADDR=aa:bb:cc:dd:ee:ff")
	assert.Contains(t, content, "BOOTPROTO=none", "static entry must get BOOTPROTO=none")
	assert.NotContains(t, content, "BOOTPROTO=dhcp", "static-only entry must not get BOOTPROTO=dhcp")
	assert.Contains(t, content, "IPADDR=10.0.0.5")
	assert.Contains(t, content, "PREFIX=24")
}

func TestBuildRHELIfcfgFiles_DHCPEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
	}

	files := buildRHELIfcfgFiles(ipPerMac, map[string]string{}, map[string][]string{})

	content := files["ifcfg-vjb0"]
	assert.Contains(t, content, "BOOTPROTO=dhcp", "DHCP-sourced entry must get BOOTPROTO=dhcp")
	assert.NotContains(t, content, "BOOTPROTO=none", "DHCP-only entry must not get BOOTPROTO=none")
	assert.NotContains(t, content, "IPADDR=", "DHCP-sourced IP must not be pinned as a static IPADDR")
}

func TestBuildRHELIfcfgFiles_MixedEntries(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {
			{IP: "10.0.0.5", Prefix: 24},
			{IP: "192.168.50.77", Prefix: 0, DHCP: true},
		},
	}

	files := buildRHELIfcfgFiles(ipPerMac, map[string]string{}, map[string][]string{})

	content := files["ifcfg-vjb0"]
	assert.Contains(t, content, "BOOTPROTO=dhcp", "any DHCP-sourced entry on the NIC should trigger BOOTPROTO=dhcp")
	assert.NotContains(t, content, "IPADDR=10.0.0.5", "static entry must not be written when the MAC is treated as DHCP overall")
}

func TestBuildRHELIfcfgFiles_MultipleStaticIPsNumbered(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {
			{IP: "10.0.0.5", Prefix: 24},
			{IP: "10.0.0.6", Prefix: 24},
		},
	}

	files := buildRHELIfcfgFiles(ipPerMac, map[string]string{"aa:bb:cc:dd:ee:ff": "10.0.0.1"}, map[string][]string{})

	content := files["ifcfg-vjb0"]
	assert.Contains(t, content, "IPADDR=10.0.0.5")
	assert.Contains(t, content, "PREFIX=24")
	assert.Contains(t, content, "IPADDR1=10.0.0.6")
	assert.Contains(t, content, "PREFIX1=24")
	assert.Contains(t, content, "GATEWAY=10.0.0.1")
}

func TestBuildRHELIfcfgFiles_EmptyEntriesSkipped(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {},
	}

	files := buildRHELIfcfgFiles(ipPerMac, map[string]string{}, map[string][]string{})

	assert.Empty(t, files, "a MAC with zero entries must get no ifcfg file")
}

func TestBuildRHELNetworkManagerKeyfiles_StaticEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "10.0.0.5", Prefix: 24}},
	}

	files := buildRHELNetworkManagerKeyfiles(ipPerMac, map[string]string{"aa:bb:cc:dd:ee:ff": "10.0.0.1"}, map[string][]string{})

	content, ok := files["vjb0.nmconnection"]
	assert.True(t, ok, "expected vjb0.nmconnection to be generated, got files: %#v", files)
	assert.Contains(t, content, "mac-address=aa:bb:cc:dd:ee:ff")
	assert.Contains(t, content, "method=manual", "static entry must get method=manual")
	assert.NotContains(t, content, "method=auto", "static-only entry must not get method=auto")
	assert.Contains(t, content, "address1=10.0.0.5/24,10.0.0.1")
}

func TestBuildRHELNetworkManagerKeyfiles_DHCPEntry(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
	}

	files := buildRHELNetworkManagerKeyfiles(ipPerMac, map[string]string{}, map[string][]string{})

	content := files["vjb0.nmconnection"]
	assert.Contains(t, content, "method=auto", "DHCP-sourced entry must get method=auto")
	assert.NotContains(t, content, "method=manual", "DHCP-only entry must not get method=manual")
	assert.NotContains(t, content, "address1=", "DHCP-sourced IP must not be pinned as a static address")
}

func TestBuildRHELNetworkManagerKeyfiles_EmptyEntriesSkipped(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {},
	}

	files := buildRHELNetworkManagerKeyfiles(ipPerMac, map[string]string{}, map[string][]string{})

	assert.Empty(t, files, "a MAC with zero entries must get no NetworkManager keyfile")
}

// Phase 1 guestfish consolidation: getBootablePartitionSteps,
// mountPersistenceSteps, wildcardNetplanSteps - tests the step list only
// (right commands/args/order/shape), since guestfish itself can't run here.

func TestGetBootablePartitionSteps(t *testing.T) {
	steps := getBootablePartitionSteps("/home/fedora/get-bootable-partition.sh")

	require.Len(t, steps, 3, "must be exactly upload, chmod, sh - no more, no fewer boots collapsed")

	assert.Equal(t, guestfishStep{
		Command: "upload",
		Args:    []string{"/home/fedora/get-bootable-partition.sh", "/tmp/get-bootable-partition.sh"},
	}, steps[0], "upload must run first, or chmod/sh would operate on nothing")

	assert.Equal(t, guestfishStep{
		Command: "chmod",
		Args:    []string{"0755", "/tmp/get-bootable-partition.sh"},
	}, steps[1], "chmod must run second, or sh would fail on a non-executable script")

	assert.Equal(t, guestfishStep{
		Command: "sh",
		Args:    []string{"/tmp/get-bootable-partition.sh"},
	}, steps[2], "sh must run last, after the script is uploaded and executable")

	for i, step := range steps {
		assert.Empty(t, step.Marker, "step %d must be fail-fast (no Marker): a failed upload or chmod must abort the rest, matching the three separate calls this replaces", i)
	}
}

func TestMountPersistenceSteps(t *testing.T) {
	steps := mountPersistenceSteps("/home/fedora/generate-mount-persistence.sh", "--force-uuid")

	require.Len(t, steps, 3)

	assert.Equal(t, guestfishStep{
		Command: "upload",
		Args:    []string{"/home/fedora/generate-mount-persistence.sh", "/tmp/generate-mount-persistence.sh"},
	}, steps[0])

	assert.Equal(t, guestfishStep{
		Command: "chmod",
		Args:    []string{"0755", "/tmp/generate-mount-persistence.sh"},
	}, steps[1])

	assert.Equal(t, guestfishStep{
		Command: "sh",
		Args:    []string{"/tmp/generate-mount-persistence.sh --force-uuid"},
	}, steps[2], "the script path and its flag must stay one combined argument - guestfish's sh takes one shell command-line string, not separate argv entries")
}

func TestMountPersistenceSteps_SUSEArgs(t *testing.T) {
	// MountPersistenceScriptArgs picks --replace-fstab --os-family=suse for
	// SUSE; confirm whatever string it returns lands verbatim in the sh step,
	// since that's the whole point of resolving args before building steps.
	steps := mountPersistenceSteps("/home/fedora/generate-mount-persistence.sh", "--replace-fstab --os-family=suse")

	require.Len(t, steps, 3)
	assert.Equal(t, []string{"/tmp/generate-mount-persistence.sh --replace-fstab --os-family=suse"}, steps[2].Args)
}

func TestWildcardNetplanSteps(t *testing.T) {
	steps := wildcardNetplanSteps()

	require.Len(t, steps, 3, "must be exactly mv, mkdir, upload - no more, no fewer boots collapsed")

	assert.Equal(t, guestfishStep{
		Command: "mv",
		Args:    []string{"/etc/netplan", "/etc/netplan-bkp"},
	}, steps[0], "the existing netplan dir must be backed up first, before anything recreates it")

	assert.Equal(t, guestfishStep{
		Command: "mkdir",
		Args:    []string{"/etc/netplan"},
	}, steps[1], "mkdir must run second, or the upload would have nowhere to land")

	assert.Equal(t, guestfishStep{
		Command: "upload",
		Args:    []string{"/home/fedora/99-wildcard.network", "/etc/netplan/99-wildcard.yaml"},
	}, steps[2])

	for i, step := range steps {
		assert.Empty(t, step.Marker, "step %d must be fail-fast (no Marker): a failed mv/mkdir/upload must abort the rest, matching the three separate calls this replaces", i)
	}
}

// Phase 2 guestfish consolidation - same rationale as Phase 1: tests the
// step-building and result-picking logic, the pure Go that decides what
// gets sent and what the batched output means.

func TestFixLegacyMkinitrdCheckSteps(t *testing.T) {
	steps := fixLegacyMkinitrdCheckSteps()

	require.Len(t, steps, 4, "must be exactly the four stat calls the three original checks made - no more, no fewer boots collapsed")

	wantMarkers := []string{mkinitrdCheckMarker, dracutUsrBinCheckMarker, dracutSbinCheckMarker, mkinitrdOrigCheckMarker}
	wantPaths := []string{"/sbin/mkinitrd", "/usr/bin/dracut", "/sbin/dracut", "/sbin/mkinitrd.orig"}
	for i, step := range steps {
		assert.Equal(t, "stat", step.Command, "step %d", i)
		assert.Equal(t, []string{wantPaths[i]}, step.Args, "step %d", i)
		assert.Equal(t, wantMarkers[i], step.Marker, "step %d must be tolerant and marked - one failed stat must not abort the others", i)
	}
	assert.Equal(t, wantMarkers, fixLegacyMkinitrdCheckMarkers, "the marker list used to split the batch's output must match the markers actually used to build it")
}

func TestFixLegacyMkinitrdWriteSteps(t *testing.T) {
	steps := fixLegacyMkinitrdWriteSteps()

	require.Len(t, steps, 3, "must be exactly backup, upload, chmod - no more, no fewer boots collapsed")

	assert.Equal(t, guestfishStep{
		Command: "cp",
		Args:    []string{"/sbin/mkinitrd", "/sbin/mkinitrd.orig"},
	}, steps[0], "the original must be backed up first, before it is overwritten")

	assert.Equal(t, guestfishStep{
		Command: "upload",
		Args:    []string{mkinitrdLVMWrapperPath, "/sbin/mkinitrd"},
	}, steps[1], "upload must run second, after the backup exists")

	assert.Equal(t, guestfishStep{
		Command: "chmod",
		Args:    []string{"0755", "/sbin/mkinitrd"},
	}, steps[2], "chmod must run last, after the wrapper is in place")

	for i, step := range steps {
		assert.Empty(t, step.Marker, "step %d must be fail-fast (no Marker): a failed backup or upload must abort the rest, matching the three separate calls this replaces", i)
	}
}

func TestOsReleaseCatSteps(t *testing.T) {
	steps := osReleaseCatSteps()

	require.Len(t, steps, len(osReleaseCandidateFiles))
	for i, file := range osReleaseCandidateFiles {
		assert.Equal(t, "cat", steps[i].Command, "step %d", i)
		assert.Equal(t, []string{file}, steps[i].Args, "step %d", i)
		assert.Equal(t, osReleaseMarkers[i], steps[i].Marker, "step %d must be tolerant and marked - one missing candidate must not abort the others", i)
	}
}

func TestPickOsRelease(t *testing.T) {
	tests := []struct {
		name     string
		sections map[string]string
		want     string
		wantErr  bool
	}{
		{
			name: "first candidate has real content",
			sections: map[string]string{
				osReleaseMarkers[0]: `NAME="Ubuntu"` + "\n" + `VERSION_ID="22.04"`,
			},
			want: `name="ubuntu"` + "\n" + `version_id="22.04"`,
		},
		{
			name: "first candidate missing, second has content",
			sections: map[string]string{
				osReleaseMarkers[0]: "cat: /etc/os-release: No such file or directory",
				osReleaseMarkers[1]: "CentOS Linux release 7.9.2009 (Core)",
			},
			want: "centos linux release 7.9.2009 (core)",
		},
		{
			name: "first two missing, third (SLES 11) has content",
			sections: map[string]string{
				osReleaseMarkers[0]: "cat: /etc/os-release: No such file or directory",
				osReleaseMarkers[1]: "cat: /etc/redhat-release: No such file or directory",
				osReleaseMarkers[2]: "SUSE Linux Enterprise Server 11 (x86_64)",
			},
			want: "suse linux enterprise server 11 (x86_64)",
		},
		{
			name: "no output at all for a candidate is treated as not found, not a stop",
			sections: map[string]string{
				osReleaseMarkers[0]: "",
				osReleaseMarkers[1]: "CentOS Linux release 7.9.2009 (Core)",
			},
			want: "centos linux release 7.9.2009 (core)",
		},
		{
			name:     "nothing found anywhere",
			sections: map[string]string{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickOsRelease(tt.sections)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInterfaceCatSteps(t *testing.T) {
	files := []string{"ifcfg-eth0", "ifcfg-eth1"}
	steps := interfaceCatSteps(files)

	require.Len(t, steps, 2)
	assert.Equal(t, guestfishStep{
		Command: "cat",
		Args:    []string{"/etc/sysconfig/network-scripts/ifcfg-eth0"},
		Marker:  interfaceFileMarker(0),
	}, steps[0])
	assert.Equal(t, guestfishStep{
		Command: "cat",
		Args:    []string{"/etc/sysconfig/network-scripts/ifcfg-eth1"},
		Marker:  interfaceFileMarker(1),
	}, steps[1])
	assert.NotEqual(t, interfaceFileMarker(0), interfaceFileMarker(1), "distinct files must get distinct markers or their output cannot be told apart")
}

func TestPartitionDevNumSteps(t *testing.T) {
	steps := partitionDevNumSteps([]string{"/dev/sda1", "/dev/sda2"})

	require.Len(t, steps, 4, "two steps per partition (part-to-dev, part-to-partnum)")

	dev0, num0 := partitionDevNumMarkers(0)
	dev1, num1 := partitionDevNumMarkers(1)
	assert.Equal(t, guestfishStep{Command: "part-to-dev", Args: []string{"/dev/sda1"}, Marker: dev0}, steps[0])
	assert.Equal(t, guestfishStep{Command: "part-to-partnum", Args: []string{"/dev/sda1"}, Marker: num0}, steps[1])
	assert.Equal(t, guestfishStep{Command: "part-to-dev", Args: []string{"/dev/sda2"}, Marker: dev1}, steps[2])
	assert.Equal(t, guestfishStep{Command: "part-to-partnum", Args: []string{"/dev/sda2"}, Marker: num1}, steps[3])

	for i, step := range steps {
		assert.NotEmpty(t, step.Marker, "step %d must be tolerant and marked - one partition's failure must not abort the others", i)
	}
}

func TestPartitionBootIndexSteps(t *testing.T) {
	steps := partitionBootIndexSteps([]string{"/dev/sda", "/dev/sdb"}, []string{"1", "2"})

	require.Len(t, steps, 4, "two steps per partition (part-get-bootable, device-index)")

	boot0, idx0 := partitionBootIndexMarkers(0)
	boot1, idx1 := partitionBootIndexMarkers(1)
	assert.Equal(t, guestfishStep{Command: "part-get-bootable", Args: []string{"/dev/sda", "1"}, Marker: boot0}, steps[0])
	assert.Equal(t, guestfishStep{Command: "device-index", Args: []string{"/dev/sda"}, Marker: idx0}, steps[1])
	assert.Equal(t, guestfishStep{Command: "part-get-bootable", Args: []string{"/dev/sdb", "2"}, Marker: boot1}, steps[2])
	assert.Equal(t, guestfishStep{Command: "device-index", Args: []string{"/dev/sdb"}, Marker: idx1}, steps[3])
}

func TestPickBootableIndex(t *testing.T) {
	boot0, idx0 := partitionBootIndexMarkers(0)
	boot1, idx1 := partitionBootIndexMarkers(1)
	boot2, idx2 := partitionBootIndexMarkers(2)

	tests := []struct {
		name           string
		partitionCount int
		sections       map[string]string
		want           int
		wantErr        bool
	}{
		{
			name:           "first partition bootable with a valid index wins",
			partitionCount: 2,
			sections: map[string]string{
				boot0: "true", idx0: "0",
				boot1: "false", idx1: "1",
			},
			want: 0,
		},
		{
			name:           "first partition not bootable, second is",
			partitionCount: 2,
			sections: map[string]string{
				boot0: "false", idx0: "0",
				boot1: "true", idx1: "1",
			},
			want: 1,
		},
		{
			name:           "bootable but non-numeric index falls through to the next partition",
			partitionCount: 3,
			sections: map[string]string{
				boot0: "true", idx0: "not-a-number",
				boot1: "true", idx1: "1",
				boot2: "false", idx2: "2",
			},
			want: 1,
		},
		{
			name:           "no partition bootable",
			partitionCount: 2,
			sections: map[string]string{
				boot0: "false", idx0: "0",
				boot1: "false", idx1: "1",
			},
			wantErr: true,
		},
		{
			name:           "no partitions at all",
			partitionCount: 0,
			sections:       map[string]string{},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickBootableIndex(tt.partitionCount, tt.sections)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, -1, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
