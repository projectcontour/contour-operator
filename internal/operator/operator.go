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
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	operatorconfig "github.com/projectcontour/contour-operator/internal/operator/config"
	contourcontroller "github.com/projectcontour/contour-operator/internal/operator/controller/contour"
	gwcontroller "github.com/projectcontour/contour-operator/internal/operator/controller/gateway"
	gccontroller "github.com/projectcontour/contour-operator/internal/operator/controller/gatewayclass"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	operatorName = "contour_operator"
)

// Operator is the scaffolding for the contour operator. It sets up dependencies
// and defines the topology of the operator and its managed components, wiring
// them together. Operator knows what specific resource types should produce
// operator events.
type Operator struct {
	client  client.Client
	manager manager.Manager
	log     logr.Logger
}

// +kubebuilder:rbac:groups=operator.projectcontour.io,resources=contours,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=operator.projectcontour.io,resources=contours/status,verbs=get;update;patch
// cert-gen needs create/update secrets.
// +kubebuilder:rbac:groups="",resources=namespaces;secrets;serviceaccounts;services,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=networking.x-k8s.io,resources=gatewayclasses;gateways;backendpolicies;httproutes;tlsroutes,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=networking.x-k8s.io,resources=gatewayclasses/status;gateways/status;backendpolicies/status;httproutes/status;tlsroutes/status,verbs=create;get;update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;ingressclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=create;get;update
// +kubebuilder:rbac:groups=projectcontour.io,resources=httpproxies;tlscertificatedelegations;extensionservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=projectcontour.io,resources=httpproxies/status;extensionservices/status,verbs=create;get;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;delete;create;update;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list

// New creates a new operator from cliCfg and opCfg.
func New(cliCfg *rest.Config, opCfg *operatorconfig.Config) (*Operator, error) {
	nonCached := []client.Object{&operatorv1alpha1.Contour{}, &gatewayv1alpha1.GatewayClass{},
		&gatewayv1alpha1.Gateway{}, &apiextensionsv1.CustomResourceDefinition{}}
	mgrOpts := manager.Options{
		Scheme:                GetOperatorScheme(),
		LeaderElection:        opCfg.LeaderElection,
		LeaderElectionID:      opCfg.LeaderElectionID,
		MetricsBindAddress:    opCfg.MetricsBindAddress,
		ClientDisableCacheFor: nonCached,
	}
	mgr, err := ctrl.NewManager(cliCfg, mgrOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// Create and register the contour controller with the operator manager.
	if _, err := contourcontroller.New(mgr, contourcontroller.Config{
		ContourImage: opCfg.ContourImage,
		EnvoyImage:   opCfg.EnvoyImage,
	}); err != nil {
		return nil, fmt.Errorf("failed to create contour controller: %w", err)
	}

	return &Operator{
		manager: mgr,
		client:  mgr.GetClient(),
		log:     ctrl.Log.WithName(operatorName),
	}, nil
}

// Start creates Gateway API controllers (if configured) and starts the operator
// synchronously until a message is received from ctx.
func (o *Operator) Start(ctx context.Context, opCfg *operatorconfig.Config) error {
	if err := o.createGatewayControllers(ctx, opCfg); err != nil {
		return fmt.Errorf("failed to create gateway controllers: %w", err)
	}

	errChan := make(chan error)
	go func() {
		errChan <- o.manager.Start(ctx)
	}()

	// Wait for the manager to exit or an explicit stop.
	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	}
}

// createGatewayControllers creates Gateway and GatewayClass controllers.
func (o *Operator) createGatewayControllers(ctx context.Context, opCfg *operatorconfig.Config) error {
	if !o.gatewayCRDsExist(ctx) {
		o.log.Info("Gateway CRDs not found; starting operator without gateway controllers")
	} else {
		// Create and register the gatewayclass controller with the operator manager.
		if _, err := gccontroller.New(o.manager); err != nil {
			return fmt.Errorf("failed to create gatewayclass controller: %w", err)
		}
		// Create and register the gateway controller with the operator manager.
		cfg := gwcontroller.Config{
			ContourImage: opCfg.ContourImage,
			EnvoyImage:   opCfg.EnvoyImage,
		}
		if _, err := gwcontroller.New(o.manager, cfg); err != nil {
			return fmt.Errorf("failed to create gateway controller: %w", err)
		}
	}
	return nil
}

// gatewayCRDsExist returns nil if Gateway CRDs exist.
func (o *Operator) gatewayCRDsExist(ctx context.Context) bool {
	gc := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gatewayclasses.networking.x-k8s.io",
		},
	}
	gw := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "gateways.networking.x-k8s.io",
		},
	}
	httproute := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "httproutes.networking.x-k8s.io",
		},
	}
	tlsroute := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "tlsroutes.networking.x-k8s.io",
		},
	}
	bp := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "backendpolicies.networking.x-k8s.io",
		},
	}
	// The list omits TCP and UDP routes since they're unsupported by Contour.
	crds := []*apiextensionsv1.CustomResourceDefinition{gc, gw, httproute, tlsroute, bp}
	for i, crd := range crds {
		key := types.NamespacedName{Name: crd.Name}
		if err := o.client.Get(ctx, key, crds[i]); err != nil {
			return false
		}
	}
	return true
}
