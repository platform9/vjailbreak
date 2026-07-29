// Copyright © 2024 The vjailbreak authors

package virtv2v

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// parseInspectOSOutput
// ---------------------------------------------------------------------------

func TestParseInspectOSOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "no roots",
			out:  "",
			want: nil,
		},
		{
			name: "single root",
			out:  "/dev/sda2\n",
			want: []string{"/dev/sda2"},
		},
		{
			// The multi-device btrfs case. Whole devices are enumerated before
			// partitions in libguestfs (daemon/listfs.ml), so the bare member
			// disk comes first.
			name: "two roots, btrfs spanning two disks",
			out:  "/dev/sdb\n/dev/sda6\n",
			want: []string{"/dev/sdb", "/dev/sda6"},
		},
		{
			name: "blank lines and padding are ignored",
			out:  "\n  /dev/sda6  \n\n",
			want: []string{"/dev/sda6"},
		},
		{
			name: "btrfs subvolume mountable",
			out:  "btrfsvol:/dev/sda6/@/.snapshots/1/snapshot\n",
			want: []string{"btrfsvol:/dev/sda6/@/.snapshots/1/snapshot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseInspectOSOutput(tt.out))
		})
	}
}

// ---------------------------------------------------------------------------
// parseMountpointsOutput
// ---------------------------------------------------------------------------

func TestParseMountpointsOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []mountSpec
	}{
		{
			name: "empty",
			out:  "",
			want: nil,
		},
		{
			// guestfish prints hashtables as "key: value" (fish/fish.c print_table),
			// and inspect-get-mountpoints is keyed by mount point.
			name: "root and boot",
			out:  "/: /dev/sda6\n/boot: /dev/sda2\n",
			want: []mountSpec{
				{Device: "/dev/sda6", MountPoint: "/"},
				{Device: "/dev/sda2", MountPoint: "/boot"},
			},
		},
		{
			name: "value containing a colon is preserved",
			out:  "/home: btrfsvol:/dev/sda6/@/home\n",
			want: []mountSpec{
				{Device: "btrfsvol:/dev/sda6/@/home", MountPoint: "/home"},
			},
		},
		{
			name: "malformed lines are skipped",
			out:  "garbage\n/: /dev/sda6\n: /dev/sdc\n/var:\n",
			want: []mountSpec{
				{Device: "/dev/sda6", MountPoint: "/"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseMountpointsOutput(tt.out))
		})
	}
}

// ---------------------------------------------------------------------------
// parsePlanProbeOutput
// ---------------------------------------------------------------------------

func TestParsePlanProbeOutput(t *testing.T) {
	// Exactly the shape buildPlanProbeScript produces for the failing
	// openSUSE 15.5 guest: two roots that are two members of one btrfs
	// filesystem, so both report the same UUID (the btrfs fsid).
	out := strings.Join([]string{
		planProbeUUIDMarker + " /dev/sdb",
		"b1f0a2c4-1111-4222-8333-444455556666",
		planProbeMPMarker + " /dev/sdb",
		"/: /dev/sdb",
		"/boot: /dev/sda2",
		planProbeUUIDMarker + " /dev/sda6",
		"b1f0a2c4-1111-4222-8333-444455556666",
		planProbeMPMarker + " /dev/sda6",
		"/: /dev/sda6",
		"/boot: /dev/sda2",
		"",
	}, "\n")

	uuids, mounts := parsePlanProbeOutput(out)

	assert.Equal(t, map[string]string{
		"/dev/sdb":  "b1f0a2c4-1111-4222-8333-444455556666",
		"/dev/sda6": "b1f0a2c4-1111-4222-8333-444455556666",
	}, uuids)

	assert.Equal(t, []mountSpec{
		{Device: "/dev/sda6", MountPoint: "/"},
		{Device: "/dev/sda2", MountPoint: "/boot"},
	}, mounts["/dev/sda6"])
}

func TestParsePlanProbeOutputToleratesMissingUUID(t *testing.T) {
	// "- vfs-uuid" is error tolerant, so a root can come back with no UUID at
	// all. That must not be silently treated as "same filesystem as everything
	// else with no UUID".
	out := strings.Join([]string{
		planProbeUUIDMarker + " /dev/sda2",
		planProbeMPMarker + " /dev/sda2",
		"/: /dev/sda2",
		"",
	}, "\n")

	uuids, mounts := parsePlanProbeOutput(out)

	assert.Equal(t, "", uuids["/dev/sda2"])
	assert.Equal(t, []mountSpec{{Device: "/dev/sda2", MountPoint: "/"}}, mounts["/dev/sda2"])
}

