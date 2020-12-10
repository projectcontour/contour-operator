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

	// DefaultContourSpecNs is the default name when spec.Namespace.Name of a Contour
	// is unspecified.
	DefaultContourSpecNs = "projectcontour"
)

// +kubebuilder:object:root=true

// Contour is the Schema for the contours API.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].reason`
type Contour struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of Contour.
	Spec ContourSpec `json:"spec,omitempty"`
	// Status defines the observed state of Contour.
	Status ContourStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContourList contains a list of Contour.
type ContourList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Contour `json:"items"`
}

// ContourSpec defines the desired state of Contour.
type ContourSpec struct {
	// Replicas is the desired number of Contour replicas. If unset,
	// defaults to 2.
	//
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// Namespace defines the schema of a Contour namespace.
	// See each field for additional details.
	//
	// +kubebuilder:default={name: "projectcontour", removeOnDeletion: false}
	Namespace NamespaceSpec `json:"namespace,omitempty"`

	// NetworkPublishing defines the schema for publishing Contour to a network.
	//
	// See each field for additional details.
	//
	NetworkPublishing NetworkPublishing `json:"networkPublishing,omitempty"`
}

// NamespaceSpec defines the schema of a Contour namespace.
type NamespaceSpec struct {
	// Name is the name of the namespace to run Contour and dependent
	// resources. If unset, defaults to "projectcontour".
	//
	// +kubebuilder:default=projectcontour
	Name string `json:"name,omitempty"`

	// RemoveOnDeletion will remove the namespace when the Contour is
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

// NetworkPublishing defines the schema for publishing Contour to a network.
type NetworkPublishing struct {
	// Envoy provides the schema for publishing the network endpoints of Envoy.
	//
	// If unset, defaults to:
	//   type:               LoadBalancerService
	//   httpContainerPort:  80
	//   httpsContainerPort: 443
	//
	Envoy EnvoyNetworkPublishing `json:"envoy,omitempty"`
}

// EnvoyNetworkPublishing defines the schema to publish Envoy to a network.
// +union
type EnvoyNetworkPublishing struct {
	// Type is the type of publishing strategy to use. Valid values are:
	//
	// * LoadBalancerService
	//
	// In this configuration, network endpoints for Envoy use container networking.
	// A Kubernetes LoadBalancer Service is created to publish Envoy network
	// endpoints. The Service uses port 80 to publish Envoy's HTTP network endpoint
	// and port 443 to publish Envoy's HTTPS network endpoint.
	//
	// See: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer
	//
	// * NodePortService
	//
	// Publishes Envoy network endpoints using a Kubernetes NodePort Service.
	//
	// In this configuration, Envoy network endpoints use container networking. A Kubernetes
	// NodePort Service is created to publish the network endpoints.
	//
	// See: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
	//
	// +unionDiscriminator
	// +kubebuilder:default=LoadBalancerService
	Type NetworkPublishingType `json:"type,omitempty"`

	// HTTPContainerPort is the HTTP port number to expose on the Envoy container.
	// This must be a valid port number, 1 < x < 65536 and differ from
	// HttpsContainerPort.
	//
	// If unset, defaults to 80.
	//
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	HTTPContainerPort int `json:"httpContainerPort,omitempty"`

	// HTTPSContainerPort is the HTTPS port number to expose on the Envoy container.
	// This must be a valid port number, 1 < x < 65536 and differ from
	// HttpContainerPort.
	//
	// If unset, defaults to 443.
	//
	// +kubebuilder:default=443
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	HTTPSContainerPort int `json:"httpsContainerPort,omitempty"`
}

// EndpointPublishingType is a way to publish network endpoints.
// +kubebuilder:validation:Enum=LoadBalancerService;NodePortService
type NetworkPublishingType string

const (
	// LoadBalancerService publishes a network endpoint using a Kubernetes LoadBalancer
	// Service.
	LoadBalancerServicePublishingType NetworkPublishingType = "LoadBalancerService"

	// NodePortService publishes a network endpoint using a Kubernetes NodePort Service.
	NodePortServicePublishingType NetworkPublishingType = "NodePortService"
)

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
