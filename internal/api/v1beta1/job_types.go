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
	"github.com/shipa-corp/ketch/internal/templates"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// JobSpec defines the desired state of Job
type JobSpec struct {
	Version      string      `json:"version"`
	Type         string      `json:"type"`
	Name         string      `json:"name"`
	Framework    string      `json:"framework"`
	Description  string      `json:"description"`
	Parallelism  int         `json:"parallelism,omitempty"`
	Completions  int         `json:"completions,omitempty"`
	Suspend      bool        `json:"suspend,omitempty"`
	BackoffLimit int         `json:"backoffLimit,omitempty"`
	Containers   []Container `json:"containers,omitempty"`
	Policy       Policy      `json:"policy"`
}

// JobStatus defines the observed state of Job
type JobStatus struct {
	Conditions []Condition         `json:"conditions,omitempty"`
	Framework  *v1.ObjectReference `json:"framework,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Job is the Schema for the jobs API
type Job struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JobSpec   `json:"spec,omitempty"`
	Status JobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// JobList contains a list of Job
type JobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Job `json:"items"`
}

// Policy represents the policy types a job can have
type Policy struct {
	RestartPolicy RestartPolicy `json:"restartPolicy"`
}

// Container represents a single container run in a Job
type Container struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
}

type RestartPolicy string

const (
	// Never Restart https://kubernetes.io/docs/concepts/workloads/controllers/job/
	Never RestartPolicy = "Never"
	// OnFailure Restart https://kubernetes.io/docs/concepts/workloads/controllers/job/
	OnFailure RestartPolicy = "OnFailure"
)

func init() {
	SchemeBuilder.Register(&Job{}, &JobList{})
}

// TemplatesConfigMapName returns a name of a configmap that contains templates used to render a helm chart.
func (j *Job) TemplatesConfigMapName() string {
	return templates.IngressConfigMapName("none")
}

// Condition looks for a condition with the provided type in the condition list and returns it.
func (s JobStatus) Condition(t ConditionType) *Condition {
	for _, c := range s.Conditions {
		if c.Type == t {
			return &c
		}
	}
	return nil
}

// SetCondition sets Status and message fields of the given type of condition to the provided values.
func (j *Job) SetCondition(t ConditionType, status v1.ConditionStatus, message string, time metav1.Time) {
	c := Condition{
		Type:               t,
		Status:             status,
		LastTransitionTime: &time,
		Message:            message,
	}
	for i, cond := range j.Status.Conditions {
		if cond.Type == t {
			if cond.Status == c.Status && cond.Message == c.Message {
				return
			}
			j.Status.Conditions[i] = c
			return
		}
	}
	j.Status.Conditions = append(j.Status.Conditions, c)
}
