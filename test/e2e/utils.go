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

// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objsvc "github.com/projectcontour/contour-operator/internal/objects/service"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
	_ = gatewayv1alpha1.AddToScheme(scheme)
}

func newClient() (client.Client, error) {
	opts := client.Options{
		Scheme: scheme,
	}
	kubeClient, err := client.New(config.GetConfigOrDie(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %v", err)
	}
	return kubeClient, nil
}

func newContour(ctx context.Context, cl client.Client, cfg objcontour.Config) (*operatorv1alpha1.Contour, error) {
	cntr := objcontour.New(cfg)
	if err := cl.Create(ctx, cntr); err != nil {
		return nil, fmt.Errorf("failed to create contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
	}
	return cntr, nil
}

func updateContour(ctx context.Context, cl client.Client, cfg objcontour.Config) (*operatorv1alpha1.Contour, error) {
	desired := objcontour.New(cfg)
	cntr := &operatorv1alpha1.Contour{}
	key := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.Name,
	}
	if err := cl.Get(ctx, key, cntr); err != nil {
		return nil, err
	}

	cntr.Spec = desired.Spec

	if err := cl.Update(ctx, cntr); err != nil {
		return cntr, err
	}
	return cntr, nil
}

func deleteContour(ctx context.Context, cl client.Client, timeout time.Duration, name, ns string) error {
	cntr := &operatorv1alpha1.Contour{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
	if err := cl.Delete(ctx, cntr); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
		}
	}

	key := types.NamespacedName{
		Name:      cntr.Name,
		Namespace: cntr.Namespace,
	}

	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		err := cl.Get(ctx, key, cntr)
		return errors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for contour %s/%s to be deleted: %v", cntr.Namespace, cntr.Name, err)
	}
	return nil
}

func deleteGateway(ctx context.Context, cl client.Client, timeout time.Duration, name, ns string) error {
	gw := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
	if err := cl.Delete(ctx, gw); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete gateway %s/%s: %v", gw.Namespace, gw.Name, err)
		}
	}

	key := types.NamespacedName{
		Name:      gw.Name,
		Namespace: gw.Namespace,
	}

	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		err := cl.Get(ctx, key, gw)
		return errors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for gateway %s/%s to be deleted: %v", gw.Namespace, gw.Name, err)
	}
	return nil
}

func deleteGatewayClass(ctx context.Context, cl client.Client, timeout time.Duration, name string) error {
	gc := &gatewayv1alpha1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if err := cl.Delete(ctx, gc); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete gatewayclass %s: %v", gc.Name, err)
		}
	}

	key := types.NamespacedName{Name: gc.Name}

	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		err := cl.Get(ctx, key, gc)
		return errors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for gatewayclass %s to be deleted: %v", gc.Name, err)
	}
	return nil
}

func newDeployment(ctx context.Context, cl client.Client, name, ns, image string, replicas int) error {
	replInt32 := int32(replicas)
	container := corev1.Container{
		Name:  name,
		Image: image,
	}
	deploy := &appsv1.Deployment{
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
	if err := cl.Create(ctx, deploy); err != nil {
		return fmt.Errorf("failed to create deployment %s/%s: %v", deploy.Namespace, deploy.Name, err)
	}
	return nil
}

func newClusterIPService(ctx context.Context, cl client.Client, name, ns string, port, targetPort int) error {
	svcPort := corev1.ServicePort{
		Port:       int32(port),
		TargetPort: intstr.IntOrString{IntVal: int32(targetPort)},
	}
	svc := &corev1.Service{
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
	if err := cl.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create service %s/%s: %v", svc.Namespace, svc.Name, err)
	}
	return nil
}

func newIngress(ctx context.Context, cl client.Client, name, ns, backendName string, backendPort int) error {
	ing := &networkingv1.Ingress{
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
	if err := cl.Create(ctx, ing); err != nil {
		return fmt.Errorf("failed to create ingress %s/%s: %v", ing.Namespace, ing.Name, err)
	}
	return nil
}

func newHTTPRouteToSvc(ctx context.Context, cl client.Client, name, ns, svc, k, v, hostname string, svcPort int32) error {
	rootPrefix := gatewayv1alpha1.HTTPPathMatch{
		Type:  gatewayv1alpha1.PathMatchPrefix,
		Value: "/",
	}
	fwdPort := gatewayv1alpha1.PortNumber(svcPort)
	svcFwd := gatewayv1alpha1.HTTPRouteForwardTo{
		ServiceName: &svc,
		Port:        &fwdPort,
	}
	match := gatewayv1alpha1.HTTPRouteMatch{
		Path: rootPrefix,
	}
	httpRule := gatewayv1alpha1.HTTPRouteRule{
		Matches:   []gatewayv1alpha1.HTTPRouteMatch{match},
		ForwardTo: []gatewayv1alpha1.HTTPRouteForwardTo{svcFwd},
	}
	h := gatewayv1alpha1.Hostname(hostname)
	route := &gatewayv1alpha1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{k: v},
		},
		Spec: gatewayv1alpha1.HTTPRouteSpec{
			Hostnames: []gatewayv1alpha1.Hostname{h},
			Rules:     []gatewayv1alpha1.HTTPRouteRule{httpRule},
		},
	}
	if err := cl.Create(ctx, route); err != nil {
		return fmt.Errorf("failed to create httproute %s/%s: %v", ns, name, err)
	}
	return nil
}

