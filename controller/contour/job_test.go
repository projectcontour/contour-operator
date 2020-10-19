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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func checkJobHasEnvVar(t *testing.T, job *batchv1.Job, name string) {
	t.Helper()

	for _, envVar := range job.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == name {
			return
		}
	}
	t.Errorf("job is missing environment variable %q", name)
}

func checkJobHasContainer(t *testing.T, job *batchv1.Job, name string) *corev1.Container {
	t.Helper()

	for _, container := range job.Spec.Template.Spec.Containers {
		if container.Name == name {
			return &container
		}
	}
	t.Errorf("job is missing container %q", name)
	return nil
}

func TestDesiredJob(t *testing.T) {
	job, err := DesiredJob(cntr, contourImage)
	if err != nil {
		t.Errorf("invalid job: %w", err)
	}

	container := checkJobHasContainer(t, job, jobContainerName)
	checkContainerHasImage(t, container, contourImage)
	checkJobHasEnvVar(t, job, jobNsEnvVar)
}
