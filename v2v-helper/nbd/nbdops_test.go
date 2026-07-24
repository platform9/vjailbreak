// Copyright © 2024 The vjailbreak authors

package nbd

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
