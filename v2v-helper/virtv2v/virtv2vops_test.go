// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestMkinitrdLVMWrapperContent verifies the embedded wrapper script contains
// the required translation logic and safety guards.
func TestMkinitrdLVMWrapperContent(t *testing.T) {
	// Verify the wrapper calls the original binary
	assert.Contains(t, mkinitrdLVMWrapper, "/sbin/mkinitrd.orig",
		"wrapper must delegate to the backed-up original")

	// Verify -d flag handling is present
	assert.Contains(t, mkinitrdLVMWrapper, "-d",
		"wrapper must handle the -d flag")

	// Verify /dev/mapper translation is present
	assert.Contains(t, mkinitrdLVMWrapper, "/dev/mapper/",
		"wrapper must translate to /dev/mapper/ path")

	// Verify argument-boundary preservation via xargs -0
	assert.Contains(t, mkinitrdLVMWrapper, "xargs -0",
		"wrapper must use xargs -0 to preserve argument boundaries across spaces")

	// Verify temp-file cleanup on exit
	assert.Contains(t, mkinitrdLVMWrapper, "trap",
		"wrapper must clean up temp file via trap")

	// Verify shebang
	assert.True(t, len(mkinitrdLVMWrapper) > 0 && mkinitrdLVMWrapper[0:2] == "#!",
		"wrapper must start with a shebang")
}

