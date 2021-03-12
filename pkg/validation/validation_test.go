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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	envoyInsecureContainerPort = int32(8080)
	envoySecureContainerPort   = int32(8443)
)

func TestValidContainerPorts(t *testing.T) {
	testCases := []struct {
		description string
		ports       []operatorv1alpha1.ContainerPort
		expected    bool
	}{
		{
			description: "default http and https ports",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: int32(8080),
				},
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: int32(8443),
				},
			},
			expected: true,
		},
		{
			description: "non-default http and https ports",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: int32(8081),
				},
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: int32(8444),
				},
			},
			expected: true,
		},
		{
			description: "duplicate port names",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoyInsecureContainerPort,
				},
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoySecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "duplicate port numbers",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoyInsecureContainerPort,
				},
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: envoyInsecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "only http port specified",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoyInsecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "only https port specified",
			ports: []operatorv1alpha1.ContainerPort{
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
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
							Name:       operatorv1alpha1.PortNameHTTP,
							PortNumber: int32(8080),
						},
						{
							Name:       operatorv1alpha1.PortNameHTTPS,
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
		err := containerPorts(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}

func TestValidServicePorts(t *testing.T) {
	testCases := []struct {
		description string
		ports       []operatorv1alpha1.ServicePort
		expected    bool
	}{
		{
			description: "default http and https ports",
			ports: []operatorv1alpha1.ServicePort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: int32(8080),
				},
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: int32(8443),
				},
			},
			expected: true,
		},
		{
			description: "non-default http and https ports",
			ports: []operatorv1alpha1.ServicePort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: int32(8081),
				},
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: int32(8444),
				},
			},
			expected: true,
		},
		{
			description: "duplicate port names",
			ports: []operatorv1alpha1.ServicePort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoyInsecureContainerPort,
				},
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoySecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "duplicate port numbers",
			ports: []operatorv1alpha1.ServicePort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoyInsecureContainerPort,
				},
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: envoyInsecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "only http port specified",
			ports: []operatorv1alpha1.ServicePort{
				{
					Name:       operatorv1alpha1.PortNameHTTP,
					PortNumber: envoyInsecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "only https port specified",
			ports: []operatorv1alpha1.ServicePort{
				{
					Name:       operatorv1alpha1.PortNameHTTPS,
					PortNumber: envoySecureContainerPort,
				},
			},
			expected: false,
		},
		{
			description: "empty ports",
			ports:       []operatorv1alpha1.ServicePort{},
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
					ServicePorts: []operatorv1alpha1.ServicePort{
						{
							Name:       operatorv1alpha1.PortNameHTTP,
							PortNumber: int32(80),
						},
						{
							Name:       operatorv1alpha1.PortNameHTTPS,
							PortNumber: int32(443),
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		if tc.ports != nil {
			cntr.Spec.NetworkPublishing.Envoy.ServicePorts = tc.ports
		}
		err := servicePorts(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}