func waitForContourStatusConditions(ctx context.Context, cl client.Client, timeout time.Duration, name, ns string, conditions ...metav1.Condition) error {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		cntr := &operatorv1alpha1.Contour{}
		if err := cl.Get(ctx, nsName, cntr); err != nil {
			return false, nil
		}

		if cntr.Status.AvailableContours != cntr.Spec.Replicas {
			return false, nil
		}

		envoyReplicas, err := envoyReplicas(ctx, cl, cntr.Spec.Namespace.Name)
		if err != nil || cntr.Status.AvailableEnvoys != envoyReplicas {
			return false, nil
		}

		expected := conditionMap(conditions...)
		current := conditionMap(cntr.Status.Conditions...)
		return conditionsMatchExpected(expected, current), nil
	})
}

func waitForGatewayClassStatusConditions(ctx context.Context, cl client.Client, timeout time.Duration, name string, conditions ...metav1.Condition) error {
	nsName := types.NamespacedName{Name: name}
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		gc := &gatewayv1alpha1.GatewayClass{}
		if err := cl.Get(ctx, nsName, gc); err != nil {
			return false, nil
		}
		expected := conditionMap(conditions...)
		current := conditionMap(gc.Status.Conditions...)
		return conditionsMatchExpected(expected, current), nil
	})
}

func waitForGatewayStatusConditions(ctx context.Context, cl client.Client, timeout time.Duration, name, ns string, conditions ...metav1.Condition) error {
	nsName := types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		gw := &gatewayv1alpha1.Gateway{}
		if err := cl.Get(ctx, nsName, gw); err != nil {
			return false, nil
		}
		expected := conditionMap(conditions...)
		current := conditionMap(gw.Status.Conditions...)
		return conditionsMatchExpected(expected, current), nil
	})
}

func waitForDeploymentStatusConditions(ctx context.Context, cl client.Client, timeout time.Duration, name, ns string, conditions ...appsv1.DeploymentCondition) error {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		deploy := &appsv1.Deployment{}
		if err := cl.Get(ctx, nsName, deploy); err != nil {
			return false, nil
		}
		expected := deploymentConditionMap(conditions...)
		current := deploymentConditionMap(deploy.Status.Conditions...)
		return deploymentConditionsMatchExpected(expected, current), nil
	})
}

func waitForPodStatusConditions(ctx context.Context, cl client.Client, timeout time.Duration, ns, name string, conditions ...corev1.PodCondition) error {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		pod := &corev1.Pod{}
		if err := cl.Get(ctx, nsName, pod); err != nil {
			return false, nil
		}
		expected := podConditionMap(conditions...)
		current := podConditionMap(pod.Status.Conditions...)
		return podConditionsMatchExpected(expected, current), nil
	})
}

func conditionMap(conditions ...metav1.Condition) map[string]string {
	conds := map[string]string{}
	for _, cond := range conditions {
		conds[cond.Type] = string(cond.Status)
	}
	return conds
}

func deploymentConditionMap(conditions ...appsv1.DeploymentCondition) map[appsv1.DeploymentConditionType]corev1.ConditionStatus {
	conds := map[appsv1.DeploymentConditionType]corev1.ConditionStatus{}
	for _, cond := range conditions {
		conds[cond.Type] = cond.Status
	}
	return conds
}

