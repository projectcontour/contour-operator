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
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objgw "github.com/projectcontour/contour-operator/internal/objects/gateway"
	retryable "github.com/projectcontour/contour-operator/internal/retryableerror"
	"github.com/projectcontour/contour-operator/pkg/slice"

	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const gatewayClassNamespacedParamRef = "Namespace"

// Contour returns true if contour is valid.
func Contour(ctx context.Context, cli client.Client, contour *operatorv1alpha1.Contour) error {
	// TODO [danehans]: Remove when https://github.com/projectcontour/contour-operator/issues/18 is fixed.
	exist, err := objcontour.OtherContoursExistInSpecNs(ctx, cli, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if other contours exist in namespace %s: %w",
			contour.Spec.Namespace.Name, err)
	}
	if exist {
		return fmt.Errorf("other contours exist in namespace %s", contour.Spec.Namespace.Name)
	}

	if err := ContainerPorts(contour); err != nil {
		return err
	}

	if contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.NodePortServicePublishingType {
		if err := NodePorts(contour); err != nil {
			return err
		}
	}

	if contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.LoadBalancerServicePublishingType {
		if err := LoadBalancerAddress(contour); err != nil {
			return err
		}
		if err := LoadBalancerProvider(contour); err != nil {
			return err
		}
	}

	return nil
}

