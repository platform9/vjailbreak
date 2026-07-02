package debugbundle

import (
	"strings"
	"testing"
	"testing/fstest"
)

func testLogsFS() fstest.MapFS {
	return fstest.MapFS{
		"migration-testvm.log":               {Data: []byte("root log line")},
		"migration-testvm/migration.001.log": {Data: []byte("subdir log line")},
		"migration-testvm/notes.txt":         {Data: []byte("not a log file")},
		"migration-othervm.log":              {Data: []byte("other vm log")},
		"unrelated.txt":                      {Data: []byte("ignore me")},
	}
}

func TestCollectDebugFileLogsFiltersByMigration(t *testing.T) {
	output := CollectDebugFileLogs(testLogsFS(), "migration-testvm")

	if !strings.Contains(output, "FILE: migration-testvm.log") || !strings.Contains(output, "root log line") {
		t.Errorf("expected root-level log file, got:\n%s", output)
	}
	if !strings.Contains(output, "FILE: migration-testvm/migration.001.log") || !strings.Contains(output, "subdir log line") {
		t.Errorf("expected subdirectory log file, got:\n%s", output)
	}
	if strings.Contains(output, "other vm log") {
		t.Errorf("must not include other migrations' logs, got:\n%s", output)
	}
	if strings.Contains(output, "not a log file") || strings.Contains(output, "ignore me") {
		t.Errorf("must only include .log files, got:\n%s", output)
	}
}

func TestCollectDebugFileLogsNoMatches(t *testing.T) {
	if output := CollectDebugFileLogs(testLogsFS(), "migration-does-not-exist"); strings.TrimSpace(output) != "" {
		t.Errorf("expected empty output for unmatched migration, got:\n%s", output)
	}
}

func TestCollectDebugFileLogsTotalSizeCap(t *testing.T) {
	originalTotal := maxDebugFilesTotalBytes
	maxDebugFilesTotalBytes = 64
	defer func() { maxDebugFilesTotalBytes = originalTotal }()

	fsys := fstest.MapFS{
		"migration-big/a.log": {Data: []byte(strings.Repeat("x", 200))},
		"migration-big/b.log": {Data: []byte("second file content")},
	}

	output := CollectDebugFileLogs(fsys, "migration-big")

	if !strings.Contains(output, "Debug log size limit reached") {
		t.Errorf("expected truncation note, got:\n%s", output)
	}
	if strings.Contains(output, "second file content") {
		t.Errorf("files after the cap must be omitted, got:\n%s", output)
	}
}
