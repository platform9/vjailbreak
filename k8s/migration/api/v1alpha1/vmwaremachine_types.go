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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VMInfo contains detailed information about a VMware virtual machine to be migrated,
// including resource allocation, network configuration, storage details, and host placement.
// This comprehensive data is necessary for accurately recreating the VM in the target environment.
type VMInfo struct {
	// Name is the name of the virtual machine
	Name string `json:"name"`
	// Datastores is the list of datastores for the virtual machine
	Datastores []string `json:"datastores,omitempty"`
	// Disks is the list of disks for the virtual machine
	Disks []string `json:"disks,omitempty"`
	// Networks is the list of networks for the virtual machine
	Networks []string `json:"networks,omitempty"`
	// IPAddress is the IP address of the virtual machine
	IPAddress string `json:"ipAddress,omitempty"`
	// VMState is the state of the virtual machine
	VMState string `json:"vmState,omitempty"`
	// OSType is the OS type of the virtual machine
	OSType string `json:"osType,omitempty"`
	// CPU is the number of CPUs in the virtual machine
	CPU int `json:"cpu,omitempty"`
	// Memory is the amount of memory in the virtual machine
	Memory int `json:"memory,omitempty"`
	// ESXiName is the name of the ESXi host
	ESXiName string `json:"esxiName,omitempty"`
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName,omitempty"`
}

// VMwareMachineSpec defines the desired state of VMwareMachine
type VMwareMachineSpec struct {
	// VMInfo is the info of the VMs in the VMwareMachine
	VMInfo VMInfo `json:"vms,omitempty"`

	// TargetFlavorId is the flavor to be used to create the target VM on openstack
	TargetFlavorID string `json:"targetFlavorId,omitempty"`
}

// VMwareMachineStatus defines the observed state of VMwareMachine
type VMwareMachineStatus struct {
	// PowerState is the state of the VMs in the VMware
	PowerState string `json:"powerState,omitempty"`

	// Migrated flag to indicate if the VMs have been migrated
	// +kubebuilder:default=false
	// +kubebuilder:validation:Required
	Migrated bool `json:"migrated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VMwareMachine is the Schema for the vmwaremachines API that represents a virtual machine
// in the VMware source environment targeted for migration. It tracks VM configuration,
// resource allocation, migration status, and target environment specifications.
// VMwareMachine resources are the primary workloads migrated from VMware to OpenStack
// as part of the migration process and contain all necessary information to recreate
// equivalent virtual machines in the target environment.
type VMwareMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMwareMachineSpec   `json:"spec,omitempty"`
	Status VMwareMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VMwareMachineList contains a list of VMwareMachine
type VMwareMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMwareMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VMwareMachine{}, &VMwareMachineList{})
}
