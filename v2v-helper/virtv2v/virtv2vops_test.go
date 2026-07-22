// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestBuildWildcardNetplanYAML_MixedEntries(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {
			{IP: "10.0.0.5", Prefix: 24},
			{IP: "192.168.50.77", Prefix: 0, DHCP: true},
		},
	}

	yaml := buildWildcardNetplanYAML(nil, map[string]string{}, ipPerMac)

	assert.Contains(t, yaml, "dhcp4: true", "any DHCP-sourced entry on the NIC should trigger dhcp4: true")
	assert.Contains(t, yaml, "- 10.0.0.5/24", "the static entry should still be written as an address")
	assert.NotContains(t, yaml, "192.168.50.77", "the DHCP-sourced entry must not appear under addresses:")
}

func TestBuildWildcardNetplanYAML_EmptyEntriesSkipped(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {},
	}

	yaml := buildWildcardNetplanYAML(nil, map[string]string{}, ipPerMac)

	assert.NotContains(t, yaml, "aa:bb:cc:dd:ee:ff", "a MAC with zero entries must get no ethernet stanza")
}

func TestBuildWildcardNetplanYAML_GatewayUnaffectedByDHCPFlag(t *testing.T) {
	ipPerMac := map[string][]vm.IpEntry{
		"aa:bb:cc:dd:ee:ff": {{IP: "192.168.50.77", Prefix: 0, DHCP: true}},
	}
	gatewayIP := map[string]string{"aa:bb:cc:dd:ee:ff": "192.168.50.1"}

	yaml := buildWildcardNetplanYAML(nil, gatewayIP, ipPerMac)

	assert.Contains(t, yaml, "via: 192.168.50.1", "gateway route must still be written for a DHCP-sourced entry")
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
