/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package contour

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	contourName       = "test-contour"
	operatorNamespace = "test-contour-operator"
	defaultNamespace  = "projectcontour"
	defaultReplicas   = int32(2)

	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	image     = "docker.io/projectcontour/contour:latest"
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

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = operatorv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	By("Creating the reconciler")
	err = (&Reconciler{
		Config: Config{
			Image: image,
		},
		Client: k8sClient,
		Log:    ctrl.Log.WithName("controllers").WithName("Contour"),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	By("Creating the operator namespace")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNamespace}}
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).ToNot(HaveOccurred())
	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("Tearing down the test environment")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNamespace}}
	err := k8sClient.Delete(context.Background(), ns)
	Expect(err).ToNot(HaveOccurred())

	By("Expecting the test environment teardown to complete")
	err = testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
