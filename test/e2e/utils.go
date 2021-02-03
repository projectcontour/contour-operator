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
	objutil "github.com/projectcontour/contour-operator/internal/object"
	operatorconfig "github.com/projectcontour/contour-operator/internal/operator/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
}

func newClient() (client.Client, error) {
	opts := client.Options{
		Scheme: scheme,
	}
	kubeClient, err := client.New(config.GetConfigOrDie(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %v\n", err)
	}
	return kubeClient, nil
}

func newContour(ctx context.Context, cl client.Client, name, ns, specNs string, remove bool, pubType operatorv1alpha1.NetworkPublishingType) (*operatorv1alpha1.Contour, error) {
	cntr := objutil.NewContour(name, ns, specNs, remove, pubType)
	if err := cl.Create(ctx, cntr); err != nil {
		return nil, fmt.Errorf("failed to create contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
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
		if err := cl.Get(ctx, key, cntr); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for contour %s/%s to be deleted: %v", cntr.Namespace, cntr.Name, err)
	}
	return nil
}

func newDeployment(ctx context.Context, cl client.Client, name, ns, image string, replicas int) error {
	deploy := objutil.NewDeployment(name, ns, image, replicas)
	if err := cl.Create(ctx, deploy); err != nil {
		return fmt.Errorf("failed to create deployment %s/%s: %v", deploy.Namespace, deploy.Name, err)
	}
	return nil
}

func newClusterIPService(ctx context.Context, cl client.Client, name, ns string, port, targetPort int) error {
	svc := objutil.NewClusterIPService(ns, name, port, targetPort)
	if err := cl.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create service %s/%s: %v", svc.Namespace, svc.Name, err)
	}
	return nil
}

func newIngress(ctx context.Context, cl client.Client, name, ns, backendName string, backendPort int) error {
	ing := objutil.NewIngress(name, ns, backendName, backendPort)
	if err := cl.Create(ctx, ing); err != nil {
		return fmt.Errorf("failed to create ingress %s/%s: %v", ing.Namespace, ing.Name, err)
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
		expected := contourConditionMap(conditions...)
		current := contourConditionMap(cntr.Status.Conditions...)
		return contourConditionsMatchExpected(expected, current), nil
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

func contourConditionMap(conditions ...metav1.Condition) map[string]string {
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

func contourConditionsMatchExpected(expected, actual map[string]string) bool {
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

func waitForHTTPResponse(url string, timeout time.Duration) error {
	var resp http.Response
	method := "GET"
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		req, _ := http.NewRequest(method, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil
		}
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

func deleteNamespace(ctx context.Context, cl client.Client, timeout time.Duration, name string) error {
	ns := objutil.NewNamespace(name)
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
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for namespace %s to be deleted: %v", ns.Name, err)
	}
	return nil
}

func waitForSpecNsDeletion(ctx context.Context, cl client.Client, timeout time.Duration, name string) error {
	ns := objutil.NewNamespace(name)

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

// updateLbSvcIpAndNodePorts updates the loadbalancer IP to "127.0.0.1" and nodeports
// to EnvoyNodePortHTTPPort and EnvoyNodePortHTTPSPort of the service referenced by ns/name.
func updateLbSvcIpAndNodePorts(ctx context.Context, cl client.Client, timeout time.Duration, ns, name string) error {
	svc, err := waitForService(ctx, kclient, 1*time.Minute, ns, name)
	if err != nil {
		return fmt.Errorf("failed to observe service %s/%s: %v", ns, name, err)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return fmt.Errorf("invalid type %s for service %s/%s", svc.Spec.Type, ns, name)
	}
	svc.Spec.LoadBalancerIP = "127.0.0.1"
	svc.Spec.Ports[0].NodePort = operatorconfig.EnvoyNodePortHTTPPort
	svc.Spec.Ports[1].NodePort = operatorconfig.EnvoyNodePortHTTPSPort
	if err := cl.Update(ctx, svc); err != nil {
		return err
	}
	return nil
}
