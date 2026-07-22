// Copyright © 2024 The vjailbreak authors

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractNBDFailureReason(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "thumbprint mismatch",
			content: `nbdkit: debug: vddk: opening
nbdkit: debug: VixDiskLibVim: Failed to verify SSL certificate: actual thumbprint=B2:31:BD:DE:9F:DB:9D:E0:78:EF:30:42:8A:41:B0:28:92:93:C8:DD expected=12
nbdkit: vddk[1]: error: cannot connect: certificate verification failed`,
			expected: "nbdkit: debug: VixDiskLibVim: Failed to verify SSL certificate: actual thumbprint=B2:31:BD:DE:9F:DB:9D:E0:78:EF:30:42:8A:41:B0:28:92:93:C8:DD expected=12",
		},
		{
			name: "ESXi DNS resolution failure",
			content: `[nbdkit] connecting to esxi.internal.example.com
nbdkit: vddk[1]: error: [NFC ERROR] Failed to connect: Host address lookup for server 'esxi.internal.example.com' failed: Name or service not known`,
			expected: "nbdkit: vddk[1]: error: [NFC ERROR] Failed to connect: Host address lookup for server 'esxi.internal.example.com' failed: Name or service not known",
		},
		{
			name: "generic DNS failure without VDDK wrapper text",
			content: `Connecting to server 10.0.0.5
some_lib: error: getaddrinfo failed: Temporary failure in name resolution`,
			expected: "some_lib: error: getaddrinfo failed: Temporary failure in name resolution",
		},
		{
			name: "port 902 blocked - connection refused",
			content: `nbdkit: vddk[1]: debug: attempting NFC connection on port 902
nbdkit: vddk[1]: error: [NFC ERROR] Failed to connect to esxi.example.com:902: Connection refused`,
			expected: "nbdkit: vddk[1]: error: [NFC ERROR] Failed to connect to esxi.example.com:902: Connection refused",
		},
		{
			name: "port 902 blocked - connection timed out",
			content: `nbdkit: vddk[1]: debug: attempting NFC connection on port 902
nbdkit: vddk[1]: error: [NFC ERROR] Failed to connect to esxi.example.com:902: Connection timed out`,
			expected: "nbdkit: vddk[1]: error: [NFC ERROR] Failed to connect to esxi.example.com:902: Connection timed out",
		},
		{
			name: "NFC out of memory on ESXi hostd",
			content: `nbdkit: vddk[3]: debug: reading 2097176 bytes
nbdkit: vddk[3]: error: [NFC ERROR] NfcFssrvrProcessErrorMsg: received NFC error 5 from server: Failed to allocate the requested 2097176 bytes`,
			expected: "nbdkit: vddk[3]: error: [NFC ERROR] NfcFssrvrProcessErrorMsg: received NFC error 5 from server: Failed to allocate the requested 2097176 bytes",
		},
		{
			name: "local VMDK lock file permission denied",
			content: `nbdkit: vddk[1]: debug: opening local file
nbdkit: vddk[1]: error: FILE: FileLockCreateEntryDirectory creation failure on '/absolute/path/to/file.vmdk.lck': Permission denied`,
			expected: "nbdkit: vddk[1]: error: FILE: FileLockCreateEntryDirectory creation failure on '/absolute/path/to/file.vmdk.lck': Permission denied",
		},
		{
			name: "independent mode disk",
			content: `nbdkit: vddk[1]: debug: GetFileName: Cannot create disk spec for disk scsi0:0. Error occurred when obtaining the file name for scsi0:0.`,
			expected: "nbdkit: vddk[1]: debug: GetFileName: Cannot create disk spec for disk scsi0:0. Error occurred when obtaining the file name for scsi0:0.",
		},
		{
			name: "missing libssl.so.3 on RHEL8",
			content: `Starting nbdkit
nbdkit: error: libssl.so.3: cannot open shared object file: No such file or directory`,
			expected: "nbdkit: error: libssl.so.3: cannot open shared object file: No such file or directory",
		},
		{
			name: "generic VDDK Error 1 open failure",
			content: `nbdkit: vddk[1]: debug: VixDiskLib: VixDiskLib_OpenEx: Cannot open disk [datastore1] vm/vm.vmdk. Error 1 (Unknown error) at 1234.`,
			expected: "nbdkit: vddk[1]: debug: VixDiskLib: VixDiskLib_OpenEx: Cannot open disk [datastore1] vm/vm.vmdk. Error 1 (Unknown error) at 1234.",
		},
		{
			name:     "nbdcopy generic error fallback",
			content:  "nbdkit: debug: some info\nnbdcopy: error: nbd_pread: unexpected disconnect",
			expected: "nbdcopy: error: nbd_pread: unexpected disconnect",
		},
		{
			name:     "generic fallback with unrecognized error line",
			content:  "some unrelated output\nsomething went wrong, error: could not open socket\nexiting",
			expected: "something went wrong, error: could not open socket",
		},
		{
			name:     "no error lines",
			content:  "everything is fine\ncopy completed\n",
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
			logPath := filepath.Join(dir, "nbd.log")
			require.NoError(t, os.WriteFile(logPath, []byte(tc.content), 0644))

			got := ExtractNBDFailureReason(logPath)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestExtractNBDFailureReason_MissingFile(t *testing.T) {
	got := ExtractNBDFailureReason("/nonexistent/path/nbd.log")
	assert.Equal(t, "", got)
}