// ---------------------------------------------------------------------------
// groupRootsByUUID
// ---------------------------------------------------------------------------

func TestGroupRootsByUUID(t *testing.T) {
	tests := []struct {
		name  string
		roots []string
		uuids map[string]string
		want  [][]string
	}{
		{
			name:  "single root",
			roots: []string{"/dev/sda2"},
			uuids: map[string]string{"/dev/sda2": "aaaa"},
			want:  [][]string{{"/dev/sda2"}},
		},
		{
			// THE regression this whole change exists for.
			name:  "two members of one btrfs filesystem collapse to one group",
			roots: []string{"/dev/sdb", "/dev/sda6"},
			uuids: map[string]string{"/dev/sdb": "aaaa", "/dev/sda6": "aaaa"},
			want:  [][]string{{"/dev/sdb", "/dev/sda6"}},
		},
		{
			name:  "genuine dual boot stays two groups",
			roots: []string{"/dev/sda1", "/dev/sdb1"},
			uuids: map[string]string{"/dev/sda1": "aaaa", "/dev/sdb1": "bbbb"},
			want:  [][]string{{"/dev/sda1"}, {"/dev/sdb1"}},
		},
		{
			name:  "uuid comparison is case insensitive",
			roots: []string{"/dev/sdb", "/dev/sda6"},
			uuids: map[string]string{"/dev/sdb": "AAAA", "/dev/sda6": "aaaa"},
			want:  [][]string{{"/dev/sdb", "/dev/sda6"}},
		},
		{
			name:  "unknown uuid is never merged",
			roots: []string{"/dev/sda1", "/dev/sdb1"},
			uuids: map[string]string{},
			want:  [][]string{{"/dev/sda1"}, {"/dev/sdb1"}},
		},
		{
			name:  "three members one filesystem",
			roots: []string{"/dev/sdb", "/dev/sdc", "/dev/sda6"},
			uuids: map[string]string{"/dev/sdb": "aaaa", "/dev/sdc": "aaaa", "/dev/sda6": "aaaa"},
			want:  [][]string{{"/dev/sdb", "/dev/sdc", "/dev/sda6"}},
		},
		{
			name:  "no roots",
			roots: nil,
			uuids: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, groupRootsByUUID(tt.roots, tt.uuids))
		})
	}
}

// ---------------------------------------------------------------------------
// chooseRootFromGroup
// ---------------------------------------------------------------------------

func TestChooseRootFromGroup(t *testing.T) {
	tests := []struct {
		name  string
		group []string
		want  string
	}{
		{
			// A partition beats a bare member disk: downstream part-to-dev,
			// device-index and fstab handling all assume a partition.
			name:  "partition preferred over bare disk",
			group: []string{"/dev/sdb", "/dev/sda6"},
			want:  "/dev/sda6",
		},
		{
			name:  "bare disk is fine when it is the only member",
			group: []string{"/dev/sdb"},
			want:  "/dev/sdb",
		},
		{
			name:  "deterministic lexical tie break between partitions",
			group: []string{"/dev/sda7", "/dev/sda6"},
			want:  "/dev/sda6",
		},
		{
			name:  "plain device preferred over a btrfsvol mountable",
			group: []string{"btrfsvol:/dev/sda6/@/.snapshots/1/snapshot", "/dev/sda6"},
			want:  "/dev/sda6",
		},
		{
			name:  "lvm path counts as a partition",
			group: []string{"/dev/sdb", "/dev/vg0/lv_root"},
			want:  "/dev/vg0/lv_root",
		},
		{
			name:  "empty group",
			group: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, chooseRootFromGroup(tt.group))
		})
	}
}

// ---------------------------------------------------------------------------
// sortMountsShortestFirst
// ---------------------------------------------------------------------------

func TestSortMountsShortestFirst(t *testing.T) {
	// Mirrors compare_keys_len in libguestfs common/options/inspect.c: shortest
	// mount point first so "/" is mounted before anything nested under it.
	mounts := []mountSpec{
		{Device: "/dev/sda9", MountPoint: "/var/log"},
		{Device: "/dev/sda2", MountPoint: "/boot"},
		{Device: "/dev/sda6", MountPoint: "/"},
		{Device: "/dev/sda8", MountPoint: "/var"},
	}

	sortMountsShortestFirst(mounts)

	got := make([]string, 0, len(mounts))
	for _, m := range mounts {
		got = append(got, m.MountPoint)
	}
	assert.Equal(t, []string{"/", "/var", "/boot", "/var/log"}, got)
}

