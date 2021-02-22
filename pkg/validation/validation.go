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

package validation

import (
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/pkg/slice"

	gatewayv1a1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const gatewayClassNamespacedParamRef = "Namespace"

// Contour returns true if contour is valid.
func Contour(contour *operatorv1alpha1.Contour) error {
	return containerPorts(contour)
}

// containerPorts validates container ports of contour, returning an
// error if the container ports do not meet the API specification.
func containerPorts(contour *operatorv1alpha1.Contour) error {
	var numsFound []int32
	var namesFound []string
	httpFound := false
	httpsFound := false
	for _, port := range contour.Spec.NetworkPublishing.Envoy.ContainerPorts {
		if len(numsFound) > 0 && slice.ContainsInt32(numsFound, port.PortNumber) {
			return fmt.Errorf("duplicate container port number %q", port.PortNumber)
		}
		numsFound = append(numsFound, port.PortNumber)
		if len(namesFound) > 0 && slice.ContainsString(namesFound, port.Name) {
			return fmt.Errorf("duplicate container port name %q", port.Name)
		}
		namesFound = append(namesFound, port.Name)
		switch {
		case port.Name == "http":
			httpFound = true
		case port.Name == "https":
			httpsFound = true
		}
	}
	if httpFound && httpsFound {
		return nil
	}
	return fmt.Errorf("http and https container ports are unspecified")
}

// GatewayClass returns true if gc is a valid GatewayClass.
func GatewayClass(gc *gatewayv1a1.GatewayClass) error {
	return parameterRef(gc)
}

// parameterRef returns true if parametersRef of gc is valid.
func parameterRef(gc *gatewayv1a1.GatewayClass) error {
	if gc.Spec.ParametersRef == nil {
		return nil
	}
	if gc.Spec.ParametersRef.Scope != gatewayClassNamespacedParamRef {
		return fmt.Errorf("invalid parametersRef for gateway class %s, only namespaced-scoped referecnes are supported", gc.Name)
	}
	group := gc.Spec.ParametersRef.Group
	if group != operatorv1alpha1.GatewayClassParamsRefGroup {
		return fmt.Errorf("invalid group %q", group)
	}
	kind := gc.Spec.ParametersRef.Kind
	if kind != operatorv1alpha1.GatewayClassParamsRefKind {
		return fmt.Errorf("invalid kind %q", kind)
	}
	return nil
}
