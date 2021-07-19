/*


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
)

func init() {
	SchemeBuilder.Register(&Framework{}, &FrameworkList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Target Namespace",type=string,JSONPath=`.status.namespace.name`
// +kubebuilder:printcolumn:name="apps",type=string,JSONPath=`.status.apps`
// +kubebuilder:printcolumn:name="quota",type=string,JSONPath=`.spec.appQuotaLimit`

// Framework is the Schema for the frameworks API
type Framework struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FrameworkSpec   `json:"spec,omitempty"`
	Status FrameworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FrameworkList contains a list of Framework
type FrameworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Framework `json:"items"`
}

// FrameworkSpec defines the desired state of Framework
type FrameworkSpec struct {
	Version string `json:"version,omitempty"`
	Name    string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	NamespaceName string `json:"namespace"`

	AppQuotaLimit *int `json:"appQuotaLimit"`

	IngressController IngressControllerSpec `json:"ingressController,omitempty"`
}

type FrameworkPhase string

const (
	FrameworkCreated FrameworkPhase = "Created"
	FrameworkFailed  FrameworkPhase = "Failed"
)

// +kubebuilder:validation:Enum=traefik;istio

// IngressControllerType is a type of an ingress controller for this framework.
type IngressControllerType string

func (t IngressControllerType) String() string { return string(t) }

const (
	TraefikIngressControllerType IngressControllerType = "traefik"
	IstioIngressControllerType   IngressControllerType = "istio"
)

// IngressControllerSpec contains configuration for an ingress controller.
type IngressControllerSpec struct {
	ClassName       string                `json:"className,omitempty"`
	ServiceEndpoint string                `json:"serviceEndpoint,omitempty"`
	IngressType     IngressControllerType `json:"type"`
	ClusterIssuer   string                `json:"clusterIssuer,omitempty"`
}

// FrameworkStatus defines the observed state of Framework
type FrameworkStatus struct {
	Phase   FrameworkPhase `json:"phase,omitempty"`
	Message string         `json:"message,omitempty"`

	Namespace *v1.ObjectReference `json:"namespace,omitempty"`
	Apps      []string            `json:"apps,omitempty"`
	Jobs      []string            `json:"jobs,omitempty"`
}

func (p *Framework) HasApp(name string) bool {
	for _, appName := range p.Status.Apps {
		if appName == name {
			return true
		}
	}
	return false
}

func (f *Framework) HasJob(name string) bool {
	for _, appName := range f.Status.Jobs {
		if appName == name {
			return true
		}
	}
	return false
}
