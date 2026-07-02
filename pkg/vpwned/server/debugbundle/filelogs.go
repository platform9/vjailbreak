package debugbundle

import (
	"fmt"
	"io/fs"
	"strings"
)

// maxDebugFileBytes caps each debug log file included from /var/log/pf9
// (32 MiB per file).
const maxDebugFileBytes = 32 << 20

// CollectDebugFileLogs reads the debug log files written under /var/log/pf9
// (passed in as fsys) that belong to the given migration, mirroring the
// UI's fetchPodDebugLogs traversal:
//   - root-level *.log files whose name contains migrationName
//   - migration-* directories whose name contains migrationName, including
//     every *.log file inside them (one level deep)
//
// Each file is emitted with a FILE: header. An empty string means no
// matching debug logs were found.
func CollectDebugFileLogs(fsys fs.FS, migrationName string) string {
	var out strings.Builder

	rootEntries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Sprintf("[Failed to list debug logs directory: %v]\n", err)
	}

	appendFile := func(path, displayName string) {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			out.WriteString(fmt.Sprintf("\n%sFILE: %s\n%s[failed to read: %v]\n", sectionSeparator, displayName, sectionSeparator, err))
			return
		}
		if len(data) > maxDebugFileBytes {
			data = data[len(data)-maxDebugFileBytes:]
		}
		out.WriteString("\n")
		out.WriteString(sectionSeparator)
		out.WriteString("FILE: " + displayName + "\n")
		out.WriteString(sectionSeparator)
		out.Write(data)
		out.WriteString("\n")
	}

	for _, entry := range rootEntries {
		name := entry.Name()
		if migrationName != "" && !strings.Contains(name, migrationName) {
			continue
		}
		switch {
		case !entry.IsDir() && strings.HasSuffix(name, ".log"):
			appendFile(name, name)
		case entry.IsDir() && strings.HasPrefix(name, "migration-"):
			subEntries, err := fs.ReadDir(fsys, name)
			if err != nil {
				out.WriteString(fmt.Sprintf("\n%sFILE: %s/\n%s[failed to list directory: %v]\n", sectionSeparator, name, sectionSeparator, err))
				continue
			}
			for _, subEntry := range subEntries {
				subName := subEntry.Name()
				if !subEntry.IsDir() && strings.HasSuffix(subName, ".log") {
					appendFile(name+"/"+subName, name+"/"+subName)
				}
			}
		}
	}

	return out.String()
}