// ContainerPorts validates container ports of contour, returning an
// error if the container ports do not meet the API specification.
func ContainerPorts(contour *operatorv1alpha1.Contour) error {
	var numsFound []int32
	var namesFound []string
	httpFound := false
	httpsFound := false
	for _, port := range contour.Spec.NetworkPublishing.Envoy.ContainerPorts {
		if len(numsFound) > 0 && slice.ContainsInt32(numsFound, port.PortNumber) {
			return fmt.Errorf("duplicate container port number %d", port.PortNumber)
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

// NodePorts validates nodeports of contour, returning an error if the nodeports
// do not meet the API specification.
func NodePorts(contour *operatorv1alpha1.Contour) error {
	ports := contour.Spec.NetworkPublishing.Envoy.NodePorts
	if ports == nil {
		// When unspecified, API server will auto-assign port numbers.
		return nil
	}
	for _, p := range ports {
		if p.Name != "http" && p.Name != "https" {
			return fmt.Errorf("invalid port name %q; only \"http\" and \"https\" are supported", p.Name)
		}
	}
	if ports[0].Name == ports[1].Name {
		return fmt.Errorf("duplicate nodeport names detected")
	}
	if ports[0].PortNumber != nil && ports[1].PortNumber != nil {
		if ports[0].PortNumber == ports[1].PortNumber {
			return fmt.Errorf("duplicate nodeport port numbers detected")
		}
	}

	return nil
}

// LoadBalancerAddress validates LoadBalancer "address" parameter of contour, returning an
// error if "address" does not meet the API specification.
func LoadBalancerAddress(contour *operatorv1alpha1.Contour) error {
	if contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type == operatorv1alpha1.AzureLoadBalancerProvider &&
		contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure != nil &&
		contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure.Address != nil {
		validationIP := net.ParseIP(*contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure.Address)
		if validationIP == nil {
			return fmt.Errorf("wrong LoadBalancer address format, should be string with IPv4 or IPv6 format")
		}
	} else if contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type == operatorv1alpha1.GCPLoadBalancerProvider &&
		contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP != nil &&
		contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP.Address != nil {
		validationIP := net.ParseIP(*contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP.Address)
		if validationIP == nil {
			return fmt.Errorf("wrong LoadBalancer address format, should be string with IPv4 or IPv6 format")
		}
	}

	return nil
}

// LoadBalancerProvider validates LoadBalancer provider parameters of contour, returning
// and error if parameters for different provider are specified the for the one specified
// with "type" parameter.
func LoadBalancerProvider(contour *operatorv1alpha1.Contour) error {
	switch contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type {
	case operatorv1alpha1.AWSLoadBalancerProvider:
		if contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure != nil ||
			contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP != nil {
			return fmt.Errorf("aws provider chosen, other providers parameters should not be specified")
		}
	case operatorv1alpha1.AzureLoadBalancerProvider:
		if contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.AWS != nil ||
			contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP != nil {
			return fmt.Errorf("azure provider chosen, other providers parameters should not be specified")
		}
	case operatorv1alpha1.GCPLoadBalancerProvider:
		if contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.AWS != nil ||
			contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure != nil {
			return fmt.Errorf("gcp provider chosen, other providers parameters should not be specified")
		}
	}

	return nil
}

// GatewayClass returns nil if gc is a valid GatewayClass,
// otherwise an error.
func GatewayClass(gc *gatewayv1alpha1.GatewayClass) error {
	return parameterRef(gc)
}

// parameterRef returns nil if parametersRef of gc is valid,
// otherwise an error.
func parameterRef(gc *gatewayv1alpha1.GatewayClass) error {
	if gc.Spec.ParametersRef == nil {
		return fmt.Errorf("invalid gatewayclass %s, missing parametersRef", gc.Name)
	}
	if gc.Spec.ParametersRef.Scope == nil || *gc.Spec.ParametersRef.Scope != gatewayClassNamespacedParamRef {
		return fmt.Errorf("invalid parametersRef for gatewayclass %s, only namespaced-scoped references are supported", gc.Name)
	}
	group := gc.Spec.ParametersRef.Group
	if group != operatorv1alpha1.GatewayClassParamsRefGroup {
		return fmt.Errorf("invalid group %q", group)
	}
	kind := gc.Spec.ParametersRef.Kind
	if kind != operatorv1alpha1.GatewayClassParamsRefKind {
		return fmt.Errorf("invalid kind %q", kind)
	}
	if gc.Spec.ParametersRef.Namespace == nil {
		return fmt.Errorf("invalid parametersRef for gatewayclass %s, missing namespace", gc.Name)
	}
	return nil
}

// Gateway returns an error if gw is an invalid Gateway. Otherwise, the referenced Contour is returned.
func Gateway(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (*operatorv1alpha1.Contour, error) {
	var errs []error

	if err := gatewayListeners(gw); err != nil {
		errs = append(errs, fmt.Errorf("failed to validate listeners for gateway %s/%s: %w", gw.Namespace,
			gw.Name, err))
	}

	if err := gatewayAddresses(gw); err != nil {
		errs = append(errs, fmt.Errorf("failed to validate addresses for gateway %s/%s: %w", gw.Namespace,
			gw.Name, err))
	}

	contour, err := gatewayContour(ctx, cli, gw)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to validate contour for gateway %s/%s: %w", gw.Namespace, gw.Name, err))
	}

	if len(errs) != 0 {
		return nil, retryable.NewMaybeRetryableAggregate(errs)
	}

	return contour, nil
}

// gatewayListeners returns an error if the listeners of the provided gw are invalid.
// TODO [danehans]: Refactor when more than 2 listeners are supported.
func gatewayListeners(gw *gatewayv1alpha1.Gateway) error {
	listeners := gw.Spec.Listeners
	for _, listener := range listeners {
		switch listener.Protocol {
		case gatewayv1alpha1.HTTPSProtocolType, gatewayv1alpha1.TLSProtocolType:
			if listener.TLS == nil {
				return fmt.Errorf("invalid listener; tls is required for protocol %s", listener.Protocol)
			}
		case gatewayv1alpha1.HTTPProtocolType:
			break
		default:
			return fmt.Errorf("invalid listener protocol %s", listener.Protocol)
		}
		// TODO [danehans]: Enable TLS validation for HTTPS/TLS listeners.
		// xref: https://github.com/projectcontour/contour-operator/issues/214
		// Validate the listener hostname.
		// When unspecified, “”, or *, all hostnames are matched.
		if listener.Hostname == nil || (*listener.Hostname == "" || *listener.Hostname == "*") {
			continue
		}
		hostname := string(*listener.Hostname)
		if ip := validation.IsValidIP(hostname); ip == nil {
			return fmt.Errorf("invalid listener hostname %s", hostname)
		}
		var errs []string
		if strings.Contains(hostname, "*") {
			errs = append(errs, validation.IsWildcardDNS1123Subdomain(hostname)...)
		} else {
			errs = append(errs, validation.IsDNS1123Subdomain(hostname)...)
		}
		if len(errs) > 0 {
			return fmt.Errorf("invalid listener hostname %s: %s", hostname, strings.Join(errs, ", "))
		}
	}
	// TODO [danehans]: Validate routes of a gateway.
	// xref: https://github.com/projectcontour/contour-operator/issues/215

	return nil
}

// gatewayAddresses returns an error if any gw addresses are invalid.
// TODO [danehans]: Refactor when named addresses are supported.
func gatewayAddresses(gw *gatewayv1alpha1.Gateway) error {
	if len(gw.Spec.Addresses) > 0 {
		for _, a := range gw.Spec.Addresses {
			if a.Type == nil || *a.Type != gatewayv1alpha1.IPAddressType {
				return fmt.Errorf("invalid address type %v", a.Type)
			}
			if ip := validation.IsValidIP(a.Value); ip != nil {
				return fmt.Errorf("invalid address value %s", a.Value)
			}
		}
	}
	return nil
}

// gatewayContour returns the contour associated with gw, if valid.
func gatewayContour(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (*operatorv1alpha1.Contour, error) {
	contour, err := objgw.ContourForGateway(ctx, cli, gw)
	if err != nil {
		return nil, fmt.Errorf("failed to get contour for gateway %s/%s: %w", gw.Namespace, gw.Name, err)
	}
	if contour.Spec.Namespace.Name != gw.Namespace {
		return nil, fmt.Errorf("invalid contour namespace %q; namespace must match gateway namespace %q",
			contour.Spec.Namespace.Name, gw.Namespace)
	}
	return contour, nil
}
