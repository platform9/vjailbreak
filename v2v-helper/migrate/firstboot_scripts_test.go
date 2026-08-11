// Copyright © 2024 The vjailbreak authors
package migrate

import (
	"errors"
	"testing"

	"github.com/platform9/vjailbreak/v2v-helper/virtv2v"
	"github.com/stretchr/testify/assert"
)

// stubPushWindowsFirstBoot replaces the user-script lookup, which otherwise reads
// the on-disk store, and restores it when the test ends.
func stubPushWindowsFirstBoot(t *testing.T, fn func(string) ([]string, error)) {
	t.Helper()

	orig := pushWindowsFirstBoot
	pushWindowsFirstBoot = fn
	t.Cleanup(func() { pushWindowsFirstBoot = orig })
}

func scriptNames(scripts []virtv2v.FirstBootWindows) []string {
	out := make([]string, 0, len(scripts))
	for _, s := range scripts {
		out = append(out, s.Script)
	}
	return out
}

func TestWindowsFirstbootScripts(t *testing.T) {
	t.Run("ldm guest gets nothing", func(t *testing.T) {
		called := false
		stubPushWindowsFirstBoot(t, func(string) ([]string, error) {
			called = true
			return []string{"user.ps1"}, nil
		})

		names, winScripts, err := windowsFirstbootScripts(true, true, true, "windows")

		assert.NoError(t, err)
		assert.Empty(t, names)
		assert.Empty(t, winScripts)
		// The store lookup must not run either: it can fail the migration, and
		// nothing in an LDM guest would ever execute what it returns.
		assert.False(t, called, "PushWindowsFirstBoot should not be called for an LDM guest")
	})

	// An LDM guest must not be failed by a store the scripts would never be read
	// from, so the lookup has to be skipped rather than merely ignored.
	t.Run("ldm guest is unaffected by a failing store", func(t *testing.T) {
		stubPushWindowsFirstBoot(t, func(string) ([]string, error) {
			return nil, errors.New("source file not found")
		})

		_, _, err := windowsFirstbootScripts(true, false, false, "windows")
		assert.NoError(t, err)
	})

	t.Run("non ldm guest gets the full set", func(t *testing.T) {
		stubPushWindowsFirstBoot(t, func(string) ([]string, error) {
			return []string{"0-user.ps1", "1-user.ps1"}, nil
		})

		names, winScripts, err := windowsFirstbootScripts(false, true, true, "windows")

		assert.NoError(t, err)
		assert.Equal(t, []string{"Firstboot-Init-Windows"}, names)
		assert.Equal(t, []string{
			"Firstboot-Scheduler.ps1",
			"install-virtio-win12.ps1",
			"Orchestrate-NICRecovery.ps1",
			"disk-online-fix.ps1",
			"vmware-tools-deletion.ps1",
			"0-user.ps1",
			"1-user.ps1",
		}, scriptNames(winScripts))

		// The scheduler must stay first: InjectFirstBootScriptsFromStore treats
		// index 0 as the runner and leaves it out of scripts.json.
		assert.Equal(t, "Firstboot-Scheduler.ps1", winScripts[0].Script)
		assert.False(t, winScripts[0].Async)
	})

	t.Run("optional scripts are gated by their flags", func(t *testing.T) {
		stubPushWindowsFirstBoot(t, func(string) ([]string, error) { return nil, nil })

		_, winScripts, err := windowsFirstbootScripts(false, false, false, "windows")

		assert.NoError(t, err)
		assert.Equal(t, []string{
			"Firstboot-Scheduler.ps1",
			"install-virtio-win12.ps1",
			"disk-online-fix.ps1",
		}, scriptNames(winScripts))
	})

	t.Run("store errors propagate for a non ldm guest", func(t *testing.T) {
		stubPushWindowsFirstBoot(t, func(string) ([]string, error) {
			return nil, errors.New("source file not found")
		})

		_, _, err := windowsFirstbootScripts(false, false, false, "windows")
		assert.Error(t, err)
	})
}
