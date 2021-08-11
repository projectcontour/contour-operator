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
	"github.com/projectcontour/contour-operator/internal/parse"

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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
	// expectedPodConditions are the expected status conditions of a pod.
	expectedPodConditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
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
	rootPrefix := &gatewayv1alpha1.HTTPPathMatch{
		Type:  pathMatchTypePtr(gatewayv1alpha1.PathMatchPrefix),
		Value: pointer.StringPtr("/"),
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

func waitForContourStatusConditions(ctx context.Context, cl client.Client, name, ns string, conditions ...metav1.Condition) error {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	return wait.PollImmediate(1*time.Second, time.Minute*5, func() (bool, error) {
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
	client := http.DefaultClient
	client.Timeout = 5 * time.Second

	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		req, _ := http.NewRequest(method, url, nil)
		resp, err := client.Do(req)
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

// podWaitForHTTPResponse will curl url from ns/name client pod using host
// header "local.projectcontour.io" until timeout, expecting a 200 status code.
func podWaitForHTTPResponse(ctx context.Context, cl client.Client, ns, name, url string, timeout time.Duration) error {
	cliPod, err := newPod(ctx, cl, ns, name, "curlimages/curl:7.75.0", []string{"sleep", "600"})
	if err != nil {
		return fmt.Errorf("failed to create pod %s/%s: %v", ns, name, err)
	}

	if err := waitForPodStatusConditions(ctx, cl, 1*time.Minute, cliPod.Namespace, cliPod.Name, expectedPodConditions...); err != nil {
		return fmt.Errorf("failed to observe expected conditions for pod %s/%s: %v", cliPod.Namespace, cliPod.Name, err)
	}

	// Curl the ingress from the client pod.
	host := fmt.Sprintf("Host: %s", "local.projectcontour.io")
	cmd := []string{"curl", "-H", host, "-s", "-w", "%{http_code}", url}
	resp := "200"
	err = wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if err := parse.StringInPodExec(cliPod.Namespace, cliPod.Name, resp, cmd); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to get http %s response for %s in pod %s/%s: %v", resp, url, cliPod.Namespace, cliPod.Name, err)
	}

	if err := deletePod(ctx, cl, cliPod.Namespace, cliPod.Name); err != nil {
		return fmt.Errorf("failed to delete pod %s/%s: %v", cliPod.Namespace, cliPod.Name, err)
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
			// Kill the pod immediately so it exits quickly on deletion.
			TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
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
		if err := cl.Get(ctx, key, ns); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}
		// Consider the namespace deleted if status is terminating.
		if ns.Status.Phase == corev1.NamespaceTerminating {
			return true, nil
		}
		// The namespace is still "Active"
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for namespace %s to be deleted: %v", ns.Name, err)
	}
	return nil
}

func deletePod(ctx context.Context, cl client.Client, ns, name string) error {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
	if err := cl.Delete(ctx, p); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete pod %s/%s: %v", p.Namespace, p.Name, err)
		}
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
		if err := cl.Get(ctx, key, ns); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}
		// Consider the namespace deleted if status is terminating.
		if ns.Status.Phase == corev1.NamespaceTerminating {
			return true, nil
		}
		// The namespace is still "Active"
		return false, nil
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
// route selection. The Gateway will contain an HTTP listener on port 80.
func newGateway(ctx context.Context, cl client.Client, ns, name, gc, k, v string) error {
	routes := &metav1.LabelSelector{
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
	gw := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: gatewayv1alpha1.GatewaySpec{
			GatewayClassName: gc,
			Listeners:        []gatewayv1alpha1.Listener{http},
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

func getDeployment(ctx context.Context, cl client.Client, name, ns string) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{}
	key := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	if err := cl.Get(ctx, key, deploy); err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %v", ns, name, err)
	}
	return deploy, nil
}

func getDeploymentImage(ctx context.Context, cl client.Client, name, ns, container string) (string, error) {
	deploy, err := getDeployment(ctx, cl, name, ns)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment %s/%s: %v", ns, name, err)
	}
	for _, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == container {
			return c.Image, nil
		}
	}
	return "", fmt.Errorf("failed to find container %s in deployment %s/%s", container, ns, name)
}

