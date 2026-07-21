// Copyright © 2024 The vjailbreak authors

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// virtV2VErrorPatterns are known virt-v2v / libguestfs / e2fsck failure
// signatures, ordered from most specific/actionable to least. When a
// virt-v2v-in-place invocation fails, its debug log is scanned (newest line
// first) against these patterns to find a concise, human-readable root cause
// to surface to the user, instead of the bare "exit status 1" that the Go
// process error alone provides.
//
// virt-v2v (an OCaml program) reports every fatal error through a single
// shared helper (Tools_utils.error in common/mltools/tools_utils.ml) which
// always prints "<prog>: error: <message>" to stderr - e.g.
// "virt-v2v-in-place: error: ...". Survivable issues go through a parallel
// Tools_utils.warning helper, printed as "<prog>: warning: <message>" to
// stdout, and never cause a non-zero exit - so warnings are intentionally
// NOT matched here. The patterns below (aside from the e2fsck/xfs heuristic,
// which is raw fsck-tool output, not virt-v2v's own text) are copied
// verbatim from the virt-v2v OCaml source so they match the exact strings
// virt-v2v prints, ahead of the generic catch-alls at the end of the list.
var virtV2VErrorPatterns = []*regexp.Regexp{
	// Host-side: less than ~1GB free in the appliance/nbdkit temp directory
	// (v2v/v2v.ml, in-place/in_place.ml, inspector/inspector.ml, open/open.ml).
	// Actionable: free up space, or set LIBGUESTFS_CACHEDIR/$TMPDIR elsewhere.
	regexp.MustCompile(`(?i)error:\s*insufficient free space in the conversion server temporary directory.*`),
	// Guest-side: the guest's own root/boot filesystem lacks the ~10-100MB
	// headroom virt-v2v needs to rebuild the initrd / inject drivers
	// (convert/convert.ml:249). Actionable: grow the guest disk before migrating.
	regexp.MustCompile(`(?i)error:\s*not enough free space for conversion on filesystem.*`),
	// Same check, but out of free inodes rather than free bytes (convert/convert.ml:255).
	regexp.MustCompile(`(?i)error:\s*not enough available inodes for conversion on.*filesystem.*`),
	// Inspection found no recognizable OS root at all (convert/choose_root.ml:35,
	// convert/mount_filesystems.ml:196). Actionable: verify the disk isn't blank
	// or running an unsupported guest OS.
	regexp.MustCompile(`(?i)error:\s*inspection could not detect the source guest.*`),
	// Guest has multiple bootable OS roots and --root wasn't set to disambiguate
	// (convert/choose_root.ml:79). Actionable: needs an explicit root selection.
	regexp.MustCompile(`(?i)error:\s*multi-boot operating systems are not supported.*`),
	// User-specified root device didn't match any detected root (convert/choose_root.ml:92).
	regexp.MustCompile(`(?i)error:\s*root device .* not found\.\s*roots found were:.*`),
	// Detected guest OS/distro combination has no virt-v2v conversion module
	// (convert/convert.ml:139). Actionable: check libguestfs's supported guest list.
	regexp.MustCompile(`(?i)error:\s*virt-v2v is unable to convert this guest type.*`),
	// xfs_repair found unrepaired errors on the guest filesystem post-conversion
	// (convert/convert.ml:342). Actionable: guest filesystem needs repair.
	regexp.MustCompile(`(?i)error:\s*detected errors on the xfs filesystem.*`),
	// Guest only has Xen paravirt kernels installed, none of which can boot
	// under KVM/OpenStack (convert/convert_linux.ml:686). Actionable: install a
	// non-Xen kernel in the guest before migrating.
	regexp.MustCompile(`(?i)error:\s*only xen kernels are installed in this guest.*`),
	// Guest is missing the tool virt-v2v needs to rebuild the initrd with
	// injected virtio drivers (convert/convert_linux.ml:823,900). Actionable:
	// install dracut/mkinitrd/update-initramfs in the guest.
	regexp.MustCompile(`(?i)error:\s*unable to rebuild initrd.*because.*(update-initramfs|mkinitrd|dracut).*not found.*`),
	// NTFS root left in an "unsafe state" by Windows Fast Startup/Hibernation,
	// so it can't be mounted read-write (convert/mount_filesystems.ml:80-84).
	// Actionable: disable Fast Startup/Hibernation in the guest, shut down
	// cleanly, and retry.
	regexp.MustCompile(`(?i)error:\s*unable to mount the disk image for writing.*windows hibernation.*`),
	// Root filesystem wasn't cleanly unmounted - VM was left running, or
	// hibernated (convert/mount_filesystems.ml:101-106). Same remediation as above.
	regexp.MustCompile(`(?i)error:\s*filesystem was mounted read-only, even though we asked for it to be mounted read-write.*`),
	// Generic write failure on the mounted root filesystem, distinct from the
	// read-only case above (convert/mount_filesystems.ml:108).
	regexp.MustCompile(`(?i)error:\s*could not write to the guest filesystem.*`),
	// Disk looks like an installer/live-CD image rather than an installed OS
	// (convert/mount_filesystems.ml:178-181).
	regexp.MustCompile(`(?i)error:\s*libguestfs thinks this is not an installed operating system.*`),
	// Catch-all: virt-v2v wraps any otherwise-unhandled libguestfs/guestfsd
	// exception (corrupt disk image, appliance launch failure, protocol
	// error, etc.) as "libguestfs error: <msg>" (common/mltools/tools_utils.ml,
	// run_main_and_handle_errors). Keep after the specific patterns above so
	// those take priority, but before the raw daemon-text heuristics below.
	regexp.MustCompile(`(?i)error:\s*libguestfs error:.*`),
	// Raw e2fsck/xfs_repair output captured verbatim from the guest filesystem
	// check virt-v2v runs before conversion - not standardized virt-v2v text,
	// but a strong signal of guest-side filesystem corruption.
	regexp.MustCompile(`(?i).*WARNING:\s*Filesystem still has errors.*`),
	// guestfsd (the in-appliance libguestfs daemon) reporting its own error
	// over the protocol, separate from virt-v2v's OCaml-side wrapping above.
	regexp.MustCompile(`(?i)guestfsd:\s*error:.*`),
	// Broad safety net: any other fatal error virt-v2v-in-place printed via
	// its shared error() helper that isn't one of the specific cases above.
	regexp.MustCompile(`(?i)virt-v2v-in-place:\s*error:.*`),
}

