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
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	"github.com/projectcontour/contour-operator/internal/parse"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

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
	// testURL is the url used to test e2e functionality.
	testURL = "http://local.projectcontour.io/"
	// expectedDeploymentConditions are the expected status conditions of a
	// deployment.
	expectedDeploymentConditions = []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
	}
	// expectedPodConditions are the expected status conditions of a pod.
	expectedPodConditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
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
	// opLogMsg is the string used to search operator log messages.
	opLogMsg = "error"
)

func TestMain(m *testing.M) {
	cl, err := newClient()
	if err != nil {
		os.Exit(1)
	}
	kclient = cl

	os.Exit(m.Run())
}

func TestOperatorDeploymentAvailable(t *testing.T) {
	t.Helper()
	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, operatorName, operatorNs, expectedDeploymentConditions...); err != nil {
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

	svcName := "envoy"
	if err := updateLbSvcIPAndNodePorts(ctx, kclient, 1*time.Minute, specNs, svcName); err != nil {
		t.Fatalf("failed to update service %s/%s: %v", specNs, svcName, err)
	}
	t.Logf("updated service %s/%s loadbalancer IP and nodeports", specNs, svcName)

	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, specNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created deployment %s/%s", specNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, specNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", specNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, specNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created service %s/%s", specNs, appName)

	if err := newIngress(ctx, kclient, appName, specNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created ingress %s/%s", specNs, appName)

	if err := waitForHTTPResponse(testURL, 1*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Scrape the operator logs for error messages.
	found, err := parse.DeploymentLogsForString(operatorNs, operatorName, operatorName, opLogMsg)
	switch {
	case err != nil:
		t.Fatalf("failed to look for string in operator %s/%s logs: %v", operatorNs, operatorName, err)
	case found:
		t.Fatalf("found %s message in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	default:
		t.Logf("no %s message observed in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, 3*time.Minute, specNs, "envoy"); err != nil {
		t.Fatalf("failed to delete contour %s/envoy: %v", specNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", specNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, specNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", specNs, err)
	}
	t.Logf("observed the deletion of namespace %s", specNs)
}

func TestContourNodePortService(t *testing.T) {
	testName := "test-nodeport-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      specNs,
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.NodePortServicePublishingType,
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, cntr.Name, cntr.Namespace, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cntr.Namespace, cntr.Name)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, specNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created deployment %s/%s", specNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, specNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", specNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, specNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created service %s/%s", specNs, appName)

	if err := newIngress(ctx, kclient, appName, specNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created ingress %s/%s", specNs, appName)

	if err := waitForHTTPResponse(testURL, 1*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Scrape the operator logs for error messages.
	found, err := parse.DeploymentLogsForString(operatorNs, operatorName, operatorName, opLogMsg)
	switch {
	case err != nil:
		t.Fatalf("failed to look for string in operator %s/%s logs: %v", operatorNs, operatorName, err)
	case found:
		t.Fatalf("found %s message in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	default:
		t.Logf("no %s message observed in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, 3*time.Minute, specNs, "envoy"); err != nil {
		t.Fatalf("failed to delete contour %s/envoy: %v", specNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", specNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, specNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", specNs, err)
	}
	t.Logf("observed the deletion of namespace %s", specNs)
}

func TestContourClusterIPService(t *testing.T) {
	testName := "test-clusterip-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      specNs,
		RemoveNs:    false,
		NetworkType: operatorv1alpha1.ClusterIPServicePublishingType,
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, cntr.Name, cntr.Namespace, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", cntr.Namespace, cntr.Name, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", cntr.Namespace, cntr.Name)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, specNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created deployment %s/%s", specNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, specNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", specNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, specNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created service %s/%s", specNs, appName)

	if err := newIngress(ctx, kclient, appName, specNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created ingress %s/%s", specNs, appName)

	// Create the client Pod.
	cliName := "test-client"
	cliPod, err := newPod(ctx, kclient, specNs, cliName, "curlimages/curl:7.75.0", []string{"sleep", "600"})
	if err != nil {
		t.Fatalf("failed to create pod %s/%s: %v", specNs, cliName, err)
	}
	if err := waitForPodStatusConditions(ctx, kclient, 1*time.Minute, cliPod.Namespace, cliPod.Name, expectedPodConditions...); err != nil {
		t.Fatalf("failed to observe expected conditions for pod %s/%s: %v", cliPod.Namespace, cliPod.Name, err)
	}
	t.Logf("observed expected status conditions for pod %s/%s", cliPod.Namespace, cliPod.Name)

	// Get the Envoy ClusterIP to curl.
	svcName := "envoy"
	ip, err := envoyClusterIP(ctx, kclient, specNs, svcName)
	if err != nil {
		t.Fatalf("failed to get clusterIP for service %s/%s: %v", specNs, svcName, err)
	}

	// Curl the ingress from the client pod.
	url := fmt.Sprintf("http://%s/", ip)
	host := fmt.Sprintf("Host: %s", "local.projectcontour.io")
	cmd := []string{"curl", "-H", host, "-s", "-w", "%{http_code}", url}
	resp := "200"
	if err := parse.StringInPodExec(specNs, cliName, resp, cmd); err != nil {
		t.Fatalf("failed to parse pod %s/%s: %v", specNs, cliName, err)
	}
	t.Logf("received http %s response for %s in pod %s/%s", resp, url, specNs, cliName)

	// Scrape the operator logs for error messages.
	found, err := parse.DeploymentLogsForString(operatorNs, operatorName, operatorName, opLogMsg)
	switch {
	case err != nil:
		t.Fatalf("failed to look for string in operator %s/%s logs: %v", operatorNs, operatorName, err)
	case found:
		t.Fatalf("found %s message in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	default:
		t.Logf("no %s message observed in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, 3*time.Minute, specNs, "envoy"); err != nil {
		t.Fatalf("failed to delete contour %s/envoy: %v", specNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", specNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, specNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", specNs, err)
	}
	t.Logf("observed the deletion of namespace %s", specNs)
}

// TestContourSpec tests some spec changes such as:
// - Enable RemoveNs.
// - Initial replicas to 4.
// - Increase replicas to 3.
func TestContourSpec(t *testing.T) {
	testName := "test-user-contour"
	cfg := objcontour.Config{
		Name:        testName,
		Namespace:   operatorNs,
		SpecNs:      specNs,
		RemoveNs:    true,
		Replicas:    4,
		NetworkType: operatorv1alpha1.NodePortServicePublishingType,
	}
	cntr, err := newContour(ctx, kclient, cfg)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s", testAppName, testName)
	if err := newDeployment(ctx, kclient, appName, specNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created deployment %s/%s", specNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, specNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", specNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", specNs, appName)

	cfg.Replicas = 3
	if _, err := updateContour(ctx, kclient, cfg); err != nil {
		t.Fatalf("failed to update contour %s/%s: %v", operatorNs, testName, err)
	}
	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	if err := newClusterIPService(ctx, kclient, appName, specNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created service %s/%s", specNs, appName)

	if err := newIngress(ctx, kclient, appName, specNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", specNs, appName, err)
	}
	t.Logf("created ingress %s/%s", specNs, appName)

	if err := waitForHTTPResponse(testURL, 1*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// Scrape the operator logs for error messages.
	found, err := parse.DeploymentLogsForString(operatorNs, operatorName, operatorName, opLogMsg)
	switch {
	case err != nil:
		t.Fatalf("failed to look for string in operator %s/%s logs: %v", operatorNs, operatorName, err)
	case found:
		t.Fatalf("found %s message in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	default:
		t.Logf("no %s message observed in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	}

	// Ensure the default contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, testName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("deleted contour %s/%s", operatorNs, testName)

	// Verify the user-defined namespace was removed by the operator.
	if err := waitForSpecNsDeletion(ctx, kclient, 5*time.Minute, specNs); err != nil {
		t.Fatalf("failed to observe the deletion of namespace %s: %v", specNs, err)
	}
	t.Logf("observed the deletion of namespace %s", specNs)
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

		if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, testName, operatorNs, expectedContourConditions...); err != nil {
			t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
		}
		t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)
	}

	// Scrape the operator logs for error messages.
	found, err := parse.DeploymentLogsForString(operatorNs, operatorName, operatorName, opLogMsg)
	switch {
	case err != nil:
		t.Fatalf("failed to look for string in operator %s/%s logs: %v", operatorNs, operatorName, err)
	case found:
		t.Fatalf("found %s message in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	default:
		t.Logf("no %s message observed in operator %s/%s logs", opLogMsg, operatorNs, operatorName)
	}

	// Ensure the default contour can be deleted and clean-up.
	for _, testName := range testNames {
		if err := deleteContour(ctx, kclient, 3*time.Minute, testName, operatorNs); err != nil {
			t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, testName, err)
		}
		t.Logf("deleted contour %s/%s", operatorNs, testName)

		// Verify the user-defined namespace was removed by the operator.
		specNs = fmt.Sprintf("%s-ns", testName)
		if err := waitForSpecNsDeletion(ctx, kclient, 5*time.Minute, specNs); err != nil {
			t.Fatalf("failed to observe the deletion of namespace %s: %v", specNs, err)
		}
		t.Logf("observed the deletion of namespace %s", specNs)
	}
}

func TestGateway(t *testing.T) {
	testName := "test-gateway"
	contourName := fmt.Sprintf("%s-contour", testName)
	gcName := "test-gatewayclass"
	cfg := objcontour.Config{
		Name:         contourName,
		Namespace:    operatorNs,
		SpecNs:       specNs,
		NetworkType:  operatorv1alpha1.NodePortServicePublishingType,
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
	if err := waitForGatewayClassStatusConditions(ctx, kclient, 1*time.Minute, gcName, expectedGatewayClassConditions...); err != nil {
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
	if err := waitForGatewayStatusConditions(ctx, kclient, 3*time.Minute, gwName, cfg.SpecNs, expectedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, 1*time.Minute, contourName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	// Create a sample workload for e2e testing.
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
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

	if err := waitForHTTPResponse(testURL, 3*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testURL, err)
	}
	t.Logf("received http response for %q", testURL)

	// TODO [danehans]: Scrape operator logs for error messages before proceeding.
	// xref: https://github.com/projectcontour/contour-operator/issues/211

	// Ensure the gateway can be deleted and clean-up.
	if err := deleteGateway(ctx, kclient, 3*time.Minute, gwName, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// Ensure the gatewayclass can be deleted and clean-up.
	if err := deleteGatewayClass(ctx, kclient, 3*time.Minute, gcName); err != nil {
		t.Fatalf("failed to delete gatewayclass %s: %v", gcName, err)
	}

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}
	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, 3*time.Minute, specNs, "envoy"); err != nil {
		t.Fatalf("failed to delete contour %s/envoy: %v", specNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", specNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, cfg.SpecNs); err != nil {
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
		SpecNs:       specNs,
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
	if err := waitForGatewayClassStatusConditions(ctx, kclient, 1*time.Minute, gcName, expectedGatewayClassConditions...); err != nil {
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
	if err := waitForGatewayStatusConditions(ctx, kclient, 3*time.Minute, gwName, cfg.SpecNs, expectedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, 1*time.Minute, contourName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	// Create a sample workload for e2e testing.
	if err := newDeployment(ctx, kclient, appName, cfg.SpecNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", cfg.SpecNs, appName, err)
	}
	t.Logf("created deployment %s/%s", cfg.SpecNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, cfg.SpecNs, expectedDeploymentConditions...); err != nil {
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

	// Create the client Pod.
	cliName := "test-client"
	cliPod, err := newPod(ctx, kclient, specNs, cliName, "curlimages/curl:7.75.0", []string{"sleep", "600"})
	if err != nil {
		t.Fatalf("failed to create pod %s/%s: %v", specNs, cliName, err)
	}
	if err := waitForPodStatusConditions(ctx, kclient, 1*time.Minute, cliPod.Namespace, cliPod.Name, expectedPodConditions...); err != nil {
		t.Fatalf("failed to observe expected conditions for pod %s/%s: %v", cliPod.Namespace, cliPod.Name, err)
	}
	t.Logf("observed expected status conditions for pod %s/%s", cliPod.Namespace, cliPod.Name)

	// Get the Envoy ClusterIP to curl.
	svcName := "envoy"
	ip, err := envoyClusterIP(ctx, kclient, specNs, svcName)
	if err != nil {
		t.Fatalf("failed to get clusterIP for service %s/%s: %v", specNs, svcName, err)
	}

	// Curl the ingress from the client pod.
	url := fmt.Sprintf("http://%s/", ip)
	host := fmt.Sprintf("Host: %s", "local.projectcontour.io")
	cmd := []string{"curl", "-H", host, "-s", "-w", "%{http_code}", url}
	resp := "200"
	// Polling until success since network seems not ready.
	// It might be related to https://github.com/projectcontour/contour-operator/issues/296
	// TODO: remove wait.PollImmediate.
	err = wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		if err := parse.StringInPodExec(specNs, cliName, resp, cmd); err != nil {
			t.Logf("observed unexpected error: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("failed to get http %s response for %s in pod %s/%s: %v", resp, url, specNs, cliName, err)
	}

	t.Logf("received http %s response for %s in pod %s/%s", resp, url, specNs, cliName)

	// TODO [danehans]: Scrape operator logs for error messages before proceeding.
	// xref: https://github.com/projectcontour/contour-operator/issues/211

	// Ensure the gateway can be deleted and clean-up.
	if err := deleteGateway(ctx, kclient, 3*time.Minute, gwName, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// Ensure the gatewayclass can be deleted and clean-up.
	if err := deleteGatewayClass(ctx, kclient, 3*time.Minute, gcName); err != nil {
		t.Fatalf("failed to delete gatewayclass %s: %v", gcName, err)
	}

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, 3*time.Minute, specNs, "envoy"); err != nil {
		t.Fatalf("failed to delete contour %s/envoy: %v", specNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", specNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, cfg.SpecNs); err != nil {
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
		SpecNs:       specNs,
		NetworkType:  operatorv1alpha1.NodePortServicePublishingType,
		GatewayClass: &gcName,
	}

	nonOwnedClass := "test-gatewayclass-not-owned"
	// Create Gateway API resources that should not be managed by the operator.
	if err := newGatewayClass(ctx, kclient, nonOwnedClass); err != nil {
		t.Fatalf("failed to create gatewayclass %s: %v", nonOwnedClass, err)
	}
	t.Logf("created gatewayclass %s", nonOwnedClass)

	// The gatewayclass should not report admitted.
	if err := waitForGatewayClassStatusConditions(ctx, kclient, 5*time.Second, nonOwnedClass, expectedNonOwnedGatewayClassConditions...); err != nil {
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
	if err := waitForGatewayStatusConditions(ctx, kclient, 5*time.Second, nonOwnedGateway, cfg.SpecNs, expectedNonOwnedGatewayConditions...); err != nil {
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
	if err := waitForGatewayClassStatusConditions(ctx, kclient, 1*time.Minute, gcName, expectedGatewayClassConditions...); err != nil {
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
	if err := waitForGatewayStatusConditions(ctx, kclient, 3*time.Minute, gwName, cfg.SpecNs, expectedGatewayConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for gateway %s/%s: %v", cfg.SpecNs, gwName, err)
	}

	// The contour should now report available.
	if err := waitForContourStatusConditions(ctx, kclient, 1*time.Minute, contourName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	gateways := []string{nonOwnedGateway, gwName}
	for _, gw := range gateways {
		// Ensure the gateway can be deleted and clean-up.
		if err := deleteGateway(ctx, kclient, 3*time.Minute, gw, cfg.SpecNs); err != nil {
			t.Fatalf("failed to delete gateway %s/%s: %v", cfg.SpecNs, gw, err)
		}
	}

	classes := []string{nonOwnedClass, gcName}
	for _, class := range classes {
		// Ensure the gatewayclass can be deleted and clean-up.
		if err := deleteGatewayClass(ctx, kclient, 3*time.Minute, class); err != nil {
			t.Fatalf("failed to delete gatewayclass %s: %v", class, err)
		}
	}

	// Ensure the contour can be deleted and clean-up.
	if err := deleteContour(ctx, kclient, 3*time.Minute, contourName, operatorNs); err != nil {
		t.Fatalf("failed to delete contour %s/%s: %v", operatorNs, contourName, err)
	}

	// Ensure the envoy service is cleaned up automatically.
	if err := waitForServiceDeletion(ctx, kclient, 3*time.Minute, specNs, "envoy"); err != nil {
		t.Fatalf("failed to delete contour %s/envoy: %v", specNs, err)
	}
	t.Logf("cleaned up envoy service %s/envoy", specNs)

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, cfg.SpecNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", cfg.SpecNs, err)
	}
	t.Logf("observed the deletion of namespace %s", cfg.SpecNs)
}
