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

//go:build e2e
// +build e2e

package example

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// Install operator from example manifest.
func installOperatorFromExample(t *testing.T, c client.Client) func(*testing.T, client.Client) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not get path to this source file (test/e2e/example/example_test.go)")
	exampleYAMLPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "examples", "operator", "operator.yaml")
	file, err := os.Open(exampleYAMLPath)
	require.NoError(t, err)
	defer file.Close()
	decoder := yaml.NewYAMLToJSONDecoder(file)

	// Use a slice rather than map so we iterate in a consistent order.
	operatorResources := []struct {
		name string
		obj  client.Object
	}{
		{name: "operator namespace", obj: new(v1.Namespace)},
		{name: "backend policy crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "contour crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "extensionservice crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "gatewayclass crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "gateway crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "httpproxy crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "httproute crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "tcproute crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "tlscertificatedelegation crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "tlsroute crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "udproute crd", obj: new(apiextensions_v1.CustomResourceDefinition)},
		{name: "operator leader election role", obj: new(rbac_v1.Role)},
		{name: "operator clusterrole", obj: new(rbac_v1.ClusterRole)},
		{name: "operator auth proxy clusterrole", obj: new(rbac_v1.ClusterRole)},
		{name: "operator metrics reader clusterrole", obj: new(rbac_v1.ClusterRole)},
		{name: "operator leader election rolebinding", obj: new(rbac_v1.RoleBinding)},
		{name: "operator clusterrolebinding", obj: new(rbac_v1.ClusterRoleBinding)},
		{name: "operator auth proxy clusterrolebinding", obj: new(rbac_v1.ClusterRoleBinding)},
		{name: "operator metrics service", obj: new(v1.Service)},
		{name: "operator deployment", obj: new(apps_v1.Deployment)},
	}

	for _, resource := range operatorResources {
		require.NoError(t, decoder.Decode(resource.obj))

		require.NoError(t, c.Create(context.TODO(), resource.obj))
		t.Log("created resource:", resource.name)
	}

	// Wait for operator deployment to be available.
	require.Eventually(t, func() bool {
		d := &apps_v1.Deployment{}
		if err := c.Get(context.TODO(), client.ObjectKeyFromObject(operatorResources[len(operatorResources)-1].obj), d); err != nil {
			return false
		}
		for _, c := range d.Status.Conditions {
			if c.Type == apps_v1.DeploymentAvailable && c.Status == v1.ConditionTrue {
				return true
			}
		}
		return false
	}, time.Minute, time.Second)
	t.Log("operator deployment available")

	// Cleanup func deletes resources in reverse order.
	return func(t *testing.T, c client.Client) {
		t.Helper()

		for i := len(operatorResources) - 1; i >= 0; i-- {
			require.NoError(t, c.Delete(context.TODO(), operatorResources[i].obj))
			t.Log("deleted resource:", operatorResources[i].name)
		}
	}
}

func TestExample(t *testing.T) {
	k8sClient, err := e2e.NewK8sClient()
	require.NoError(t, err)
	cleanup := installOperatorFromExample(t, k8sClient)
	defer cleanup(t, k8sClient)

	testCases := map[string]func(*testing.T, client.Client){
		"Basic Ingress example": testBasicIngressExample,
		"Gateway API example":   testGatewayExample,
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc(t, k8sClient)
		})
	}
}

