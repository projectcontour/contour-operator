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
	"os"
	"testing"
	"time"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/config"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const timeout = 3 * time.Minute

var (
	// kclient is the Kubernetes client used for e2e tests.
	kclient client.Client
	// ctx is an empty context used for client calls.
	ctx = context.TODO()
	// operatorName is the name of the operator.
	operatorName = "contour-operator"
	// operatorNs is the name of the operator's namespace.
	operatorNs = "contour-operator"
	// specNs is the spec.namespace.name of a Contour.
	specNs = "projectcontour"
	// expectedDeploymentConditions are the expected status conditions of a
	// deployment.
	expectedDeploymentConditions = []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
	}
	// expectedContourConditions are the expected status conditions of a
	// contour.
	expectedContourConditions = []metav1.Condition{
		{Type: operatorv1alpha1.ContourAvailableConditionType, Status: metav1.ConditionTrue},
		// TODO [danehans]: Update when additional status conditions are added to Contour.
	}
	// expectedGatewayClassConditions are the expected status conditions of a GatewayClass.
	expectedGatewayClassConditions = []metav1.Condition{
		{Type: string(gatewayv1alpha1.GatewayClassConditionStatusAdmitted), Status: metav1.ConditionTrue},
	}
	// expectedNonOwnedGatewayClassConditions are the expected status conditions of a GatewayClass
	// not owned by the operator.
	expectedNonOwnedGatewayClassConditions = []metav1.Condition{
		{Type: string(gatewayv1alpha1.GatewayClassConditionStatusAdmitted), Status: metav1.ConditionFalse},
	}
	// expectedGatewayConditions are the expected status conditions of a Gateway.
	expectedGatewayConditions = []metav1.Condition{
		{Type: string(gatewayv1alpha1.GatewayConditionReady), Status: metav1.ConditionTrue},
	}
	// expectedNonOwnedGatewayConditions are the expected status conditions of a Gateway
	// not owned by the operator.
	expectedNonOwnedGatewayConditions = []metav1.Condition{
		{Type: string(gatewayv1alpha1.GatewayConditionScheduled), Status: metav1.ConditionFalse},
	}
	// testAppName is the name of the application used for e2e testing.
	testAppName = "kuard"
	// testAppImage is the image used by the e2e test application.
	testAppImage = "gcr.io/kuar-demo/kuard-amd64:1"
	// testAppReplicas is the number of replicas used for the e2e test application's
	// deployment.
	testAppReplicas = 3
	// isKind determines if tests should be tuned to run in a kind cluster.
	isKind = true
)

