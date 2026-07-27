// Copyright © 2024 The vjailbreak authors

package nbd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasswordRedactionLogic verifies that passwords are properly redacted from command strings
// before being logged.
//
// IMPORTANT FOR DEVELOPERS:
// When logging commands that contain sensitive information (passwords, tokens, API keys, etc.),
// always build a redacted command string BEFORE passing it to logging functions.
//
// Example usage pattern (from nbdops.go StartNBDServer):
//   1. Create your command with the actual password
//   2. Build a redacted string by replacing password with [REDACTED]
//   3. Pass the redacted string to utils.AddDebugOutputToFileWithCommand()
//
//   password := "SecretPassword123"
//   cmd := exec.Command("nbdkit", "server=vcenter.local", fmt.Sprintf("password=%s", password))
//
//   // Build redacted command string
//   cmdstring := ""
//   for _, arg := range cmd.Args {
//       if strings.Contains(arg, password) {
//           cmdstring += "password=[REDACTED] "
//       } else {
//           cmdstring += fmt.Sprintf("%s ", arg)
//       }
//   }
//
//   // Use the redacted string for logging
//   utils.AddDebugOutputToFileWithCommand(cmd, cmdstring)
//
// WARNING: Never pass cmd.String() directly to logging functions if the command contains
// sensitive data. Always build a redacted version first.
func TestPasswordRedactionLogic(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "SimplePassword123"},
		{"complex password", "C0mpl3x!P@ssw0rd#2024"},
		{"password with spaces", "My Secret Password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the command that would be created in StartNBDServer
			cmdArgs := []string{
				"nbdkit",
				"server=vcenter.local",
				"user=admin",
				fmt.Sprintf("password=%s", tt.password),
				"thumbprint=AA:BB:CC",
			}

			cmd := &exec.Cmd{Args: cmdArgs}

			// Apply the same redaction logic from nbdops.go lines 120-127
			cmdstring := ""
			for _, arg := range cmd.Args {
				if strings.Contains(arg, tt.password) {
					cmdstring += "password=[REDACTED] "
				} else {
					cmdstring += fmt.Sprintf("%s ", arg)
				}
			}

			// Verify password is redacted
			assert.Contains(t, cmdstring, "password=[REDACTED]")
			assert.NotContains(t, cmdstring, tt.password)

			// Verify other parameters are visible
			assert.Contains(t, cmdstring, "server=vcenter.local")
			assert.Contains(t, cmdstring, "user=admin")
		})
	}
}

// TestBuildNbdcopyArgs covers the --target-is-zero decision used by
// CopyDisk. Encrypted Cinder destinations must never receive
// --target-is-zero: encryption is applied transparently at the QEMU layer,
// so a region skipped as "already zero" was never actually written through
// the encryption engine and does not decrypt back to zero on readback. See
// buildNbdcopyArgs's doc comment for the full explanation.
func TestBuildNbdcopyArgs(t *testing.T) {
	const sockUrl = "nbd+unix:///?socket=/tmp/nbdkit-test/nbdkit.sock"
	const dest = "/dev/sda"

	tests := []struct {
		name          string
		destEncrypted bool
		wantArgs      []string
	}{
		{
			name:          "plain (unencrypted) destination keeps --target-is-zero",
			destEncrypted: false,
			wantArgs:      []string{"--progress=3", "--target-is-zero", sockUrl, dest},
		},
		{
			name:          "encrypted destination drops --target-is-zero and does a dense copy",
			destEncrypted: true,
			wantArgs:      []string{"--progress=3", sockUrl, dest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs := buildNbdcopyArgs(sockUrl, dest, tt.destEncrypted)
			assert.Equal(t, tt.wantArgs, gotArgs)
		})
	}
}

// TestBuildNbdcopyArgs_NeverSkipsZeroTrustSilently is a focused regression
// guard: whatever buildNbdcopyArgs does, an encrypted destination must never
// end up with --target-is-zero in the argument list, and a plain
// destination must always keep the optimization (so cold-migration copy
// speed doesn't silently regress for the common, unencrypted case).
func TestBuildNbdcopyArgs_NeverSkipsZeroTrustSilently(t *testing.T) {
	encryptedArgs := buildNbdcopyArgs("sock", "dest", true)
	assert.NotContains(t, encryptedArgs, "--target-is-zero",
		"encrypted destinations must not use --target-is-zero")

	plainArgs := buildNbdcopyArgs("sock", "dest", false)
	assert.Contains(t, plainArgs, "--target-is-zero",
		"plain destinations should keep the --target-is-zero optimization")
}

// newFilledTempFile creates a temp file of size bytes, filled with 0xFF so
// that "became zero" is observable (a fresh file is already zero, which would
// make the assertions meaningless).
func newFilledTempFile(t *testing.T, size int) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dest.img")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{0xFF}, size), 0o644))
	fd, err := os.OpenFile(path, os.O_RDWR, 0o644)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fd.Close() })
	return fd
}

func readAllFrom(t *testing.T, fd *os.File, size int) []byte {
	t.Helper()
	buf := make([]byte, size)
	_, err := fd.ReadAt(buf, 0)
	require.NoError(t, err)
	return buf
}

// TestWriteZeros verifies that writeZeros zeroes exactly the requested range
// and leaves the surrounding bytes untouched. This is the code path an
// encrypted destination always takes (real zeros written through the device),
// so getting the bounds right matters: an off-by-one would either leave stale
// data inside the range or clobber data outside it.
func TestWriteZeros(t *testing.T) {
	const size = 4096
	const zeroStart = 1024
	const zeroLen = 2048

	fd := newFilledTempFile(t, size)
	require.NoError(t, writeZeros(fd, zeroStart, zeroLen))

	got := readAllFrom(t, fd, size)
	for i := 0; i < size; i++ {
		if i >= zeroStart && i < zeroStart+zeroLen {
			assert.Equalf(t, byte(0x00), got[i], "byte %d should be zeroed", i)
		} else {
			assert.Equalf(t, byte(0xFF), got[i], "byte %d outside the range must be untouched", i)
		}
	}
}

// TestZeroRange_ReadsBackAsZero checks the observable contract of zeroRange
// for both destination types: after the call, the target range must read back
// as zero. For the encrypted case this exercises the writeZeros path directly
// (no fallocate). For the plain case it exercises the punch-hole path, with a
// pwrite fallback on filesystems that don't support FALLOC_FL_PUNCH_HOLE;
// either way the range must read as zero. The surrounding bytes must be
// preserved in both cases.
func TestZeroRange_ReadsBackAsZero(t *testing.T) {
	const size = 4096
	const zeroStart = 512
	const zeroLen = 1024

	for _, destEncrypted := range []bool{true, false} {
		name := "plain"
		if destEncrypted {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			fd := newFilledTempFile(t, size)
			require.NoError(t, zeroRange(fd, zeroStart, zeroLen, destEncrypted))

			got := readAllFrom(t, fd, size)
			for i := zeroStart; i < zeroStart+zeroLen; i++ {
				assert.Equalf(t, byte(0x00), got[i], "byte %d should read back as zero", i)
			}
			// Surrounding bytes preserved.
			assert.Equal(t, byte(0xFF), got[zeroStart-1], "byte before range must be untouched")
			assert.Equal(t, byte(0xFF), got[zeroStart+zeroLen], "byte after range must be untouched")
		})
	}
}
