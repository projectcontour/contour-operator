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

package util

import (
	"strings"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NewClusterRole makes a ClusterRole object using the provided name
// for the object's name.
func NewClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind: "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// NewClusterRoleBinding makes a ClusterRoleBinding object using
// the provided name for the object's name.
func NewClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind: "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// NewRole makes a Role object using the provided ns/name for
// the object's namespace and name.
func NewRole(ns, name string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind: "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// NewRoleBinding makes a RoleBinding object using the provided
// ns/name for the object's namespace and name.
func NewRoleBinding(ns, name string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind: "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// NewServiceAccount makes a ServiceAccount object using the
// provided ns/name for the object's namespace and name.
func NewServiceAccount(ns, name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind: rbacv1.ServiceAccountKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// NewUnprivilegedPodSecurity makes a a non-root PodSecurityContext object
// using 65534 as the user and group ID.
func NewUnprivilegedPodSecurity() *corev1.PodSecurityContext {
	user := int64(65534)
	group := int64(65534)
	nonRoot := true
	return &corev1.PodSecurityContext{
		RunAsUser:    &user,
		RunAsGroup:   &group,
		RunAsNonRoot: &nonRoot,
	}
}

// NewNamespace makes a Namespace object using the provided name
// for the object's name.
func NewNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// NewContour makes a Contour object using the provided ns/name
// for the object's namespace and name.
func NewContour(name, ns string) *operatorv1alpha1.Contour {
	return &operatorv1alpha1.Contour{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

// NewDeployment makes a Deployment object using the provided parameters.
func NewDeployment(name, ns, image string, replicas int) *appsv1.Deployment {
	replInt32 := int32(replicas)
	container := corev1.Container{
		Name:  name,
		Image: image,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    map[string]string{"app": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replInt32,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}
}

// NewClusterIPService makes a Service object of type ClusterIP
// with a single port/targetPort using the provided parameters.
func NewClusterIPService(ns, name string, port, targetPort int) *corev1.Service {
	svcPort := corev1.ServicePort{
		Port:       int32(port),
		TargetPort: intstr.IntOrString{IntVal: int32(targetPort)},
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    map[string]string{"app": name},
		},
		Spec: corev1.ServiceSpec{
			Ports:    []corev1.ServicePort{svcPort},
			Selector: map[string]string{"app": name},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

// NewIngress makes an Ingress using the provided ns/name for the
// object's namespace/name and backendName/backendPort as the name
// and port of the backend Service.
func NewIngress(name, ns, backendName string, backendPort int) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": name},
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: backendName,
					Port: networkingv1.ServiceBackendPort{
						Number: int32(backendPort),
					},
				},
			},
		},
	}
}

// TagFromImage returns the tag from the provided image or an
// empty string if the image does not contain a tag.
func TagFromImage(image string) string {
	if strings.Contains(image, ":") {
		parsed := strings.Split(image, ":")
		return parsed[1]
	}
	return ""
}
