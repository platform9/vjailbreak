// Copyright © 2024 The vjailbreak authors

package virtv2v

// Mount-plan resolution for guestfish.
//
// guestfish's -i aborts with "multi-boot operating systems are not supported"
// whenever inspect_os() returns more than one root (common/options/inspect.c).
// It counts roots, it does not compare them, so a btrfs filesystem spanning
// several devices trips it: every member carries a full superblock and libblkid
// has no "btrfs_member" type for libguestfs' "_member" filter (daemon/listfs.ml).
//
// Instead we enumerate roots without -i, collapse members of one filesystem by
// UUID, and mount explicitly, mirroring inspect_mount_root: shortest mount point
// first, a failed "/" fatal, everything else best effort.
//
// How it fits together
//
//	resolveMountPlan          the only entry point; everything else serves it
//	  cacheKey                seen these disks before? reuse the answer
//	  runScript               round 1: ask which root filesystems exist
//	  parseRoots              read that answer
//	  rootDetailsScript       round 2: ask each root for its UUID and mount points
//	  runScript
//	  parseRootDetails        read that answer (via parseMountpoints)
//	  buildPlan               decide - pure Go, no guestfish
//	    groupByFilesystem       same UUID means one filesystem  <- the btrfs fix
//	    pickRoot / rootRank     choose one member to represent it
//	    hasRoot                 make sure "/" is in the list
//	    sortMounts              parents before children
//	  describeMounts          log the result
//
//	mountScript               later callers turn a plan into guestfish commands
//	  mountLine
//	    guestfishLine
//	      quoteArg

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

// Section markers, emitted with guestfish's `echo` builtin.
const (
	uuidMarker   = "---VJB-UUID---"
	mountsMarker = "---VJB-MP---"
)

type mountSpec struct {
	// Device is a libguestfs mountable: "/dev/sda6", or "btrfsvol:/dev/sda6/@/home".
	Device     string
	MountPoint string
	Options    string
}

type mountPlan struct {
	Root   string
	Mounts []mountSpec
}

// Resolving a plan costs two appliance boots and the disks do not change
// identity within a migration, so memoise per disk set.
var (
	mountPlanCacheMu sync.Mutex
	mountPlanCache   = map[string]mountPlan{}
)

// resolveMountPlan works out what to mount for a set of disks - the job
// `guestfish -i` used to do. Two guestfish runs, then cached per disk set.
func resolveMountPlan(disks []vm.VMDisk) (mountPlan, error) {
	if len(disks) == 0 {
		return mountPlan{}, errors.New("no disks supplied; cannot resolve a guest mount plan")
	}

	key := cacheKey(disks)

	mountPlanCacheMu.Lock()
	defer mountPlanCacheMu.Unlock()

	if plan, ok := mountPlanCache[key]; ok {
		return plan, nil
	}

	// Boot 1: enumerate roots. No -i, so a root filesystem spanning several
	// devices cannot abort this.
	out, err := runScript(disks, "run\ninspect-os\n")
	if err != nil {
		return mountPlan{}, fmt.Errorf("failed to enumerate guest root filesystems: %w", err)
	}
	roots := parseRoots(out)
	if len(roots) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}

	// Boot 2: identity and mount points for every candidate root at once, so
	// that the multi-root case does not need a third boot.
	detailsOut, err := runScript(disks, rootDetailsScript(roots))
	if err != nil {
		return mountPlan{}, fmt.Errorf("failed to read details for guest root filesystems %v: %w", roots, err)
	}
	uuids, mounts := parseRootDetails(detailsOut)

	plan, err := buildPlan(roots, uuids, mounts)
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

// cacheKey builds the lookup key for the plan cache, which is just the disk
// paths joined together.
func cacheKey(disks []vm.VMDisk) string {
	paths := make([]string, 0, len(disks))
	for _, disk := range disks {
		paths = append(paths, disk.Path)
	}
	return strings.Join(paths, "|")
}

