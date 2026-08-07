// Copyright © 2024 The vjailbreak authors

package esxissh

import "strings"

// shellQuote wraps s in single quotes so it is passed to the remote shell as a
// single argument, regardless of spaces or other metacharacters it contains.
//
// Commands are sent to ESXi as a single string over SSH and parsed by the remote
// shell, which splits on whitespace. VM names routinely contain spaces (and
// therefore so do datastore folder and VMDK paths), so unquoted interpolation
// silently truncates the path at the first space.
//
// Embedded single quotes are escaped using the POSIX idiom of closing the quoted
// section, emitting an escaped quote, and reopening:
//
//	suhas's VM  ->  'suhas'\''s VM'
//
// which the shell reassembles into the original string. This is safe for every
// byte except NUL, which cannot appear in a path.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellQuotePrefixed quotes path while leaving prefix outside the quotes, for
// arguments such as vmkfstools' "rdm:<device>" that combine a literal marker
// with a path. The shell concatenates adjacent quoted and unquoted sections, so
// rdm:'/vmfs/devices/disks/naa.xxx' resolves to rdm:/vmfs/devices/disks/naa.xxx.
func shellQuotePrefixed(prefix, path string) string {
	return prefix + shellQuote(path)
}

// shellQuoteGlob quotes dir but appends pattern unquoted so the remote shell
// still performs glob expansion, e.g. '/vmfs/volumes/ds/My VM'/*.vmdk
func shellQuoteGlob(dir, pattern string) string {
	return shellQuote(dir) + pattern
}

// lsFilenameColumn is the index of the filename column in `ls -l` output on
// ESXi (BusyBox): perms, links, owner, group, size, month, day, time, name.
const lsFilenameColumn = 8

// parseLsFilename extracts the filename from a single `ls -l` line, preserving
// spaces within the name. Returns "" if the line is not a well-formed entry.
func parseLsFilename(line string) string {
	fields := strings.Fields(line)
	if len(fields) <= lsFilenameColumn {
		return ""
	}
	return strings.Join(fields[lsFilenameColumn:], " ")
}
