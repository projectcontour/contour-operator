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
	"context"
	"fmt"
	"net"
	"strings"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objgc "github.com/projectcontour/contour-operator/internal/objects/gatewayclass"
	"github.com/projectcontour/contour-operator/pkg/slice"

	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
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
func GatewayClass(gc *gatewayv1alpha1.GatewayClass) error {
	return parameterRef(gc)
}

// parameterRef returns true if parametersRef of gc is valid.
func parameterRef(gc *gatewayv1alpha1.GatewayClass) error {
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

// Gateway returns an error if gw is an invalid Gateway.
func Gateway(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) error {
	if _, err := objgc.Get(ctx, cli, gw.Spec.GatewayClassName); err != nil {
		return fmt.Errorf("failed to get gatewayclass for gateway %s/%s: %w", gw.Namespace, gw.Name, err)
	}
	if err := gatewayListeners(gw); err != nil {
		return fmt.Errorf("failed to validate listeners for gateway %s/%s: %w", gw.Namespace,
			gw.Name, err)
	}
	if err := gatewayAddresses(gw); err != nil {
		return fmt.Errorf("failed to validate addresses for gateway %s/%s: %w", gw.Namespace,
			gw.Name, err)
	}
	return nil
}

// gatewayListeners returns an error if the listeners of the provided gw are invalid.
// TODO [danehans]: Refactor when more than 2 listeners are supported.
func gatewayListeners(gw *gatewayv1alpha1.Gateway) error {
	listeners := gw.Spec.Listeners
	if len(listeners) != 2 {
		return fmt.Errorf("%d is an invalid number of listeners", len(gw.Spec.Listeners))
	}
	if listeners[0].Port == listeners[1].Port {
		return fmt.Errorf("invalid listeners, port %v is non-unique", listeners[0].Port)
	}
	for _, listener := range listeners {
		if listener.Hostname != nil {
			hostname := string(*listener.Hostname)
			if hostname != "" && hostname != "*" {
				// According to the Gateway spec, a listener hostname cannot be an IP address.
				if ip := net.ParseIP(hostname); ip != nil {
					return fmt.Errorf("hostname %q must be a DNS name, not an IP address", hostname)
				}
				if strings.Contains(hostname, "*") {
					if err := validation.IsWildcardDNS1123Subdomain(hostname); err != nil {
						return fmt.Errorf("hostname %q must be a DNS name: %v", hostname, err)
					}
				} else {
					if err := validation.IsDNS1123Subdomain(hostname); err != nil {
						return fmt.Errorf("hostname %q must be a DNS name: %v", hostname, err)
					}
				}
			}
		}
	}

	return nil
}

// gatewayAddresses returns an error if any gw addresses are invalid.
// TODO [danehans]: Refactor when named addresses are supported.
func gatewayAddresses(gw *gatewayv1alpha1.Gateway) error {
	if len(gw.Spec.Addresses) > 0 {
		for _, a := range gw.Spec.Addresses {
			if a.Type != gatewayv1alpha1.IPAddressType {
				return fmt.Errorf("invalid address type %v", a.Type)
			}
			if ip := validation.IsValidIP(a.Value); ip != nil {
				return fmt.Errorf("invalid address value %s", a.Value)
			}
		}
	}
	return nil
}
