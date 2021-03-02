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
	"testing"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/pkg/slice"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	envoyInsecureContainerPort = int32(8080)
	envoySecureContainerPort   = int32(8443)
)

func TestValidContour(t *testing.T) {
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
		err := Contour(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestValidGatewayListeners(t *testing.T) {
	testCases := []struct {
		description string
		mutate      func(gw *gatewayv1alpha1.Gateway)
		expected    bool
	}{
		{
			description: "default settings",
			mutate:      func(_ *gatewayv1alpha1.Gateway) {},
			expected:    true,
		},
		{
			description: "empty hostname",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Hostname = nil
				gw.Spec.Listeners[1].Hostname = nil
			},
			expected: true,
		},
		{
			description: "wildcard as hostname",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				host := gatewayv1alpha1.Hostname("*")
				gw.Spec.Listeners[0].Hostname = &host
				gw.Spec.Listeners[1].Hostname = &host
			},
			expected: true,
		},
		{
			description: "wildcard in hostname",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				host := gatewayv1alpha1.Hostname("*.projectcontour.io")
				gw.Spec.Listeners[0].Hostname = &host
				gw.Spec.Listeners[1].Hostname = &host
			},
			expected: true,
		},
		{
			description: "empty string as hostname",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				host := gatewayv1alpha1.Hostname("")
				gw.Spec.Listeners[0].Hostname = &host
				gw.Spec.Listeners[1].Hostname = &host
			},
			expected: true,
		},
		{
			description: "IP address for hostname",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				host := gatewayv1alpha1.Hostname("1.2.3.4")
				gw.Spec.Listeners[0].Hostname = &host
			},
			expected: false,
		},
		{
			description: "one listener",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				l := slice.RemoveGatewayListener(gw.Spec.Listeners, gw.Spec.Listeners[0])
				gw.Spec.Listeners = l
			},
			expected: false,
		},
		{
			description: "three listeners",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				l := gatewayv1alpha1.Listener{
					Hostname: gw.Spec.Listeners[0].Hostname,
					Port:     gatewayv1alpha1.PortNumber(int32(8443)),
					Protocol: gw.Spec.Listeners[0].Protocol,
					TLS:      gw.Spec.Listeners[0].TLS,
					Routes:   gw.Spec.Listeners[0].Routes,
				}
				gw.Spec.Listeners = append(gw.Spec.Listeners, l)
			},
			expected: false,
		},
		{
			description: "identical listener ports",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Port = gw.Spec.Listeners[1].Port
			},
			expected: false,
		},
		{
			description: "udp listener",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Protocol = gatewayv1alpha1.UDPProtocolType
			},
			expected: false,
		},
		{
			description: "tcp listener",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Protocol = gatewayv1alpha1.TCPProtocolType
			},
			expected: false,
		},
		{
			description: "invalid group",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Routes.Group = "unsupported.x-k8s.io"
			},
			expected: false,
		},
		{
			description: "invalid kind",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Routes.Kind = "UnsupportedKind"
			},
			expected: false,
		},
		{
			description: "invalid route kind for http listener",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Protocol = gatewayv1alpha1.TLSProtocolType
			},
			expected: false,
		},
		{
			description: "invalid route kind for http listener",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Routes.Kind = routeKindTLS
			},
			expected: false,
		},
		{
			description: "invalid route kind for https listener",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Listeners[0].Protocol = gatewayv1alpha1.HTTPSProtocolType
				gw.Spec.Listeners[0].Routes.Kind = routeKindTLS
			},
			expected: false,
		},
	}

	hostname := gatewayv1alpha1.Hostname("test.local.host")
	listeners := []gatewayv1alpha1.Listener{
		{
			Hostname: &hostname,
			Port:     gatewayv1alpha1.PortNumber(int32(80)),
			Protocol: gatewayv1alpha1.HTTPProtocolType,
			Routes: gatewayv1alpha1.RouteBindingSelector{
				Namespaces: gatewayv1alpha1.RouteNamespaces{
					From: gatewayv1alpha1.RouteSelectSame,
				},
				Selector: metav1.LabelSelector{},
				Group:    gatewayv1alpha1.GroupName,
				Kind:     routeKindHTTP,
			},
		},
		{
			Hostname: &hostname,
			Port:     gatewayv1alpha1.PortNumber(int32(443)),
			Protocol: gatewayv1alpha1.HTTPSProtocolType,
			Routes: gatewayv1alpha1.RouteBindingSelector{
				Namespaces: gatewayv1alpha1.RouteNamespaces{
					From: gatewayv1alpha1.RouteSelectSame,
				},
				Selector: metav1.LabelSelector{},
				Group:    gatewayv1alpha1.GroupName,
				Kind:     routeKindHTTP,
			},
		},
	}
	gwName := "test-gateway-listeners"
	gcName := "test-gatewayclass"
	gw := &gatewayv1alpha1.Gateway{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: fmt.Sprintf("%s-ns", gwName),
		},
		Spec: gatewayv1alpha1.GatewaySpec{
			GatewayClassName: gcName,
			Listeners:        listeners,
		},
	}

	for _, tc := range testCases {
		copy := gw.DeepCopy()
		tc.mutate(copy)
		err := gatewayListeners(copy)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestValidGatewayAddresses(t *testing.T) {
	testCases := []struct {
		description string
		mutate      func(gw *gatewayv1alpha1.Gateway)
		expected    bool
	}{
		{
			description: "one valid IPv4 address",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Addresses = []gatewayv1alpha1.GatewayAddress{
					{
						Type:  gatewayv1alpha1.IPAddressType,
						Value: "1.2.3.4",
					},
				}
			},
			expected: true,
		},
		{
			description: "invalid address type",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Addresses = []gatewayv1alpha1.GatewayAddress{
					{
						Type:  gatewayv1alpha1.NamedAddressType,
						Value: "my-named-address",
					},
				}
			},
			expected: false,
		},
		{
			description: "one invalid IPv4 address",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Addresses = []gatewayv1alpha1.GatewayAddress{
					{
						Type:  gatewayv1alpha1.IPAddressType,
						Value: "1.2.3..4",
					},
				}
			},
			expected: false,
		},
		{
			description: "two valid IPv4 addresses",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Addresses = []gatewayv1alpha1.GatewayAddress{
					{
						Type:  gatewayv1alpha1.IPAddressType,
						Value: "1.2.3.4",
					},
					{
						Type:  gatewayv1alpha1.IPAddressType,
						Value: "5.6.7.8",
					},
				}
			},
			expected: true,
		},
		{
			description: "mix of valid and invalid IPv4 addresses",
			mutate: func(gw *gatewayv1alpha1.Gateway) {
				gw.Spec.Addresses = []gatewayv1alpha1.GatewayAddress{
					{
						Type:  gatewayv1alpha1.IPAddressType,
						Value: "1.2.3.4",
					},
					{
						Type:  gatewayv1alpha1.IPAddressType,
						Value: "5.6.7.*",
					},
				}
			},
			expected: false,
		},
	}

	gwName := "test-gateway-addresses"
	gw := &gatewayv1alpha1.Gateway{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: fmt.Sprintf("%s-ns", gwName),
		},
	}

	for _, tc := range testCases {
		copy := gw.DeepCopy()
		tc.mutate(copy)
		err := gatewayAddresses(copy)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}