func setDeploymentImage(ctx context.Context, cl client.Client, name, ns, container, image string) error {
	deploy, err := getDeployment(ctx, cl, name, ns)
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %v", ns, name, err)
	}
	updated := deploy.DeepCopy()
	for i, c := range updated.Spec.Template.Spec.Containers {
		if c.Name == container {
			updated.Spec.Template.Spec.Containers[i].Image = image
			if err := cl.Update(ctx, updated); err != nil {
				return fmt.Errorf("failed to update deployment %s/%s: %v", updated.Namespace, updated.Name, err)
			}
			return nil
		}
	}
	return fmt.Errorf("failed to find container %s in deployment %s/%s", container, ns, name)
}

// waitForImage waits for all pods identified by key/val labels in namespace ns to contain
// the expected image for the provided container until timeout is reached.
func waitForImage(ctx context.Context, cl client.Client, timeout time.Duration, ns, key, val, container, expected string) error {
	pods := &corev1.PodList{}
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if err := cl.List(ctx, pods, client.MatchingLabels{key: val}, client.InNamespace(ns)); err != nil {
			return false, nil
		}
		for _, p := range pods.Items {
			for _, c := range p.Spec.Containers {
				if c.Name == container && c.Image != expected {
					return false, nil
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

// waitForIngressLB polls Ingress ns/name until timeout, returning the status load-balancer
// IP or hostname.
func waitForIngressLB(ctx context.Context, cl client.Client, timeout time.Duration, ns, name string) (string, error) {
	nsName := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	ing := &networkingv1.Ingress{}
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if err := cl.Get(ctx, nsName, ing); err != nil {
			return false, nil
		}
		if ing.Status.LoadBalancer.Ingress == nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("timed out waiting for ingress %s/%s load-balancer: %v", ns, name, err)
	}

	switch {
	case len(ing.Status.LoadBalancer.Ingress[0].Hostname) > 0:
		return ing.Status.LoadBalancer.Ingress[0].Hostname, nil
	case len(ing.Status.LoadBalancer.Ingress[0].IP) > 0:
		return ing.Status.LoadBalancer.Ingress[0].IP, nil
	}
	return "", fmt.Errorf("failed to determine ingress %s/%s load-balancer: %v", ns, name, err)
}

// isKindCluster returns true if the cluster under test was provisioned using kind.
func isKindCluster(ctx context.Context, cl client.Client) (bool, error) {
	ds := &appsv1.DaemonSet{}
	key := types.NamespacedName{
		Namespace: "kube-system",
		Name:      "kindnet",
	}
	if err := cl.Get(ctx, key, ds); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get daemonset %s/%s: %v", key.Namespace, key.Name, err)
	}
	return true, nil
}

// getWorkerNodeIP returns the ip address of a worker node.
func getWorkerNodeIP(ctx context.Context, cl client.Client) (string, error) {
	nodes := &corev1.NodeList{}
	if err := cl.List(ctx, nodes, client.MatchingLabels{"node-role.kubernetes.io/worker": ""}); err != nil {
		return "", fmt.Errorf("failed to list worker nodes: %v", err)
	}
	for _, node := range nodes.Items {
		for _, c := range node.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				for _, a := range node.Status.Addresses {
					switch {
					// Prefer external IP over internal IP
					case a.Type == corev1.NodeExternalIP:
						return a.Address, nil
					case a.Type == corev1.NodeInternalIP:
						return a.Address, nil
						// Consider adding support for other address types.
					}
				}
			}
		}
	}
	return "", fmt.Errorf("failed to get worker node ip address")
}

// labelWorkerNodes applies the node role label to nodes that are not labeled as control-plane
// and do not contain the worker node label.
func labelWorkerNodes(ctx context.Context, cl client.Client) error {
	nodes := &corev1.NodeList{}
	if err := cl.List(ctx, nodes); err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}
	var workers []corev1.Node
	for _, node := range nodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			continue
		}
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; !ok {
			node.Labels["node-role.kubernetes.io/worker"] = ""
			workers = append(workers, node)
		}
	}
	for i, worker := range workers {
		if err := cl.Update(ctx, &workers[i]); err != nil {
			return fmt.Errorf("failed to update node %s: %v", err, worker.Name)
		}
	}
	return nil
}

func pathMatchTypePtr(t gatewayv1alpha1.PathMatchType) *gatewayv1alpha1.PathMatchType {
	return &t
}
