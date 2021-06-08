/*
Copyright 2021.

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

package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// Default is whether the component is applied by default when creating an Application
	Default bool `json:"default"`

	// Workload represents an identifier to a workload type
	Workload WorkloadTypeDescriptor `json:"workload"`

	// Schematic
	// +optional
	Schematic Schematic `json:"schematic"`
}

// WorkloadTypeDescriptor defines a workload
type WorkloadTypeDescriptor struct {
	// Definition is a reference to Workload definition via Group, Version, Kind
	Definition WorkloadGVK `json:"workloadGVK"`
}

// WorkloadGVK defines the version and kind for a workload
type WorkloadGVK struct {
	// Group defines the object's group
	Group string `json:"group"`

	// APIVersion defines the versioned schema of this representation of an object.
	APIVersion string `json:"apiVersion"`

	// Kind is a string value representing the REST resource this object represents.
	Kind string `json:"kind"`
}

type Parameter struct {
	Name       string   `json:"name"`
	Value      string   `json:"value"`
	Required   bool     `json:"required"`
	Type       string   `json:"type"`
	FieldPaths []string `json:"fieldPaths"`
}

type Schematic struct {
	// Kube is a template with parameter list for a certain Kubernetes workload resource as a component.
	Kube *Kube `json:"kube"`
}

type Kube struct {
	Templates []KubeTemplate `json:"templates"`
}

type KubeTemplate struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	Template   runtime.RawExtension `json:"object"`
	Parameters []Parameter          `json:"parameters,omitempty"`
}

type ComponentType string

// ComponentStatus defines the observed state of Component
type ComponentStatus struct {
	// Status reflects the observed core v1 status of a resource
	Status v1.ConditionStatus `json:"status"`
	// Message contains any details (e.g. error) regarding a status
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
