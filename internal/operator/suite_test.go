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

package operator

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	operatorconfig "github.com/projectcontour/contour-operator/internal/operator/config"
	contourcontroller "github.com/projectcontour/contour-operator/internal/operator/controller/contour"
	"github.com/projectcontour/contour-operator/internal/slice"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// Define utility constants for object names, testing timeouts/durations intervals, etc.
const (
	testContourName        = "test-contour"
	testOperatorNs         = "test-contour-operator"
	defaultNamespace       = "projectcontour"
	defaultGatewayClassRef = "None"
	defaultReplicas        = int32(2)

	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var (
	testEnv *envtest.Environment

	cntr = &operatorv1alpha1.Contour{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testContourName,
			Namespace: testOperatorNs,
		},
	}

	ctx       = context.Background()
	finalizer = contourcontroller.ContourFinalizer

	operator *Operator
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("Bootstrapping the test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	cliCfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cliCfg).ToNot(BeNil())

	opCfg := operatorconfig.NewConfig()
	operator, err = New(cliCfg, opCfg)
	Expect(err).ToNot(HaveOccurred())
	go func() {
		err = operator.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	By("Creating the operator namespace")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testOperatorNs}}
	err = operator.client.Create(context.Background(), ns)
	Expect(err).ToNot(HaveOccurred())
	close(done)
}, 60)

var _ = Describe("Run controller", func() {
	Context("When creating a contour", func() {
		It("Should set default fields", func() {
			By("By creating a contour with a nil spec")
			Expect(operator.client.Create(ctx, cntr)).Should(Succeed())

			key := types.NamespacedName{
				Namespace: cntr.Namespace,
				Name:      cntr.Name,
			}
			By("Expecting default replicas")
			Eventually(func() int32 {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Replicas
			}, timeout, interval).Should(Equal(defaultReplicas))

			By("Expecting default namespace")
			Eventually(func() string {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.Name
			}, timeout, interval).Should(Equal(defaultNamespace))

			By("Expecting default remove namespace on deletion")
			Eventually(func() bool {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.RemoveOnDeletion
			}, timeout, interval).Should(Equal(false))

			By("Expecting default network publishing type")
			Eventually(func() operatorv1alpha1.NetworkPublishingType {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.NetworkPublishing.Envoy.Type
			}, timeout, interval).Should(Equal(operatorv1alpha1.LoadBalancerServicePublishingType))

			By("Expecting default load balancer provider")
			Eventually(func() operatorv1alpha1.LoadBalancerProviderType {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type
			}, timeout, interval).Should(Equal(operatorv1alpha1.AWSLoadBalancerProvider))

			By("Expecting default load balancer scope")
			Eventually(func() operatorv1alpha1.LoadBalancerScope {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.NetworkPublishing.Envoy.LoadBalancer.Scope
			}, timeout, interval).Should(Equal(operatorv1alpha1.ExternalLoadBalancer))

			By("Expecting default gatewayClassRef")
			Eventually(func() string {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.GatewayClassRef
			}, timeout, interval).Should(Equal(defaultGatewayClassRef))

			// Update the contour
			By("By updating a contour spec")
			updated := &operatorv1alpha1.Contour{}
			Expect(operator.client.Get(ctx, key, updated)).Should(Succeed())
			updatedReplicas := int32(1)
			updatedNs := defaultNamespace + "-updated"
			updatedRemoveNs := true
			updated.Spec.Replicas = updatedReplicas
			updated.Spec.Namespace.Name = updatedNs
			updated.Spec.Namespace.RemoveOnDeletion = updatedRemoveNs
			Expect(operator.client.Update(ctx, updated)).Should(Succeed())

			By("Expecting replicas to be updated")
			Eventually(func() int32 {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Replicas
			}, timeout, interval).Should(Equal(updatedReplicas))

			By("Expecting namespace to be updated")
			Eventually(func() string {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.Name
			}, timeout, interval).Should(Equal(updatedNs))

			By("Expecting remove namespace on deletion to be updated")
			Eventually(func() bool {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Spec.Namespace.RemoveOnDeletion
			}, timeout, interval).Should(Equal(updatedRemoveNs))

			By("Expecting to delete contour successfully")
			Eventually(func() error {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return operator.client.Delete(ctx, f)
			}, timeout, interval).Should(Succeed())

			By("Expecting contour deletion to finish")
			Eventually(func() error {
				f := &operatorv1alpha1.Contour{}
				return operator.client.Get(ctx, key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
		It("Should be finalized", func() {
			finalizerSuffix := "-finalizer"

			key := types.NamespacedName{
				Name:      cntr.Name + finalizerSuffix,
				Namespace: cntr.Namespace,
			}

			By("By creating a contour")
			created := &operatorv1alpha1.Contour{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
				},
				Spec: operatorv1alpha1.ContourSpec{
					Namespace: operatorv1alpha1.NamespaceSpec{
						Name:             defaultNamespace + finalizerSuffix,
						RemoveOnDeletion: true,
					},
				},
			}
			Expect(operator.client.Create(ctx, created)).Should(Succeed())

			By("Expecting the contour finalizer")
			Eventually(func() []string {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Finalizers
			}, timeout, interval).Should(ContainElement(finalizer))

			By("Resetting the contour finalizer")
			updated := &operatorv1alpha1.Contour{}
			Expect(operator.client.Get(ctx, key, updated)).Should(Succeed())
			updated.Finalizers = slice.RemoveString(updated.Finalizers, finalizer)
			Expect(operator.client.Update(ctx, updated)).Should(Succeed())

			By("Expecting the contour to be re-finalized")
			Eventually(func() []string {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return f.Finalizers
			}, timeout, interval).Should(ContainElement(finalizer))

			By("Expecting to delete contour successfully")
			Eventually(func() error {
				f := &operatorv1alpha1.Contour{}
				Expect(operator.client.Get(ctx, key, f)).Should(Succeed())
				return operator.client.Delete(ctx, f)
			}, timeout, interval).Should(Succeed())

			By("Expecting contour delete to finish")
			Eventually(func() error {
				f := &operatorv1alpha1.Contour{}
				return operator.client.Get(ctx, key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	By("Tearing down the test environment")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testOperatorNs}}
	err := operator.client.Delete(context.Background(), ns)
	Expect(err).ToNot(HaveOccurred())

	By("Expecting the test environment teardown to complete")
	err = testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