// runScript runs a guestfish script with every disk attached, read-only and with
// no -i, and returns what it printed. Used before a plan exists.
func runScript(disks []vm.VMDisk, script string) (string, error) {
	cmd := exec.Command("guestfish", "--ro")
	for _, disk := range disks {
		cmd.Args = append(cmd.Args, "-a", disk.Path)
	}
	// Scoped to this exec rather than os.Setenv, which is a process-wide write
	// and races with any other guestfish call in flight.
	cmd.Env = append(os.Environ(), "LIBGUESTFS_BACKEND=direct")
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

// parseRoots turns the output of `inspect-os` into a list of root filesystems,
// one per line. Blank lines and stray whitespace are ignored.
func parseRoots(out string) []string {
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

// rootDetailsScript builds the round-2 script. For every root it asks two
// questions - what is your UUID, and what should be mounted where.
func rootDetailsScript(roots []string) string {
	var b strings.Builder
	b.WriteString("run\n")
	// inspect-get-mountpoints reads the inspection data that only inspect-os
	// populates (daemon/inspect.ml search_for_root), and that state does not
	// survive across guestfish processes - so it has to be re-run here or every
	// mountpoints query fails with "no inspection data". Its output lands before
	// the first marker and is discarded by parseRootDetails.
	b.WriteString("inspect-os\n")
	for _, root := range roots {
		fmt.Fprintf(&b, "echo %s %s\n", uuidMarker, quoteArg(root))
		fmt.Fprintf(&b, "- vfs-uuid %s\n", quoteArg(root))
		fmt.Fprintf(&b, "echo %s %s\n", mountsMarker, quoteArg(root))
		fmt.Fprintf(&b, "- inspect-get-mountpoints %s\n", quoteArg(root))
	}
	return b.String()
}

// parseRootDetails reads the round-2 answers, using the markers to tell which
// root each one belongs to. A root with no UUID maps to "" - identity unknown.
func parseRootDetails(out string) (map[string]string, map[string][]mountSpec) {
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
			mounts[current] = parseMountpoints(text)
		}
		buf = nil
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, uuidMarker):
			flush()
			section = sectionUUID
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, uuidMarker))
		case strings.HasPrefix(trimmed, mountsMarker):
			flush()
			section = sectionMountpoints
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, mountsMarker))
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

// parseMountpoints turns `inspect-get-mountpoints` output into mount specs.
// guestfish prints "mountpoint: device" per line; the device may contain a colon.
func parseMountpoints(out string) []mountSpec {
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

// buildPlan decides what to mount: group the roots, pick one to represent the
// filesystem, make sure "/" is there, and sort. No guestfish, so it is testable.
func buildPlan(roots []string, uuids map[string]string, mounts map[string][]mountSpec) (mountPlan, error) {
	if len(roots) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}

	groups := groupByFilesystem(roots, uuids)
	if len(groups) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}
	if len(groups) > 1 {
		return mountPlan{}, fmt.Errorf(
			"guest looks genuinely multi-boot: found %d separate root filesystems %s. "+
				"vJailbreak cannot pick one automatically",
			len(groups), describeGroups(groups))
	}

	group := groups[0]
	root := pickRoot(group)
	if root == "" {
		return mountPlan{}, errors.New("could not choose a root filesystem for the guest")
	}

	plan := mountPlan{Root: root}
	plan.Mounts = append(plan.Mounts, mounts[root]...)
	if !hasRoot(plan.Mounts) {
		// No fstab (Windows), or an fstab that never named "/". Mounting the
		// root mountable itself is what inspect_get_mountpoints falls back to.
		plan.Mounts = append(plan.Mounts, mountSpec{Device: root, MountPoint: "/"})
	}
	sortMounts(plan.Mounts)

	return plan, nil
}

