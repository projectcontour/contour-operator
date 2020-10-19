package contour

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func checkDeploymentHasEnvVar(t *testing.T, deploy *appsv1.Deployment, name string) {
	t.Helper()

	for _, envVar := range deploy.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == name {
			return
		}
	}
	t.Errorf("deployment is missing environment variable %q", name)
}

func checkDeploymentHasContainer(t *testing.T, deploy *appsv1.Deployment, name string, expect bool) *corev1.Container {
	t.Helper()

	if deploy.Spec.Template.Spec.Containers == nil {
		t.Error("deployment has no containers")
	}

	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == name {
			if expect {
				return &container
			}
			t.Errorf("deployment has unexpected %q container", name)
		}
	}
	if expect {
		t.Errorf("deployment has no %q container", name)
	}
	return nil
}

func checkDeploymentHasLabels(t *testing.T, deploy *appsv1.Deployment, expected map[string]string) {
	t.Helper()

	if apiequality.Semantic.DeepEqual(deploy.Labels, expected) {
		return
	}

	t.Errorf("deployment has unexpected %q labels", deploy.Labels)
}

func TestDesiredDeployment(t *testing.T) {
	deploy, err := DesiredDeployment(cntr, contourImage)
	if err != nil {
		t.Errorf("invalid deployment: %w", err)
	}

	container := checkDeploymentHasContainer(t, deploy, contourContainerName, true)
	checkContainerHasImage(t, container, contourImage)
	checkDeploymentHasEnvVar(t, deploy, contourNsEnvVar)
	checkDeploymentHasEnvVar(t, deploy, contourPodEnvVar)
	checkDeploymentHasLabels(t, deploy, deploy.Labels)
}
