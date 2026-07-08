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

func TestCollectDebugFilesFiltersByMigration(t *testing.T) {
	files, warnings := CollectDebugFiles(testLogsFS(), "migration-testvm")

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	byPath := filesByPath(files)
	if byPath["debug-logs/migration-testvm.log"] != "root log line" {
		t.Errorf("expected root-level log file, got %v", byPath)
	}
	if byPath["debug-logs/migration-testvm/migration.001.log"] != "subdir log line" {
		t.Errorf("expected subdirectory log file, got %v", byPath)
	}
	if len(files) != 2 {
		t.Errorf("expected only .log files for the migration, got %v", byPath)
	}
}

func TestCollectDebugFilesNoMatches(t *testing.T) {
	files, warnings := CollectDebugFiles(testLogsFS(), "migration-does-not-exist")

	if len(files) != 0 || len(warnings) != 0 {
		t.Errorf("expected empty result for unmatched migration, got files=%v warnings=%v", files, warnings)
	}
}

func TestCollectDebugFilesTotalSizeCap(t *testing.T) {
	originalTotal := maxDebugFilesTotalBytes
	maxDebugFilesTotalBytes = 64
	defer func() { maxDebugFilesTotalBytes = originalTotal }()

	fsys := fstest.MapFS{
		"migration-big/a.log": {Data: []byte(strings.Repeat("x", 200))},
		"migration-big/b.log": {Data: []byte("second file content")},
	}

	files, warnings := CollectDebugFiles(fsys, "migration-big")

	capWarning := false
	for _, warning := range warnings {
		if strings.Contains(warning, "Debug log size limit reached") {
			capWarning = true
		}
	}
	if !capWarning {
		t.Errorf("expected size-cap warning, got %v", warnings)
	}
	byPath := filesByPath(files)
	if _, ok := byPath["debug-logs/migration-big/b.log"]; ok {
		t.Errorf("files after the cap must be omitted, got %v", byPath)
	}
}

func TestCollectDebugFilesPerFileTailCap(t *testing.T) {
	originalPerFile := maxDebugFileBytes
	maxDebugFileBytes = 8
	defer func() { maxDebugFileBytes = originalPerFile }()

	fsys := fstest.MapFS{
		"migration-tail.log": {Data: []byte("0123456789ABCDEF")},
	}

	files, warnings := CollectDebugFiles(fsys, "migration-tail")

	byPath := filesByPath(files)
	if byPath["debug-logs/migration-tail.log"] != "89ABCDEF" {
		t.Errorf("expected tail of the file, got %q", byPath["debug-logs/migration-tail.log"])
	}
	truncWarning := false
	for _, warning := range warnings {
		if strings.Contains(warning, "truncated to last") {
			truncWarning = true
		}
	}
	if !truncWarning {
		t.Errorf("expected truncation warning, got %v", warnings)
	}
}
