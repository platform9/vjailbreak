// Copyright © 2024 The vjailbreak authors
package migrate

import (
	"errors"
	"testing"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
	"github.com/stretchr/testify/assert"
)

// stubGuestCommands replaces the two libguestfs entry points detectBootVolume
// uses and restores them when the test ends.
func stubGuestCommands(
	t *testing.T,
	perDisk func(path, command string, write bool) (string, error),
	allDisks func(disks []vm.VMDisk, command string, write bool, args ...string) (string, error),
) {
	t.Helper()

	origPerDisk := runCommandInGuest
	origAllDisks := runCommandInGuestAllVolumes
	runCommandInGuest = perDisk
	runCommandInGuestAllVolumes = allDisks

	t.Cleanup(func() {
		runCommandInGuest = origPerDisk
		runCommandInGuestAllVolumes = origAllDisks
	})
}

func disks(paths ...string) []vm.VMDisk {
	out := make([]vm.VMDisk, 0, len(paths))
	for _, path := range paths {
		out = append(out, vm.VMDisk{Name: path, Path: path})
	}
	return out
}

func TestDetectBootVolume(t *testing.T) {
	errNoRoot := errors.New("inspect-os: no operating system found")

	tests := []struct {
		name string
		// perDiskAnswers maps a disk path to what the per-disk probe returns.
		// A missing entry means that disk is not individually inspectable.
		perDiskAnswers map[string]string
		allDisksAnswer string
		allDisksErr    error
		vmDisks        []vm.VMDisk

		wantIndex          int
		wantOSPath         string
		wantAllDisksCalled bool
	}{
		{
			// The common case, and the one the per-disk loop exists to protect:
			// a Windows guest whose system disk is basic and whose data disks are
			// LDM. Inspecting one disk at a time means the LDM disks are never
			// seen, so there is only ever one root and migration succeeds.
			name:               "windows system disk with ldm data disks",
			vmDisks:            disks("/dev/sda", "/dev/sdb", "/dev/sdc"),
			perDiskAnswers:     map[string]string{"/dev/sda": "/dev/sda2\n"},
			wantIndex:          0,
			wantOSPath:         "/dev/sda2",
			wantAllDisksCalled: false,
		},
		{
			// The boot disk is not the first one, so the loop has to keep going
			// past the disks that answer with nothing.
			name:               "boot volume on a later disk",
			vmDisks:            disks("/dev/sda", "/dev/sdb"),
			perDiskAnswers:     map[string]string{"/dev/sdb": "  /dev/sdb1  "},
			wantIndex:          1,
			wantOSPath:         "/dev/sdb1",
			wantAllDisksCalled: false,
		},
		{
			// An empty answer is not a boot volume. Without this check the first
			// disk would win with an empty osPath.
			name:               "empty answer is not treated as a hit",
			vmDisks:            disks("/dev/sda", "/dev/sdb"),
			perDiskAnswers:     map[string]string{"/dev/sda": "   ", "/dev/sdb": "/dev/sdb1"},
			wantIndex:          1,
			wantOSPath:         "/dev/sdb1",
			wantAllDisksCalled: false,
		},
		{
			// Multi-device btrfs: neither member is mountable alone, so the
			// per-disk pass finds nothing and the all-disks fallback answers.
			// The index stays -1 because the root spans both disks; the
			// OS-specific handlers re-derive it.
			name:               "multi device root falls back to all disks",
			vmDisks:            disks("/dev/sda", "/dev/sdb"),
			perDiskAnswers:     nil,
			allDisksAnswer:     "/dev/sda6\n",
			wantIndex:          -1,
			wantOSPath:         "/dev/sda6",
			wantAllDisksCalled: true,
		},
		{
			// Nothing is inspectable at all. This is not fatal here - the
			// handlers produce a more specific error - so err must stay nil.
			name:               "no boot volume anywhere is not fatal",
			vmDisks:            disks("/dev/sda"),
			perDiskAnswers:     nil,
			allDisksErr:        errNoRoot,
			wantIndex:          -1,
			wantOSPath:         "",
			wantAllDisksCalled: true,
		},
		{
			// No disks means neither probe should run.
			name:               "no disks attached",
			vmDisks:            nil,
			wantIndex:          -1,
			wantOSPath:         "",
			wantAllDisksCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var perDiskCalls []string
			allDisksCalled := false

			stubGuestCommands(t,
				func(path, _ string, _ bool) (string, error) {
					perDiskCalls = append(perDiskCalls, path)
					if ans, ok := tt.perDiskAnswers[path]; ok {
						return ans, nil
					}
					return "", errNoRoot
				},
				func(_ []vm.VMDisk, _ string, _ bool, _ ...string) (string, error) {
					allDisksCalled = true
					return tt.allDisksAnswer, tt.allDisksErr
				},
			)

			migobj := &Migrate{}
			idx, osPath, err := migobj.detectBootVolume(vm.VMInfo{VMDisks: tt.vmDisks}, "inspect-os")

			assert.NoError(t, err)
			assert.Equal(t, tt.wantIndex, idx)
			assert.Equal(t, tt.wantOSPath, osPath)
			assert.Equal(t, tt.wantAllDisksCalled, allDisksCalled)

			if tt.wantIndex >= 0 {
				// The loop must stop at the first disk that answers rather than
				// probing every disk; each extra probe is a full appliance boot.
				assert.Len(t, perDiskCalls, tt.wantIndex+1)
			}
		})
	}
}
