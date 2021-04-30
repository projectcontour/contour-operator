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
		err := containerPorts(cntr)
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
		err := nodePorts(cntr)
		if err != nil && tc.expected {
			t.Fatalf("%q: failed with error: %#v", tc.description, err)
		}
		if err == nil && !tc.expected {
			t.Fatalf("%q: expected to fail but received no error", tc.description)
		}
	}
}
