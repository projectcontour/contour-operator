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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// Contour is the Schema for the contours API.
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
	// +optional
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`
}

// ContourStatus defines the observed state of Contour.
type ContourStatus struct {
	// availableReplicas is the number of observed available Contour replicas
	// according to the deployment.
	AvailableReplicas int32 `json:"availableReplicas"`
}

func init() {
	SchemeBuilder.Register(&Contour{}, &ContourList{})
}
