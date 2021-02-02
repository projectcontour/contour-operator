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

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/operator/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Define utility constants for object names, testing timeouts/durations intervals, etc.
const (
	testContourName   = "test-contour"
	testContourNs     = testContourName + "-ns"
	testOperatorNs    = testContourName + "-operator"
	testContourSpecNs = config.DefaultContourSpecNs
	testContourImage  = config.DefaultContourImage
	testEnvoyImage    = config.DefaultEnvoyImage
)

var (
	cntr = &operatorv1alpha1.Contour{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testContourName,
			Namespace: testOperatorNs,
		},
	}

	ownerLabels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: cntr.Name,
		operatorv1alpha1.OwningContourNsLabel:   cntr.Namespace,
	}
)

func checkContainerHasImage(t *testing.T, container *corev1.Container, image string) {
	t.Helper()

	if container.Image == image {
		return
	}
	t.Errorf("container is missing image %q", image)
}
