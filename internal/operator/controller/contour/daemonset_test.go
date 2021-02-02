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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func checkDaemonSetHasEnvVar(t *testing.T, ds *appsv1.DaemonSet, container, name string) {
	t.Helper()

	if container == envoyInitContainerName {
		for i, c := range ds.Spec.Template.Spec.InitContainers {
			if c.Name == container {
				for _, envVar := range ds.Spec.Template.Spec.InitContainers[i].Env {
					if envVar.Name == name {
						return
					}
				}
			}
		}
	} else {
		for i, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == container {
				for _, envVar := range ds.Spec.Template.Spec.Containers[i].Env {
					if envVar.Name == name {
						return
					}
				}
			}
		}
	}

	t.Errorf("daemonset is missing environment variable %q", name)
}

func checkDaemonSetHasContainer(t *testing.T, ds *appsv1.DaemonSet, name string, expect bool) *corev1.Container {
	t.Helper()

	if ds.Spec.Template.Spec.Containers == nil {
		t.Error("daemonset has no containers")
	}

	if name == envoyInitContainerName {
		for _, container := range ds.Spec.Template.Spec.InitContainers {
			if container.Name == name {
				if expect {
					return &container
				}
				t.Errorf("daemonset has unexpected %q init container", name)
			}
		}
	} else {
		for _, container := range ds.Spec.Template.Spec.Containers {
			if container.Name == name {
				if expect {
					return &container
				}
				t.Errorf("daemonset has unexpected %q container", name)
			}
		}
	}
	if expect {
		t.Errorf("daemonset has no %q container", name)
	}
	return nil
}

func checkDaemonSetHasLabels(t *testing.T, ds *appsv1.DaemonSet, expected map[string]string) {
	t.Helper()

	if apiequality.Semantic.DeepEqual(ds.Labels, expected) {
		return
	}

	t.Errorf("daemonset has unexpected %q labels", ds.Labels)
}

func checkContainerHasPort(t *testing.T, ds *appsv1.DaemonSet, port int32) {
	t.Helper()

	for _, c := range ds.Spec.Template.Spec.Containers {
		for _, p := range c.Ports {
			if p.ContainerPort == port {
				return
			}
		}
	}
	t.Errorf("container is missing containerPort %q", port)
}

func TestDesiredDaemonSet(t *testing.T) {
	lb := operatorv1alpha1.LoadBalancerServicePublishingType
	ctr := objutil.NewContour(testContourName, testContourNs, testContourSpecNs, false, lb)
	ds := DesiredDaemonSet(ctr, testContourImage, testEnvoyImage)

	container := checkDaemonSetHasContainer(t, ds, EnvoyContainerName, true)
	checkContainerHasImage(t, container, testEnvoyImage)
	container = checkDaemonSetHasContainer(t, ds, ShutdownContainerName, true)
	checkContainerHasImage(t, container, testContourImage)
	container = checkDaemonSetHasContainer(t, ds, envoyInitContainerName, true)
	checkContainerHasImage(t, container, testContourImage)
	checkDaemonSetHasEnvVar(t, ds, EnvoyContainerName, envoyNsEnvVar)
	checkDaemonSetHasEnvVar(t, ds, EnvoyContainerName, envoyPodEnvVar)
	checkDaemonSetHasEnvVar(t, ds, envoyInitContainerName, envoyNsEnvVar)
	checkDaemonSetHasLabels(t, ds, ds.Labels)
	for _, port := range cntr.Spec.NetworkPublishing.Envoy.ContainerPorts {
		checkContainerHasPort(t, ds, port.PortNumber)
	}
}
