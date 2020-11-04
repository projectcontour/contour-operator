// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// OwningContourNameLabel is the owner reference label used for a Contour
	// created by the operator. The value should be the name of the contour.
	OwningContourNameLabel = "contour.operator.projectcontour.io/owning-contour-name"

	// OwningContourNsLabel is the owner reference label used for a Contour
	// created by the operator. The value should be the namespace of the contour.
	OwningContourNsLabel = "contour.operator.projectcontour.io/owning-contour-namespace"
)

// +kubebuilder:object:root=true

// Contour is the Schema for the contours API.
// +kubebuilder:subresource:status
type Contour struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContourSpec   `json:"spec,omitempty"`
	Status ContourStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContourList contains a list of Contour
type ContourList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Contour `json:"items"`
}

// ContourSpec defines the desired state of Contour.
type ContourSpec struct {
	// replicas is the desired number of Contour replicas. If unset,
	// defaults to 2.
	//
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// namespace defines the schema of a Contour namespace.
	// See each field for additional details.
	//
	// +kubebuilder:default={name: "projectcontour", removeOnDeletion: false}
	Namespace NamespaceSpec `json:"namespace,omitempty"`
}

// NamespaceSpec defines the schema of a Contour namespace.
type NamespaceSpec struct {
	// name is the name of the namespace to run Contour and dependant
	// resources. If unset, defaults to "projectcontour".
	//
	// +kubebuilder:default=projectcontour
	Name string `json:"name,omitempty"`

	// removeOnDeletion will remove the namespace when the Contour is
	// deleted. If set to True, deletion will not occur if any of the
	// following conditions exist:
	//
	// 1. The Contour namespace is "default", "kube-system" or the
	//    contour-operator's namespace.
	//
	// 2. Another Contour exists in the namespace.
	//
	// +kubebuilder:default=false
	RemoveOnDeletion bool `json:"removeOnDeletion,omitempty"`
}

const (
	// Available indicates that the contour is running and available.
	ContourAvailableConditionType = "Available"
)

// ContourStatus defines the observed state of Contour.
type ContourStatus struct {
	// AvailableContours is the number of observed available replicas
	// according to the Contour deployment. The deployment and its pods
	// will reside in the namespace specified by spec.namespace.name of
	// the contour.
	AvailableContours int32 `json:"availableContours"`

	// AvailableEnvoys is the number of observed available pods from
	// the Envoy daemonset. The daemonset and its pods will reside in the
	// namespace specified by spec.namespace.name of the contour.
	AvailableEnvoys int32 `json:"availableEnvoys"`

	// Conditions represent the observations of a contour's current state.
	// Known condition types are "Available". Reference the condition type
	// for additional details.
	//
	// TODO [danehans]: Add support for "Progressing" and "Degraded"
	// condition types.
	//
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Contour{}, &ContourList{})
}
