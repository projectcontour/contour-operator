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

	"github.com/projectcontour/contour-operator/internal/config"
	"github.com/projectcontour/contour-operator/internal/operator"
	"github.com/projectcontour/contour-operator/internal/parse"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	opCfg config.Config
)

func main() {
	flag.StringVar(&opCfg.ContourImage, "contour-image", config.DefaultContourImage,
		"The container image used for the managed Contour.")
	flag.StringVar(&opCfg.EnvoyImage, "envoy-image", config.DefaultEnvoyImage,
		"The container image used for the managed Envoy.")
	flag.StringVar(&opCfg.MetricsBindAddress, "metrics-addr", config.DefaultMetricsAddr, "The "+
		"address the metric endpoint binds to. It can be set to \"0\" to disable serving metrics.")
	flag.BoolVar(&opCfg.LeaderElection, "enable-leader-election", config.DefaultEnableLeaderElection,
		"Enable leader election for the operator. Enabling this will ensure there is only one active operator.")
	flag.Parse()

	opCfg.LeaderElectionID = config.DefaultEnableLeaderElectionID

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	setupLog := ctrl.Log.WithName("setup")

	images := []string{opCfg.ContourImage, opCfg.EnvoyImage}
	for _, image := range images {
		// Parse will not handle short digests.
		if err := parse.Image(image); err != nil {
			setupLog.Error(err, "invalid image reference", "value", image)
			os.Exit(1)
		}
	}

	setupLog.Info("using contour", "image", opCfg.ContourImage)
	setupLog.Info("using envoy", "image", opCfg.EnvoyImage)

	op, err := operator.New(ctrl.GetConfigOrDie(), &opCfg)
	if err != nil {
		setupLog.Error(err, "failed to create contour operator")
		os.Exit(1)
	}

	setupLog.Info("starting contour operator")
	if err := op.Start(ctrl.SetupSignalHandler(), &opCfg); err != nil {
		setupLog.Error(err, "failed to start contour operator")
		os.Exit(1)
	}
}
