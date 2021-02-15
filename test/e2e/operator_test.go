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
	"github.com/projectcontour/contour-operator/internal/operator/config"
	"github.com/projectcontour/contour-operator/internal/parse"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// defaultContourNs is the default spec.namespace.name of a Contour.
	defaultContourNs = config.DefaultContourSpecNs
	// testUrl is the url used to test e2e functionality.
	testUrl = "http://local.projectcontour.io/"
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
	removeNs := false
	lb := operatorv1alpha1.LoadBalancerServicePublishingType
	cntr, err := newContour(ctx, kclient, testName, operatorNs, defaultContourNs, removeNs, lb)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	svcName := "envoy"
	if err := updateLbSvcIpAndNodePorts(ctx, kclient, 1*time.Minute, defaultContourNs, svcName); err != nil {
		t.Fatalf("failed to update service %s/%s: %v", defaultContourNs, svcName, err)
	}
	t.Logf("updated service %s/%s loadbalancer IP and nodeports", defaultContourNs, svcName)

	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s",testAppName, testName)
	if err:= newDeployment(ctx, kclient, appName, defaultContourNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("created deployment %s/%s", defaultContourNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, defaultContourNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", defaultContourNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, defaultContourNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("created service %s/%s", defaultContourNs, appName)

	if err := newIngress(ctx, kclient, appName, defaultContourNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("created ingress %s/%s", defaultContourNs, appName)

	if err := waitForHTTPResponse(testUrl, 1*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testUrl, err)
	}
	t.Logf("received http response for %q", testUrl)

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

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, defaultContourNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", defaultContourNs, err)
	}
}

func TestContourNodePortService(t *testing.T) {
	testName := "test-nodeport-contour"
	nodePort := operatorv1alpha1.NodePortServicePublishingType
	cntr, err := newContour(ctx, kclient, testName, operatorNs, defaultContourNs, false, nodePort)
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
	if err:= newDeployment(ctx, kclient, appName, defaultContourNs, testAppImage, testAppReplicas); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("created deployment %s/%s", defaultContourNs, appName)

	if err := waitForDeploymentStatusConditions(ctx, kclient, 3*time.Minute, appName, defaultContourNs, expectedDeploymentConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for deployment %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("observed expected status conditions for deployment %s/%s", defaultContourNs, appName)

	if err := newClusterIPService(ctx, kclient, appName, defaultContourNs, 80, 8080); err != nil {
		t.Fatalf("failed to create service %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("created service %s/%s", defaultContourNs, appName)

	if err := newIngress(ctx, kclient, appName, defaultContourNs, appName, 80); err != nil {
		t.Fatalf("failed to create ingress %s/%s: %v", defaultContourNs, appName, err)
	}
	t.Logf("created ingress %s/%s", defaultContourNs, appName)

	if err := waitForHTTPResponse(testUrl, 1*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testUrl, err)
	}
	t.Logf("received http response for %q", testUrl)

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

	// Delete the operand namespace since contour.spec.namespace.removeOnDeletion
	// defaults to false.
	if err := deleteNamespace(ctx, kclient, 5*time.Minute, defaultContourNs); err != nil {
		t.Fatalf("failed to delete namespace %s: %v", defaultContourNs, err)
	}
}

func TestContourSpecNs(t *testing.T) {
	testName := "test-user-contour"
	specNs := fmt.Sprintf("%s-%s", defaultContourNs, testName)
	removeNs := true
	nodePort := operatorv1alpha1.NodePortServicePublishingType
	cntr, err := newContour(ctx, kclient, testName, operatorNs, specNs, removeNs, nodePort)
	if err != nil {
		t.Fatalf("failed to create contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("created contour %s/%s", cntr.Namespace, cntr.Name)

	if err := waitForContourStatusConditions(ctx, kclient, 5*time.Minute, testName, operatorNs, expectedContourConditions...); err != nil {
		t.Fatalf("failed to observe expected status conditions for contour %s/%s: %v", operatorNs, testName, err)
	}
	t.Logf("observed expected status conditions for contour %s/%s", testName, operatorNs)

	// Create a sample workload for e2e testing.
	appName := fmt.Sprintf("%s-%s",testAppName, testName)
	if err:= newDeployment(ctx, kclient, appName, specNs, testAppImage, testAppReplicas); err != nil {
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

	if err := waitForHTTPResponse(testUrl, 1*time.Minute); err != nil {
		t.Fatalf("failed to receive http response for %q: %v", testUrl, err)
	}
	t.Logf("received http response for %q", testUrl)

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

	// Verify the user-defined namespace was removed by the operator.
	if err := waitForSpecNsDeletion(ctx, kclient, 5*time.Minute, specNs); err != nil {
		t.Fatalf("failed to observe the deletion of namespace %s: %v", specNs, err)
	}
	t.Logf("observed the deletion of namespace %s", specNs)
}