// cleanLogLine strips carriage returns and other terminal control
// characters that virt-v2v/e2fsck emit (e.g. "\r", "^M") and trims
// whitespace, so the extracted line is safe to embed in an error message,
// Kubernetes Event, or CRD status field.
func cleanLogLine(line string) string {
	line = strings.ReplaceAll(line, "\r", "")
	line = strings.TrimSpace(line)
	return line
}

// ExtractVirtV2VFailureReason scans a virt-v2v debug log file and returns the
// most relevant human-readable error line describing why the conversion
// failed. It first looks (newest line first) for known virt-v2v/libguestfs/
// e2fsck failure signatures; if none match, it falls back to the last line
// containing "error:" (virt-v2v's own convention for fatal errors). Returns
// "" if the log can't be read or no error line is found.
func ExtractVirtV2VFailureReason(logPath string) string {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		line := cleanLogLine(lines[i])
		if line == "" {
			continue
		}
		for _, re := range virtV2VErrorPatterns {
			if re.MatchString(line) {
				return line
			}
		}
	}

	// Generic fallback: last line containing "error:" case-insensitively.
	for i := len(lines) - 1; i >= 0; i-- {
		line := cleanLogLine(lines[i])
		if line == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), "error:") {
			return line
		}
	}

	return ""
}

// GetLatestLogFilePath returns the path of the most recently created debug
// log file for the given migration and category (e.g. LogCategoryVirtV2V),
// matching the "<category>.<timestamp>.log" naming convention used by
// AddDebugOutputToFileWithCommandCategory. Returns an error if no matching
// log file is found.
func GetLatestLogFilePath(migrationName, category string) (string, error) {
	attemptDir := filepath.Join(logsBaseDir, migrationName)
	pattern := filepath.Join(attemptDir, fmt.Sprintf("%s.*.log", category))

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to glob log files: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no log files found for migration %q category %q", migrationName, category)
	}

	// The timestamp format ("2006-01-02-15:04:05") used in filenames is
	// lexicographically sortable, so the last entry after sorting is the
	// most recently created log file.
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}
