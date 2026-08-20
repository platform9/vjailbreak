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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatastoreInfo holds datastore information including backing device NAA
type DatastoreInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Capacity    int64  `json:"capacity"`
	FreeSpace   int64  `json:"freeSpace"`
	BackingNAA  string `json:"backingNAA"`  // NAA identifier of the backing LUN (for VMFS datastores)
	BackingUUID string `json:"backingUUID"` // UUID of the backing device
	MoID        string `json:"moID"`        // Managed object ID of the datastore
}

// ArrayCredsInfo holds the actual storage array credentials after decoding from secret
type ArrayCredsInfo struct {
	// Hostname is the storage array IP address or hostname
	Hostname string
	// Username is the storage array username
	Username string
	// Password is the storage array password
	Password string
	// SkipSSLVerification is whether to skip SSL certificate verification
	SkipSSLVerification bool
}

// ArrayCredsSpec defines the desired state of ArrayCreds
type ArrayCredsSpec struct {
	// VendorType is the storage array vendor type (e.g., pure, ontap, hpalletra)
	VendorType string `json:"vendorType"`

	// SecretRef is the reference to the Kubernetes secret holding storage array credentials
	SecretRef corev1.ObjectReference `json:"secretRef,omitempty"`

	// OpenStackMapping is the openstack mapping for this array
	OpenStackMapping OpenstackMapping `json:"openstackMapping,omitempty"`

	// AutoDiscovered indicates if this ArrayCreds was auto-discovered from OpenStack
	// +optional
	AutoDiscovered bool `json:"autoDiscovered,omitempty"`

	// NetAppConfig holds NetApp-specific configuration. Required when
	// VendorType is "netapp" before a migration can run. May be left empty
	// at creation time so the controller can surface available SVMs/FlexVols
	// on status.backendTargets for interactive selection.
	// +optional
	NetAppConfig *NetAppConfig `json:"netAppConfig,omitempty"`

	// MappingMode selects how target LUNs are exposed to the ESXi host during
	// Storage-Accelerated-Copy. "auto" (default) uses vendor-native mapping when
	// the provider implements it, falling back to Cinder os-initialize_connection
	// otherwise. "native" hard-requires vendor-native (validation fails for
	// providers that don't implement it). "cinder" forces the Cinder fallback
	// even on Pure/NetApp (useful for testing the fallback path).
	// +kubebuilder:validation:Enum=auto;native;cinder
	// +optional
	MappingMode string `json:"mappingMode,omitempty"`

	// VantaraConfig holds Hitachi Vantara (VSP) specific configuration.
	// Optional; used when VendorType is "vantara".
	// +optional
	VantaraConfig *VantaraConfig `json:"vantaraConfig,omitempty"`
}

// MappingMode values for ArrayCredsSpec.MappingMode. An empty MappingMode is
// equivalent to MappingModeAuto everywhere it is consumed.
const (
	// MappingModeAuto uses vendor-native mapping when the provider
	// implements it, falling back to the Cinder mapper otherwise.
	MappingModeAuto = "auto"
	// MappingModeNative hard-requires vendor-native mapping; ArrayCreds
	// validation fails for providers that don't implement it.
	MappingModeNative = "native"
	// MappingModeCinder forces the Cinder os-initialize_connection fallback
	// even on vendors with native mapping (useful for testing the fallback).
	MappingModeCinder = "cinder"
)

// NetAppConfig holds NetApp ONTAP-specific targeting information. Both fields
// must be set for the NetApp provider to create LUNs.
type NetAppConfig struct {
	// SVM is the Storage Virtual Machine name that owns the target FlexVol.
	// +optional
	SVM string `json:"svm,omitempty"`
	// FlexVol is the FlexVol name (within the SVM) where LUNs will be created.
	// +optional
	FlexVol string `json:"flexVol,omitempty"`
}

// VantaraConfig holds Hitachi Vantara (VSP family) specific targeting
// information consumed by the vantara storage provider. LUN-to-ESXi mapping
// for this vendor is delegated to the Hitachi Cinder driver (MappingMode
// auto/cinder), so only volume-placement settings live here.
type VantaraConfig struct {
	// PoolID is the decimal DP pool ID where target LDEVs are created.
	// Optional when the array has exactly one DP pool (auto-selected).
	// +optional
	PoolID string `json:"poolId,omitempty"`
	// RESTPort overrides the Configuration Manager REST API port (default 443).
	// +optional
	RESTPort string `json:"restPort,omitempty"`
}

// OpenstackMapping holds the OpenStack Cinder configuration mapping
type OpenstackMapping struct {
	// VolumeType is the Cinder volume type associated with this mapping
	VolumeType string `json:"volumeType"`
	// CinderBackendName is the Cinder backend name for this mapping
	// This is the backend configured in cinder.conf (e.g., "pure-01")
	CinderBackendName string `json:"cinderBackendName"`
	// CinderBackendPool is the pool name within the backend (optional)
	CinderBackendPool string `json:"cinderBackendPool,omitempty"`
	// CinderHost is the full Cinder host string for manage API
	// Format: hostname@backend or hostname@backend#pool (e.g., "pcd-ce@pure-iscsi-1#vt-pure-iscsi")
	CinderHost string `json:"cinderHost,omitempty"`
}

// ArrayCredsStatus defines the observed state of ArrayCreds
type ArrayCredsStatus struct {
	// ArrayValidationStatus is the status of the storage array validation
	// Possible values: Pending, Succeeded, Failed, AwaitingCredentials
	ArrayValidationStatus string `json:"arrayValidationStatus,omitempty"`
	// ArrayValidationMessage is the message associated with the storage array validation
	ArrayValidationMessage string `json:"arrayValidationMessage,omitempty"`
	// DataStore is the list of datastores associated with this array
	DataStore []DatastoreInfo `json:"dataStore,omitempty"`
	// Phase indicates the current phase of the ArrayCreds
	// Possible values: Discovered, Configured, Validated, Failed, NeedsBackendSelection
	Phase string `json:"phase,omitempty"`
	// BackendTargets is a vendor-neutral two-level tree of selectable array
	// targets (e.g., NetApp SVMs -> FlexVols). Populated by the controller
	// after successful credential validation for vendors that expose a
	// selectable target hierarchy. UIs consume this to render a picker.
	// +optional
	BackendTargets []BackendTargetGroup `json:"backendTargets,omitempty"`
}

// BackendTarget is a single selectable target within a BackendTargetGroup
// (e.g., a NetApp FlexVol, a Dell pool).
type BackendTarget struct {
	Name string `json:"name"`
	UUID string `json:"uuid,omitempty"`
}

// BackendTargetGroup groups related targets (e.g., a NetApp SVM and its FlexVols).
type BackendTargetGroup struct {
	Name     string          `json:"name"`
	UUID     string          `json:"uuid,omitempty"`
	Children []BackendTarget `json:"children,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.spec.vendorType`,name=Vendor,type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.openstackMapping.volumeType`,name=VolumeType,type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.openstackMapping.cinderBackendName`,name=Backend,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name=Phase,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.arrayValidationStatus`,name=Status,type=string

// ArrayCreds is the Schema for the storage array credentials API that defines authentication
// and connection details for storage arrays. It provides a secure way to store and validate
// storage array credentials for use in migration operations, supporting multiple vendors, but as of now
// We have qualified pure storage flash array.
type ArrayCreds struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArrayCredsSpec   `json:"spec,omitempty"`
	Status ArrayCredsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ArrayCredsList contains a list of ArrayCreds
type ArrayCredsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArrayCreds `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ArrayCreds{}, &ArrayCredsList{})
}
