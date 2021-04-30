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

package service

import (
	"fmt"
	"testing"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objcfg "github.com/projectcontour/contour-operator/internal/objects/sharedconfig"

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

func checkServiceHasType(t *testing.T, svc *corev1.Service, svcType corev1.ServiceType) {
	t.Helper()

	if svc.Spec.Type != svcType {
		t.Errorf("service is missing type %s", svcType)
	}
}

func checkServiceHasExternalTrafficPolicy(t *testing.T, svc *corev1.Service, policy corev1.ServiceExternalTrafficPolicyType) {
	t.Helper()

	if svc.Spec.ExternalTrafficPolicy != policy {
		t.Errorf("service is missing external traffic policy type %s", policy)
	}
}

func TestDesiredContourService(t *testing.T) {
	name := "svc-test"
	cfg := objcontour.Config{
		Name:        name,
		Namespace:   fmt.Sprintf("%s-ns", name),
		SpecNs:      "projectcontour",
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.LoadBalancerServicePublishingType,
	}
	cntr := objcontour.New(cfg)
	svc := DesiredContourService(cntr)
	xdsPort := objcfg.XDSPort
	checkServiceHasPort(t, svc, xdsPort)
	checkServiceHasTargetPort(t, svc, xdsPort)
	checkServiceHasPortName(t, svc, "xds")
	checkServiceHasPortProtocol(t, svc, corev1.ProtocolTCP)
}

func TestDesiredEnvoyService(t *testing.T) {
	name := "svc-test"
	cfg := objcontour.Config{
		Name:        name,
		Namespace:   fmt.Sprintf("%s-ns", name),
		SpecNs:      "projectcontour",
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.NodePortServicePublishingType,
	}
	cntr := objcontour.New(cfg)
	svc := DesiredEnvoyService(cntr)
	checkServiceHasType(t, svc, corev1.ServiceTypeNodePort)
	checkServiceHasExternalTrafficPolicy(t, svc, corev1.ServiceExternalTrafficPolicyTypeLocal)
	checkServiceHasPort(t, svc, EnvoyServiceHTTPPort)
	checkServiceHasPort(t, svc, EnvoyServiceHTTPSPort)
	checkServiceHasNodeport(t, svc, EnvoyNodePortHTTPPort)
	checkServiceHasNodeport(t, svc, EnvoyNodePortHTTPSPort)
	for _, port := range cntr.Spec.NetworkPublishing.Envoy.ContainerPorts {
		checkServiceHasTargetPort(t, svc, port.PortNumber)
	}
	checkServiceHasPortName(t, svc, "http")
	checkServiceHasPortName(t, svc, "https")
	checkServiceHasPortProtocol(t, svc, corev1.ProtocolTCP)
	// Check LB annotations for the different provider types.
	cntr.Spec.NetworkPublishing.Envoy.Type = operatorv1alpha1.LoadBalancerServicePublishingType
	cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.Scope = operatorv1alpha1.ExternalLoadBalancer
	cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AWSLoadBalancerProvider
	svc = DesiredContourService(cntr)
	checkServiceHasType(t, svc, corev1.ServiceTypeLoadBalancer)
	checkServiceHasExternalTrafficPolicy(t, svc, corev1.ServiceExternalTrafficPolicyTypeLocal)
	checkServiceHasAnnotation(t, svc, awsLbBackendProtoAnnotation)
	cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.Scope = operatorv1alpha1.InternalLoadBalancer
	svc = DesiredEnvoyService(cntr)
	checkServiceHasAnnotation(t, svc, awsInternalLBAnnotation)
	cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AzureLoadBalancerProvider
	svc = DesiredEnvoyService(cntr)
	checkServiceHasAnnotation(t, svc, azureInternalLBAnnotation)
	cntr.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.GCPLoadBalancerProvider
	svc = DesiredEnvoyService(cntr)
	checkServiceHasAnnotation(t, svc, gcpLBTypeAnnotation)
	// Set network publishing type to ClusterIPService and verify the service type is as expected.
	cntr.Spec.NetworkPublishing.Envoy.Type = operatorv1alpha1.ClusterIPServicePublishingType
	svc = DesiredEnvoyService(cntr)
	checkServiceHasType(t, svc, corev1.ServiceTypeClusterIP)
}