func podConditionMap(conditions ...corev1.PodCondition) map[corev1.PodConditionType]corev1.ConditionStatus {
	conds := map[corev1.PodConditionType]corev1.ConditionStatus{}
	for _, cond := range conditions {
		conds[cond.Type] = cond.Status
	}
	return conds
}

func conditionsMatchExpected(expected, actual map[string]string) bool {
	filtered := map[string]string{}
	for k := range actual {
		if _, comparable := expected[k]; comparable {
			filtered[k] = actual[k]
		}
	}
	return reflect.DeepEqual(expected, filtered)
}

func deploymentConditionsMatchExpected(expected, actual map[appsv1.DeploymentConditionType]corev1.ConditionStatus) bool {
	filtered := map[appsv1.DeploymentConditionType]corev1.ConditionStatus{}
	for k := range actual {
		if _, comparable := expected[k]; comparable {
			filtered[k] = actual[k]
		}
	}
	return reflect.DeepEqual(expected, filtered)
}

func podConditionsMatchExpected(expected, actual map[corev1.PodConditionType]corev1.ConditionStatus) bool {
	filtered := map[corev1.PodConditionType]corev1.ConditionStatus{}
	for k := range actual {
		if _, comparable := expected[k]; comparable {
			filtered[k] = actual[k]
		}
	}
	return reflect.DeepEqual(expected, filtered)
}

func waitForHTTPResponse(url string, timeout time.Duration) error {
	var resp http.Response
	method := "GET"
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		req, _ := http.NewRequest(method, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("%s %q failed with status %s: %v", method, url, resp.Status, err)
	}
	return nil
}

// newNs creates a Namespace object using the provided name for the object's name.
func newNs(ctx context.Context, cl client.Client, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := cl.Create(ctx, ns); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace %s: %v", ns.Name, err)
		}
	}
	return nil
}

// newPod creates a Pod resource using name as the Pod's name, ns as
// the Pod's namespace, image as the Pod container's image and cmd as the
// Pod container's command.
func newPod(ctx context.Context, cl client.Client, ns, name, image string, cmd []string) (*corev1.Pod, error) {
	c := corev1.Container{
		Name:    name,
		Image:   image,
		Command: cmd,
	}
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{c},
		},
	}
	if err := cl.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to create pod %s/%s: %v", p.Namespace, p.Name, err)
	}
	return p, nil
}

func deleteNamespace(ctx context.Context, cl client.Client, timeout time.Duration, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := cl.Delete(ctx, ns); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace %s: %v", ns.Name, err)
		}
	}

	key := types.NamespacedName{
		Name: ns.Name,
	}

	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		err := cl.Get(ctx, key, ns)
		return errors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for namespace %s to be deleted: %v", ns.Name, err)
	}
	return nil
}

func waitForSpecNsDeletion(ctx context.Context, cl client.Client, timeout time.Duration, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	key := types.NamespacedName{
		Name: ns.Name,
	}

	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		err := cl.Get(ctx, key, ns)
		return errors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for namespace %s to be deleted: %v", ns.Name, err)
	}
	return nil
}

func waitForService(ctx context.Context, cl client.Client, timeout time.Duration, ns, name string) (*corev1.Service, error) {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	svc := &corev1.Service{}
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if err := cl.Get(ctx, nsName, svc); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("timed out waiting for service %s/%s: %v", ns, name, err)
	}
	return svc, nil
}

func waitForServiceDeletion(ctx context.Context, cl client.Client, timeout time.Duration, ns, name string) error {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	svc := &corev1.Service{}
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		err := cl.Get(ctx, nsName, svc)
		return errors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for service %s/%s: %v", ns, name, err)
	}
	return nil
}

// updateLbSvcIPAndNodePorts updates the loadbalancer IP to "127.0.0.1" and nodeports
// to EnvoyNodePortHTTPPort and EnvoyNodePortHTTPSPort of the service referenced by ns/name.
func updateLbSvcIPAndNodePorts(ctx context.Context, cl client.Client, timeout time.Duration, ns, name string) error {
	svc, err := waitForService(ctx, cl, timeout, ns, name)
	if err != nil {
		return fmt.Errorf("failed to observe service %s/%s: %v", ns, name, err)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return fmt.Errorf("invalid type %s for service %s/%s", svc.Spec.Type, ns, name)
	}
	svc.Spec.LoadBalancerIP = "127.0.0.1"
	svc.Spec.Ports[0].NodePort = objsvc.EnvoyNodePortHTTPPort
	svc.Spec.Ports[1].NodePort = objsvc.EnvoyNodePortHTTPSPort
	if err := cl.Update(ctx, svc); err != nil {
		return err
	}
	return nil
}

