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
	"crypto/x509"
	"encoding/pem"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objgc "github.com/projectcontour/contour-operator/internal/objects/gatewayclass"
	objsecret "github.com/projectcontour/contour-operator/internal/objects/secret"
	"github.com/projectcontour/contour-operator/pkg/slice"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	gatewayClassNamespacedParamRef = "Namespace"
	// RouteKindHTTP is a route of kind HTTPRoute.
	RouteKindHTTP = "HTTPRoute"
	// RouteKindTLS is a route of kind TLSRoute.
	RouteKindTLS = "TLSRoute"
	// kindSecret is a certificateRef of kind Secret.
	kindSecret = "Secret"
	// groupCore is a certificateRef of group core.
	groupCore = "core"
	// certPEMBlock is the type taken from the preamble of a PEM-encoded structure.
	certPEMBlock = "CERTIFICATE"
)

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
	if err := GatewayListeners(ctx, cli, gw); err != nil {
		return fmt.Errorf("failed to validate listeners for gateway %s/%s: %w", gw.Namespace,
			gw.Name, err)
	}
	if err := GatewayAddresses(gw); err != nil {
		return fmt.Errorf("failed to validate addresses for gateway %s/%s: %w", gw.Namespace,
			gw.Name, err)
	}
	return nil
}

// GatewayListeners returns an error if the listeners of the provided gw are invalid.
// TODO [danehans]: Refactor when more than 2 listeners are supported.
func GatewayListeners(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) error {
	listeners := gw.Spec.Listeners
	if len(listeners) != 2 {
		return fmt.Errorf("%d is an invalid number of listeners", len(gw.Spec.Listeners))
	}
	if listeners[0].Port == listeners[1].Port {
		return fmt.Errorf("invalid listeners, port %v is non-unique", listeners[0].Port)
	}
	for _, listener := range listeners {
		// Note: Contour only supports http, https, and tls routes.
		if listener.Protocol != gatewayv1alpha1.HTTPProtocolType &&
			listener.Protocol != gatewayv1alpha1.HTTPSProtocolType &&
			listener.Protocol != gatewayv1alpha1.TLSProtocolType {
			return fmt.Errorf("invalid listener protocol %s", listener.Protocol)
		}
		if (listener.Protocol == gatewayv1alpha1.HTTPSProtocolType ||
			listener.Protocol == gatewayv1alpha1.TLSProtocolType) &&
			listener.TLS == nil {
			return fmt.Errorf("invalid listener; TLS configuration required with protocol %s", listener.Protocol)
		}
		if listener.TLS != nil {
			mode := listener.TLS.Mode
			if mode == gatewayv1alpha1.TLSModeTerminate && listener.TLS.CertificateRef == nil {
				return fmt.Errorf("invalid listener; certificateRef is required for mode %s", mode)
			}
			if listener.TLS.CertificateRef != nil {
				certKind := listener.TLS.CertificateRef.Kind
				if certKind != kindSecret {
					return fmt.Errorf("invalid certificateRef kind %s; only kind %s is supported", certKind,
						kindSecret)
				}
				certGroup := listener.TLS.CertificateRef.Group
				if certGroup != groupCore && certGroup != corev1.GroupName {
					return fmt.Errorf("invalid certificateRef group %s; only group %s or group %s are supported",
						certGroup, groupCore, corev1.GroupName)
				}
				certName := listener.TLS.CertificateRef.Name
				secret, err := objsecret.Get(ctx, cli, gw.Namespace, certName)
				if err != nil {
					return fmt.Errorf("failed to get secret %s/%s: %w", gw.Namespace, certName, err)
				}
				if secret.Type != corev1.SecretTypeTLS {
					return fmt.Errorf("invalid type for secret %s/%s; only type %s is supported",
						secret.Namespace, secret.Name, corev1.SecretTypeTLS)
				}
				if err := secretCertData(secret); err != nil {
					return fmt.Errorf("failed to validate certificate data for secret %s/%s: %v",
						secret.Namespace, secret.Name, err)
				}
			}
		}
		if listener.Hostname != nil {
			hostname := string(*listener.Hostname)
			if hostname != "" && hostname != "*" {
				// According to the Gateway spec, a listener hostname cannot be an IP address.
				if ip := validation.IsValidIP(hostname); ip == nil {
					return fmt.Errorf("invalid listener hostname %s", hostname)
				}
				if subErrs := validation.IsDNS1123Subdomain(hostname); subErrs != nil {
					if wildErrs := validation.IsWildcardDNS1123Subdomain(hostname); wildErrs != nil {
						return fmt.Errorf("invalid listener hostname %s", hostname)
					}
				}
			}
		}
		if listener.Routes.Group != gatewayv1alpha1.GroupName {
			return fmt.Errorf("invalid route group %s; only %s is supported", listener.Routes.Group,
				gatewayv1alpha1.GroupName)
		}
		if listener.Routes.Kind != RouteKindHTTP && listener.Routes.Kind != RouteKindTLS {
			return fmt.Errorf("invalid route kind %s; only %s and %s are supported", listener.Routes.Kind,
				RouteKindHTTP, RouteKindTLS)
		}
		if (listener.Protocol == gatewayv1alpha1.HTTPProtocolType ||
			listener.Protocol == gatewayv1alpha1.HTTPSProtocolType) &&
			listener.Routes.Kind != RouteKindHTTP {
			return fmt.Errorf("invalid route kind %s for listener protocol %s", listener.Routes.Kind,
				listener.Protocol)
		}
		if listener.Protocol == gatewayv1alpha1.TLSProtocolType && listener.Routes.Kind != RouteKindTLS {
			return fmt.Errorf("invalid route kind %s for listener protocol %s", listener.Routes.Kind,
				listener.Protocol)
		}
		if listener.Routes.Namespaces.From == gatewayv1alpha1.RouteSelectSelector &&
			(listener.Routes.Namespaces.Selector.MatchExpressions == nil &&
				listener.Routes.Namespaces.Selector.MatchLabels == nil) {
			return fmt.Errorf("invalid route namespace selection; selector must be specified")
		}
	}
	return nil
}