// ---------------------------------------------------------------------------
// quoteGuestfishArg
// ---------------------------------------------------------------------------

func TestQuoteGuestfishArg(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain path", in: "/tmp/x.sh", want: `"/tmp/x.sh"`},
		{name: "space is protected", in: "/tmp/x.sh --flag", want: `"/tmp/x.sh --flag"`},
		{name: "double quote escaped", in: `a"b`, want: `"a\"b"`},
		{name: "backslash escaped", in: `a\b`, want: `"a\\b"`},
		// Escaping the backslash first is what stops "\n" in a path from being
		// reinterpreted as a newline by guestfish's double-quote parser.
		{name: "backslash n is not a newline", in: `a\nb`, want: `"a\\nb"`},
		{name: "empty", in: "", want: `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quoteGuestfishArg(tt.in))
		})
	}
}

// ---------------------------------------------------------------------------
// mountCommandLine / buildGuestfishMountScript
// ---------------------------------------------------------------------------

func TestMountCommandLine(t *testing.T) {
	tests := []struct {
		name  string
		spec  mountSpec
		write bool
		want  string
	}{
		{
			name:  "read only uses mount-ro",
			spec:  mountSpec{Device: "/dev/sda6", MountPoint: "/"},
			write: false,
			want:  `mount-ro "/dev/sda6" "/"`,
		},
		{
			name:  "writable uses mount",
			spec:  mountSpec{Device: "/dev/sda6", MountPoint: "/"},
			write: true,
			want:  `mount "/dev/sda6" "/"`,
		},
		{
			name:  "options use mount-options and gain ro when read only",
			spec:  mountSpec{Device: "/dev/sda6", MountPoint: "/", Options: "subvol=@"},
			write: false,
			want:  `mount-options "ro,subvol=@" "/dev/sda6" "/"`,
		},
		{
			name:  "options are passed through when writable",
			spec:  mountSpec{Device: "/dev/sda6", MountPoint: "/", Options: "subvol=@"},
			write: true,
			want:  `mount-options "subvol=@" "/dev/sda6" "/"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mountCommandLine(tt.spec, tt.write))
		})
	}
}

func TestBuildGuestfishMountScript(t *testing.T) {
	plan := mountPlan{
		Root: "/dev/sda6",
		Mounts: []mountSpec{
			{Device: "/dev/sda6", MountPoint: "/"},
			{Device: "/dev/sda2", MountPoint: "/boot"},
		},
	}

	got := buildGuestfishMountScript(plan, true)

	// "/" must be fatal on failure, everything else best effort - this mirrors
	// inspect_mount_root in libguestfs common/options/inspect.c.
	want := "run\n" +
		`mount "/dev/sda6" "/"` + "\n" +
		`- mount "/dev/sda2" "/boot"` + "\n"
	assert.Equal(t, want, got)
}

func TestBuildGuestfishMountScriptNeverEmitsInspector(t *testing.T) {
	// Regression guard. Reintroducing -i anywhere in this path resurrects the
	// "multi-boot operating systems are not supported" failure on any guest
	// whose root filesystem spans more than one device.
	plan := mountPlan{
		Root:   "/dev/sda6",
		Mounts: []mountSpec{{Device: "/dev/sda6", MountPoint: "/"}},
	}

	for _, write := range []bool{false, true} {
		script := buildGuestfishMountScript(plan, write)
		assert.NotContains(t, script, "-i")
		assert.NotContains(t, script, "inspect-os")
		assert.True(t, strings.HasPrefix(script, "run\n"), "script must launch the appliance first")
	}
}

// ---------------------------------------------------------------------------
// formatGuestfishCommand
// ---------------------------------------------------------------------------

func TestFormatGuestfishCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "no args",
			command: "list-partitions",
			want:    "list-partitions",
		},
		{
			name:    "upload two paths",
			command: "upload",
			args:    []string{"/home/fedora/get-bootable-partition.sh", "/tmp/get-bootable-partition.sh"},
			want:    `upload "/home/fedora/get-bootable-partition.sh" "/tmp/get-bootable-partition.sh"`,
		},
		{
			// RunMountPersistenceScript passes script + flags as ONE argv token.
			// Quoting keeps it one token on stdin instead of splitting on space.
			name:    "single arg containing spaces stays one token",
			command: "sh",
			args:    []string{"/tmp/generate-mount-persistence.sh --replace-fstab"},
			want:    `sh "/tmp/generate-mount-persistence.sh --replace-fstab"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatGuestfishCommand(tt.command, tt.args...))
		})
	}
}

// ---------------------------------------------------------------------------
// buildPlanProbeScript
// ---------------------------------------------------------------------------

func TestBuildPlanProbeScript(t *testing.T) {
	got := buildPlanProbeScript([]string{"/dev/sdb", "/dev/sda6"})

	want := "run\n" +
		"echo " + planProbeUUIDMarker + ` "/dev/sdb"` + "\n" +
		`- vfs-uuid "/dev/sdb"` + "\n" +
		"echo " + planProbeMPMarker + ` "/dev/sdb"` + "\n" +
		`- inspect-get-mountpoints "/dev/sdb"` + "\n" +
		"echo " + planProbeUUIDMarker + ` "/dev/sda6"` + "\n" +
		`- vfs-uuid "/dev/sda6"` + "\n" +
		"echo " + planProbeMPMarker + ` "/dev/sda6"` + "\n" +
		`- inspect-get-mountpoints "/dev/sda6"` + "\n"

	assert.Equal(t, want, got)
}

// ---------------------------------------------------------------------------
// planFromProbe - the whole reduction, end to end, without touching guestfish
// ---------------------------------------------------------------------------

func TestPlanFromProbe(t *testing.T) {
	t.Run("multi-device btrfs resolves to a single root", func(t *testing.T) {
		roots := []string{"/dev/sdb", "/dev/sda6"}
		uuids := map[string]string{
			"/dev/sdb":  "aaaa",
			"/dev/sda6": "aaaa",
		}
		mounts := map[string][]mountSpec{
			"/dev/sda6": {
				{Device: "/dev/sda2", MountPoint: "/boot"},
				{Device: "/dev/sda6", MountPoint: "/"},
			},
		}

		plan, err := planFromProbe(roots, uuids, mounts)

		assert.NoError(t, err)
		assert.Equal(t, "/dev/sda6", plan.Root)
		assert.Equal(t, []mountSpec{
			{Device: "/dev/sda6", MountPoint: "/"},
			{Device: "/dev/sda2", MountPoint: "/boot"},
		}, plan.Mounts)
	})

	t.Run("genuine multi-boot is rejected with a useful message", func(t *testing.T) {
		roots := []string{"/dev/sda1", "/dev/sdb1"}
		uuids := map[string]string{"/dev/sda1": "aaaa", "/dev/sdb1": "bbbb"}

		_, err := planFromProbe(roots, uuids, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "multi-boot")
		assert.Contains(t, err.Error(), "/dev/sda1")
		assert.Contains(t, err.Error(), "/dev/sdb1")
	})

	t.Run("no roots is an error", func(t *testing.T) {
		_, err := planFromProbe(nil, nil, nil)
		assert.Error(t, err)
	})

	t.Run("guest without fstab falls back to mounting only the root", func(t *testing.T) {
		// Windows guests: inspect_get_mountpoints returns just the root
		// (daemon/inspect.ml).
		plan, err := planFromProbe(
			[]string{"/dev/sda2"},
			map[string]string{"/dev/sda2": "aaaa"},
			nil,
		)

		assert.NoError(t, err)
		assert.Equal(t, "/dev/sda2", plan.Root)
		assert.Equal(t, []mountSpec{{Device: "/dev/sda2", MountPoint: "/"}}, plan.Mounts)
	})

	t.Run("root mount is synthesised when fstab omits it", func(t *testing.T) {
		plan, err := planFromProbe(
			[]string{"/dev/sda6"},
			map[string]string{"/dev/sda6": "aaaa"},
			map[string][]mountSpec{
				"/dev/sda6": {{Device: "/dev/sda2", MountPoint: "/boot"}},
			},
		)

		assert.NoError(t, err)
		assert.Equal(t, []mountSpec{
			{Device: "/dev/sda6", MountPoint: "/"},
			{Device: "/dev/sda2", MountPoint: "/boot"},
		}, plan.Mounts)
	})
}
