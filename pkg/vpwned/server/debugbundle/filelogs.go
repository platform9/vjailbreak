package debugbundle

import (
	"fmt"
	"io/fs"
	"strings"
)

// maxDebugFileBytes caps each debug log file included from /var/log/pf9
// (32 MiB per file, keeping the tail). Variable so tests can shrink it.
var maxDebugFileBytes = 32 << 20

// maxDebugFilesTotalBytes caps the combined size of all debug log files in
// one bundle (128 MiB). Together with the pod log cap this keeps the bundle
// under the gateway's gRPC receive limit. Variable so tests can shrink it.
var maxDebugFilesTotalBytes = 128 << 20

// CollectDebugFiles reads the debug log files written under /var/log/pf9
// (passed in as fsys) that belong to the given migration, mirroring the
// traversal the UI previously performed:
//   - root-level *.log files whose name contains migrationName
//   - migration-* directories whose name contains migrationName, including
//     every *.log file inside them (one level deep)
//
// Each file becomes a debug-logs/ archive entry. Read failures and the
// total-size cap are reported as warnings.
func CollectDebugFiles(fsys fs.FS, migrationName string) ([]ArchiveFile, []string) {
	var files []ArchiveFile
	var warnings []string

	rootEntries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, []string{fmt.Sprintf("Failed to list debug logs directory: %v", err)}
	}

	totalBytes := 0
	truncated := false
	appendFile := func(path string) {
		if truncated {
			return
		}
		if totalBytes >= maxDebugFilesTotalBytes {
			truncated = true
			warnings = append(warnings, fmt.Sprintf("Debug log size limit reached (%d MiB) — remaining files omitted", maxDebugFilesTotalBytes>>20))
			return
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to read debug log %s: %v", path, err))
			return
		}
		if len(data) > maxDebugFileBytes {
			data = data[len(data)-maxDebugFileBytes:]
			warnings = append(warnings, fmt.Sprintf("Debug log %s truncated to last %d MiB", path, maxDebugFileBytes>>20))
		}
		totalBytes += len(data)
		files = append(files, ArchiveFile{Path: "debug-logs/" + path, Data: data})
	}

	for _, entry := range rootEntries {
		name := entry.Name()
		if migrationName != "" && !strings.Contains(name, migrationName) {
			continue
		}
		switch {
		case !entry.IsDir() && strings.HasSuffix(name, ".log"):
			appendFile(name)
		case entry.IsDir() && strings.HasPrefix(name, "migration-"):
			subEntries, err := fs.ReadDir(fsys, name)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to list debug logs in %s: %v", name, err))
				continue
			}
			for _, subEntry := range subEntries {
				subName := subEntry.Name()
				if !subEntry.IsDir() && strings.HasSuffix(subName, ".log") {
					appendFile(name + "/" + subName)
				}
			}
		}
	}

	return files, warnings
}
