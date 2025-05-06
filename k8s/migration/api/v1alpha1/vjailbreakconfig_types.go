package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VjailbreakConfigSpec defines the desired state of VjailbreakConfig
type VjailbreakConfigSpec struct {
	// Debug enables or disables debug logging for all vjailbreak components
	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	Debug bool `json:"debug,omitempty"`
}

//+kubebuilder:object:root=true

// VjailbreakConfig is the Schema for the vjailbreakconfigs API
type VjailbreakConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VjailbreakConfigSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// VjailbreakConfigList contains a list of VjailbreakConfig
type VjailbreakConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VjailbreakConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VjailbreakConfig{}, &VjailbreakConfigList{})
}
