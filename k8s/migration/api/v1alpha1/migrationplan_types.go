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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type MigrationPlanStrategy struct {
	// +kubebuilder:validation:Enum=hot;cold
	Type           string      `json:"type"`
	DataCopyStart  metav1.Time `json:"dataCopyStart"`
	VMCutoverStart metav1.Time `json:"vmCutoverStart,omitempty"`
	VMCutoverEnd   metav1.Time `json:"vmCutoverEnd,omitempty"`
}

// // +kubebuilder:validation:Type=array
// type VMSteps struct {
// 	VirtualMachine []string `json:"-"`
// }

// // WorkflowStep is an anonymous list inside of ParallelSteps (i.e. it does not have a key), so it needs its own
// // custom Unmarshaller
// func (vms *VMSteps) UnmarshalJSON(value []byte) error {
// 	// Since we are writing a custom unmarshaller, we have to enforce the "DisallowUnknownFields" requirement manually.

// 	// First, get a generic representation of the contents
// 	var candidate []map[string]interface{}
// 	err := json.Unmarshal(value, &candidate)
// 	if err != nil {
// 		return err
// 	}

// 	// Generate a list of all the available JSON fields of the WorkflowStep struct
// 	availableFields := map[string]bool{}
// 	reflectType := reflect.TypeOf("")
// 	for i := 0; i < reflectType.NumField(); i++ {
// 		cleanString := strings.ReplaceAll(reflectType.Field(i).Tag.Get("json"), ",omitempty", "")
// 		availableFields[cleanString] = true
// 	}

// 	// Enforce that no unknown fields are present
// 	for _, step := range candidate {
// 		for key := range step {
// 			if _, ok := availableFields[key]; !ok {
// 				return fmt.Errorf(`json: unknown field "%s"`, key)
// 			}
// 		}
// 	}

// 	// Finally, attempt to fully unmarshal the struct
// 	err = json.Unmarshal(value, &vms.VirtualMachine)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// MigrationPlanSpec defines the desired state of MigrationPlan
type MigrationPlanSpec struct {
	MigrationTemplate string                `json:"migrationTemplate"`
	MigrationStrategy MigrationPlanStrategy `json:"migrationStrategy"`
	VirtualMachines   [][]string            `json:"virtualmachines"`
}

// MigrationPlanStatus defines the observed state of MigrationPlan
type MigrationPlanStatus struct {
	MigrationStatus string `json:"migrationStatus"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MigrationPlan is the Schema for the migrationplans API
type MigrationPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigrationPlanSpec   `json:"spec,omitempty"`
	Status MigrationPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigrationPlanList contains a list of MigrationPlan
type MigrationPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigrationPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MigrationPlan{}, &MigrationPlanList{})
}
