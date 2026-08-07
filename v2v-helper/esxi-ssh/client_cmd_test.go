// Copyright © 2024 The vjailbreak authors

package esxissh

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestConvertDatastorePathToFilesystemPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "datastore path without spaces",
			input: "[ds-prod-01] web01/web01.vmdk",
			want:  "/vmfs/volumes/ds-prod-01/web01/web01.vmdk",
		},
		{
			name:  "datastore path with spaces in folder and file",
			input: "[platform9-cluster1-pure4-vol1] mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
			want:  "/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
		},
		{
			name:  "datastore name containing a space",
			input: "[My Datastore] web01/web01.vmdk",
			want:  "/vmfs/volumes/My Datastore/web01/web01.vmdk",
		},
		{
			name:  "apostrophe in folder name",
			input: "[ds-prod-01] suhas's VM/suhas's VM.vmdk",
			want:  "/vmfs/volumes/ds-prod-01/suhas's VM/suhas's VM.vmdk",
		},
		{
			name:  "already a filesystem path is unchanged",
			input: "/vmfs/volumes/ds-prod-01/web01/web01.vmdk",
			want:  "/vmfs/volumes/ds-prod-01/web01/web01.vmdk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertDatastorePathToFilesystemPath(tt.input); got != tt.want {
				t.Errorf("convertDatastorePathToFilesystemPath(%q)\n got: %q\nwant: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildRDMDescriptorPath(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		timestamp int64
		want      string
	}{
		{
			name:      "path without spaces",
			source:    "/vmfs/volumes/ds/web01/web01.vmdk",
			timestamp: 1700000000,
			want:      "/vmfs/volumes/ds/web01/web01-rdm-1700000000.vmdk",
		},
		{
			name:      "path with spaces keeps the folder intact",
			source:    "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
			timestamp: 1786096774,
			want:      "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2-rdm-1786096774.vmdk",
		},
		{
			name:      "path with apostrophe",
			source:    "/vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
			timestamp: 1700000000,
			want:      "/vmfs/volumes/ds/suhas's VM/suhas's VM-rdm-1700000000.vmdk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildRDMDescriptorPath(tt.source, tt.timestamp); got != tt.want {
				t.Errorf("buildRDMDescriptorPath(%q, %d)\n got: %q\nwant: %q", tt.source, tt.timestamp, got, tt.want)
			}
		})
	}
}

