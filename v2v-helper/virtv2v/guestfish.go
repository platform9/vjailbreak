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

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

// Section markers, emitted with guestfish's `echo` builtin.
const (
	planProbeUUIDMarker  = "---VJB-UUID---"
	planProbeMPMarker    = "---VJB-MP---"
	guestfishBatchMarker = "---VJB-CMD---"
)

// guestfishCmd is one command in a batch. Tolerate prefixes it with "-" so a
// failure does not abort the rest of the batch.
//
// Never batch an RBufferOut command: those write straight to fd 1 with
// full_write(), bypassing the stdio buffer `echo` uses (generator/fish.ml), so
// their output overtakes the markers and lands in the wrong slot. "read-file" is
// RBufferOut; "cat" is RString and is safe.
type guestfishCmd struct {
	Name     string
	Args     []string
	Tolerate bool
}

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

// ldmVolumePrefix is how libguestfs names a volume assembled from a Windows
// Dynamic Disk group: "ldm_vol_<machine>-<group>_<volume>" under /dev/mapper
// (daemon/ldm.c, via ldmtool). A root with this prefix means the system volume
// itself lives on a dynamic disk.
const ldmVolumePrefix = "/dev/mapper/ldm_vol_"

func isLDMDevice(device string) bool {
	return strings.HasPrefix(strings.TrimSpace(device), ldmVolumePrefix)
}

// Resolving a plan costs two appliance boots and the disks do not change
// identity within a migration, so memoise per disk set.
var (
	mountPlanCacheMu sync.Mutex
	mountPlanCache   = map[string]mountPlan{}
)

// parseInspectOSOutput returns the roots inspect-os found, one per line, with
// exact duplicates collapsed. Ldm.list_ldm_volumes reports a volume once per
// member disk, so a Windows Dynamic Disk yields the same device path twice; the
// same path can never be two operating systems, and deduping here means the
// collapse does not depend on vfs-uuid, which is tolerated and may fail.
func parseInspectOSOutput(out string) []string {
	var roots []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		roots = append(roots, line)
	}
	return roots
}

// parseMountpointsOutput parses `inspect-get-mountpoints`, which guestfish
// renders as "mountpoint: device" per line. The device may contain a colon.
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
// Backslash is escaped first, or "\n" in a path becomes a newline.
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

func formatGuestfishCommand(command string, args ...string) string {
	var b strings.Builder
	b.WriteString(command)
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(quoteGuestfishArg(arg))
	}
	return b.String()
}

// mountCommandLine renders one mount. Read-only callers must use mount-ro
// because the drives were added with --ro.
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

// buildGuestfishMountScript renders the preamble every invocation needs. Callers
// append their own command lines.
func buildGuestfishMountScript(plan mountPlan, write bool) string {
	var b strings.Builder
	b.WriteString("run\n")
	for _, spec := range plan.Mounts {
		// "-" stops guestfish exiting if that one command fails; fstab can name
		// filesystems that are absent, and inspect_mount_root tolerates those.
		if spec.MountPoint != "/" {
			b.WriteString("- ")
		}
		b.WriteString(mountCommandLine(spec, write))
		b.WriteByte('\n')
	}
	return b.String()
}

// buildGuestfishBatchScript renders several commands into one script separated by
// echo markers. Each guestfish invocation boots the appliance (~25s), so batching
// independent commands is the difference between one boot and N.
func buildGuestfishBatchScript(plan mountPlan, write bool, cmds []guestfishCmd) string {
	var b strings.Builder
	b.WriteString(buildGuestfishMountScript(plan, write))
	for i, c := range cmds {
		fmt.Fprintf(&b, "echo %s %d\n", guestfishBatchMarker, i)
		if c.Tolerate {
			b.WriteString("- ")
		}
		b.WriteString(formatGuestfishCommand(c.Name, c.Args...))
		b.WriteByte('\n')
	}
	return b.String()
}

// splitGuestfishBatchOutput returns one entry per command, in order. Commands
// that produced nothing - including tolerated failures - come back empty.
func splitGuestfishBatchOutput(out string, n int) []string {
	results := make([]string, n)
	idx := -1
	var buf []string

	flush := func() {
		if idx >= 0 && idx < n {
			results[idx] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
		buf = nil
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, guestfishBatchMarker) {
			flush()
			v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, guestfishBatchMarker)))
			if err != nil {
				idx = -1
			} else {
				idx = v
			}
			continue
		}
		// Keep the line verbatim: this may be file contents, where blank lines
		// and indentation matter. Only the joined result gets trimmed.
		if idx >= 0 {
			buf = append(buf, line)
		}
	}
	flush()

	return results
}

// buildPlanProbeScript asks each candidate root for its UUID and mount points,
// error tolerant so one unusable root cannot abort the probe.
func buildPlanProbeScript(roots []string) string {
	var b strings.Builder
	b.WriteString("run\n")
	// Required: inspect-get-mountpoints reads state only inspect-os populates, and
	// that state does not survive across guestfish processes. Without this every
	// mountpoints query fails with "no inspection data" and plans lose /boot.
	// Output lands before the first marker, so the parser discards it.
	b.WriteString("inspect-os\n")
	for _, root := range roots {
		fmt.Fprintf(&b, "echo %s %s\n", planProbeUUIDMarker, quoteGuestfishArg(root))
		fmt.Fprintf(&b, "- vfs-uuid %s\n", quoteGuestfishArg(root))
		fmt.Fprintf(&b, "echo %s %s\n", planProbeMPMarker, quoteGuestfishArg(root))
		fmt.Fprintf(&b, "- inspect-get-mountpoints %s\n", quoteGuestfishArg(root))
	}
	return b.String()
}

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
		default:
			if current != "" {
				buf = append(buf, trimmed)
			}
		}
	}
	flush()

	return uuids, mounts
}

