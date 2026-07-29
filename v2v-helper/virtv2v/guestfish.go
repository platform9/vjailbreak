// Copyright © 2024 The vjailbreak authors

package virtv2v

// Mount-plan resolution for guestfish.
//
// Why this file exists
// -------------------
// Every guest operation used to run `guestfish -a ... -i`, where -i is
// libguestfs' "inspector" option. -i calls guestfs_inspect_os() and then aborts
// outright if more than one root is returned:
//
//	if (roots[1] != NULL) {
//	    fprintf (stderr, _("%s: multi-boot operating systems are not supported\n" ...
//	    exit (EXIT_FAILURE);
//	}
//	                                   -- libguestfs common/options/inspect.c
//
// That check counts roots; it does not compare operating systems. A btrfs
// filesystem spanning several devices therefore trips it, because btrfs writes a
// complete superblock (same fsid, different device UUID) to every member device
// and libblkid has no "btrfs_member" type to distinguish a member from a
// standalone filesystem. libguestfs' own guard against container devices is a
// string test on the blkid type:
//
//	else if String.ends_with "_member" vfs_type then ()
//	                                   -- libguestfs daemon/listfs.ml
//
// which catches LVM2_member, linux_raid_member and zfs_member but structurally
// cannot catch btrfs. So `btrfs device add /dev/sdb /` on an otherwise ordinary
// guest makes inspect_os() report the same root twice - once per member - and
// every guestfish call in the migration fails.
//
// What we do instead
// ------------------
// Resolve the mount plan ourselves: enumerate roots without -i (so nothing can
// abort), collapse roots that are members of one filesystem by comparing the
// filesystem UUID, then mount explicitly. The mount sequence deliberately
// mirrors inspect_mount_root() in common/options/inspect.c - shortest mount
// point first, a failed "/" is fatal, every other mount point is best effort -
// so that for the common single-root guest the result is byte-for-byte what -i
// was already doing.

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// Markers used to delimit per-root sections in the probe script's output. They
// are emitted with guestfish's `echo` builtin, which simply prints its
// arguments (fish/echo.c), so they arrive on stdout interleaved with the
// command output in a predictable order.
const (
	planProbeUUIDMarker = "---VJB-UUID---"
	planProbeMPMarker   = "---VJB-MP---"
)

// mountSpec is a single filesystem to mount inside the guestfish appliance.
type mountSpec struct {
	// Device is a libguestfs mountable: usually a device such as "/dev/sda6",
	// but possibly a subvolume mountable such as "btrfsvol:/dev/sda6/@/home".
	Device string
	// MountPoint is the guest path, e.g. "/" or "/boot".
	MountPoint string
	// Options are mount options, empty in almost all cases.
	Options string
}

// mountPlan is what `guestfish -i` would have mounted, computed explicitly.
type mountPlan struct {
	// Root is the mountable chosen to represent the guest's root filesystem.
	Root string
	// Mounts is ordered shortest mount point first.
	Mounts []mountSpec
}

// mountPlanCache memoises the plan per set of disks. Resolving it costs two
// appliance boots, and the disks do not change identity within a migration.
var (
	mountPlanCacheMu sync.Mutex
	mountPlanCache   = map[string]mountPlan{}
)

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

// parseInspectOSOutput turns `inspect-os` output into a list of roots.
func parseInspectOSOutput(out string) []string {
	var roots []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		roots = append(roots, line)
	}
	return roots
}

// parseMountpointsOutput parses `inspect-get-mountpoints` output. guestfish
// renders a hashtable as "key: value" per line (print_table in fish/fish.c) and
// this hashtable is keyed by mount point, so a line reads "/boot: /dev/sda2".
// The value may itself contain a colon, hence the split on the first ": ".
func parseMountpointsOutput(out string) []mountSpec {
	var specs []mountSpec
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ": ")
		if idx <= 0 {
			continue
		}
		mountPoint := strings.TrimSpace(line[:idx])
		device := strings.TrimSpace(line[idx+len(": "):])
		if mountPoint == "" || device == "" {
			continue
		}
		specs = append(specs, mountSpec{Device: device, MountPoint: mountPoint})
	}
	return specs
}

