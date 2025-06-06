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
	"slices"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PropagationSpec defines the desired state of Propagation
type PropagationSpec struct {
	PollInterval metav1.Duration    `json:"pollInterval,omitempty"`
	Backend      PropagationBackend `json:"backend,omitempty"`
	Deployment   Deployment         `json:"deployment,omitempty"`
}

type PropagationBackend struct {
	// TODO: document and add examples
	// oci://my-registry.my-domain/kuberik/system
	// s3://my-bucket-n41nkl1n4/kuberik/system
	// +kubebuilder:validation:Pattern="^(oci|s3):\\/\\/.+$"
	// +optional
	BaseUrl string `json:"baseUrl,omitempty"`

	// The secret name containing the authentication credentials
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

type Deployment struct {
	// Name of the deployment.
	Name string `json:"name,omitempty"`

	// Reads the exact version that was deployed so that accurate version can be published in the status.
	// +optional
	Version LocalObjectField `json:"version,omitempty"`

	// [!CAUTION]
	// Not implemented!
	//
	// Selector for Health objects which will be taken into account to determine if the deployment is healthy or not.
	HealthSelector HealthSelector `json:"healthSelector,omitempty"`
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

	// Propagtion will only be performed after all the deployments specified as dependencies report
	// continous healthy states for the specifed duration.
	// In case there's multiple versions satisfying the condition the newest one will be used.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	BakeTime metav1.Duration `json:"bakeTime,omitempty"`
}

// DeployConditions defines the conditions for when and how the deployment should be propagated.
type DeployConditions struct {
	// Propagation will use these conditions to determine to which version to propagate.
	DeployAfter `json:"deployAfter,omitempty"`

	// Propagation will not proceed until all the listed deployments have the same version as the current deployment.
	DeployWith []string `json:"deployedWith,omitempty"`
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
	DeploymentStatus          DeploymentStatus           `json:"deploymentStatus,omitempty"`
	Conditions                []metav1.Condition         `json:"conditions,omitempty"`
	DeploymentStatusesReports []DeploymentStatusesReport `json:"deploymentStatusesReports,omitempty"`
	DeployConditions          DeployConditions           `json:"deployConditions,omitempty"`
}

func (s *PropagationStatus) FindDeploymentStatusReport(deployment string) *DeploymentStatusesReport {
	for i, report := range s.DeploymentStatusesReports {
		if report.DeploymentName == deployment {
			return &s.DeploymentStatusesReports[i]
		}
	}
	return nil
}

// History of deployment statuses of an other Propagation
type DeploymentStatusesReport struct {
	DeploymentName string             `json:"deploymentName,omitempty"`
	Statuses       []DeploymentStatus `json:"statuses,omitempty"`
}

func (r *DeploymentStatusesReport) VersionHealthyDuration(version string) time.Duration {
	now := time.Now()
	for i, s1 := range r.Statuses {
		if s1.Version != version {
			continue
		}
		until := now
		for _, s2 := range r.Statuses[i:] {
			if s2.State != HealthStateHealthy {
				if s2.Version == s1.Version {
					return 0
				} else {
					until = s2.Start.Time
				}
			}
		}
		return until.Sub(s1.Start.Time)
	}
	return 0
}

func (r *DeploymentStatusesReport) AppendStatus(status DeploymentStatus) {
	lastStatus := r.LastStatus()
	if lastStatus == nil {
		r.Statuses = append(r.Statuses, status)
	} else if lastStatus.Version == status.Version && lastStatus.State == HealthStatePending {
		lastStatus.State = status.State
		if status.State == HealthStateHealthy {
			lastStatus.Start = status.Start
		}
	} else {
		r.Statuses = append(r.Statuses, status)
	}
	statusCount := len(r.Statuses)
	if statusCount >= 2 && r.Statuses[statusCount-2].Version == status.Version &&
		r.Statuses[statusCount-2].State == status.State {
		r.Statuses = r.Statuses[:statusCount-1]
	}
}

func (r *DeploymentStatusesReport) LastStatus() *DeploymentStatus {
	statusCount := len(r.Statuses)
	if statusCount == 0 {
		return nil
	} else {
		return &r.Statuses[statusCount-1]
	}
}

type DeploymentStatus struct {
	Version string      `json:"version,omitempty"`
	Start   metav1.Time `json:"start,omitempty"`

	//+kubebuilder:validation:Enum=Healthy;Pending;Degraded
	State HealthState `json:"state,omitempty"`
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

func (p *Propagation) NextVersion() string {
	if len(p.Status.DeployConditions.DeployAfter.Deployments) == 0 {
		return name.DefaultTag
	}
	versions := []string{}
	for _, report := range p.Status.DeploymentStatusesReports {
		if report.DeploymentName == p.Status.DeployConditions.DeployAfter.Deployments[0] {
			for i := len(report.Statuses) - 1; i >= 0; i-- {
				version := report.Statuses[i].Version
				if !slices.Contains(versions, version) {
					versions = append(versions, version)
				}
			}
		}
	}

versions:
	for _, v := range versions {
		for _, r := range p.Status.DeploymentStatusesReports {
			if !slices.Contains(p.Status.DeployConditions.DeployAfter.Deployments, r.DeploymentName) {
				continue
			}
			if p.Status.DeployConditions.DeployAfter.BakeTime.Duration > r.VersionHealthyDuration(v) {
				continue versions
			}
		}
		return v
	}
	return ""
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
