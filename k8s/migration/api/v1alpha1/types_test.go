/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHostStatusDeepCopy(t *testing.T) {
	original := HostStatus{
		Name:             "esxi-01.example.com",
		VMCount:          5,
		InMaintenance:    true,
		MaintenanceState: "inMaintenance",
	}
	copied := original.DeepCopy()
	if copied == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if *copied != original {
		t.Errorf("DeepCopy result differs: got %+v, want %+v", *copied, original)
	}
	copied.Name = "changed"
	if original.Name == "changed" {
		t.Error("DeepCopy is not independent of original")
	}
}

func TestVMwareClusterSpecDeepCopy(t *testing.T) {
	tests := []struct {
		name string
		spec VMwareClusterSpec
	}{
		{
			name: "all fields nil pointers",
			spec: VMwareClusterSpec{
				Name:  "cluster-1",
				Hosts: []string{"host-a", "host-b"},
				VMwareCredsRef: corev1.LocalObjectReference{
					Name: "vcenter-creds",
				},
			},
		},
		{
			name: "with BMConfigRef and PCDClusterRef",
			spec: VMwareClusterSpec{
				Name:  "cluster-2",
				Hosts: []string{"host-a"},
				VMwareCredsRef: corev1.LocalObjectReference{
					Name: "vcenter-creds",
				},
				BMConfigRef: &corev1.LocalObjectReference{
					Name: "bmconfig-1",
				},
				PCDClusterRef: &corev1.LocalObjectReference{
					Name: "pcdcluster-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copied := tt.spec.DeepCopy()
			if copied == nil {
				t.Fatal("DeepCopy returned nil")
			}
			// Verify pointer independence
			if tt.spec.BMConfigRef != nil {
				if copied.BMConfigRef == tt.spec.BMConfigRef {
					t.Error("BMConfigRef pointer not copied (same address)")
				}
				if copied.BMConfigRef.Name != tt.spec.BMConfigRef.Name {
					t.Errorf("BMConfigRef.Name mismatch: got %s, want %s", copied.BMConfigRef.Name, tt.spec.BMConfigRef.Name)
				}
			}
			if tt.spec.PCDClusterRef != nil {
				if copied.PCDClusterRef == tt.spec.PCDClusterRef {
					t.Error("PCDClusterRef pointer not copied (same address)")
				}
				if copied.PCDClusterRef.Name != tt.spec.PCDClusterRef.Name {
					t.Errorf("PCDClusterRef.Name mismatch: got %s, want %s", copied.PCDClusterRef.Name, tt.spec.PCDClusterRef.Name)
				}
			}
		})
	}
}

func TestVMwareClusterStatusDeepCopy(t *testing.T) {
	status := VMwareClusterStatus{
		Phase: VMwareClusterRunning,
		HostStatuses: []HostStatus{
			{Name: "host-a", VMCount: 3, InMaintenance: false},
			{Name: "host-b", VMCount: 0, InMaintenance: true, MaintenanceState: "inMaintenance"},
		},
		Conditions: []metav1.Condition{
			{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "Polled",
			},
		},
		LastPollError: "",
	}

	copied := status.DeepCopy()
	if copied == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if len(copied.HostStatuses) != len(status.HostStatuses) {
		t.Errorf("HostStatuses length mismatch: got %d, want %d", len(copied.HostStatuses), len(status.HostStatuses))
	}
	// Verify slice independence
	copied.HostStatuses[0].Name = "changed"
	if status.HostStatuses[0].Name == "changed" {
		t.Error("HostStatuses slice not independently copied")
	}
}

func TestESXIMigrationSpecDeepCopy(t *testing.T) {
	tests := []struct {
		name              string
		spec              ESXIMigrationSpec
		wantPlanRefNil    bool
		wantBMConfigNil   bool
		wantPCDClusterNil bool
	}{
		{
			name: "plan-based mode (RollingMigrationPlanRef set)",
			spec: ESXIMigrationSpec{
				ESXiName:          "esxi-01",
				OpenstackCredsRef: corev1.LocalObjectReference{Name: "os-creds"},
				VMwareCredsRef:    corev1.LocalObjectReference{Name: "vc-creds"},
				RollingMigrationPlanRef: &corev1.LocalObjectReference{
					Name: "plan-1",
				},
			},
			wantPlanRefNil:    false,
			wantBMConfigNil:   true,
			wantPCDClusterNil: true,
		},
		{
			name: "standalone mode (all three refs set, no plan)",
			spec: ESXIMigrationSpec{
				ESXiName:          "esxi-02",
				OpenstackCredsRef: corev1.LocalObjectReference{Name: "os-creds"},
				VMwareCredsRef:    corev1.LocalObjectReference{Name: "vc-creds"},
				BMConfigRef:       &corev1.LocalObjectReference{Name: "bmc-1"},
				PCDClusterRef:     &corev1.LocalObjectReference{Name: "pcd-1"},
			},
			wantPlanRefNil:    true,
			wantBMConfigNil:   false,
			wantPCDClusterNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copied := tt.spec.DeepCopy()
			if copied == nil {
				t.Fatal("DeepCopy returned nil")
			}

			if (copied.RollingMigrationPlanRef == nil) != tt.wantPlanRefNil {
				t.Errorf("RollingMigrationPlanRef nil=%v, want nil=%v", copied.RollingMigrationPlanRef == nil, tt.wantPlanRefNil)
			}
			if (copied.BMConfigRef == nil) != tt.wantBMConfigNil {
				t.Errorf("BMConfigRef nil=%v, want nil=%v", copied.BMConfigRef == nil, tt.wantBMConfigNil)
			}
			if (copied.PCDClusterRef == nil) != tt.wantPCDClusterNil {
				t.Errorf("PCDClusterRef nil=%v, want nil=%v", copied.PCDClusterRef == nil, tt.wantPCDClusterNil)
			}

			// Verify pointer independence for non-nil refs
			if tt.spec.RollingMigrationPlanRef != nil && copied.RollingMigrationPlanRef != nil {
				if copied.RollingMigrationPlanRef == tt.spec.RollingMigrationPlanRef {
					t.Error("RollingMigrationPlanRef not independently copied")
				}
			}
		})
	}
}