func TestMain(m *testing.M) {
	cl, err := newClient()
	if err != nil {
		os.Exit(1)
	}
	kclient = cl

	isKind, err = isKindCluster(ctx, kclient)
	if err != nil {
		os.Exit(1)
	}

	if isKind {
		if err := labelWorkerNodes(ctx, kclient); err != nil {
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

func TestOperatorDeploymentAvailable(t *testing.T) {
	t.Helper()
	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, operatorName, operatorNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected conditions for deployment %s/%s: %v", operatorNs, operatorName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", operatorNs, operatorName)
}

func TestDefaultContour(t *testing.T) {
	testName := "test-default-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      specNs,
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.LoadBalancerServicePublishingType,
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if isKind {
		svcName := "envoy"
		if err := updateLbSvcIPAndNodePorts(ctx, kclient, timeout, cfg.SpecNs, svcName); err != nil {
			t.Fatalf("failed to update service %s/%s: %v", cfg.SpecNs, svcName, err)
		}
		t.Logf("updated service %s/%s loadbalancer IP and nodeports", cfg.SpecNs, svcName)
	}

	if err := waitForContourStatusConditions(ctx, kclient, timeout, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", operatorNs, testName)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newIngress(ctx, kclient, appName, cfg.SpecNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created ingress %s/%s", cfg.SpecNs, appName)

	// testURL is the url used to test e2e functionality.
	testURL := "http://local.projectcontour.io/"

	if isKind {
		if err := waitForHTTPResponse(testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	} else {
		lb, err := waitForIngressLB(ctx, kclient, timeout, cfg.SpecNs, appName)
		if err != nil {
			t.Fatalf("failed to get ingress %s/%s load-balancer: %v", cfg.SpecNs, appName, err)
		}
		t.Logf("ingress %s/%s has load-balancer %s", cfg.SpecNs, appName, lb)
		testURL = fmt.Sprintf("http://%s/", lb)

		// Curl the ingress from a client pod.
		cliName := "test-client"
		if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, timeout, cfg.SpecNs, "envoy"); err != nil {
		t.Fatalf("failed to delete envoy service %s/envoy: %v", cfg.SpecNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", cfg.SpecNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

func TestContourNodePortService(t *testing.T) {
	testName := "test-nodeport-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      fmt.Sprintf("%s-nodeport", specNs),
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.NodePortServicePublishingType,
		NodePorts:   objcontour.MakeNodePorts(map[string]int{"http": 30080, "https": 30443}),
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, timeout, cntr.Name, cntr.Namespace, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cntr.Namespace, cntr.Name)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newIngress(ctx, kclient, appName, cfg.SpecNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created ingress %s/%s", cfg.SpecNs, appName)

	// testURL is the url used to test e2e functionality.
	testURL := "http://local.projectcontour.io/"

	var ip string
	if isKind {
		if err := waitForHTTPResponse(testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	} else {
		// Get the IP of a worker node to test the nodeport service.
		ip, err = getWorkerNodeIP(ctx, kclient)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("using worker node ip %s", ip)

		// Curl the ingress from a client pod.
		testURL = fmt.Sprintf("http://%s:30080/", ip)
		cliName := "test-client"
		if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	}

	// update the contour nodeports. Note that kind is configured to map port 81>30081 and 444>30444.
	key := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.Name,
	}
	if err := kclient.Get(ctx, key, cntr); err != nil {
		t.Fatalf("failed to get contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}

	cntr.Spec.NetworkPublishing.Envoy.NodePorts = objcontour.MakeNodePorts(map[string]int{"http": 30081, "https": 30444})
	if err := kclient.Update(ctx, cntr); err != nil {
		t.Fatalf("failed to update contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("updated contour %s/%s", cfg.Namespace, cfg.Name)

	// Update the kuard service port.
	svc := &corev1.Service{}
	key = types.NamespacedName{
		Namespace: cfg.SpecNs,
		Name:      appName,
	}
	if err := kclient.Get(ctx, key, svc); err != nil {
		t.Fatalf("failed to get service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	svc.Spec.Ports[0].Port = int32(81)
	if err := kclient.Update(ctx, svc); err != nil {
		t.Fatalf("failed to update service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("updated service %s/%s", cfg.SpecNs, appName)

	// Update the kuard ingress port.
	ing := &networkingv1.Ingress{}
	key = types.NamespacedName{
		Namespace: cfg.SpecNs,
		Name:      appName,
	}
	if err := kclient.Get(ctx, key, ing); err != nil {
		t.Fatalf("failed to get ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	ing.Spec.DefaultBackend.Service.Port.Number = int32(81)
	if err := kclient.Update(ctx, ing); err != nil {
		t.Fatalf("failed to update ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("updated ingress %s/%s", cfg.SpecNs, appName)

	// Update the testURL port.
	testURL = "http://local.projectcontour.io:81/"

	if isKind {
		if err := waitForHTTPResponse(testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	} else {
		// Curl the ingress from a client pod.
		testURL = fmt.Sprintf("http://%s:30081/", ip)
		cliName := "test-client"
		if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, cfg.Name, cfg.Namespace); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("deleted contour %s/%s", cfg.Namespace, cfg.Name)

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, timeout, cfg.SpecNs, "envoy"); err != nil {
		t.Fatalf("failed to delete envoy service %s/envoy: %v", cfg.SpecNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", cfg.SpecNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

func TestContourClusterIPService(t *testing.T) {
	testName := "test-clusterip-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      fmt.Sprintf("%s-clusterip", specNs),
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.ClusterIPServicePublishingType,
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, timeout, cntr.Name, cntr.Namespace, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cntr.Namespace, cntr.Name)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newIngress(ctx, kclient, appName, cfg.SpecNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created ingress %s/%s", cfg.SpecNs, appName)

	// Get the Envoy ClusterIP to curl.
	svcName := "envoy"
	ip, err := envoyClusterIP(ctx, kclient, cfg.SpecNs, svcName)
	if err != nil {
		t.Fatalf("failed to get clusterIP for service %s/%s: %v", cfg.SpecNs, svcName, err)
	}

	// Curl the ingress from a client pod.
	testURL := fmt.Sprintf("http://%s/", ip)
	cliName := "test-client"
	if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, timeout, cfg.SpecNs, "envoy"); err != nil {
		t.Fatalf("failed to delete envoy service %s/envoy: %v", cfg.SpecNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", cfg.SpecNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

// TestContourSpec tests some spec changes such as:
// - Enable RemoveNs.
// - Initial replicas to 4.
// - Decrease replicas to 3.
func TestContourSpec(t *testing.T) {
	testName := "test-user-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      fmt.Sprintf("%s-contourspec", specNs),
		RemoveNs:    true,
		Replicas:    4,
		NetworkType: operatorv1alpha1.NodePortServicePublishingType,
		NodePorts:   objcontour.MakeNodePorts(map[string]int{"http": 30080, "https": 30443}),
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, timeout, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", operatorNs, testName)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	cfg.Replicas = 3
	if _, err := updateContour(ctx, kclient, cfg); err != nil {
		t.Fatalf("failed to update contour %s/%s: %v", operatorNs, testName, err)
	}
	if err := waitForContourStatusConditions(ctx, kclient, timeout, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", operatorNs, testName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newIngress(ctx, kclient, appName, cfg.SpecNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created ingress %s/%s", cfg.SpecNs, appName)

	// testURL is the url used to test e2e functionality.
	testURL := "http://local.projectcontour.io/"

	if isKind {
		if err := waitForHTTPResponse(testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	} else {
		// Get the IP of a worker node to test the nodeport service.
		ip, err := getWorkerNodeIP(ctx, kclient)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("using worker node ip %s", ip)

		// Curl the ingress from the client pod.
		testURL = fmt.Sprintf("http://%s:30080/", ip)
		cliName := "test-client"
		if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Verify the user-defined namespace was removed by the operator.
	if err := waitForSpecNsDeletion(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to observe the deletion of namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

func TestMultipleContours(t *testing.T) {
	testNames := []string{"test-user-contour", "test-user-contour-2"}
	for _, testName := range testNames {
		cfg := objcontour.Config{
			Name:        testName,
			Namespace:   operatorNs,
			SpecNs:      fmt.Sprintf("%s-ns", testName),
			RemoveNs:    true,
			NetworkType: operatorv1alpha1.LoadBalancerServicePublishingType,
		}
		cntr, err := newContour(ctx, kclient, cfg)
		if err != nil {
			t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
		}
		t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

		if err := waitForContourStatusConditions(ctx, kclient, timeout, testName, operatorNs, expectedContourConditions...); err != nil {
			t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
		}
		t.Logf("observed expected status conditions for contour %s/%s", operatorNs, testName)
	}

	// Ensure the default contour can be deleted and clean-up.
	for _, testName := range testNames {
		if err := deleteContour(ctx, kclient, timeout, testName, operatorNs); err != nil {
			t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
		}
		t.Logf("deleted contour %s/%s", operatorNs, testName)

		// Verify the user-defined namespace was removed by the operator.
		ns := fmt.Sprintf("%s-ns", testName)
		if err := waitForSpecNsDeletion(ctx, kclient, timeout, ns); err != nil {
			t.Fatalf("failed to observe the deletion of namespace %s: %v", ns, err)
		}
		t.Logf("observed the deletion of namespace %s", ns)
	}
}

func TestGateway(t *testing.T) {
	testName := "test-gateway"
	contourName := fmt.Sprintf("%s-contour", testName)
	gcName := "test-gatewayclass"
	cfg := objcontour.Config{
		Name:         contourName,
		Namespace:    operatorNs,
		SpecNs:       fmt.Sprintf("%s-gateway", specNs),
		NetworkType:  operatorv1alpha1.NodePortServicePublishingType,
		NodePorts:    objcontour.MakeNodePorts(map[string]int{"http": 30080, "https": 30443}),
		GatewayClass: &gcName,
	}

	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, contourName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := newOperatorGatewayClass(ctx, kclient, gcName, operatorNs, contourName); err != nil {
		t.Fatalf("failed to create gatewayclass %s: %v", gcName, err)
	}
	t.Logf("created gatewayclass %s", gcName)

	// The gatewayclass should now report admitted.
	if err := waitForGatewayClassStatusConditions(ctx, kclient, timeout, gcName, expectedGatewayClassConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gatewayclass %s: %v", gcName, err)
	}

	// Create the gateway namespace if it doesn't exist.
	if err := newNs(ctx, kclient, cfg.SpecNs); err != nil {
		t.Fatalf("failed to create namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("created namespace %s", cfg.SpecNs)

	// Create the gateway. The gateway must be projectcontour/contour until the following issue is fixed:
	// https://github.com/projectcontour/contour-operator/issues/241
	gwName := "contour"
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newGateway(ctx, kclient, cfg.SpecNs, gwName, gcName, "app", appName); err != nil {
		t.Fatalf("failed to create gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}
	t.Logf("created gateway %s/%s", cfg.SpecNs, gwName)

	// The gateway should report admitted.
	if err := waitForGatewayStatusConditions(ctx, kclient, timeout, gwName, cfg.SpecNs, expectedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, timeout, contourName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cfg.Namespace, cfg.Name)

	// Create a sample workload for e2e testing.
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newHTTPRouteToSvc(ctx, kclient, appName, cfg.SpecNs, appName, "app", appName, "local.projectcontour.io", int32(80)); err != nil {
		t.Fatalf("failed to create httproute %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created httproute %s/%s", cfg.SpecNs, appName)

	// testURL is the url used to test e2e functionality.
	testURL := "http://local.projectcontour.io/"

	if isKind {
		if err := waitForHTTPResponse(testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	} else {
		// Get the IP of a worker node to test the nodeport service.
		ip, err := getWorkerNodeIP(ctx, kclient)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("using worker node ip %s", ip)

		// Curl the httproute from the client pod.
		testURL = fmt.Sprintf("http://%s:30080/", ip)
		cliName := "test-client"
		if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
			t.Fatalf("failed to receive http response for %q: %v", testURL, err)
		}
		t.Logf("received http response for %q", testURL)
	}

	// Ensure the gateway can be deleted and clean-up.
	if err := deleteGateway(ctx, kclient, timeout, gwName, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// Ensure the gatewayclass can be deleted and clean-up.
	if err := deleteGatewayClass(ctx, kclient, timeout, gcName); err != nil {
		t.Fatalf("failed to delete gatewayclass %s: %v", gcName, err)
	}

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}
	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, timeout, cfg.SpecNs, "envoy"); err != nil {
		t.Fatalf("failed to delete envoy service %s/envoy: %v", cfg.SpecNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", cfg.SpecNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

func TestGatewayClusterIP(t *testing.T) {
	testName := "test-clusterip-gateway"
	contourName := fmt.Sprintf("%s-contour", testName)
	gcName := "test-gatewayclass"
	cfg := objcontour.Config{
		Name:         contourName,
		Namespace:    operatorNs,
		SpecNs:       fmt.Sprintf("%s-gateway-clusterip", specNs),
		NetworkType:  operatorv1alpha1.ClusterIPServicePublishingType,
		GatewayClass: &gcName,
	}

	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, contourName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := newOperatorGatewayClass(ctx, kclient, gcName, operatorNs, contourName); err != nil {
		t.Fatalf("failed to create gatewayclass %s: %v", gcName, err)
	}
	t.Logf("created gatewayclass %s", gcName)

	// The gatewayclass should now report admitted.
	if err := waitForGatewayClassStatusConditions(ctx, kclient, timeout, gcName, expectedGatewayClassConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gatewayclass %s: %v", gcName, err)
	}

	// Create the gateway namespace if it doesn't exist.
	if err := newNs(ctx, kclient, cfg.SpecNs); err != nil {
		t.Fatalf("failed to create namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("created namespace %s", cfg.SpecNs)

	// Create the gateway. The gateway must be projectcontour/contour until the following issue is fixed:
	// https://github.com/projectcontour/contour-operator/issues/241
	gwName := "contour"
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newGateway(ctx, kclient, cfg.SpecNs, gwName, gcName, "app", appName); err != nil {
		t.Fatalf("failed to create gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}
	t.Logf("created gateway %s/%s", cfg.SpecNs, gwName)

	// The gateway should report admitted.
	if err := waitForGatewayStatusConditions(ctx, kclient, timeout, gwName, cfg.SpecNs, expectedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, timeout, contourName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", operatorNs, testName)

	// Create a sample workload for e2e testing.
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newHTTPRouteToSvc(ctx, kclient, appName, cfg.SpecNs, appName, "app", appName, "local.projectcontour.io", int32(80)); err != nil {
		t.Fatalf("failed to create httproute %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created httproute %s/%s", cfg.SpecNs, appName)

	// Get the Envoy ClusterIP to curl.
	svcName := "envoy"
	ip, err := envoyClusterIP(ctx, kclient, cfg.SpecNs, svcName)
	if err != nil {
		t.Fatalf("failed to get clusterIP for service %s/%s: %v", cfg.SpecNs, svcName, err)
	}

	// Curl the httproute from the client pod.
	testURL := fmt.Sprintf("http://%s/", ip)
	cliName := "test-client"
	if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Ensure the gateway can be deleted and clean-up.
	if err := deleteGateway(ctx, kclient, timeout, gwName, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// Ensure the gatewayclass can be deleted and clean-up.
	if err := deleteGatewayClass(ctx, kclient, timeout, gcName); err != nil {
		t.Fatalf("failed to delete gatewayclass %s: %v", gcName, err)
	}

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, timeout, cfg.SpecNs, "envoy"); err != nil {
		t.Fatalf("failed to delete envoy service %s/envoy: %v", cfg.SpecNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", cfg.SpecNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

// TestGatewayOwnership ensures the operator only manages Gateway resources that it owns,
// i.e. a gatewayclass that specifies the controller as the operator.
func TestGatewayOwnership(t *testing.T) {
	testName := "test-gateway-owned"
	contourName := fmt.Sprintf("%s-contour", testName)
	gcName := "test-gatewayclass-owned"
	cfg := objcontour.Config{
		Name:         contourName,
		Namespace:    operatorNs,
		SpecNs:       fmt.Sprintf("%s-gateway-ownership", specNs),
		NetworkType:  operatorv1alpha1.NodePortServicePublishingType,
		NodePorts:    objcontour.MakeNodePorts(map[string]int{"http": 30080, "https": 30443}),
		GatewayClass: &gcName,
	}

	nonOwnedClass := "test-gatewayclass-not-owned"
	// Create Gateway API resources that should not be managed by the operator.
	if err := newGatewayClass(ctx, kclient, nonOwnedClass); err != nil {
		t.Fatalf("failed to create gatewayclass %s: %v", nonOwnedClass, err)
	}
	t.Logf("created gatewayclass %s", nonOwnedClass)

	// The gatewayclass should not report admitted.
	if err := waitForGatewayClassStatusConditions(ctx, kclient, timeout, nonOwnedClass, expectedNonOwnedGatewayClassConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gatewayclass %s: %v", nonOwnedClass, err)
	}

	// Create the namespace used by the non-owned gateway
	if err := newNs(ctx, kclient, cfg.SpecNs); err != nil {
		t.Fatalf("failed to create namespace %s: %v", cfg.SpecNs, err)
	}

	nonOwnedGateway := "other-vendor"
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newGateway(ctx, kclient, cfg.SpecNs, nonOwnedGateway, nonOwnedClass, "app", appName); err != nil {
		t.Fatalf("failed to create gateway %s/%s: %v", cfg.SpecNs, nonOwnedGateway, err)
	}
	t.Logf("created gateway %s/%s", cfg.SpecNs, nonOwnedGateway)

	// The gateway should not report scheduled.
	if err := waitForGatewayStatusConditions(ctx, kclient, timeout, nonOwnedGateway, cfg.SpecNs, expectedNonOwnedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, nonOwnedGateway, err)
	}

	// Create the Contour and Gateway API resources that should be managed by the operator.
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, contourName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := newOperatorGatewayClass(ctx, kclient, gcName, operatorNs, contourName); err != nil {
		t.Fatalf("failed to create gatewayclass %s: %v", gcName, err)
	}
	t.Logf("created gatewayclass %s", gcName)

	// The gatewayclass should now report admitted.
	if err := waitForGatewayClassStatusConditions(ctx, kclient, timeout, gcName, expectedGatewayClassConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gatewayclass %s: %v", gcName, err)
	}

	// Create the gateway. The gateway must be projectcontour/contour until the following issue is fixed:
	// https://github.com/projectcontour/contour-operator/issues/241
	gwName := "contour"
	if err := newGateway(ctx, kclient, cfg.SpecNs, gwName, gcName, "app", appName); err != nil {
		t.Fatalf("failed to create gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}
	t.Logf("created gateway %s/%s", cfg.SpecNs, gwName)

	// The gateway should report admitted.
	if err := waitForGatewayStatusConditions(ctx, kclient, timeout, gwName, cfg.SpecNs, expectedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, timeout, contourName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", operatorNs, testName)

	gateways := []string{nonOwnedGateway, gwName}
	for _, gw := range gateways {
		// Ensure the gateway can be deleted and clean-up.
		if err := deleteGateway(ctx, kclient, timeout, gw, cfg.SpecNs); err != nil {
			t.Fatalf("failed to delete gateway %s/%s: %v", cfg.SpecNs, gw, err)
		}
	}

	classes := []string{nonOwnedClass, gcName}
	for _, class := range classes {
		// Ensure the gatewayclass can be deleted and clean-up.
		if err := deleteGatewayClass(ctx, kclient, timeout, class); err != nil {
			t.Fatalf("failed to delete gatewayclass %s: %v", class, err)
		}
	}

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, timeout, cfg.SpecNs, "envoy"); err != nil {
		t.Fatalf("failed to delete envoy service %s/envoy: %v", cfg.SpecNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", cfg.SpecNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, timeout, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}

// TestOperatorUpgrade tests an instance of the Contour custom resource while
// upgrading the operator from release "latest" to the current version/branch.
func TestOperatorUpgrade(t *testing.T) {
	// Get the current image to use for upgrade testing.
	current, err := getDeploymentImage(ctx, kclient, operatorName, operatorNs, operatorName)
	if err != nil {
		t.Fatalf("failed to get image for deployment %s/%s", operatorNs, operatorName)
	}
	// Ensure the current image is not the "latest" release.
	latest := "docker.io/projectcontour/contour-operator:latest"
	if current == latest {
		t.Fatalf("unexpected image %s for deployment %s/%s", current, operatorNs, operatorName)
	}
	t.Logf("found image %s for deployment %s/%s", current, operatorNs, operatorName)

	// Set the operator image as "latest" to simulate the previous release.
	if err := setDeploymentImage(ctx, kclient, operatorName, operatorNs, operatorName, latest); err != nil {
		t.Fatalf("failed to set image %s for deployment %s/%s: %v", latest, operatorNs, operatorName, err)
	}
	t.Logf("set image %s for deployment %s/%s", latest, operatorNs, operatorName)

	// Wait for the operator container to use image "latest".
	if err := waitForImage(ctx, kclient, timeout, operatorNs, "control-plane", operatorName, operatorName, latest); err != nil {
		t.Fatalf("failed to observe image %s for deployment %s/%s: %v", latest, operatorNs, operatorName, err)
	}
	t.Logf("observed image %s for deployment %s/%s", latest, operatorNs, operatorName)

	// Ensure the operator's deployment becomes available before proceeding.
	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, operatorName, operatorNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected conditions for deployment %s/%s: %v", operatorNs, operatorName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", operatorNs, operatorName)

	testName := "upgrade"
	contourName := fmt.Sprintf("%s-contour", testName)
	cfg := objcontour.Config{
		Name:        contourName,
		Namespace:   operatorNs,
		RemoveNs:    true,
		SpecNs:      fmt.Sprintf("%s-%s", testName, specNs),
		NetworkType: operatorv1alpha1.ClusterIPServicePublishingType,
	}

	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, timeout, cfg.Name, cfg.Namespace, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cfg.Namespace, cfg.Name)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	// Wait for the sample workload to become available.
	if err := waitForDeploymentStatusConditions(ctx, kclient, timeout, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", cfg.SpecNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, cfg.SpecNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created service %s/%s", cfg.SpecNs, appName)

	if err := newIngress(ctx, kclient, appName, cfg.SpecNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created ingress %s/%s", cfg.SpecNs, appName)

	// Get the Envoy ClusterIP to curl.
	svcName := "envoy"
	ip, err := envoyClusterIP(ctx, kclient, cfg.SpecNs, svcName)
	if err != nil {
		t.Fatalf("failed to get clusterIP for service %s/%s: %v", cfg.SpecNs, svcName, err)
	}

	// Create the client pod to test connectivity to the sample workload.
	cliName := "test-client"
	testURL := fmt.Sprintf("http://%s/", ip)
	if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Simulate an upgrade from the previous release, i.e. image "latest", to the current release.
	if err := setDeploymentImage(ctx, kclient, operatorName, operatorNs, operatorName, current); err != nil {
		t.Fatalf("failed to set image %s for deployment %s/%s: %v", latest, operatorNs, operatorName, err)
	}
	t.Logf("set image %s for deployment %s/%s", current, operatorNs, operatorName)

	// Wait for the operator container to use the updated image.
	if err := waitForImage(ctx, kclient, timeout, operatorNs, "control-plane", operatorName, operatorName, current); err != nil {
		t.Fatalf("failed to observe image %s for deployment %s/%s: %v", current, operatorNs, operatorName, err)
	}
	t.Logf("observed image %s for deployment %s/%s", current, operatorNs, operatorName)

	// Wait for the contour containers to use the current tag.
	wantContourImage := config.DefaultContourImage
	if err := waitForImage(ctx, kclient, timeout, cfg.SpecNs, "app", "contour", "contour", wantContourImage); err != nil {
		t.Fatalf("failed to observe image %s for deployment %s/contour: %v", wantContourImage, cfg.SpecNs, err)
	}
	t.Logf("observed image %s for deployment %s/contour", wantContourImage, cfg.SpecNs)

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, timeout, cfg.Name, cfg.Namespace, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cfg.Namespace, cfg.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cfg.Namespace, cfg.Name)

	if err := podWaitForHTTPResponse(ctx, kclient, cfg.SpecNs, cliName, testURL, timeout); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, timeout, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}
}
