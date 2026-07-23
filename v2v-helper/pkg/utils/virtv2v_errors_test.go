// Copyright © 2024 The vjailbreak authors

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVirtV2VFailureReason(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "free space error",
			content: `[   0.0] Setting up the source
[ 103.7] Checking for sufficient free disk space in the guest
virt-v2v-in-place: error: not enough free space for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed
If reporting bugs, run virt-v2v-in-place with debugging enabled and include the complete output:`,
			expected: "virt-v2v-in-place: error: not enough free space for conversion on filesystem '/corefiles'.  0.0 MB free < 10 MB needed",
		},
		{
			name: "e2fsck filesystem corruption",
			content: "Pass 1: Checking inodes, blocks, and sizes\r\n" +
				"/dev/sda1: ********** WARNING: Filesystem still has errors **********\r\n" +
				"\r\n" +
				"/dev/sda1: 514134/10354688 files (0.2% non-contiguous), 16468634/41418496 blocks\r\n" +
				"guestfsd: error: e2fsck 1.47.2 (1-Jan-2025)virt-v2v-in-place: error: libguestfs error: e2fsck: e2fsck 1.47.2 (1-Jan-2025)\n",
			expected: "guestfsd: error: e2fsck 1.47.2 (1-Jan-2025)virt-v2v-in-place: error: libguestfs error: e2fsck: e2fsck 1.47.2 (1-Jan-2025)",
		},
		{
			name:     "generic fallback with unrecognized error line",
			content:  "some unrelated output\nsomething went wrong, error: disk not found\nrm -rf -- '/tmp/v2v.abc'",
			expected: "something went wrong, error: disk not found",
		},
		{
			name: "insufficient free space in conversion server temp directory",
			content: `[   0.0] Setting up the source
virt-v2v-in-place: error: insufficient free space in the conversion server temporary directory /var/tmp/v2v.abcd
If reporting bugs, run virt-v2v-in-place with debugging enabled and include the complete output:`,
			expected: "virt-v2v-in-place: error: insufficient free space in the conversion server temporary directory /var/tmp/v2v.abcd",
		},
		{
			name: "multi-boot guest without --root",
			content: `[  10.2] Inspecting the source
virt-v2v-in-place: error: multi-boot operating systems are not supported by virt-v2v-in-place. Use the --root option to select the root filesystem to convert.`,
			expected: "virt-v2v-in-place: error: multi-boot operating systems are not supported by virt-v2v-in-place. Use the --root option to select the root filesystem to convert.",
		},
		{
			name: "no OS detected during inspection",
			content: `[   5.0] Inspecting the source
virt-v2v-in-place: error: inspection could not detect the source guest (or physical machine).`,
			expected: "virt-v2v-in-place: error: inspection could not detect the source guest (or physical machine).",
		},
		{
			name: "xfs filesystem errors detected post-conversion",
			content: `[ 120.0] Converting Linux to run on KVM
virt-v2v-in-place: error: detected errors on the XFS filesystem on /dev/sda1`,
			expected: "virt-v2v-in-place: error: detected errors on the XFS filesystem on /dev/sda1",
		},
		{
			name: "windows hibernation blocks NTFS mount",
			content: `[  15.0] Setting up the source
virt-v2v-in-place: error: unable to mount the disk image for writing. This has probably happened because Windows Hibernation or Fast Restart is being used in this guest. You have to disable this (in the guest) in order to use virt-v2v.

Original error message: NTFS partition is in an unsafe state`,
			expected: "virt-v2v-in-place: error: unable to mount the disk image for writing. This has probably happened because Windows Hibernation or Fast Restart is being used in this guest. You have to disable this (in the guest) in order to use virt-v2v.",
		},
		{
			name: "unrecognized guest type has no conversion module",
			content: `[   8.0] Inspecting the source
virt-v2v-in-place: error: virt-v2v is unable to convert this guest type (linux/unknown)`,
			expected: "virt-v2v-in-place: error: virt-v2v is unable to convert this guest type (linux/unknown)",
		},
		{
			name: "missing initrd rebuild tool in guest",
			content: `[  90.0] Converting Debian
virt-v2v-in-place: error: unable to rebuild initrd (/boot/initrd.img-5.10.0) because update-initramfs was not found in the guest`,
			expected: "virt-v2v-in-place: error: unable to rebuild initrd (/boot/initrd.img-5.10.0) because update-initramfs was not found in the guest",
		},
		{
			name:     "no error lines",
			content:  "everything is fine\nconversion succeeded\n",
			expected: "",
		},
		{
			name:     "empty file",
			content:  "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "virtv2v.log")
			require.NoError(t, os.WriteFile(logPath, []byte(tc.content), 0644))

			got := ExtractVirtV2VFailureReason(logPath)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestExtractVirtV2VFailureReason_MissingFile(t *testing.T) {
	got := ExtractVirtV2VFailureReason("/nonexistent/path/virtv2v.log")
	assert.Equal(t, "", got)
}

func TestGetLatestLogFilePath(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	attemptDir := filepath.Join(tmpDir, migrationName)
	require.NoError(t, os.MkdirAll(attemptDir, 0755))

	older := filepath.Join(attemptDir, "virtv2v.2024-01-01-10:00:00.log")
	newer := filepath.Join(attemptDir, "virtv2v.2024-01-02-10:00:00.log")
	other := filepath.Join(attemptDir, "nbd.2024-01-03-10:00:00.log")

	require.NoError(t, os.WriteFile(older, []byte("old"), 0644))
	require.NoError(t, os.WriteFile(newer, []byte("new"), 0644))
	require.NoError(t, os.WriteFile(other, []byte("other"), 0644))

	got, err := GetLatestLogFilePath(migrationName, LogCategoryVirtV2V)
	require.NoError(t, err)
	assert.Equal(t, newer, got)
}

func TestGetLatestLogFilePath_NoMatches(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)
	_ = tmpDir

	_, err := GetLatestLogFilePath(migrationName, LogCategoryVirtV2V)
	assert.Error(t, err)
}