// quoteGuestfishArg double-quotes an argument for guestfish's stdin parser.
// Inside double quotes guestfish interprets backslash escapes (\n, \t, \", \\ -
// see the QUOTING section of guestfish(1)), so the backslash itself has to be
// escaped first or a Windows-style path would be mangled.
func quoteGuestfishArg(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '\\' || c == '"' {
			b.WriteByte('\\')
			b.WriteByte(c)
		} else {
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// formatGuestfishCommand renders one guestfish command line, quoting every
// argument so that an argument containing spaces stays a single token.
func formatGuestfishCommand(command string, args ...string) string {
	var b strings.Builder
	b.WriteString(command)
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(quoteGuestfishArg(arg))
	}
	return b.String()
}

// mountCommandLine renders the guestfish command that mounts one filesystem.
// Read-only callers must use mount-ro because the drives were added with --ro.
func mountCommandLine(spec mountSpec, write bool) string {
	if spec.Options != "" {
		options := spec.Options
		if !write {
			options = "ro," + options
		}
		return formatGuestfishCommand("mount-options", options, spec.Device, spec.MountPoint)
	}
	command := "mount-ro"
	if write {
		command = "mount"
	}
	return formatGuestfishCommand(command, spec.Device, spec.MountPoint)
}

// buildGuestfishMountScript renders the preamble every guestfish invocation
// needs: launch the appliance, then mount the plan. Callers append their own
// command lines to the result.
func buildGuestfishMountScript(plan mountPlan, write bool) string {
	var b strings.Builder
	b.WriteString("run\n")
	for _, spec := range plan.Mounts {
		// Prefixing a command with "-" tells guestfish not to exit if that one
		// command fails (guestfish(1), EXIT ON ERROR BEHAVIOUR). inspect-get-
		// mountpoints is explicitly documented as possibly returning
		// filesystems that are absent or unmountable, and inspect_mount_root
		// ignores those failures for everything except "/", so we do the same.
		if spec.MountPoint != "/" {
			b.WriteString("- ")
		}
		b.WriteString(mountCommandLine(spec, write))
		b.WriteByte('\n')
	}
	return b.String()
}

// buildPlanProbeScript asks, for every candidate root, its filesystem UUID and
// its mount points. Both queries are error tolerant so that one unusable root
// cannot abort the whole probe, and both are wrapped in echo markers so the
// interleaved output can be attributed back to the right root.
func buildPlanProbeScript(roots []string) string {
	var b strings.Builder
	b.WriteString("run\n")
	for _, root := range roots {
		fmt.Fprintf(&b, "echo %s %s\n", planProbeUUIDMarker, quoteGuestfishArg(root))
		fmt.Fprintf(&b, "- vfs-uuid %s\n", quoteGuestfishArg(root))
		fmt.Fprintf(&b, "echo %s %s\n", planProbeMPMarker, quoteGuestfishArg(root))
		fmt.Fprintf(&b, "- inspect-get-mountpoints %s\n", quoteGuestfishArg(root))
	}
	return b.String()
}

// parsePlanProbeOutput splits probe output back into per-root UUIDs and mount
// points. A root whose UUID query failed maps to "", which groupRootsByUUID
// treats as "identity unknown" rather than "same as other unknowns".
func parsePlanProbeOutput(out string) (map[string]string, map[string][]mountSpec) {
	uuids := make(map[string]string)
	mounts := make(map[string][]mountSpec)

	const (
		sectionNone = iota
		sectionUUID
		sectionMountpoints
	)

	section := sectionNone
	current := ""
	var buf []string

	flush := func() {
		if current == "" {
			buf = nil
			return
		}
		text := strings.Join(buf, "\n")
		switch section {
		case sectionUUID:
			uuids[current] = strings.ToLower(strings.TrimSpace(text))
		case sectionMountpoints:
			mounts[current] = parseMountpointsOutput(text)
		}
		buf = nil
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, planProbeUUIDMarker):
			flush()
			section = sectionUUID
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, planProbeUUIDMarker))
		case strings.HasPrefix(trimmed, planProbeMPMarker):
			flush()
			section = sectionMountpoints
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, planProbeMPMarker))
		case trimmed == "":
			// Ignore blank lines inside a section.
		default:
			if current != "" {
				buf = append(buf, trimmed)
			}
		}
	}
	flush()

	return uuids, mounts
}

