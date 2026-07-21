// Copyright © 2024 The vjailbreak authors

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestLogging(t *testing.T) (tmpDir string, migrationName string) {
	t.Helper()

	tmpDir = t.TempDir()
	prevBaseDir := logsBaseDir
	logsBaseDir = tmpDir

	prevEnv, hadEnv := os.LookupEnv("VMWARE_MACHINE_OBJECT_NAME")
	require.NoError(t, os.Setenv("VMWARE_MACHINE_OBJECT_NAME", "test-vm"))

	t.Cleanup(func() {
		logsBaseDir = prevBaseDir
		if hadEnv {
			os.Setenv("VMWARE_MACHINE_OBJECT_NAME", prevEnv)
		} else {
			os.Unsetenv("VMWARE_MACHINE_OBJECT_NAME")
		}
	})

	migrationName, err := GetMigrationObjectName()
	require.NoError(t, err)
	return tmpDir, migrationName
}

func listLogFiles(t *testing.T, tmpDir, migrationName string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(tmpDir, migrationName))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestAddDebugOutputToFileWithCommandCategory_SeparatesFilesByCategory(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	nbdCmd := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(nbdCmd, "nbdkit --verbose", LogCategoryNBD)

	virtCmd := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(virtCmd, "virt-v2v-in-place -v", LogCategoryVirtV2V)

	assert.NotNil(t, nbdCmd.Stdout)
	assert.NotNil(t, virtCmd.Stdout)
	assert.NotSame(t, nbdCmd.Stdout, virtCmd.Stdout)

	CloseLogFile(nbdCmd)
	CloseLogFile(virtCmd)

	files := listLogFiles(t, tmpDir, migrationName)
	require.Len(t, files, 2)

	var sawNBD, sawVirtV2V bool
	for _, f := range files {
		switch {
		case strings.HasPrefix(f, LogCategoryNBD+"."):
			sawNBD = true
		case strings.HasPrefix(f, LogCategoryVirtV2V+"."):
			sawVirtV2V = true
		}
	}
	assert.True(t, sawNBD, "expected an nbd-prefixed log file, got: %v", files)
	assert.True(t, sawVirtV2V, "expected a virtv2v-prefixed log file, got: %v", files)
}

func TestAddDebugOutputToFileWithCommandCategory_ContentIsIsolated(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	nbdCmd := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(nbdCmd, "nbdcopy --progress=3", LogCategoryNBD)
	require.NoError(t, RunCommandWithLogFileRedactedCategory(exec.Command("true"), "nbdcopy --progress=3", LogCategoryNBD))

	require.NoError(t, RunCommandWithLogFileRedactedCategory(exec.Command("true"), "virt-v2v-in-place -v", LogCategoryVirtV2V))

	CloseLogFile(nbdCmd)

	files := listLogFiles(t, tmpDir, migrationName)
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(tmpDir, migrationName, f))
		require.NoError(t, err)
		if strings.HasPrefix(f, LogCategoryNBD+".") {
			assert.Contains(t, string(content), "nbdcopy")
			assert.NotContains(t, string(content), "virt-v2v-in-place")
		}
		if strings.HasPrefix(f, LogCategoryVirtV2V+".") {
			assert.Contains(t, string(content), "virt-v2v-in-place")
			assert.NotContains(t, string(content), "nbdcopy")
		}
	}
}

func TestRunCommandWithLogFileCategory_DefaultsToGeneralCategory(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	cmd := exec.Command("true")
	require.NoError(t, RunCommandWithLogFile(cmd))

	files := listLogFiles(t, tmpDir, migrationName)
	require.Len(t, files, 1)
	assert.True(t, strings.HasPrefix(files[0], LogCategoryGeneral+"."))
}

func TestCloseLogFile_RefCountsAcrossCommandsInSameCategory(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	cmd1 := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(cmd1, "cmd1", LogCategoryNBD)
	cmd2 := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(cmd2, "cmd2", LogCategoryNBD)

	w1, ok := cmd1.Stdout.(*redactingWriter)
	require.True(t, ok)
	w2, ok := cmd2.Stdout.(*redactingWriter)
	require.True(t, ok)
	assert.NotSame(t, w1, w2)
	assert.Same(t, w1.w, w2.w)

	CloseLogFile(cmd1)
	key := migrationLogKey{migrationName: migrationName, category: LogCategoryNBD}
	logMutex.Lock()
	_, stillOpen := migrationLogs[key]
	logMutex.Unlock()
	assert.True(t, stillOpen)

	CloseLogFile(cmd2)
	logMutex.Lock()
	_, stillOpenAfter := migrationLogs[key]
	logMutex.Unlock()
	assert.False(t, stillOpenAfter)

	files := listLogFiles(t, tmpDir, migrationName)
	require.Len(t, files, 1)
}

func TestCleanupMigrationLogs_ClosesAllCategories(t *testing.T) {
	_, migrationName := withTestLogging(t)

	cmd1 := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(cmd1, "cmd1", LogCategoryNBD)
	cmd2 := exec.Command("true")
	AddDebugOutputToFileWithCommandCategory(cmd2, "cmd2", LogCategoryVirtV2V)

	CleanupMigrationLogs(migrationName)

	logMutex.Lock()
	defer logMutex.Unlock()
	for key := range migrationLogs {
		assert.NotEqual(t, migrationName, key.migrationName)
	}
	for _, entry := range commandLogs {
		assert.NotEqual(t, migrationName, entry.key.migrationName)
	}
}

func TestAddDebugOutputToFileWithCommandCategory_RedactsSecretsFromChildOutput(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	secret := "s3cr3t-value"
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo 'key=%s'", secret))

	require.NoError(t, RunCommandWithLogFileRedactedCategory(cmd, "cmd [REDACTED]", LogCategoryNBD, secret))

	files := listLogFiles(t, tmpDir, migrationName)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(tmpDir, migrationName, files[0]))
	require.NoError(t, err)
	assert.NotContains(t, string(content), secret)
	assert.Contains(t, string(content), redactedPlaceholder)
}

func TestRedactingWriter_FlushesRedactedPartialLineOnClose(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	secret := "s3cr3t-value"
	cmd := exec.Command("sh", "-c", fmt.Sprintf("printf 'key=%s'", secret))

	AddDebugOutputToFileWithCommandCategory(cmd, "cmd [REDACTED]", LogCategoryNBD, secret)
	require.NoError(t, cmd.Run())
	CloseLogFile(cmd)

	files := listLogFiles(t, tmpDir, migrationName)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(tmpDir, migrationName, files[0]))
	require.NoError(t, err)
	assert.NotContains(t, string(content), secret)
	assert.Contains(t, string(content), redactedPlaceholder)
}

func TestRedactingWriter_NoSecretsPassesThroughUnmodified(t *testing.T) {
	tmpDir, migrationName := withTestLogging(t)

	cmd := exec.Command("sh", "-c", "echo 'plain output'")
	require.NoError(t, RunCommandWithLogFileRedactedCategory(cmd, "echo ...", LogCategoryGeneral))

	files := listLogFiles(t, tmpDir, migrationName)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(tmpDir, migrationName, files[0]))
	require.NoError(t, err)
	assert.Contains(t, string(content), "plain output")
	assert.NotContains(t, string(content), redactedPlaceholder)
}