// groupRootsByUUID groups roots that are the same physical filesystem. For btrfs
// the filesystem UUID is the fsid, identical on every member device.
func groupRootsByUUID(roots []string, uuids map[string]string) [][]string {
	var groups [][]string
	indexByUUID := make(map[string]int)

	for _, root := range roots {
		uuid := strings.ToLower(strings.TrimSpace(uuids[root]))
		if uuid == "" {
			// Identity unknown, so never merge it.
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

// rootPreferenceRank ranks representatives of one filesystem, lower being better.
// Any member mounts the same filesystem, but downstream part-to-dev and
// device-index assume a partition.
func rootPreferenceRank(root string) int {
	switch {
	case strings.Contains(root, ":"): // a mountable such as btrfsvol:...
		return 2
	case isBareDisk(root):
		return 1
	default:
		return 0
	}
}

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
func sortMountsShortestFirst(mounts []mountSpec) {
	sort.SliceStable(mounts, func(i, j int) bool {
		if len(mounts[i].MountPoint) != len(mounts[j].MountPoint) {
			return len(mounts[i].MountPoint) < len(mounts[j].MountPoint)
		}
		return mounts[i].MountPoint < mounts[j].MountPoint
	})
}

func hasRootMount(mounts []mountSpec) bool {
	for _, spec := range mounts {
		if spec.MountPoint == "/" {
			return true
		}
	}
	return false
}

func describeRootGroups(groups [][]string) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, "["+strings.Join(group, " ")+"]")
	}
	return strings.Join(parts, " and ")
}

func describeMounts(mounts []mountSpec) string {
	parts := make([]string, 0, len(mounts))
	for _, spec := range mounts {
		parts = append(parts, spec.MountPoint+"="+spec.Device)
	}
	return strings.Join(parts, ",")
}

// planFromProbe is the reduction from probe results to a mount plan, kept free of
// process execution so it can be unit tested.
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

	root := chooseRootFromGroup(groups[0])
	if root == "" {
		return mountPlan{}, errors.New("could not choose a root filesystem for the guest")
	}

	plan := mountPlan{Root: root}
	plan.Mounts = append(plan.Mounts, mounts[root]...)
	if !hasRootMount(plan.Mounts) {
		// No fstab (Windows), or an fstab that never named "/".
		plan.Mounts = append(plan.Mounts, mountSpec{Device: root, MountPoint: "/"})
	}
	sortMountsShortestFirst(plan.Mounts)

	return plan, nil
}

func diskSetKey(disks []vm.VMDisk) string {
	paths := make([]string, 0, len(disks))
	for _, disk := range disks {
		paths = append(paths, disk.Path)
	}
	return strings.Join(paths, "|")
}

// runGuestfishScript runs a script with no -i and nothing pre-mounted. It is the
// one entry point that cannot depend on a mount plan, since it resolves them.
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

	// Boot 1: enumerate roots. No -i, so a root spanning several devices cannot
	// abort this.
	out, err := runGuestfishScript(disks, "run\ninspect-os\n")
	if err != nil {
		return mountPlan{}, fmt.Errorf("failed to enumerate guest root filesystems: %w", err)
	}
	roots := parseInspectOSOutput(out)
	if len(roots) == 0 {
		return mountPlan{}, errors.New("no operating system was found on the supplied disks")
	}

	// Boot 2: identity and mount points for every root at once, so the multi-root
	// case needs no third boot.
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

// runCommandsInGuestAllVolumes runs several commands in ONE appliance boot and
// returns one output per command, in order and verbatim. Not lowercased, unlike
// RunCommandInGuestAllVolumes: device paths can be upper case (VolGroup00).
func runCommandsInGuestAllVolumes(disks []vm.VMDisk, write bool, cmds ...guestfishCmd) ([]string, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	os.Setenv("LIBGUESTFS_BACKEND", "direct")

	plan, err := resolveMountPlan(disks)
	if err != nil {
		return make([]string, len(cmds)), fmt.Errorf("failed to run guest commands: %w", err)
	}

	option := "--ro"
	if write {
		option = "--rw"
	}
	cmd := exec.Command("guestfish", option)
	for _, disk := range disks {
		cmd.Args = append(cmd.Args, "-a", disk.Path)
	}
	cmd.Stdin = strings.NewReader(buildGuestfishBatchScript(plan, write, cmds))

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name)
	}
	log.Printf("Executing %s with %d batched command(s): %s", cmd.String(), len(cmds), strings.Join(names, ", "))

	runErr := cmd.Run()
	if stderrBuf.Len() > 0 {
		log.Printf("guestfish stderr (batch): %s", strings.TrimSpace(stderrBuf.String()))
	}

	outputs := splitGuestfishBatchOutput(stdoutBuf.String(), len(cmds))

	if runErr != nil {
		return outputs, fmt.Errorf("failed to run batched guest commands (%s): %v: %s",
			strings.Join(names, ", "), runErr, strings.TrimSpace(stderrBuf.String()))
	}
	return outputs, nil
}
