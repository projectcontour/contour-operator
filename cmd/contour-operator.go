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

package main

import (
	"flag"
	"os"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	contourcontroller "github.com/projectcontour/contour-operator/controller/contour"
	"github.com/projectcontour/contour-operator/controller/manager"
	oputil "github.com/projectcontour/contour-operator/util"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = operatorv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

var (
	contourImage         string
	envoyImage           string
	metricsAddr          string
	enableLeaderElection bool
)

func main() {
	flag.StringVar(&contourImage, "contour-image", "docker.io/projectcontour/contour:main",
		"The container image used for the managed Contour.")
	flag.StringVar(&envoyImage, "envoy-image", "docker.io/envoyproxy/envoy:v1.16.2",
		"The container image used for the managed Envoy.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	setupLog := ctrl.Log.WithName("setup")

	images := []string{contourImage, envoyImage}
	for _, image := range images {
		// Parse will not handle short digests.
		if err := oputil.ParseImage(image); err != nil {
			setupLog.Error(err, "invalid image reference", "value", image)
			os.Exit(1)
		}
	}

	mgrOpts := ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "0d879e31.projectcontour.io",
	}

	cntrCfg := contourcontroller.Config{
		ContourImage: contourImage,
		EnvoyImage:   envoyImage,
	}
	setupLog.Info("using contour", "image", cntrCfg.ContourImage)
	setupLog.Info("using envoy", "image", cntrCfg.EnvoyImage)

	mgr, err := manager.NewContourManager(mgrOpts, cntrCfg)
	if err != nil {
		setupLog.Error(err, "failed to create a manager")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting contour-operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "failed to start contour-operator")
		os.Exit(1)
	}
}
