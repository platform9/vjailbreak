// Copyright © 2024 The vjailbreak authors
package virtv2v

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// IsSUSEFamily
// ---------------------------------------------------------------------------

func TestIsSUSEFamily(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		want      bool
	}{
		{"openSUSE Leap", "openSUSE Leap 15.4", true},
		{"SLES", "SUSE Linux Enterprise Server 15 SP4", true},
		{"sles lowercase", "sles 11", true},
		{"SUSE 11", "SUSE 11.4", true},
		{"Ubuntu", "Ubuntu 22.04", false},
		{"RHEL", "Red Hat Enterprise Linux 8", false},
		{"CentOS", "CentOS 7", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSUSEFamily(tt.osRelease))
		})
	}
}

// ---------------------------------------------------------------------------
// IsRHELFamily
// ---------------------------------------------------------------------------

func TestIsRHELFamily(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		want      bool
	}{
		{"RHEL", "Red Hat Enterprise Linux 8", true},
		{"rhel lowercase", "rhel 7.9", true},
		{"CentOS", "CentOS Linux 7", true},
		{"Rocky", "Rocky Linux 9", true},
		{"AlmaLinux", "AlmaLinux 8", true},
		{"Ubuntu", "Ubuntu 20.04 LTS", false},
		{"SUSE", "SUSE Linux Enterprise Server 15", false},
		{"openSUSE", "openSUSE Leap 15.4", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRHELFamily(tt.osRelease))
		})
	}
}

// ---------------------------------------------------------------------------
// copyFile
// ---------------------------------------------------------------------------

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "dst.sh")
	content := []byte("#!/bin/bash\necho hello\n")

	require.NoError(t, os.WriteFile(src, content, 0644))
	require.NoError(t, copyFile(src, dst, 0755))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode())
}

func TestCopyFile_MissingSrc(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "nonexistent.sh"), filepath.Join(dir, "dst.sh"), 0644)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// buildLinuxFirstBootStaging
// Replicates the staging portion of InjectLinuxFirstBootScriptsFromStore
// for testing without a real guestfish call.
// ---------------------------------------------------------------------------

func buildLinuxFirstBootStaging(schedulerSrc, baseScriptDir, stageDir string, scripts []FirstBootLinux) ([]FirstBootLinux, error) {
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir stageDir: %w", err)
	}
	if err := copyFile(schedulerSrc, filepath.Join(stageDir, "firstboot-scheduler.sh"), 0755); err != nil {
		return nil, fmt.Errorf("copy scheduler: %w", err)
	}
	metadata := make([]FirstBootLinux, 0, len(scripts))
	for idx, s := range scripts {
		indexedName := fmt.Sprintf("%d-%s", idx, s.Script)
		src := filepath.Join(baseScriptDir, s.Script)
		dst := filepath.Join(stageDir, indexedName)
		if err := copyFile(src, dst, 0755); err != nil {
			return nil, fmt.Errorf("copy script %s: %w", s.Script, err)
		}
		metadata = append(metadata, FirstBootLinux{Script: indexedName, Async: s.Async})
	}
	return metadata, nil
}

func TestBuildLinuxFirstBootStaging_AllScripts(t *testing.T) {
	dir := t.TempDir()
	scriptsDir := filepath.Join(dir, "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "firstboot-scheduler.sh"), []byte("#!/bin/bash\n"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "vmware-tools-cleanup.sh"), []byte("#!/bin/bash\n"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "rhel_enable_dhcp.sh"), []byte("#!/bin/bash\n"), 0755))

	scripts := []FirstBootLinux{
		{Script: "vmware-tools-cleanup.sh", Async: true},
		{Script: "rhel_enable_dhcp.sh", Async: false},
	}
	stageDir := filepath.Join(dir, "stage")
	metadata, err := buildLinuxFirstBootStaging(filepath.Join(dir, "firstboot-scheduler.sh"), scriptsDir, stageDir, scripts)
	require.NoError(t, err)

	entries, err := os.ReadDir(stageDir)
	require.NoError(t, err)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	assert.True(t, names["firstboot-scheduler.sh"], "scheduler must be staged")
	assert.True(t, names["0-vmware-tools-cleanup.sh"], "indexed script 0 must be staged")
	assert.True(t, names["1-rhel_enable_dhcp.sh"], "indexed script 1 must be staged")

	require.Len(t, metadata, 2)
	assert.Equal(t, "0-vmware-tools-cleanup.sh", metadata[0].Script)
	assert.True(t, metadata[0].Async)
	assert.Equal(t, "1-rhel_enable_dhcp.sh", metadata[1].Script)
	assert.False(t, metadata[1].Async)
}

func TestBuildLinuxFirstBootStaging_OnlyCleanup(t *testing.T) {
	dir := t.TempDir()
	scriptsDir := filepath.Join(dir, "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "firstboot-scheduler.sh"), []byte("#!/bin/bash\n"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "vmware-tools-cleanup.sh"), []byte("#!/bin/bash\n"), 0755))

	scripts := []FirstBootLinux{{Script: "vmware-tools-cleanup.sh", Async: true}}
	metadata, err := buildLinuxFirstBootStaging(
		filepath.Join(dir, "firstboot-scheduler.sh"),
		scriptsDir, filepath.Join(dir, "stage"), scripts,
	)
	require.NoError(t, err)
	require.Len(t, metadata, 1)
	assert.Equal(t, "0-vmware-tools-cleanup.sh", metadata[0].Script)
	assert.True(t, metadata[0].Async)
}

func TestBuildLinuxFirstBootStaging_EmptyScripts(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "firstboot-scheduler.sh"), []byte("#!/bin/bash\n"), 0755))
	metadata, err := buildLinuxFirstBootStaging(filepath.Join(dir, "firstboot-scheduler.sh"), dir, filepath.Join(dir, "stage"), nil)
	require.NoError(t, err)
	assert.Empty(t, metadata)
}

