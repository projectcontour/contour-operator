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

package validation_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/operator"
	"github.com/projectcontour/contour-operator/pkg/validation"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	envoyInsecureContainerPort = int32(8080)
	envoySecureContainerPort   = int32(8443)
)

func TestContainerPorts(t *testing.T) {
	testCases := []struct {
		description string
		ports       []operatorv1alpha1.ContainerPort
		expected    bool
	}{
		{
			description: "default http and https port",
			expected:    true,
		},
		{
			description: "non-default http and https ports",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       "http",
					PortNumber: int32(8081),
				},
				{
					Name:       "https",
					PortNumber: int32(8444),
				},
			},
			expected: true,
		},
		{
			description: "duplicate port names",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       "http",
					PortNumber: envoyInsecureContainerPort,
				},
				{
					Name:       "http",
					PortNumber: envoySecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "duplicate port numbers",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       "http",
					PortNumber: envoyInsecureContainerPort,
				},
				{
					Name:       "https",
					PortNumber: envoyInsecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "only http port specified",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       "http",
					PortNumber: envoyInsecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "only https port specified",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       "https",
					PortNumber: envoySecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "empty ports",
			ports:       []operatorv1alpha1.ContainerPort{},
			expected:    false,
		},
	}

	name := "test-validation"
	cntr := &operatorv1alpha1.Contour{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fmt.Sprintf("%s-ns", name),
		},
		Spec: operatorv1alpha1.ContourSpec{
			Namespace: operatorv1alpha1.NamespaceSpec{Name: "projectcontour"},
			NetworkPublishing: operatorv1alpha1.NetworkPublishing{
				Envoy: operatorv1alpha1.EnvoyNetworkPublishing{
					Type: operatorv1alpha1.LoadBalancerServicePublishingType,
					ContainerPorts: []operatorv1alpha1.ContainerPort{
						{
							Name:       "http",
							PortNumber: int32(8080),
						},
						{
							Name:       "https",
							PortNumber: int32(8443),
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		if tc.ports != nil {
			cntr.Spec.NetworkPublishing.Envoy.ContainerPorts = tc.ports
		}
		err := validation.ContainerPorts(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestLoadBalancerIP(t *testing.T) {
	testCases := []struct {
		description string
		address     string
		expected    bool
	}{
		{
			description: "default load balancer type service without IP specified",
			expected:    true,
		},
		{
			description: "user-specified load balancer IPv4 address",
			address:     "1.2.3.4",
			expected:    true,
		},
		{
			description: "user-specified load balancer IPv6 address",
			address:     "2607:f0d0:1002:51::4",
			expected:    true,
		},
		{
			description: "invalid IPv4 address",
			address:     "1.2..4",
			expected:    false,
		},
		{
			description: "invalid IPv6 address",
			address:     "2607:f0d0:1002:51:::4",
			expected:    false,
		},
	}

	name := "test-validation"
	cntr := &operatorv1alpha1.Contour{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fmt.Sprintf("%s-ns", name),
		},
		Spec: operatorv1alpha1.ContourSpec{
			Namespace: operatorv1alpha1.NamespaceSpec{Name: "projectcontour"},
			NetworkPublishing: operatorv1alpha1.NetworkPublishing{
				Envoy: operatorv1alpha1.EnvoyNetworkPublishing{
					Type: operatorv1alpha1.LoadBalancerServicePublishingType,
					LoadBalancer: operatorv1alpha1.LoadBalancerStrategy{
						Scope: "External",
						ProviderParameters: operatorv1alpha1.ProviderLoadBalancerParameters{
							Type: "GCP",
							GCP:  &operatorv1alpha1.GCPLoadBalancerParameters{},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		if tc.address != "" {
			cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP.Address = &tc.address
		}

		err := validation.LoadBalancerAddress(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestLoadBalancerProvider(t *testing.T) {
	testCases := []struct {
		description        string
		provider           operatorv1alpha1.LoadBalancerProviderType
		additionalProvider operatorv1alpha1.LoadBalancerProviderType
		expected           bool
	}{
		{
			description: "default load balancer parameters",
			expected:    true,
		},
		{
			description:        "aws provider with azure provider parameters specified",
			provider:           "AWS",
			additionalProvider: "Azure",
			expected:           false,
		},
		{
			description:        "aws provider with gcp provider parameters specified",
			provider:           "AWS",
			additionalProvider: "GCP",
			expected:           false,
		},
		{
			description:        "azure provider with aws provider parameters specified",
			provider:           "Azure",
			additionalProvider: "AWS",
			expected:           false,
		},
		{
			description:        "azure provider with gcp provider parameters specified",
			provider:           "Azure",
			additionalProvider: "GCP",
			expected:           false,
		},
		{
			description:        "gcp provider with aws provider parameters specified",
			provider:           "GCP",
			additionalProvider: "AWS",
			expected:           false,
		},
		{
			description:        "gcp provider with azure provider parameters specified",
			provider:           "GCP",
			additionalProvider: "Azure",
			expected:           false,
		},
	}

	name := "test-validation"
	testString := "projectcontour"
	cntr := &operatorv1alpha1.Contour{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fmt.Sprintf("%s-ns", name),
		},
		Spec: operatorv1alpha1.ContourSpec{
			Namespace: operatorv1alpha1.NamespaceSpec{Name: "projectcontour"},
			NetworkPublishing: operatorv1alpha1.NetworkPublishing{
				Envoy: operatorv1alpha1.EnvoyNetworkPublishing{
					Type: operatorv1alpha1.LoadBalancerServicePublishingType,
					LoadBalancer: operatorv1alpha1.LoadBalancerStrategy{
						Scope: "External",
						ProviderParameters: operatorv1alpha1.ProviderLoadBalancerParameters{
							AWS:   &operatorv1alpha1.AWSLoadBalancerParameters{},
							Azure: &operatorv1alpha1.AzureLoadBalancerParameters{},
							GCP:   &operatorv1alpha1.GCPLoadBalancerParameters{},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		switch tc.provider {
		case "AWS":
			cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AWSLoadBalancerProvider
			switch tc.additionalProvider {
			case "Azure":
				cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure.Subnet = &testString
			case "GCP":
				cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP.Subnet = &testString
			}
		case "Azure":
			cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AzureLoadBalancerProvider
			switch tc.additionalProvider {
			case "AWS":
				cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.AWS.AllocationIDs = strings.Split(testString, "")
			case "GCP":
				cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.GCP.Subnet = &testString
			}
		case "GCP":
			cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.GCPLoadBalancerProvider
			switch tc.additionalProvider {
			case "AWS":
				cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.AWS.AllocationIDs = strings.Split(testString, "")
			case "Azure":
				cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Azure.Subnet = &testString
			}
		}
		err := validation.LoadBalancerProvider(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestNodePorts(t *testing.T) {
	httpPort := int32(30080)
	httpsPort := int32(30443)

	testCases := []struct {
		description string
		ports       []operatorv1alpha1.NodePort
		expected    bool
	}{
		{
			description: "default http and https nodeports",
			expected:    true,
		},
		{
			description: "user-specified http and https nodeports",
			ports: []operatorv1alpha1.NodePort{
				{
					Name:       "http",
					PortNumber: &httpPort,
				},
				{
					Name:       "https",
					PortNumber: &httpsPort,
				},
			},
			expected: true,
		},
		{
			description: "invalid port name",
			ports: []operatorv1alpha1.NodePort{
				{
					Name:       "http",
					PortNumber: &httpPort,
				},
				{
					Name:       "foo",
					PortNumber: &httpsPort,
				},
			},
			expected: false,
		},
		{
			description: "auto-assigned https port number",
			ports: []operatorv1alpha1.NodePort{
				{
					Name:       "http",
					PortNumber: &httpPort,
				},
				{
					Name: "https",
				},
			},
			expected: true,
		},
		{
			description: "auto-assigned http and https port numbers",
			ports: []operatorv1alpha1.NodePort{
				{
					Name: "http",
				},
				{
					Name: "https",
				},
			},
			expected: true,
		},
		{
			description: "duplicate nodeport names",
			ports: []operatorv1alpha1.NodePort{
				{
					Name:       "http",
					PortNumber: &httpPort,
				},
				{
					Name:       "http",
					PortNumber: &httpsPort,
				},
			},
			expected: false,
		},
		{
			description: "duplicate nodeport numbers",
			ports: []operatorv1alpha1.NodePort{
				{
					Name:       "http",
					PortNumber: &httpPort,
				},
				{
					Name:       "https",
					PortNumber: &httpPort,
				},
			},
			expected: false,
		},
	}

	name := "test-validation"
	cntr := &operatorv1alpha1.Contour{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fmt.Sprintf("%s-ns", name),
		},
		Spec: operatorv1alpha1.ContourSpec{
			Namespace: operatorv1alpha1.NamespaceSpec{Name: "projectcontour"},
			NetworkPublishing: operatorv1alpha1.NetworkPublishing{
				Envoy: operatorv1alpha1.EnvoyNetworkPublishing{
					Type: operatorv1alpha1.NodePortServicePublishingType,
					NodePorts: []operatorv1alpha1.NodePort{
						{
							Name:       "http",
							PortNumber: &httpPort,
						},
						{
							Name:       "https",
							PortNumber: &httpsPort,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		if tc.ports != nil {
			cntr.Spec.NetworkPublishing.Envoy.NodePorts = tc.ports
		}
		err := validation.NodePorts(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestGatewayClass(t *testing.T) {

	testCases := map[string]struct {
		gc       *gatewayv1alpha1.GatewayClass
		expected bool
	}{
		"happy path": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Scope:     pointer.StringPtr("Namespace"),
						Group:     "operator.projectcontour.io",
						Kind:      "Contour",
						Name:      "a-contour",
						Namespace: pointer.StringPtr("a-namespace"),
					},
				},
			},
			expected: true,
		},
		"missing scope": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Scope:     nil,
						Group:     "operator.projectcontour.io",
						Kind:      "Contour",
						Name:      "a-contour",
						Namespace: pointer.StringPtr("a-namespace"),
					},
				},
			},
			expected: false,
		},
		"invalid scope": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Scope:     pointer.StringPtr("Cluster"),
						Group:     "operator.projectcontour.io",
						Kind:      "Contour",
						Name:      "a-contour",
						Namespace: pointer.StringPtr("a-namespace"),
					},
				},
			},
			expected: false,
		},
		"invalid group": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Scope:     pointer.StringPtr("Namespace"),
						Group:     "operator.not-projectcontour.io",
						Kind:      "Contour",
						Name:      "a-contour",
						Namespace: pointer.StringPtr("a-namespace"),
					},
				},
			},
			expected: false,
		},
		"invalid kind": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Scope:     pointer.StringPtr("Namespace"),
						Group:     "operator.projectcontour.io",
						Kind:      "NotContour",
						Name:      "a-contour",
						Namespace: pointer.StringPtr("a-namespace"),
					},
				},
			},
			expected: false,
		},
		"missing namespace": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Scope:     pointer.StringPtr("Namespace"),
						Group:     "operator.projectcontour.io",
						Kind:      "Contour",
						Name:      "a-contour",
						Namespace: nil,
					},
				},
			},
			expected: false,
		},
		"missing parameters ref": {
			gc: &gatewayv1alpha1.GatewayClass{
				Spec: gatewayv1alpha1.GatewayClassSpec{
					ParametersRef: nil,
				},
			},
			expected: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validation.GatewayClass(tc.gc)
			if tc.expected && err != nil {
				t.Fatal("expected gateway class to be valid")
			}
			if !tc.expected && err == nil {
				t.Fatal("expected gateway class to be invalid")
			}
		})
	}
}

func TestGateway(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gateway",
		},
	}

	// Hostnames used by test cases.
	empty := gatewayv1alpha1.Hostname("")
	wildcard := gatewayv1alpha1.Hostname("*")
	subWildcard := gatewayv1alpha1.Hostname("*.foo.com")
	invalidSubomain := gatewayv1alpha1.Hostname(".com.")
	ip := gatewayv1alpha1.Hostname("1.2.3.4")
	invalidWildcard := gatewayv1alpha1.Hostname("foo.*.com")

	// Address types used by test cases.
	ipType := gatewayv1alpha1.IPAddressType
	namedType := gatewayv1alpha1.NamedAddressType

	testCases := map[string]struct {
		contour  *operatorv1alpha1.Contour
		gc       *gatewayv1alpha1.GatewayClass
		gateway  *gatewayv1alpha1.Gateway
		expected bool
	}{
		"valid nil host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-nil-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("valid-nil-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-nil-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "valid-nil-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-nil-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "valid-nil-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"valid zero value host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-zero-value-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("valid-zero-value-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-zero-value-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "valid-zero-value-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-zero-value-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "valid-zero-value-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &empty,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"valid wildcard host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-wildcard-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("valid-wildcard-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-wildcard-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "valid-wildcard-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-wildcard-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "valid-wildcard-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &wildcard,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"valid wildcard subdomain host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-wildcard-subdomain-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("valid-wildcard-subdomain-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-wildcard-subdomain-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "valid-wildcard-subdomain-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-wildcard-subdomain-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "valid-wildcard-subdomain-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &subWildcard,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"invalid subdomain host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-subdomain-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-subdomain-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-subdomain-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-subdomain-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-subdomain-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-subdomain-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &invalidSubomain,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid ip host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-ip-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-ip-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-ip-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-ip-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-ip-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-ip-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &ip,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid wildcard host gateway": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-wildcard-host-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-wildcard-host-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-wildcard-host-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-wildcard-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-wildcard-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-wildcard-host-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &invalidWildcard,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid listener, tls protocol without config": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-listener-protocol-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-listener-protocol-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-listener-protocol-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-wildcard-host-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-listener-protocol-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-listener-protocol-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.TLSProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid gatewayclass reference": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gatewayclass-reference-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-gatewayclass-reference-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-gatewayclass-reference-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-gatewayclass-reference-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gatewayclass-reference-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "nonexistent",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid gatewayclass no parameters ref": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gatewayclass-no-parameters-ref-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-gatewayclass-no-parameters-ref-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-gatewayclass-no-parameters-ref-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gatewayclass-no-parameters-ref-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-gatewayclass-no-parameters-ref-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid contour spec ns": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-contour-spec-ns-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: "not-gw-ns",
					},
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-contour-spec-ns-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-contour-spec-ns-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-contour-spec-ns-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-contour-spec-ns-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"valid gateway ip address": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-gateway-ip-address-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("valid-gateway-ip-address-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-gateway-ip-address-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "valid-gateway-ip-address-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-gateway-ip-address-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "valid-gateway-ip-address-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
					Addresses: []gatewayv1alpha1.GatewayAddress{
						{
							Type:  &ipType,
							Value: "1.2.3.4",
						},
					},
				},
			},
			expected: true,
		},
		"invalid gateway ip address": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gateway-ip-address-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-gateway-ip-address-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-gateway-ip-address-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-gateway-ip-address-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gateway-ip-address-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-gateway-ip-address-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
					Addresses: []gatewayv1alpha1.GatewayAddress{
						{
							Type:  &ipType,
							Value: "1.2..3.4",
						},
					},
				},
			},
			expected: false,
		},
		"invalid gateway address type": {
			contour: &operatorv1alpha1.Contour{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gateway-address-type-contour",
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name: ns.Name,
					},
					GatewayClassRef: pointer.StringPtr("invalid-gateway-address-type-gc"),
				},
			},
			gc: &gatewayv1alpha1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-gateway-address-type-gc",
				},
				Spec: gatewayv1alpha1.GatewayClassSpec{
					Controller: operatorv1alpha1.GatewayClassControllerRef,
					ParametersRef: &gatewayv1alpha1.ParametersReference{
						Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
						Kind:      "Contour",
						Name:      "invalid-gateway-address-type-contour",
						Scope:     pointer.StringPtr("Namespace"),
						Namespace: pointer.StringPtr(ns.Name),
					},
				},
			},
			gateway: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gateway-address-type-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "invalid-gateway-address-type-gc",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
					Addresses: []gatewayv1alpha1.GatewayAddress{
						{
							Type:  &namedType,
							Value: "unsupported",
						},
					},
				},
			},
			expected: false,
		},
	}

	// Create the client
	builder := fake.NewClientBuilder()
	builder.WithScheme(operator.GetOperatorScheme())
	cl := builder.Build()
	// Create dependent resources of a gateway.
	if err := cl.Create(context.TODO(), ns); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if err := cl.Create(context.TODO(), tc.contour); err != nil {
				t.Fatalf("failed to create contour: %v", err)
			}

			if err := cl.Create(context.TODO(), tc.gc); err != nil {
				t.Fatalf("failed to create gatewayclass: %v", err)
			}

			if err := cl.Create(context.TODO(), tc.gateway); err != nil {
				t.Fatalf("failed to create gateway: %v", err)
			}

			cntr, err := validation.Gateway(context.TODO(), cl, tc.gateway)
			switch {
			case tc.expected && err != nil:
				t.Fatalf("expected gateway to be valid: %v", err)
			case !tc.expected && err == nil:
				t.Fatal("expected gateway to be invalid")
			default:
				if reflect.DeepEqual(cntr, tc.contour) {
					t.Fatalf("expected %v; got %v", tc.contour, cntr)
				}
			}
		})
	}
}