func TestBuildVmkfstoolsRDMCloneCmd(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		device     string
		descriptor string
		want       string
	}{
		{
			name:       "no spaces",
			source:     "/vmfs/volumes/ds/web01/web01.vmdk",
			device:     "/vmfs/devices/disks/naa.6000000000000000000000000000001",
			descriptor: "/vmfs/volumes/ds/web01/web01-rdm-1700000000.vmdk",
			want: "vmkfstools -i '/vmfs/volumes/ds/web01/web01.vmdk' " +
				"-d rdm:'/vmfs/devices/disks/naa.6000000000000000000000000000001' " +
				"'/vmfs/volumes/ds/web01/web01-rdm-1700000000.vmdk'",
		},
		{
			name:       "regression: spaces in VM name (issue #2269)",
			source:     "/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
			device:     "/vmfs/devices/disks/naa.624a9370ae0bbb05e168463d00016e91",
			descriptor: "/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2-rdm-1786096774.vmdk",
			want: "vmkfstools -i '/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk' " +
				"-d rdm:'/vmfs/devices/disks/naa.624a9370ae0bbb05e168463d00016e91' " +
				"'/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2-rdm-1786096774.vmdk'",
		},
		{
			name:       "apostrophe in VM name",
			source:     "/vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
			device:     "/vmfs/devices/disks/naa.6000000000000000000000000000001",
			descriptor: "/vmfs/volumes/ds/suhas's VM/suhas's VM-rdm-1700000000.vmdk",
			want: `vmkfstools -i '/vmfs/volumes/ds/suhas'\''s VM/suhas'\''s VM.vmdk' ` +
				"-d rdm:'/vmfs/devices/disks/naa.6000000000000000000000000000001' " +
				`'/vmfs/volumes/ds/suhas'\''s VM/suhas'\''s VM-rdm-1700000000.vmdk'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildVmkfstoolsRDMCloneCmd(tt.source, tt.device, tt.descriptor); got != tt.want {
				t.Errorf("buildVmkfstoolsRDMCloneCmd()\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

// TestBuildVmkfstoolsRDMCloneCmdArgumentCount is the regression test for issue
// #2269. The command must reach vmkfstools as exactly 4 arguments no matter what
// the VM is called; before the fix, a space in the VM name produced 9.
func TestBuildVmkfstoolsRDMCloneCmdArgumentCount(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	const device = "/vmfs/devices/disks/naa.624a9370ae0bbb05e168463d00016e91"

	tests := []struct {
		name       string
		source     string
		descriptor string
	}{
		{
			name:       "no spaces",
			source:     "/vmfs/volumes/ds/web01/web01.vmdk",
			descriptor: "/vmfs/volumes/ds/web01/web01-rdm-1700000000.vmdk",
		},
		{
			name:       "spaces in VM name",
			source:     "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk",
			descriptor: "/vmfs/volumes/ds/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2-rdm-1786096774.vmdk",
		},
		{
			name:       "apostrophe in VM name",
			source:     "/vmfs/volumes/ds/suhas's VM/suhas's VM.vmdk",
			descriptor: "/vmfs/volumes/ds/suhas's VM/suhas's VM-rdm-1700000000.vmdk",
		},
		{
			name:       "datastore name with spaces",
			source:     "/vmfs/volumes/My Datastore/My VM/My VM.vmdk",
			descriptor: "/vmfs/volumes/My Datastore/My VM/My VM-rdm-1700000000.vmdk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildVmkfstoolsRDMCloneCmd(tt.source, device, tt.descriptor)

			// Replace the binary with `set --` so the shell parses the arguments
			// exactly as it would for vmkfstools, then report them one per line.
			args := strings.TrimPrefix(cmd, "vmkfstools ")
			out, err := exec.Command("sh", "-c", "set -- "+args+`; for a; do echo "$a"; done`).Output()
			if err != nil {
				t.Fatalf("shell rejected command %s: %v", cmd, err)
			}

			got := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
			want := []string{"-i", tt.source, "-d", "rdm:" + device, tt.descriptor}

			if len(got) != len(want) {
				t.Fatalf("argument count mismatch: got %d, want %d\nargs: %q", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("arg %d\n got: %q\nwant: %q", i, got[i], want[i])
				}
			}
		})
	}
}

// TestVmkfstoolsSourceArgSurvivesShell pins the precise defect: the source path
// handed to vmkfstools must be the whole path, not the text before the first space.
func TestVmkfstoolsSourceArgSurvivesShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	source := "/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28 03_47/mgmt1.grid.cyso.net_07-28 03_47_2.vmdk"
	descriptor := buildRDMDescriptorPath(source, 1786096774)
	cmd := buildVmkfstoolsRDMCloneCmd(source, "/vmfs/devices/disks/naa.624a9370ae0bbb05e168463d00016e91", descriptor)

	args := strings.TrimPrefix(cmd, "vmkfstools ")
	out, err := exec.Command("sh", "-c", "set -- "+args+`; echo "$2"`).Output()
	if err != nil {
		t.Fatalf("shell rejected command: %v", err)
	}

	got := strings.TrimRight(string(out), "\n")
	if got != source {
		t.Errorf("source argument was mangled by the shell\n got: %q\nwant: %q", got, source)
	}
	if got == "/vmfs/volumes/platform9-cluster1-pure4-vol1/mgmt1.grid.cyso.net_07-28" {
		t.Error("source truncated at first space — issue #2269 has regressed")
	}
}

// TestQuotedPathsAreNotGlobbedOrExpanded ensures a path is never reinterpreted by
// the shell, which would let a crafted VM name run commands on the ESXi host.
func TestQuotedPathsAreNotGlobbedOrExpanded(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	hostile := []string{
		"/vmfs/volumes/ds/$(echo pwned)/disk.vmdk",
		"/vmfs/volumes/ds/`echo pwned`/disk.vmdk",
		"/vmfs/volumes/ds/${HOME}/disk.vmdk",
		"/vmfs/volumes/ds/$HOME/disk.vmdk",
		"/vmfs/volumes/ds/*/disk.vmdk",
		"/vmfs/volumes/ds/VM $# $@ $1/disk.vmdk",
		"/vmfs/volumes/ds/x; touch /tmp/vjb-pwned/disk.vmdk",
		"/vmfs/volumes/ds/x && touch /tmp/vjb-pwned/disk.vmdk",
		"~/disk.vmdk",
	}

	for _, in := range hostile {
		t.Run(in, func(t *testing.T) {
			out, err := exec.Command("sh", "-c", "set -- "+shellQuote(in)+`; echo "$1"`).Output()
			if err != nil {
				t.Fatalf("shell rejected quoted form: %v", err)
			}
			if got := strings.TrimRight(string(out), "\n"); got != in {
				t.Errorf("path was expanded by the shell\n got: %q\nwant: %q", got, in)
			}
		})
	}
}

// TestHostileVMNameHasNoSideEffects asserts that a VM name crafted to inject a
// command produces no side effects when the clone command is parsed by the
// shell. A VM name is attacker-influenced input on shared vCenter estates.
func TestHostileVMNameHasNoSideEffects(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	canary := dir + "/canary"

	hostile := []string{
		"/vmfs/volumes/ds/$(touch " + canary + ")/disk.vmdk",
		"/vmfs/volumes/ds/`touch " + canary + "`/disk.vmdk",
		"/vmfs/volumes/ds/x; touch " + canary + "/disk.vmdk",
		"/vmfs/volumes/ds/x && touch " + canary + "/disk.vmdk",
		"/vmfs/volumes/ds/x > " + canary + "/disk.vmdk",
	}

	for _, in := range hostile {
		cmd := buildVmkfstoolsRDMCloneCmd(in, "/vmfs/devices/disks/naa.1", in)
		args := strings.TrimPrefix(cmd, "vmkfstools ")
		// Parse the arguments exactly as the shell would for vmkfstools.
		if _, err := exec.Command("sh", "-c", "set -- "+args+"; echo $# >/dev/null").Output(); err != nil {
			t.Fatalf("shell rejected command for %q: %v", in, err)
		}
	}

	if _, err := os.Stat(canary); !os.IsNotExist(err) {
		t.Fatalf("command injection succeeded: %s was created", canary)
	}
}
