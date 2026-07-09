package debugbundle

import (
	"fmt"
	"io/fs"
	"strings"
)

// ListDebugLogPaths finds the debug log files under /var/log/pf9 (passed in
// as fsys) that belong to the given migration:
//   - root-level *.log files whose name contains migrationName
//   - migration-* directories whose name contains migrationName, including
//     every *.log file inside them (one level deep)
//
// Only paths are returned; the caller streams the file contents. Listing
// failures are reported as warnings.
func ListDebugLogPaths(fsys fs.FS, migrationName string) ([]string, []string) {
	var paths []string
	var warnings []string

	rootEntries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, []string{fmt.Sprintf("Failed to list debug logs directory: %v", err)}
	}

	for _, entry := range rootEntries {
		name := entry.Name()
		if migrationName != "" && !strings.Contains(name, migrationName) {
			continue
		}
		switch {
		case !entry.IsDir() && strings.HasSuffix(name, ".log"):
			paths = append(paths, name)
		case entry.IsDir() && strings.HasPrefix(name, "migration-"):
			subEntries, err := fs.ReadDir(fsys, name)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to list debug logs in %s: %v", name, err))
				continue
			}
			for _, subEntry := range subEntries {
				subName := subEntry.Name()
				if !subEntry.IsDir() && strings.HasSuffix(subName, ".log") {
					paths = append(paths, name+"/"+subName)
				}
			}
		}
	}

	return paths, warnings
}
