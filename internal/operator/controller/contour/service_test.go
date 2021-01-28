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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	testName = "test-svc"
	testNs   = testName + "-ns"
)

func checkServiceHasPort(t *testing.T, svc *corev1.Service, port int) {
	t.Helper()

	for _, p := range svc.Spec.Ports {
		if p.Port == int32(port) {
			return
		}
	}
	t.Errorf("service is missing port %q", port)
}

func checkServiceHasTargetPort(t *testing.T, svc *corev1.Service, port int) {
	t.Helper()

	intStrPort := intstr.IntOrString{IntVal: int32(port)}
	for _, p := range svc.Spec.Ports {
		if p.TargetPort == intStrPort {
			return
		}
	}
	t.Errorf("service is missing targetPort %q", port)
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

func TestDesiredContourService(t *testing.T) {
	ctr := &operatorv1alpha1.Contour{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNs,
		},
	}

	svc := DesiredContourService(ctr)

	checkServiceHasPort(t, svc, xdsPort)
	checkServiceHasTargetPort(t, svc, xdsPort)
	checkServiceHasPortName(t, svc, "xds")
	checkServiceHasPortProtocol(t, svc, corev1.ProtocolTCP)
}

func TestDesiredEnvoyService(t *testing.T) {
	ctr := &operatorv1alpha1.Contour{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNs,
		},
	}

	svc := DesiredEnvoyService(ctr)

	checkServiceHasPort(t, svc, httpPort)
	checkServiceHasPort(t, svc, httpsPort)
	checkServiceHasTargetPort(t, svc, httpPort)
	checkServiceHasTargetPort(t, svc, httpsPort)
	checkServiceHasPortName(t, svc, "http")
	checkServiceHasPortName(t, svc, "https")
	checkServiceHasPortProtocol(t, svc, corev1.ProtocolTCP)
}