// groupByFilesystem puts roots that are the same filesystem into one group, using
// the UUID - identical on every btrfs member disk, which is what fixes the bug.
func groupByFilesystem(roots []string, uuids map[string]string) [][]string {
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

// pickRoot chooses one member of a group to stand for the whole filesystem.
// Best rank wins, then lowest name, so the choice is always the same.
func pickRoot(group []string) string {
	best := ""
	bestRank := 0
	for i, candidate := range group {
		rank := rootRank(candidate)
		if i == 0 || rank < bestRank || (rank == bestRank && candidate < best) {
			best = candidate
			bestRank = rank
		}
	}
	return best
}

// rootRank scores how good a device is at representing a filesystem, lower being
// better: a real partition, then a whole disk, then a subvolume mountable.
func rootRank(root string) int {
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

// hasRoot reports whether the plan already mounts "/".
// If nothing did, the caller adds it.
func hasRoot(mounts []mountSpec) bool {
	for _, spec := range mounts {
		if spec.MountPoint == "/" {
			return true
		}
	}
	return false
}

// sortMounts orders mounts shortest mount point first, so a parent is always
// mounted before anything nested inside it.
func sortMounts(mounts []mountSpec) {
	sort.SliceStable(mounts, func(i, j int) bool {
		if len(mounts[i].MountPoint) != len(mounts[j].MountPoint) {
			return len(mounts[i].MountPoint) < len(mounts[j].MountPoint)
		}
		return mounts[i].MountPoint < mounts[j].MountPoint
	})
}

// describeGroups formats root groups for an error message,
// e.g. "[/dev/sda1] and [/dev/sdb1]".
func describeGroups(groups [][]string) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, "["+strings.Join(group, " ")+"]")
	}
	return strings.Join(parts, " and ")
}

// describeMounts formats a plan for the log, as
// "/=/dev/sda6,/boot=/dev/sda2".
func describeMounts(mounts []mountSpec) string {
	parts := make([]string, 0, len(mounts))
	for _, spec := range mounts {
		parts = append(parts, spec.MountPoint+"="+spec.Device)
	}
	return strings.Join(parts, ",")
}

// mountScript builds the start of every guestfish script: launch the appliance,
// then mount everything in the plan. Callers append their own commands.
func mountScript(plan mountPlan, write bool) string {
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
		b.WriteString(mountLine(spec, write))
		b.WriteByte('\n')
	}
	return b.String()
}

// mountLine renders the guestfish command that mounts one filesystem.
// Read-only callers get mount-ro, because the drives were added with --ro.
func mountLine(spec mountSpec, write bool) string {
	if spec.Options != "" {
		options := spec.Options
		if !write {
			options = "ro," + options
		}
		return guestfishLine("mount-options", options, spec.Device, spec.MountPoint)
	}
	command := "mount-ro"
	if write {
		command = "mount"
	}
	return guestfishLine(command, spec.Device, spec.MountPoint)
}

// guestmountArgs builds the guestmount command line for a mount plan.
//
// guestmount's -m takes one colon-delimited string, "dev[:mountpoint[:options]]",
// unlike guestfish's mount command which takes separate arguments. A mountable
// such as "btrfsvol:/dev/sda6/@/home" already contains a colon and would be
// mis-parsed as device "btrfsvol", so those are dropped. Plain devices still
// mount; callers fall back to the root alone if the result is not enough.
func guestmountArgs(disks []vm.VMDisk, mounts []mountSpec, mountPoint string) []string {
	args := []string{"--rw"}
	for _, disk := range disks {
		args = append(args, "-a", disk.Path)
	}
	for _, spec := range mounts {
		if strings.Contains(spec.Device, ":") {
			log.Printf("Skipping %s for guestmount: %q is a mountable, which -m cannot express",
				spec.MountPoint, spec.Device)
			continue
		}
		mountArg := spec.Device + ":" + spec.MountPoint
		if spec.Options != "" {
			mountArg += ":" + spec.Options
		}
		args = append(args, "-m", mountArg)
	}
	return append(args, mountPoint)
}

// guestfishLine builds one line of a guestfish script: the command followed by
// its arguments, each quoted so a value with spaces stays one argument.
func guestfishLine(command string, args ...string) string {
	var b strings.Builder
	b.WriteString(command)
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(quoteArg(arg))
	}
	return b.String()
}

// quoteArg wraps an argument in double quotes for guestfish's stdin parser.
// Backslashes are escaped first, or a Windows-style path would be mangled.
func quoteArg(s string) string {
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