// TestMkinitrdWrapperWritable verifies the wrapper can be written to disk with
// correct permissions (mirrors the write step inside FixLegacyMkinitrd).
func TestMkinitrdWrapperWritable(t *testing.T) {
	dir := t.TempDir()
	wrapperPath := filepath.Join(dir, "mkinitrd-lvm-wrapper.sh")

	err := os.WriteFile(wrapperPath, []byte(mkinitrdLVMWrapper), 0755)
	require.NoError(t, err, "wrapper should be writable")

	info, err := os.Stat(wrapperPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
		"wrapper file should be executable")

	content, err := os.ReadFile(wrapperPath)
	require.NoError(t, err)
	assert.Equal(t, mkinitrdLVMWrapper, string(content),
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
// FixGrubDeviceMap – regex replacement logic
//
// The guestfish I/O calls require real block devices; we test the pure
// substitution regex in isolation so it can run on any platform.
// ---------------------------------------------------------------------------

// deviceMapReplace mirrors the regex replacement inside FixGrubDeviceMap.
func deviceMapReplace(content string) string {
	re := regexp.MustCompile(`/dev/sd([a-z])`)
	return re.ReplaceAllString(content, "/dev/vd$1")
}

func TestDeviceMapReplacement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		change bool // whether we expect a replacement to occur
	}{
		{
			name:   "4-disk SLES 11 (hd3 on sdd)",
			input:  "(hd0) /dev/sda\n(hd1) /dev/sdb\n(hd2) /dev/sdc\n(hd3) /dev/sdd\n",
			want:   "(hd0) /dev/vda\n(hd1) /dev/vdb\n(hd2) /dev/vdc\n(hd3) /dev/vdd\n",
			change: true,
		},
		{
			name:   "single disk",
			input:  "(hd0) /dev/sda\n",
			want:   "(hd0) /dev/vda\n",
			change: true,
		},
		{
			name:   "already vd paths – no change needed",
			input:  "(hd0) /dev/vda\n(hd1) /dev/vdb\n(hd2) /dev/vdc\n(hd3) /dev/vdd\n",
			want:   "(hd0) /dev/vda\n(hd1) /dev/vdb\n(hd2) /dev/vdc\n(hd3) /dev/vdd\n",
			change: false,
		},
		{
			name:   "mixed – only sd entries replaced",
			input:  "(hd0) /dev/vda\n(hd1) /dev/sdb\n",
			want:   "(hd0) /dev/vda\n(hd1) /dev/vdb\n",
			change: true,
		},
		{
			name:   "sda through sdz are all handled",
			input:  "/dev/sdz",
			want:   "/dev/vdz",
			change: true,
		},
		{
			name:   "non-sd device (xvd) left untouched",
			input:  "(hd0) /dev/xvda\n",
			want:   "(hd0) /dev/xvda\n",
			change: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deviceMapReplace(tt.input)
			assert.Equal(t, tt.want, got)
			if tt.change {
				assert.NotEqual(t, tt.input, got, "expected content to change but it did not")
			} else {
				assert.Equal(t, tt.input, got, "expected content unchanged but it was modified")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReinstallGrubLegacy – pure-logic tests (no guestfish required)
// ---------------------------------------------------------------------------

// grubDiskList mirrors the disk-list generation logic inside ReinstallGrubLegacy
// so we can unit-test it without a real guestfish instance.
func grubDiskList(n int) (sdList, vdList string) {
	sdNames := make([]string, n)
	vdNames := make([]string, n)
	for i := 0; i < n; i++ {
		sdNames[i] = fmt.Sprintf("/dev/sd%c", rune('a'+i))
		vdNames[i] = fmt.Sprintf("/dev/vd%c", rune('a'+i))
	}
	return strings.Join(sdNames, " "), strings.Join(vdNames, " ")
}

// TestGrubDiskListExact verifies that the generated disk lists contain EXACTLY
// n entries, with the correct device names, and never include the appliance
// scratch disk (sd{a+n} / vd{a+n}).
func TestGrubDiskListExact(t *testing.T) {
	tests := []struct {
		diskCount int
		wantSD    string
		wantVD    string
	}{
		{1, "/dev/sda", "/dev/vda"},
		{2, "/dev/sda /dev/sdb", "/dev/vda /dev/vdb"},
		{4, "/dev/sda /dev/sdb /dev/sdc /dev/sdd", "/dev/vda /dev/vdb /dev/vdc /dev/vdd"},
		// 4-disk SLES 11 LVM case: must NOT include /dev/sde (appliance disk)
		{4, "/dev/sda /dev/sdb /dev/sdc /dev/sdd", "/dev/vda /dev/vdb /dev/vdc /dev/vdd"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("n=%d", tt.diskCount), func(t *testing.T) {
			sd, vd := grubDiskList(tt.diskCount)
			assert.Equal(t, tt.wantSD, sd, "sd list mismatch")
			assert.Equal(t, tt.wantVD, vd, "vd list mismatch")

			// Verify appliance disk is excluded
			applianceSd := fmt.Sprintf("/dev/sd%c", rune('a'+tt.diskCount))
			applianceVd := fmt.Sprintf("/dev/vd%c", rune('a'+tt.diskCount))
			assert.NotContains(t, sd, applianceSd, "sd list must not contain appliance disk %s", applianceSd)
			assert.NotContains(t, vd, applianceVd, "vd list must not contain appliance disk %s", applianceVd)
		})
	}
}

// TestGrubMenuLstRootReplace verifies the sed pattern used in ReinstallGrubLegacy
// to patch "root (hdX,Y)" lines in menu.lst with the correct GRUB root.
// The pattern must match the GRUB root command but NOT the kernel root= parameter.
func grubMenuLstRootReplace(content, grubRoot string) string {
	re := regexp.MustCompile(`root \(hd[0-9]+,[0-9]+\)`)
	return re.ReplaceAllString(content, "root "+grubRoot)
}

func TestGrubMenuLstRootReplace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		grubRoot string
		want     string
	}{
		{
			name: "correct root already set – no change",
			input: "\troot (hd3,0)\n\tkernel /vmlinuz root=/dev/vg_root/lv_root\n",
			grubRoot: "(hd3,0)",
			want:     "\troot (hd3,0)\n\tkernel /vmlinuz root=/dev/vg_root/lv_root\n",
		},
		{
			name: "wrong root (hd0,0) replaced with (hd3,0)",
			input: "\troot (hd0,0)\n\tkernel /vmlinuz root=/dev/vg_root/lv_root\n",
			grubRoot: "(hd3,0)",
			want:     "\troot (hd3,0)\n\tkernel /vmlinuz root=/dev/vg_root/lv_root\n",
		},
		{
			name: "multiple title blocks all fixed",
			input: "title SLES 11\n\troot (hd0,0)\n\tkernel /vmlinuz\ntitle SLES 11 fallback\n\troot (hd0,0)\n\tkernel /vmlinuz.old\n",
			grubRoot: "(hd3,0)",
			want:     "title SLES 11\n\troot (hd3,0)\n\tkernel /vmlinuz\ntitle SLES 11 fallback\n\troot (hd3,0)\n\tkernel /vmlinuz.old\n",
		},
		{
			name: "kernel root= parameter NOT modified",
			input: "\troot (hd0,0)\n\tkernel /vmlinuz root=/dev/sda1\n",
			grubRoot: "(hd3,0)",
			want:     "\troot (hd3,0)\n\tkernel /vmlinuz root=/dev/sda1\n",
		},
		{
			name: "no root command in menu.lst – unchanged",
			input: "default 0\ntimeout 5\n",
			grubRoot: "(hd3,0)",
			want:     "default 0\ntimeout 5\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grubMenuLstRootReplace(tt.input, tt.grubRoot)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDeviceMapRoundTrip verifies that writing and reading back the fixed
// content produces the expected device.map file on disk.
func TestDeviceMapRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "device.map")

	input := "(hd0) /dev/sda\n(hd1) /dev/sdb\n(hd2) /dev/sdc\n(hd3) /dev/sdd\n"
	expected := "(hd0) /dev/vda\n(hd1) /dev/vdb\n(hd2) /dev/vdc\n(hd3) /dev/vdd\n"

	require.NoError(t, os.WriteFile(path, []byte(input), 0644))

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	fixed := deviceMapReplace(string(content))
	require.NoError(t, os.WriteFile(path, []byte(fixed), 0644))

	result, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(result))
}
