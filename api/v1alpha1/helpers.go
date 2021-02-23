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

const (
	// GatewayClassControllerRef identifies contour operator as the managing controller
	// of a GatewayClass.
	GatewayClassControllerRef = "projectcontour.io/contour-operator"

	// GatewayClassParamsRefGroup identifies contour operator as the group name of a
	// GatewayClass.
	GatewayClassParamsRefGroup = "operator.projectcontour.io"

	// GatewayClassParamsRefKind identifies Contour as the kind name of a GatewayClass.
	GatewayClassParamsRefKind = "Contour"

	// GatewayFinalizer is the name of the finalizer used for a Gateway.
	GatewayFinalizer = "gateway.networking.x-k8s.io/finalizer"

	// OwningGatewayNameLabel is the owner reference label used for a Gateway
	// managed by the operator. The value should be the name of the Gateway.
	OwningGatewayNameLabel = "contour.operator.projectcontour.io/owning-gateway-name"

	// OwningGatewayNsLabel is the owner reference label used for a Gateway
	// managed by the operator. The value should be the namespace of the Gateway.
	OwningGatewayNsLabel = "contour.operator.projectcontour.io/owning-gateway-namespace"
)

// IsFinalized returns true if Contour is finalized.
func (c *Contour) IsFinalized() bool {
	for _, f := range c.Finalizers {
		if f == ContourFinalizer {
			return true
		}
	}
	return false
}

// GatewayClassSet returns true if gatewayClassRef is set for Contour.
func (c *Contour) GatewayClassSet() bool {
	return c.Spec.GatewayClassRef != nil
}