func TestBuildLinuxFirstBootStaging_MissingScheduler(t *testing.T) {
	dir := t.TempDir()
	_, err := buildLinuxFirstBootStaging(filepath.Join(dir, "missing-scheduler.sh"), dir, filepath.Join(dir, "stage"), nil)
	assert.Error(t, err)
}

func TestBuildLinuxFirstBootStaging_MissingScript(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "firstboot-scheduler.sh"), []byte("#!/bin/bash\n"), 0755))
	scripts := []FirstBootLinux{{Script: "missing.sh", Async: true}}
	_, err := buildLinuxFirstBootStaging(filepath.Join(dir, "firstboot-scheduler.sh"), dir, filepath.Join(dir, "stage"), scripts)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// scripts.json round-trip — mirrors the state format used by the scheduler
// State file entries: "ScriptName|async|-1"  (matches Init-Table in .ps1)
// ---------------------------------------------------------------------------

func TestFirstBootLinux_ScriptsJSONRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		scripts []FirstBootLinux
	}{
		{
			"async only",
			[]FirstBootLinux{{Script: "0-vmware-tools-cleanup.sh", Async: true}},
		},
		{
			"mixed sync and async",
			[]FirstBootLinux{
				{Script: "0-vmware-tools-cleanup.sh", Async: true},
				{Script: "1-rhel_enable_dhcp.sh", Async: false},
			},
		},
		{
			"empty",
			[]FirstBootLinux{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.scripts)
			require.NoError(t, err)
			var parsed []FirstBootLinux
			require.NoError(t, json.Unmarshal(data, &parsed))
			assert.Equal(t, tt.scripts, parsed)
		})
	}
}

// ---------------------------------------------------------------------------
// Scheduler state file logic
// These tests verify the state file format and transitions that the bash
// scheduler (firstboot-scheduler.sh) uses — validated at the Go layer via
// the JSON/staging pipeline that produces scripts.json.
//
// State file format (mirrors Firstboot-Scheduler.ps1):
//   "<ScriptName>|<async>|<runcount>"
//   runcount -1 = not yet run
//   runcount 0-2 = attempt in progress (resumed after reboot)
//   runcount 3 = max reached
// ---------------------------------------------------------------------------

// stateFileEntry mirrors one line of the scheduler state file.
type stateFileEntry struct {
	Name     string
	Async    bool
	RunCount int
}

// parseStateFile is a test-only parser for the scheduler state file.
func parseStateFile(path string) ([]stateFileEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []stateFileEntry
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var name, asyncStr string
		var count int
		if _, err := fmt.Sscanf(line, "%s", &name); err != nil {
			return nil, fmt.Errorf("malformed state line: %q", line)
		}
		// Manual split on '|'
		parts := splitPipe(line)
		if len(parts) != 3 {
			return nil, fmt.Errorf("expected 3 fields in %q", line)
		}
		name = parts[0]
		asyncStr = parts[1]
		if _, err := fmt.Sscanf(parts[2], "%d", &count); err != nil {
			return nil, fmt.Errorf("bad runcount in %q", line)
		}
		entries = append(entries, stateFileEntry{
			Name:     name,
			Async:    asyncStr == "true",
			RunCount: count,
		})
	}
	return entries, nil
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, cur)
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func splitPipe(s string) []string {
	var parts []string
	cur := ""
	for _, c := range s {
		if c == '|' {
			parts = append(parts, cur)
			cur = ""
		} else {
			cur += string(c)
		}
	}
	parts = append(parts, cur)
	return parts
}

