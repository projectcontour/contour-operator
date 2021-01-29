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

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func checkNamespaceName(t *testing.T, ns *corev1.Namespace, expected string) {
	t.Helper()

	if ns.Name == expected {
		return
	}

	t.Errorf("namespace has unexpected name %q", ns.Name)
}

func checkNamespaceLabels(t *testing.T, ns *corev1.Namespace, expected map[string]string) {
	t.Helper()

	if apiequality.Semantic.DeepEqual(ns.Labels, expected) {
		return
	}

	t.Errorf("namespace has unexpected %q labels", ns.Labels)
}

func TestDesiredNamespace(t *testing.T) {
	ns := DesiredNamespace(cntr)
	checkNamespaceName(t, ns, cntr.Spec.Namespace.Name)
	checkNamespaceLabels(t, ns, ownerLabels)
}