func testGatewayExample(t *testing.T, c client.Client) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not get path to this source file (test/e2e/example/example_test.go)")
	exampleYAMLPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "examples", "gateway", "gateway-nodeport.yaml")
	file, err := os.Open(exampleYAMLPath)
	require.NoError(t, err)
	defer file.Close()
	decoder := yaml.NewYAMLToJSONDecoder(file)

	testNS := &v1.Namespace{}
	require.NoError(t, decoder.Decode(testNS))
	testNS.Name = "test-gateway-example"
	require.NoError(t, c.Create(context.TODO(), testNS))
	defer func() {
		require.NoError(t, c.Delete(context.TODO(), testNS))
		// Wait for namespace to be deleted.
		require.Eventually(t, func() bool {
			return c.Get(context.TODO(), client.ObjectKeyFromObject(testNS), testNS) != nil
		}, time.Minute*3, time.Second)
		t.Log("deleted namespace:", testNS.Name)
	}()

	contourInstance := &operatorv1alpha1.Contour{}
	require.NoError(t, decoder.Decode(contourInstance))
	contourInstance.Spec.Namespace = operatorv1alpha1.NamespaceSpec{
		Name: testNS.Name,
	}
	require.NoError(t, c.Create(context.TODO(), contourInstance))
	t.Logf("created contour: %s/%s\n", contourInstance.Namespace, contourInstance.Name)
	defer func() {
		require.NoError(t, c.Delete(context.TODO(), contourInstance))
		// Wait for contour to be deleted.
		require.Eventually(t, func() bool {
			return c.Get(context.TODO(), client.ObjectKeyFromObject(contourInstance), contourInstance) != nil
		}, time.Minute*3, time.Second)
		t.Logf("deleted contour %s/%s\n", contourInstance.Namespace, contourInstance.Name)
	}()

	require.NoError(t, e2e.WaitForContourAvailable(c, contourInstance.Name, contourInstance.Namespace))
	t.Log("contour available")

	gc := &gatewayv1alpha1.GatewayClass{}
	require.NoError(t, decoder.Decode(gc))
	require.NoError(t, c.Create(context.TODO(), gc))
	t.Log("created gatewayclass:", gc.Name)

	gw := &gatewayv1alpha1.Gateway{}
	require.NoError(t, decoder.Decode(gw))
	gw.Namespace = testNS.Name
	require.NoError(t, c.Create(context.TODO(), gw))
	t.Logf("created gateway %s/%s\n", gw.Namespace, gw.Name)
	defer func() {
		require.NoError(t, c.Delete(context.TODO(), gw))
		t.Log("deleted gateway:", gw.Name)
		require.NoError(t, c.Delete(context.TODO(), gc))
		t.Log("deleted gatewayclass:", gc.Name)
	}()

	// Create a sample workload for e2e testing.
	require.NoError(t, e2e.NewDeployment(c, "kuard", testNS.Name, "gcr.io/kuar-demo/kuard-amd64:1", 1))
	require.NoError(t, e2e.WaitForDeploymentAvailable(c, "kuard", testNS.Name))
	require.NoError(t, e2e.NewClusterIPService(c, "kuard", testNS.Name, 80, 8080))
	require.NoError(t, e2e.NewHTTPRoute(c, "kuard", testNS.Name, "kuard", "app", "kuard", "local.projectcontour.io", int32(80)))
	t.Log("deployed test app and HTTPRoute")

	assert.NoError(t, e2e.WaitForHTTPResponse("http://local.projectcontour.io/", time.Minute))
	t.Log("request to test app succeeded")
}

func testBasicIngressExample(t *testing.T, c client.Client) {
	testNS := &v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "test-basic-ingress-example",
		},
	}
	require.NoError(t, c.Create(context.TODO(), testNS))
	defer func() {
		require.NoError(t, c.Delete(context.TODO(), testNS))
		// Wait for namespace to be deleted.
		require.Eventually(t, func() bool {
			return c.Get(context.TODO(), client.ObjectKeyFromObject(testNS), testNS) != nil
		}, time.Minute*3, time.Second)
		t.Log("deleted namespace:", testNS.Name)
	}()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not get path to this source file (test/e2e/example/example_test.go)")
	exampleYAMLPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "examples", "contour", "contour-nodeport.yaml")
	file, err := os.Open(exampleYAMLPath)
	require.NoError(t, err)
	defer file.Close()
	decoder := yaml.NewYAMLToJSONDecoder(file)

	contourInstance := &operatorv1alpha1.Contour{}
	require.NoError(t, decoder.Decode(contourInstance))
	contourInstance.Spec.Namespace = operatorv1alpha1.NamespaceSpec{
		Name: testNS.Name,
	}
	require.NoError(t, c.Create(context.TODO(), contourInstance))
	t.Logf("created contour: %s/%s\n", contourInstance.Namespace, contourInstance.Name)
	defer func() {
		require.NoError(t, c.Delete(context.TODO(), contourInstance))
		// Wait for contour to be deleted.
		require.Eventually(t, func() bool {
			return c.Get(context.TODO(), client.ObjectKeyFromObject(contourInstance), contourInstance) != nil
		}, time.Minute*3, time.Second)
		t.Logf("deleted contour %s/%s\n", contourInstance.Namespace, contourInstance.Name)
	}()

	require.NoError(t, e2e.WaitForContourAvailable(c, contourInstance.Name, contourInstance.Namespace))
	t.Log("contour available")

	// Create a sample workload for e2e testing.
	require.NoError(t, e2e.NewDeployment(c, "kuard", testNS.Name, "gcr.io/kuar-demo/kuard-amd64:1", 1))
	require.NoError(t, e2e.WaitForDeploymentAvailable(c, "kuard", testNS.Name))
	require.NoError(t, e2e.NewClusterIPService(c, "kuard", testNS.Name, 80, 8080))
	require.NoError(t, e2e.NewIngress(c, "kuard", testNS.Name, "kuard", 80))
	t.Log("deployed test app and Ingress")

	assert.NoError(t, e2e.WaitForHTTPResponse("http://local.projectcontour.io/", time.Minute))
	t.Log("request to test app succeeded")
}
