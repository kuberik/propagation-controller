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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReleasePromotionSpec defines the desired state of ReleasePromotion.
type ReleasePromotionSpec struct {
	// Protocol defines the type of repository protocol to use (e.g. oci, s3)
	// +kubebuilder:validation:Enum=oci;s3
	// +kubebuilder:default=oci
	Protocol string `json:"protocol,omitempty"`

	// ReleasesRepository specifies the path to the releases repository
	// +kubebuilder:validation:Required
	// +required
	ReleasesRepository string `json:"releasesRepository,omitempty"`

	// TargetRepository specifies the path where releases should be promoted to
	// +kubebuilder:validation:Required
	// +required
	TargetRepository string `json:"targetRepository,omitempty"`

	// Auth contains reference to the secret with authentication credentials
	// +optional
	Auth *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

type ReleaseRepository struct {
	// TODO: document and add examples
	// oci://my-registry.my-domain/kuberik/system
	// s3://my-bucket-n41nkl1n4/kuberik/system
	// +kubebuilder:validation:Pattern="^(oci|s3):\\/\\/.+$"
	// +optional
	Url string `json:"baseUrl,omitempty"`

	// The secret name containing the authentication credentials
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// ReleasePromotionStatus defines the observed state of ReleasePromotion.
type ReleasePromotionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions represents the current state of the release promotion process.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ReleasePromotion is the Schema for the releasepromotions API.
type ReleasePromotion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleasePromotionSpec   `json:"spec,omitempty"`
	Status ReleasePromotionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleasePromotionList contains a list of ReleasePromotion.
type ReleasePromotionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleasePromotion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleasePromotion{}, &ReleasePromotionList{})
}