// GatewayAddresses returns an error if any gw addresses are invalid.
// TODO [danehans]: Refactor when named addresses are supported.
func GatewayAddresses(gw *gatewayv1alpha1.Gateway) error {
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

// secretCertData validates that secret contains a certificate/key pair.
// The secret must contain valid PEM encoded certificates.
func secretCertData(secret *corev1.Secret) error {
	certKey := corev1.TLSCertKey
	if _, ok := secret.Data[certKey]; !ok {
		return fmt.Errorf("secret %s/%s is missing data key %s", secret.Namespace, secret.Name, certKey)
	}
	if _, ok := secret.Data[corev1.TLSPrivateKeyKey]; !ok {
		return fmt.Errorf("secret %s/%s is missing data key %s", secret.Namespace, secret.Name,
			corev1.TLSPrivateKeyKey)
	}
	certData := secret.Data[certKey]
	if len(certData) == 0 {
		return fmt.Errorf("secret %s/%s data key %s is empty", secret.Namespace, secret.Name, certKey)
	}
	if err := certificateData(certData); err != nil {
		return fmt.Errorf("failed parsing certificate data from secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

// certificateData decodes certData, ensuring each PEM block is type
// "CERTIFICATE" and the block can be parsed as an x509 certificate.
func certificateData(certData []byte) error {
	var block *pem.Block
	for len(certData) != 0 {
		block, certData = pem.Decode(certData)
		if block == nil {
			return fmt.Errorf("failed to parse certificate PEM")
		}
		if block.Type != certPEMBlock {
			return fmt.Errorf("invalid certificate PEM, must be of type %s", certPEMBlock)

		}
		_, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
	}
	return nil
}
