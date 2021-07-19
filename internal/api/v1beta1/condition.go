package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionType string

// These are valid conditions of app.
const (

	// Scheduled indicates whether the has been processed by ketch-controller.
	Scheduled ConditionType = "Scheduled"
)

// Condition contains details for the current condition of this app.
type Condition struct {

	// Type of the condition.
	Type ConditionType `json:"type"`

	// Status of the condition.
	Status v1.ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp corresponding to the last status.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// A human readable message indicating details about why the application is in this condition.
	Message string `json:"message,omitempty"`
}
