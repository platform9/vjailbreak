package debugbundle

import (
	"sort"
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

func TestListDebugLogPathsFiltersByMigration(t *testing.T) {
	paths, warnings := ListDebugLogPaths(testLogsFS(), "migration-testvm")

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	sort.Strings(paths)
	expected := []string{"migration-testvm.log", "migration-testvm/migration.001.log"}
	if len(paths) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, paths)
	}
	for i, path := range expected {
		if paths[i] != path {
			t.Errorf("expected %s at %d, got %s", path, i, paths[i])
		}
	}
}

func TestListDebugLogPathsNoMatches(t *testing.T) {
	paths, warnings := ListDebugLogPaths(testLogsFS(), "migration-does-not-exist")

	if len(paths) != 0 || len(warnings) != 0 {
		t.Errorf("expected empty result, got paths=%v warnings=%v", paths, warnings)
	}
}
