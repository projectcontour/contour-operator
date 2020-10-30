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
	"context"
	"testing"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Run controller", func() {

	Context("When creating a contour", func() {
		It("Defaults should be set", func() {
			ctx := context.Background()

			key := types.NamespacedName{
				Name:      cntr.Name,
				Namespace: cntr.Namespace,
			}

			By("By creating a contour with a nil spec")
			created := &operatorv1alpha1.Contour{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
			}
			Expect(k8sClient.Create(ctx, created)).Should(Succeed())

			By("Expecting default replicas")
			Eventually(func() int32 {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Replicas
			}, timeout, interval).Should(Equal(defaultReplicas))

			By("Expecting default namespace")
			Eventually(func() string {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.Name
			}, timeout, interval).Should(Equal(defaultNamespace))

			By("Expecting default remove namespace on deletion")
			Eventually(func() bool {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.RemoveOnDeletion
			}, timeout, interval).Should(Equal(false))

			// Update the contour
			By("By updating a contour spec")
			updated := &operatorv1alpha1.Contour{}
			Expect(k8sClient.Get(ctx, key, updated)).Should(Succeed())
			updatedReplicas := int32(1)
			updatedNs := defaultNamespace + "-updated"
			updatedRemoveNs := true
			updated.Spec.Replicas = updatedReplicas
			updated.Spec.Namespace.Name = updatedNs
			updated.Spec.Namespace.RemoveOnDeletion = updatedRemoveNs
			Expect(k8sClient.Update(ctx, updated)).Should(Succeed())

			By("Expecting replicas to be updated")
			Eventually(func() int32 {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Replicas
			}, timeout, interval).Should(Equal(updatedReplicas))

			By("Expecting namespace to be updated")
			Eventually(func() string {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.Name
			}, timeout, interval).Should(Equal(updatedNs))

			By("Expecting remove namespace on deletion to be updated")
			Eventually(func() bool {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.RemoveOnDeletion
			}, timeout, interval).Should(Equal(updatedRemoveNs))

			By("Expecting to delete contour successfully")
			Eventually(func() error {
				f := &operatorv1alpha1.Contour{}
				Expect(k8sClient.Get(ctx, key, f)).Should(Succeed())
				return k8sClient.Delete(ctx, f)
			}, timeout, interval).Should(Succeed())

			By("Expecting contour deletion to finish")
			Eventually(func() error {
				f := &operatorv1alpha1.Contour{}
				return k8sClient.Get(ctx, key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})

func checkContainerHasImage(t *testing.T, container *corev1.Container, image string) {
	t.Helper()

	if container.Image == image {
		return
	}
	t.Errorf("container is missing image %q", image)
}
