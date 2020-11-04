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

package manager

import (
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	contourcontroller "github.com/projectcontour/contour-operator/controller/contour"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	scheme = runtime.NewScheme()
	mgrLog = ctrl.Log.WithName("manager")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.AddToScheme(scheme)
}

// NewContourManager creates a Manager with options specified by opts with
// a Reconciler configured by cfg that reconciles Contour objects.
func NewContourManager(opts ctrl.Options, cfg contourcontroller.Config) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create contour manager: %w", err)
	}

	reconciler := newContourReconciler(mgr, cfg)

	if err := builder.
		ControllerManagedBy(mgr).
		For(&operatorv1alpha1.Contour{}).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, enqueueRequestForOwningContour()).
		Watches(&source.Kind{Type: &appsv1.DaemonSet{}}, enqueueRequestForOwningContour()).
		Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to build contour manager: %w", err)
	}

	return mgr, nil
}

// newContourReconciler creates a Reconciler configured by cfg for the provided mgr.
func newContourReconciler(mgr manager.Manager, cfg contourcontroller.Config) *contourcontroller.Reconciler {
	return &contourcontroller.Reconciler{
		Config: cfg,
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Contour"),
		Scheme: mgr.GetScheme(),
	}
}

// enqueueRequestForOwningContour returns an event handler that maps events to
// objects containing "contour.operator.projectcontour.io/owning-contour-namespace"
// and "contour.operator.projectcontour.io/owning-contour-name" labels.
func enqueueRequestForOwningContour() handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			labels := a.Meta.GetLabels()
			ns, nsFound := labels[operatorv1alpha1.OwningContourNsLabel]
			name, nameFound := labels[operatorv1alpha1.OwningContourNameLabel]
			if nsFound && nameFound {
				mgrLog.Info("queueing contour", "namespace", ns, "name", name, "related", a.Meta.GetSelfLink())
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: ns,
							Name:      name,
						},
					},
				}
			}
			return []reconcile.Request{}
		}),
	}
}