// groupRootsByUUID groups roots that are the same physical filesystem. For
// btrfs the filesystem UUID is the fsid, identical on every member device, so
// two members of one filesystem land in one group while a genuine dual-boot
// guest keeps its roots apart. First-appearance order is preserved.
func groupRootsByUUID(roots []string, uuids map[string]string) [][]string {
	var groups [][]string
	indexByUUID := make(map[string]int)

	for _, root := range roots {
		uuid := strings.ToLower(strings.TrimSpace(uuids[root]))
		if uuid == "" {
			// Identity unknown: we cannot prove this is the same filesystem as
			// anything else, so never merge it.
			groups = append(groups, []string{root})
			continue
		}
		if i, ok := indexByUUID[uuid]; ok {
			groups[i] = append(groups[i], root)
			continue
		}
		indexByUUID[uuid] = len(groups)
		groups = append(groups, []string{root})
	}

	return groups
}

// rootPreferenceRank ranks candidate representatives of one filesystem, lower
// being better. Any member mounts the same filesystem so guestfish is
// indifferent, but downstream part-to-dev, device-index and fstab handling all
// assume a partition, so a partition beats a bare member disk.
func rootPreferenceRank(root string) int {
	switch {
	case strings.Contains(root, ":"):
		// A mountable such as "btrfsvol:/dev/sda6/@/home": usable, but a plain
		// device is a better representative.
		return 2
	case isBareDisk(root):
		return 1
	default:
		return 0
	}
}

// chooseRootFromGroup picks a stable representative for one filesystem.
func chooseRootFromGroup(group []string) string {
	best := ""
	bestRank := 0
	for i, candidate := range group {
		rank := rootPreferenceRank(candidate)
		if i == 0 || rank < bestRank || (rank == bestRank && candidate < best) {
			best = candidate
			bestRank = rank
		}
	}
	return best
}

// sortMountsShortestFirst orders mounts so parents are mounted before children.
// Mirrors compare_keys_len in libguestfs common/options/inspect.c.
func sortMountsShortestFirst(mounts []mountSpec) {
	sort.SliceStable(mounts, func(i, j int) bool {
		if len(mounts[i].MountPoint) != len(mounts[j].MountPoint) {
			return len(mounts[i].MountPoint) < len(mounts[j].MountPoint)
		}
		return mounts[i].MountPoint < mounts[j].MountPoint
	})
}

// hasRootMount reports whether the plan already mounts "/".
func hasRootMount(mounts []mountSpec) bool {
	for _, spec := range mounts {
		if spec.MountPoint == "/" {
			return true
		}
	}
	return false
}

// describeRootGroups renders groups for an error message, e.g.
// "[/dev/sda1] and [/dev/sdb1]".
func describeRootGroups(groups [][]string) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, "["+strings.Join(group, " ")+"]")
	}
	return strings.Join(parts, " and ")
}

// describeMounts renders a plan for logging.
func describeMounts(mounts []mountSpec) string {
	parts := make([]string, 0, len(mounts))
	for _, spec := range mounts {
		parts = append(parts, spec.MountPoint+"="+spec.Device)
	}
	return strings.Join(parts, ",")
}

