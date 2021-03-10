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

package gatewayclass

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objgc "github.com/projectcontour/contour-operator/internal/objects/gatewayclass"
	"github.com/projectcontour/contour-operator/internal/operator/status"
	retryable "github.com/projectcontour/contour-operator/internal/retryableerror"
	"github.com/projectcontour/contour-operator/pkg/validation"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	controllerName = "gatewayclass_controller"
)

// Reconciler reconciles a GatewayClass object.
type reconciler struct {
	client client.Client
	log    logr.Logger
}

// New creates the gatewayclass controller from mgr. The controller will be pre-configured
// to watch for GatewayClass objects.
func New(mgr manager.Manager) (controller.Controller, error) {
	r := &reconciler{
		client: mgr.GetClient(),
		log:    ctrl.Log.WithName(controllerName),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}
	// Only enqueue GatewayClass objects that specify the operator as the controller.
	if err := c.Watch(&source.Kind{Type: &gatewayv1alpha1.GatewayClass{}}, r.enqueueRequestForGatewayClass()); err != nil {
		return nil, err
	}
	// Watch Contour objects to properly surface GatewayClass status conditions.
	if err := c.Watch(&source.Kind{Type: &operatorv1alpha1.Contour{}}, r.enqueueRequestForGatewayClassRef()); err != nil {
		return nil, err
	}
	return c, nil
}

// enqueueRequestForGatewayClass returns an event handler that maps events to
// GatewayClass objects owned by the operator.
func (r *reconciler) enqueueRequestForGatewayClass() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
		gc := a.(*gatewayv1alpha1.GatewayClass)
		if objgc.IsController(gc.Spec.Controller) {
			name := gc.Name
			r.log.Info("queueing gatewayclass", "name", name)
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{Name: name},
				},
			}
		}
		return []reconcile.Request{}
	})
}

// enqueueRequestForGatewayClassRef returns an event handler that maps events
// to Contour objects that contain a gatewayClassRef.
func (r *reconciler) enqueueRequestForGatewayClassRef() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
		contour := a.(*operatorv1alpha1.Contour)
		if contour.Spec.GatewayClassRef != nil {
			name := *contour.Spec.GatewayClassRef
			r.log.Info("queueing contour for gatewayclass", "name", name)
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{Name: name},
				},
			}
		}
		return []reconcile.Request{}
	})
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.log.WithValues("gatewayclass", req.NamespacedName)

	r.log.Info("reconciling", "request", req)

	gc := &gatewayv1alpha1.GatewayClass{}
	key := types.NamespacedName{Name: req.Name}
	var errs []error
	if err := r.client.Get(ctx, key, gc); err != nil {
		if errors.IsNotFound(err) {
			// This means the gatewayclass was already deleted/finalized and there are
			// stale queue entries (or something edge triggering from a related
			// resource that got deleted async).
			r.log.Info("gatewayclass not found; reconciliation will be skipped", "request", req)
			return ctrl.Result{}, nil
		}
		// Error reading the object, so requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get gatewayclass %s: %w", req.Name, err)
	}
	// The gatewayclass is safe to process.
	desired := gc.ObjectMeta.DeletionTimestamp.IsZero()
	if desired {
		gcValid := false
		cntrValid := false
		cntrAvailable := false
		cntr, err := validation.GatewayClass(ctx, r.client, gc)
		if err == nil {
			gcValid = true
		}
		if cntr != nil {
			cntrAvailable = cntr.Available()
			cntrValid = cntr.Valid()
		}
		if err := status.SyncGatewayClass(ctx, r.client, gc, gcValid, cntrValid, cntrAvailable); err != nil {
			errs = append(errs, fmt.Errorf("failed to sync status for gatewayclass %s: %w", gc.Name, err))
		} else {
			r.log.Info("synced status for gatewayclass", "name", gc.Name)
		}
	}
	return ctrl.Result{}, retryable.NewMaybeRetryableAggregate(errs)
}
