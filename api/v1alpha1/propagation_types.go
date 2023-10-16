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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PropagationSpec defines the desired state of Propagation
type PropagationSpec struct {
	Backend     PropagationBackend `json:"backend,omitempty"`
	Deployment  Deployment         `json:"deployment,omitempty"`
	DeployAfter DeployAfter        `json:"deployAfter,omitempty"`
}

type PropagationBackend struct {
	// TODO: document and add examples
	// oci://my-registry.my-domain/kuberik/system
	// s3://my-bucket-n41nkl1n4/kuberik/system
	// +kubebuilder:validation:Pattern="^(oci|s3):\\/\\/.+$"
	// +optional
	BasePath string `json:"basePath,omitempty"`

	// The secret name containing the authentication credentials
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

type Deployment struct {
	// Propagation will not proceed if, as a result of propagation, there will be more than two active versions
	// deployed across the deployments within the same deployment group. This can be used if there are multiple sites
	// which are deployed separately but represent the same environment. In that case the rollout of a single version
	// can be preformed across all sites before starting a rollout for a newer version.
	Group string `json:"group,omitempty"`

	// Name of the deployment. This value can be used in other Propagations to determine the order in which the
	// deployments are propagated.
	Name string `json:"name,omitempty"`

	// Reads the exact version that was deployed so that accurate version can be published in the status.
	// +optional
	Version LocalObjectField `json:"version,omitempty"`

	// TODO:
	HealthSelector HealthSelector `json:"healthSelector,omitempty"`

	// Specify for how long the history of healths will be kept. This needs to be at least larger than
	// `spec.deployAfter.interval` of deployments which are deployed after this one.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	HealthHistoryDurationLimit metav1.Duration `json:"healthHistoryDurationLimit,omitempty"`
}

type HealthSelector struct {
	// TODO:
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
	// TODO:
	NamespaceSelector metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

type DeployAfter struct {
	// Propagation will proceed only after all listed deployments report
	// a healthy version for the specified amount of time.
	Deployments []string `json:"deployments,omitempty"`

	// TODO:
	Groups []string `json:"groups,omitempty"`

	// Propagtion will only be performed after all the deployments specified as dependencies report
	// continous healthy states for the specifed duration.
	// In case there's multiple versions satisfying the condition the newest one will be used.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	Interval metav1.Duration `json:"interval,omitempty"`
}

type LocalObjectField struct {
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty"`
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +optional
	Name string `json:"name,omitempty"`
	// API version of the referent.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// If referring to a piece of an object instead of an entire object, this string
	// should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
	// For example, if the object reference is to a container within a pod, this would take on a value like:
	// "spec.containers{name}" (where "name" refers to the name of the container that triggered
	// the event) or if no container name is specified "spec.containers[2]" (container with
	// index 2 in this pod). This syntax is chosen only to have some well-defined way of
	// referencing a part of an object.
	// +required
	FieldPath string `json:"fieldPath,omitempty"`
}

// PropagationStatus defines the observed state of Propagation
type PropagationStatus struct {
	DeploymentStatusHistory []DeploymentStatusReport `json:"deploymentStatusHistory,omitempty"`
}

type DeploymentStatusReport struct {
	Version string      `json:"version,omitempty"`
	Start   metav1.Time `json:"start,omitempty"`

	//+kubebuilder:validation:Enum=Healthy;Pending;Degraded
	Status HealthState `json:"healthy,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Propagation is the Schema for the propagations API
type Propagation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PropagationSpec   `json:"spec,omitempty"`
	Status PropagationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PropagationList contains a list of Propagation
type PropagationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Propagation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Propagation{}, &PropagationList{})
}