// planFromProbe is the whole reduction from probe results to a mount plan. Kept
// free of process execution so the interesting logic is unit testable.
func planFromProbe(roots []string, uuids map[string]string, mounts map[string][]mountSpec) (mountPlan, error) {
	if len(roots) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}

	groups := groupRootsByUUID(roots, uuids)
	if len(groups) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}
	if len(groups) > 1 {
		return mountPlan{}, fmt.Errorf(
			"guest looks genuinely multi-boot: found %d separate root filesystems %s. "+
				"vJailbreak cannot pick one automatically",
			len(groups), describeRootGroups(groups))
	}

	group := groups[0]
	root := chooseRootFromGroup(group)
	if root == "" {
		return mountPlan{}, errors.New("could not choose a root filesystem for the guest")
	}

	plan := mountPlan{Root: root}
	plan.Mounts = append(plan.Mounts, mounts[root]...)
	if !hasRootMount(plan.Mounts) {
		// No fstab (Windows), or an fstab that never named "/". Mounting the
		// root mountable itself is what inspect_get_mountpoints falls back to.
		plan.Mounts = append(plan.Mounts, mountSpec{Device: root, MountPoint: "/"})
	}
	sortMountsShortestFirst(plan.Mounts)

	return plan, nil
}

// ---------------------------------------------------------------------------
// Resolution (runs guestfish)
// ---------------------------------------------------------------------------

// diskSetKey identifies a set of disks for the plan cache.
func diskSetKey(disks []vm.VMDisk) string {
	paths := make([]string, 0, len(disks))
	for _, disk := range disks {
		paths = append(paths, disk.Path)
	}
	return strings.Join(paths, "|")
}

// runGuestfishScript runs a script with no -i and nothing pre-mounted. This is
// the only guestfish entry point that must not depend on a mount plan, because
// it is what resolves the plan in the first place.
func runGuestfishScript(disks []vm.VMDisk, script string) (string, error) {
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	cmd := exec.Command("guestfish", "--ro")
	for _, disk := range disks {
		cmd.Args = append(cmd.Args, "-a", disk.Path)
	}
	cmd.Stdin = strings.NewReader(script)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	log.Printf("Executing %s with script:\n%s", cmd.String(), script)
	err := cmd.Run()
	if stderrBuf.Len() > 0 {
		log.Printf("guestfish stderr (mount plan): %s", strings.TrimSpace(stderrBuf.String()))
	}
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderrBuf.String()))
	}
	return stdoutBuf.String(), nil
}

// resolveMountPlan works out what to mount for these disks, memoising the
// result. Two appliance boots: one to enumerate roots, one to identify them.
func resolveMountPlan(disks []vm.VMDisk) (mountPlan, error) {
	if len(disks) == 0 {
		return mountPlan{}, errors.New("no disks supplied; cannot resolve a guest mount plan")
	}

	key := diskSetKey(disks)

	mountPlanCacheMu.Lock()
	defer mountPlanCacheMu.Unlock()

	if plan, ok := mountPlanCache[key]; ok {
		return plan, nil
	}

	// Boot 1: enumerate roots. No -i, so a root filesystem spanning several
	// devices cannot abort this.
	out, err := runGuestfishScript(disks, "run\ninspect-os\n")
	if err != nil {
		return mountPlan{}, fmt.Errorf("failed to enumerate guest root filesystems: %w", err)
	}
	roots := parseInspectOSOutput(out)
	if len(roots) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}

	// Boot 2: identity and mount points for every candidate root at once, so
	// that the multi-root case does not need a third boot.
	probeOut, err := runGuestfishScript(disks, buildPlanProbeScript(roots))
	if err != nil {
		return mountPlan{}, fmt.Errorf("failed to probe guest root filesystems %v: %w", roots, err)
	}
	uuids, mounts := parsePlanProbeOutput(probeOut)

	plan, err := planFromProbe(roots, uuids, mounts)
	if err != nil {
		return mountPlan{}, err
	}

	if len(roots) > 1 {
		log.Printf("Root filesystem spans %d devices (%s); using %s to represent it",
			len(roots), strings.Join(roots, ", "), plan.Root)
	}
	log.Printf("Resolved guest mount plan: root=%s mounts=%s", plan.Root, describeMounts(plan.Mounts))

	mountPlanCache[key] = plan
	return plan, nil
}
