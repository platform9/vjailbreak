package controller

import (
	"testing"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	corev1 "k8s.io/api/core/v1"
)

// The type-Directory hostPath mount is a hard VDDK requirement of its own, separate from validateVDDKPresence.
func TestVDDKVolumesFollowCopyMethod(t *testing.T) {
	tests := []struct {
		name              string
		storageCopyMethod string
		wantVDDK          bool
	}{
		{
			name:              "empty (normal) copy method mounts VDDK",
			storageCopyMethod: "",
			wantVDDK:          true,
		},
		{
			name:              "normal copy method mounts VDDK",
			storageCopyMethod: "normal",
			wantVDDK:          true,
		},
		{
			name:              "StorageAcceleratedCopy does not mount VDDK",
			storageCopyMethod: constants.StorageCopyMethod,
			wantVDDK:          false,
		},
		{
			name:              "HotAdd does not mount VDDK",
			storageCopyMethod: constants.HotAddCopyMethod,
			wantVDDK:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := vddkVolumeMounts(tt.storageCopyMethod)
			volumes := vddkVolumes(tt.storageCopyMethod)

			if tt.wantVDDK {
				if len(mounts) != 1 || mounts[0].Name != "vddk" {
					t.Fatalf("expected a single vddk mount, got %+v", mounts)
				}
				if mounts[0].MountPath != "/home/fedora/vmware-vix-disklib-distrib" {
					t.Errorf("unexpected mount path %q", mounts[0].MountPath)
				}
				if len(volumes) != 1 || volumes[0].Name != "vddk" {
					t.Fatalf("expected a single vddk volume, got %+v", volumes)
				}
				if volumes[0].HostPath == nil {
					t.Fatal("expected vddk volume to be a hostPath")
				}
				if got := volumes[0].HostPath.Path; got != "/home/ubuntu/vmware-vix-disklib-distrib" {
					t.Errorf("unexpected hostPath %q", got)
				}
				if got := volumes[0].HostPath.Type; got == nil || *got != corev1.HostPathDirectory {
					t.Errorf("expected hostPath type Directory, got %v", got)
				}
				return
			}

			if len(mounts) != 0 {
				t.Errorf("expected no vddk mount for %q, got %+v", tt.storageCopyMethod, mounts)
			}
			if len(volumes) != 0 {
				t.Errorf("expected no vddk volume for %q, got %+v", tt.storageCopyMethod, volumes)
			}
		})
	}
}

// A mount without its backing volume fails pod admission, so the helpers must never disagree.
func TestVDDKVolumeAndMountStayInSync(t *testing.T) {
	for _, method := range []string{"", "normal", constants.StorageCopyMethod, constants.HotAddCopyMethod, "SomeFutureMethod"} {
		if len(vddkVolumeMounts(method)) != len(vddkVolumes(method)) {
			t.Errorf("mount/volume mismatch for storageCopyMethod %q", method)
		}
	}
}
