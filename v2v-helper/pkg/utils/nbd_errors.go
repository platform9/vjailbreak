// Copyright © 2024 The vjailbreak authors

package utils

import (
	"os"
	"regexp"
	"strings"
)

// nbdErrorPatterns are known nbdkit / VMware VDDK / ESXi-NFC failure
// signatures for the disk-copy phase (nbdkit serving the source VMDK over
// NBD, and nbdcopy/libnbd reading from it), ordered from most specific/
// actionable to least. Mirrors the approach used for virt-v2v conversion
// errors in virtv2v_errors.go: when a copy-phase command fails, its debug
// log is scanned (newest line first) against these patterns to find a
// concise, human-readable root cause instead of a bare "exit status 1" or
// "connection refused".
//
// Confidence note: unlike virt-v2v (open source OCaml, exact strings
// verified from source), VDDK itself is a closed-source VMware library.
// The nbdkit-side message formats below (FileLockCreateEntryDirectory, NFC
// out-of-memory, the SSL thumbprint mismatch debug line, the generic
// "Error 1 (Unknown error)" open failure, and the libssl.so.3 RHEL8 bug)
// are copied verbatim from the official nbdkit-vddk-plugin(1) man page
// (libguestfs.org/nbdkit-vddk-plugin.1.html), so those are exact. The DNS/
// connection-refused/timeout patterns are deliberately generic OS-level
// socket/getaddrinfo error text (glibc's "Name or service not known",
// standard "Connection refused"/"timed out"/"No route to host", plus the
// commonly-reported VDDK wrapper phrase "Host address lookup for server ...
// failed") rather than one exact VDDK string, because VMware doesn't
// publish the exact wording and it's been observed to vary slightly across
// VDDK/ESXi versions in third-party backup vendors' KB articles.
var nbdErrorPatterns = []*regexp.Regexp{
	// SSL (SHA1) thumbprint configured for the ESXi/vCenter host doesn't
	// match the server's actual certificate. Exact text from the nbdkit-vddk
	// man page's THUMBPRINTS section. Actionable: refresh VMwareCreds'
	// stored thumbprint.
	regexp.MustCompile(`(?i)failed to verify ssl certificate:\s*actual thumbprint=.*expected=.*`),
	// DNS/hostname resolution failure reaching the ESXi/vCenter host, most
	// commonly reported as this VDDK wrapper phrase, or the underlying
	// glibc getaddrinfo error text it wraps. Actionable: fix DNS or use an
	// IP address for the vCenter/ESXi hostname in VMwareCreds.
	regexp.MustCompile(`(?i)host address lookup for server .* failed.*`),
	regexp.MustCompile(`(?i)(name or service not known|temporary failure in name resolution|nodename nor servname provided|could not resolve host)`),
	// TCP-level connection failures reaching the ESXi/vCenter host - the
	// class of error you get when port 902 (NFC) or 443 is firewalled,
	// the host is down, or routing is broken. Actionable: check firewall/
	// security group rules for ports 902 and 443 between the vjailbreak
	// appliance and the ESXi host.
	regexp.MustCompile(`(?i)(connection refused|connection timed out|operation timed out|no route to host|network is unreachable)`),
	// ESXi hostd's NFC service ran out of memory serving the request -
	// common with several simultaneous large-block copies. Exact text from
	// the nbdkit-vddk man page's "Out of memory errors" section. Actionable:
	// raise hostd's NFC <maxMemory> on the ESXi host, or reduce concurrent
	// migrations / request size.
	regexp.MustCompile(`(?i)\[nfc error\]\s*nfcfssrvrprocesserrormsg:.*failed to allocate.*`),
	// VDDK needs to create a ".lck" file next to a locally-opened VMDK and
	// the directory isn't writable. Exact text from the nbdkit-vddk man
	// page's "FileLockCreateEntryDirectory errors" section. Only relevant
	// for local-file transports, not the remote ESXi case, but cheap to
	// keep. Actionable: fix permissions, or open read-only.
	regexp.MustCompile(`(?i)filelockcreateentrydirectory creation\s*failure on.*permission denied`),
	// VDDK ≥ 7 bug: cannot open "Independent" mode disks. Exact text from
	// the nbdkit-vddk man page's "Error 1 (Unknown error)" section.
	// Actionable: change the disk's Disk Mode away from Independent, or
	// use an older VDDK.
	regexp.MustCompile(`(?i)getfilename:\s*cannot create disk spec for disk.*`),
	// RHEL 8 + VDDK >= 8.0.2 packaging bug: the bundled VDDK can't find
	// libssl.so.3. Exact text from the nbdkit-vddk man page. Actionable:
	// downgrade to VDDK 8.0.1 in the appliance image, or install libssl.so.3.
	regexp.MustCompile(`(?i)libssl\.so\.3:\s*cannot open shared object file.*`),
	// VDDK's famously uninformative catch-all open failure. Exact text from
	// the nbdkit-vddk man page. On its own this tells the user almost
	// nothing (that's the whole point of the man page section about it),
	// but it's a strong signal to point them at the thumbprint or disk-mode
	// causes above if none of the more specific patterns matched.
	regexp.MustCompile(`(?i)vixdisklib_openex:\s*cannot open disk.*error \d+.*`),
	// Broad safety nets: any other fatal error nbdkit's VDDK plugin, nbdkit
	// itself, or nbdcopy printed that isn't one of the specific cases above.
	regexp.MustCompile(`(?i)nbdkit:\s*vddk\[\d+\]:\s*error:.*`),
	regexp.MustCompile(`(?i)nbdkit:\s*error:.*`),
	regexp.MustCompile(`(?i)nbdcopy:\s*error:.*`),
}

// ExtractNBDFailureReason scans the disk-copy phase's debug log file (the
// "nbd" category log, containing nbdkit's and nbdcopy's combined stdout/
// stderr for a migration) and returns the most relevant human-readable
// error line describing why the copy failed. It first looks (newest line
// first) for known nbdkit/VDDK/ESXi-NFC failure signatures; if none match,
// it falls back to the last line containing "error:". Returns "" if the log
// can't be read or no error line is found.
func ExtractNBDFailureReason(logPath string) string {
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
		for _, re := range nbdErrorPatterns {
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