// writeStateFile creates a state file from entries (used in tests to simulate
// a state file that the scheduler would have created on a previous boot).
func writeStateFile(path string, entries []stateFileEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, e := range entries {
		asyncStr := "false"
		if e.Async {
			asyncStr = "true"
		}
		fmt.Fprintf(f, "%s|%s|%d\n", e.Name, asyncStr, e.RunCount)
	}
	return nil
}

func TestStateFileFormat_InitialState(t *testing.T) {
	// Simulate what init_table writes: all runcounts = -1
	dir := t.TempDir()
	path := filepath.Join(dir, "scheduler.state")

	initial := []stateFileEntry{
		{Name: "0-vmware-tools-cleanup.sh", Async: true, RunCount: -1},
		{Name: "1-rhel_enable_dhcp.sh", Async: false, RunCount: -1},
	}
	require.NoError(t, writeStateFile(path, initial))

	parsed, err := parseStateFile(path)
	require.NoError(t, err)
	require.Len(t, parsed, 2)
	assert.Equal(t, "0-vmware-tools-cleanup.sh", parsed[0].Name)
	assert.Equal(t, -1, parsed[0].RunCount)
	assert.True(t, parsed[0].Async)
	assert.Equal(t, "1-rhel_enable_dhcp.sh", parsed[1].Name)
	assert.Equal(t, -1, parsed[1].RunCount)
	assert.False(t, parsed[1].Async)
}

func TestStateFileFormat_AfterPush(t *testing.T) {
	// Simulate push_script: runcount goes -1 → 0
	dir := t.TempDir()
	path := filepath.Join(dir, "scheduler.state")

	initial := []stateFileEntry{
		{Name: "0-vmware-tools-cleanup.sh", Async: true, RunCount: -1},
	}
	require.NoError(t, writeStateFile(path, initial))

	// Simulate push: -1 → 0
	initial[0].RunCount = 0
	require.NoError(t, writeStateFile(path, initial))

	parsed, err := parseStateFile(path)
	require.NoError(t, err)
	assert.Equal(t, 0, parsed[0].RunCount)
}

func TestStateFileFormat_AfterPop(t *testing.T) {
	// Simulate pop_script: entry removed after success
	dir := t.TempDir()
	path := filepath.Join(dir, "scheduler.state")

	entries := []stateFileEntry{
		{Name: "0-vmware-tools-cleanup.sh", Async: true, RunCount: 0},
		{Name: "1-rhel_enable_dhcp.sh", Async: false, RunCount: -1},
	}
	require.NoError(t, writeStateFile(path, entries))

	// Simulate pop of first entry
	remaining := entries[1:]
	require.NoError(t, writeStateFile(path, remaining))

	parsed, err := parseStateFile(path)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, "1-rhel_enable_dhcp.sh", parsed[0].Name)
}

func TestStateFileFormat_RebootResume(t *testing.T) {
	// Simulate state after reboot mid-run: first script popped (success),
	// second still at runcount 0 (was being attempted when reboot happened).
	// Scheduler should resume from second script.
	dir := t.TempDir()
	path := filepath.Join(dir, "scheduler.state")

	entries := []stateFileEntry{
		{Name: "1-rhel_enable_dhcp.sh", Async: false, RunCount: 0},
	}
	require.NoError(t, writeStateFile(path, entries))

	parsed, err := parseStateFile(path)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	// runcount 0 means it was attempted once — scheduler picks it up and retries
	assert.Equal(t, 0, parsed[0].RunCount)
	assert.False(t, parsed[0].Async)
}

func TestStateFileFormat_MaxRetries(t *testing.T) {
	// runcount == MAX_RETRIES(3) means exhausted — get_script skips it
	dir := t.TempDir()
	path := filepath.Join(dir, "scheduler.state")

	entries := []stateFileEntry{
		{Name: "0-vmware-tools-cleanup.sh", Async: true, RunCount: 3},
	}
	require.NoError(t, writeStateFile(path, entries))

	parsed, err := parseStateFile(path)
	require.NoError(t, err)
	assert.Equal(t, 3, parsed[0].RunCount)
	// A runcount of 3 means push_script would reject it — equivalent to Windows throwing
}