// newOperatorGatewayClass creates a GatewayClass object using the provided name for the object's
// name and contourNs/contourName for the referenced Contour.
func newOperatorGatewayClass(ctx context.Context, cl client.Client, name, contourNs, contourName string) error {
	gc := &gatewayv1alpha1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1alpha1.GatewayClassSpec{
			Controller: operatorv1alpha1.GatewayClassControllerRef,
			ParametersRef: &gatewayv1alpha1.ParametersReference{
				Group:     operatorv1alpha1.GatewayClassParamsRefGroup,
				Kind:      operatorv1alpha1.GatewayClassParamsRefKind,
				Name:      contourName,
				Scope:     "Namespace",
				Namespace: contourNs,
			},
		},
	}
	if err := cl.Create(ctx, gc); err != nil {
		return fmt.Errorf("failed to create gatewayclass %s: %v", name, err)
	}
	return nil
}

// newGatewayClass creates a GatewayClass object using the provided name as
// the name of the GatewayClass and controller string.
func newGatewayClass(ctx context.Context, cl client.Client, name string) error {
	gc := &gatewayv1alpha1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1alpha1.GatewayClassSpec{
			Controller: name,
		},
	}
	if err := cl.Create(ctx, gc); err != nil {
		return fmt.Errorf("failed to create gatewayclass %s: %v", name, err)
	}
	return nil
}

// newGateway creates a Gateway object using the provided ns/name for the object's
// ns/name, gc for the gatewayClassName, and k/v for the key-value pair used for
// route selection. The Gateway will contain an HTTP listener on port 80 and an
// HTTPS listener on port 443.
func newGateway(ctx context.Context, cl client.Client, ns, name, gc, k, v string) error {
	routes := metav1.LabelSelector{
		MatchLabels: map[string]string{k: v},
	}
	http := gatewayv1alpha1.Listener{
		Port:     gatewayv1alpha1.PortNumber(int32(80)),
		Protocol: gatewayv1alpha1.HTTPProtocolType,
		Routes: gatewayv1alpha1.RouteBindingSelector{
			Kind:     "HTTPRoute",
			Selector: routes,
		},
	}
	https := gatewayv1alpha1.Listener{
		Port:     gatewayv1alpha1.PortNumber(int32(443)),
		Protocol: gatewayv1alpha1.HTTPSProtocolType,
		Routes: gatewayv1alpha1.RouteBindingSelector{
			Kind:     "HTTPRoute",
			Selector: routes,
		},
	}
	gw := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: gatewayv1alpha1.GatewaySpec{
			GatewayClassName: gc,
			Listeners:        []gatewayv1alpha1.Listener{http, https},
		},
	}
	if err := cl.Create(ctx, gw); err != nil {
		return fmt.Errorf("failed to create gateway %s/%s: %v", ns, name, err)
	}
	return nil
}

// envoyClusterIP returns the clusterIP for a service that matches the provided ns/name.
func envoyClusterIP(ctx context.Context, cl client.Client, ns, name string) (string, error) {
	svc := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	if err := cl.Get(ctx, key, svc); err != nil {
		return "", fmt.Errorf("failed to get service %s/%s: %v", ns, name, err)
	}
	if len(svc.Spec.ClusterIP) > 0 {
		return svc.Spec.ClusterIP, nil
	}
	return "", fmt.Errorf("service %s/%s does not have a clusterIP", ns, name)
}

// envoyReplicas returns the number of envoy pods running by numberReady in daemonset status.
func envoyReplicas(ctx context.Context, cl client.Client, ns string) (int32, error) {
	ds := &appsv1.DaemonSet{}
	key := types.NamespacedName{
		Namespace: ns,
		Name:      "envoy",
	}
	if err := cl.Get(ctx, key, ds); err != nil {
		return 0, fmt.Errorf("failed to get daemonset %s/envoy: %v", ns, err)

	}
	return ds.Status.NumberReady, nil
}
