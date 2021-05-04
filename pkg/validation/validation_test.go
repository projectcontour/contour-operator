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
	"testing"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
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
	// Initialize the gateway dependent resources.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "projectcontour",
		},
	}

	gcName := "test-gc"

	cfg := objcontour.Config{
		Name:         "test",
		Namespace:    "projectcontour",
		SpecNs:       ns.Name,
		RemoveNs:     true,
		Replicas:     1,
		NetworkType:  operatorv1alpha1.ClusterIPServicePublishingType,
		NodePorts:    nil,
		GatewayClass: &gcName,
	}

	cntr := objcontour.New(cfg)

	gc := &gatewayv1alpha1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: gcName,
		},
		Spec: gatewayv1alpha1.GatewayClassSpec{
			Controller: operatorv1alpha1.GatewayClassControllerRef,
			ParametersRef: &gatewayv1alpha1.ParametersReference{
				Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
				Kind:      "Contour",
				Name:      cntr.Name,
				Scope:     pointer.StringPtr("Namespace"),
				Namespace: pointer.StringPtr(cntr.Namespace),
			},
		},
		Status: gatewayv1alpha1.GatewayClassStatus{},
	}

	// Hostnames used by test cases.
	wildcard := gatewayv1alpha1.Hostname("*")
	subWildcard := gatewayv1alpha1.Hostname("*.foo.com")
	invalidWildcard := gatewayv1alpha1.Hostname("foo.*.com")
	empty := gatewayv1alpha1.Hostname("")
	ip := gatewayv1alpha1.Hostname("1.2.3.4")

	// Address types used by test cases.
	ipType := gatewayv1alpha1.IPAddressType
	namedType := gatewayv1alpha1.NamedAddressType

	testCases := map[string]struct {
		gw       *gatewayv1alpha1.Gateway
		expected bool
	}{
		"valid nil host gateway": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-nil-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"valid zero value host gateway": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-zero-value-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &empty,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"valid wildcard host gateway": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-wildcard-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &wildcard,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"valid wildcard subdomain host gateway": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-wildcard-subdomain-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &subWildcard,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: true,
		},
		"invalid ip host gateway": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-ip-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &ip,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid wildcard host gateway": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-wildcard-host-gateway",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Hostname: &invalidWildcard,
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid gatewayclass reference": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gatewayclass-reference",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: "nonexistent",
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid number of gateway listeners": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-number-of-gateway-listeners",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"duplicate listener port numbers": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "duplicate-listener-port-numbers",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"invalid listener protocol": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-listener-protocol",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.TLSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
				},
			},
			expected: false,
		},
		"valid gateway ip address": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "valid-gateway-ip-address",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
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
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gateway-ip-address",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
							Protocol: gatewayv1alpha1.HTTPProtocolType,
						},
					},
					Addresses: []gatewayv1alpha1.GatewayAddress{
						{
							Type:  &ipType,
							Value: "1..2.3.4",
						},
					},
				},
			},
			expected: false,
		},
		"invalid gateway address type": {
			gw: &gatewayv1alpha1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "invalid-gateway-address-type",
				},
				Spec: gatewayv1alpha1.GatewaySpec{
					GatewayClassName: gc.Name,
					Listeners: []gatewayv1alpha1.Listener{
						{
							Port:     gatewayv1alpha1.PortNumber(int32(1)),
							Protocol: gatewayv1alpha1.HTTPSProtocolType,
						},
						{
							Port:     gatewayv1alpha1.PortNumber(int32(2)),
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

	// Create gateway dependent resources
	if err := cl.Create(context.TODO(), ns); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	if err := cl.Create(context.TODO(), cntr); err != nil {
		t.Fatalf("failed to create contour: %v", err)
	}
	if err := cl.Create(context.TODO(), gc); err != nil {
		t.Fatalf("failed to create gatewayclass: %v", err)
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if err := cl.Create(context.TODO(), tc.gw); err != nil {
				t.Fatalf("failed to create gateway: %v", err)
			}
			_, err := validation.Gateway(context.TODO(), cl, tc.gw)
			if tc.expected && err != nil {
				t.Fatalf("expected gateway to be valid: %v", err)
			}
			if !tc.expected && err == nil {
				t.Fatal("expected gateway to be invalid")
			}
		})
	}
}
