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

package contour

import (
	"testing"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objutil "github.com/projectcontour/contour-operator/internal/object"
	"github.com/projectcontour/contour-operator/internal/operator/config"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func checkServiceHasPort(t *testing.T, svc *corev1.Service, port int32) {
	t.Helper()

	for _, p := range svc.Spec.Ports {
		if p.Port == port {
			return
		}
	}
	t.Errorf("service is missing port %q", port)
}

func checkServiceHasNodeport(t *testing.T, svc *corev1.Service, port int32) {
	t.Helper()

	for _, p := range svc.Spec.Ports {
		if p.NodePort == port {
			return
		}
	}
	t.Errorf("service is missing nodeport %q", port)
}

func checkServiceHasTargetPort(t *testing.T, svc *corev1.Service, port int32) {
	t.Helper()

	intStrPort := intstr.IntOrString{IntVal: port}
	for _, p := range svc.Spec.Ports {
		if p.TargetPort == intStrPort {
			return
		}
	}
	t.Errorf("service is missing targetPort %d", port)
}

func checkServiceHasPortName(t *testing.T, svc *corev1.Service, name string) {
	t.Helper()

	for _, p := range svc.Spec.Ports {
		if p.Name == name {
			return
		}
	}
	t.Errorf("service is missing port name %q", name)
}

func checkServiceHasPortProtocol(t *testing.T, svc *corev1.Service, protocol corev1.Protocol) {
	t.Helper()

	for _, p := range svc.Spec.Ports {
		if p.Protocol == protocol {
			return
		}
	}
	t.Errorf("service is missing port protocol %q", protocol)
}

func checkServiceHasAnnotation(t *testing.T, svc *corev1.Service, key string) {
	t.Helper()

	if svc.Annotations == nil {
		t.Errorf("service is missing annotations")
	}
	for k := range svc.Annotations {
		if k == key {
			return
		}
	}

	t.Errorf("service is missing annotation %q", key)
}

func TestDesiredContourService(t *testing.T) {
	lb := operatorv1alpha1.LoadBalancerServicePublishingType
	ctr := objutil.NewContour(testContourName, testContourNs, testContourSpecNs, true, lb)

	svc := DesiredContourService(ctr)

	checkServiceHasPort(t, svc, xdsPort)
	checkServiceHasTargetPort(t, svc, xdsPort)
	checkServiceHasPortName(t, svc, "xds")
	checkServiceHasPortProtocol(t, svc, corev1.ProtocolTCP)
}

func TestDesiredEnvoyService(t *testing.T) {
	nodePort := operatorv1alpha1.NodePortServicePublishingType
	ctr := objutil.NewContour(testContourName, testContourNs, testContourSpecNs, false, nodePort)
	svc := DesiredEnvoyService(ctr)

	checkServiceHasPort(t, svc, config.EnvoyServiceHTTPPort)
	checkServiceHasPort(t, svc, config.EnvoyServiceHTTPSPort)
	checkServiceHasNodeport(t, svc, config.EnvoyNodePortHTTPPort)
	checkServiceHasNodeport(t, svc, config.EnvoyNodePortHTTPSPort)
	for _, port := range ctr.Spec.NetworkPublishing.Envoy.ContainerPorts {
		checkServiceHasTargetPort(t, svc, port.PortNumber)
	}
	checkServiceHasPortName(t, svc, "http")
	checkServiceHasPortName(t, svc, "https")
	checkServiceHasPortProtocol(t, svc, corev1.ProtocolTCP)
	// Check LB annotations for the different provider types.
	ctr.Spec.NetworkPublishing.Envoy.Type = operatorv1alpha1.LoadBalancerServicePublishingType
	ctr.Spec.NetworkPublishing.Envoy.LoadBalancer.Scope = operatorv1alpha1.ExternalLoadBalancer
	ctr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AWSLoadBalancerProvider
	svc = DesiredEnvoyService(ctr)
	checkServiceHasAnnotation(t, svc, awsLbBackendProtoAnnotation)
	ctr.Spec.NetworkPublishing.Envoy.LoadBalancer.Scope = operatorv1alpha1.InternalLoadBalancer
	svc = DesiredEnvoyService(ctr)
	checkServiceHasAnnotation(t, svc, awsInternalLBAnnotation)
	ctr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AzureLoadBalancerProvider
	svc = DesiredEnvoyService(ctr)
	checkServiceHasAnnotation(t, svc, azureInternalLBAnnotation)
	ctr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.GCPLoadBalancerProvider
	svc = DesiredEnvoyService(ctr)
	checkServiceHasAnnotation(t, svc, gcpLBTypeAnnotation)
}
