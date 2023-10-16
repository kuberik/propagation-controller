/*
Copyright 2023.

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

// HealthSpec defines the desired state of Health
type HealthSpec struct {
	// Reported state of this Health.
	// +kubebuilder:validation:Enum=Healthy;Pending;Degraded
	State HealthState `json:"state,omitempty"`
}

type HealthState string

const (
	HealthStateHealthy   HealthState = "Healthy"
	HealthStatePending   HealthState = "Pending"
	HealthStateUnhealthy HealthState = "Unhealthy"
)

// HealthStatus defines the observed state of Health
type HealthStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Health is the Schema for the healths API
type Health struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthSpec   `json:"spec,omitempty"`
	Status HealthStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HealthList contains a list of Health
type HealthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Health `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Health{}, &HealthList{})
}
