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

package gateway

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objutil "github.com/projectcontour/contour-operator/internal/objects"
	objcm "github.com/projectcontour/contour-operator/internal/objects/configmap"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objds "github.com/projectcontour/contour-operator/internal/objects/daemonset"
	objdeploy "github.com/projectcontour/contour-operator/internal/objects/deployment"
	objgw "github.com/projectcontour/contour-operator/internal/objects/gateway"
	objgc "github.com/projectcontour/contour-operator/internal/objects/gatewayclass"
	objjob "github.com/projectcontour/contour-operator/internal/objects/job"
	objns "github.com/projectcontour/contour-operator/internal/objects/namespace"
	objsvc "github.com/projectcontour/contour-operator/internal/objects/service"
	"github.com/projectcontour/contour-operator/internal/operator/status"
	retryable "github.com/projectcontour/contour-operator/internal/retryableerror"
	"github.com/projectcontour/contour-operator/pkg/validation"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	controllerName = "gateway_controller"
)

// Config holds all the things necessary for the controller to run.
type Config struct {
	// ContourImage is the name of the Contour container image.
	ContourImage string
	// EnvoyImage is the name of the Envoy container image.
	EnvoyImage string
}

// reconciler reconciles a Gateway object.
type reconciler struct {
	config Config
	client client.Client
	log    logr.Logger
}

// New creates the gateway controller from mgr. The controller will be pre-configured
// to watch for Gateway objects across all namespaces.
func New(mgr manager.Manager, cfg Config) (controller.Controller, error) {
	r := &reconciler{
		client: mgr.GetClient(),
		config: cfg,
		log:    ctrl.Log.WithName(controllerName),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &gatewayv1alpha1.Gateway{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.log.WithValues("gateway", req.NamespacedName)

	r.log.Info("reconciling", "request", req)

	gw := &gatewayv1alpha1.Gateway{}
	key := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	var errs []error
	if err := r.client.Get(ctx, key, gw); err != nil {
		if errors.IsNotFound(err) {
			// This means the gateway was already deleted/finalized and there are
			// stale queue entries (or something edge triggering from a related
			// resource that got deleted async).
			r.log.Info("gateway not found; reconciliation will be skipped", "request", req)
			return ctrl.Result{}, nil
		}
		// Error reading the object, so requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get gateway %s/%s: %w", req.Namespace, req.Name, err)
	}

	// The gateway is safe to process.
	desired := gw.ObjectMeta.DeletionTimestamp.IsZero()
	if desired {
		switch {
		case objgw.IsFinalized(gw):
			if err := validation.Gateway(ctx, r.client, gw); err != nil {
				errs = append(errs, fmt.Errorf("failed to validate gateway %s/%s: %w", gw.Namespace, gw.Name, err))
			}
			if err := r.ensureGateway(ctx, gw); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get ensure gateway %s/%s: %w", req.Namespace, req.Name, err)
			}
			// The gateway is valid, so finalize dependent resources of gateway.
			gc, err := objgc.Get(ctx, r.client, gw.Spec.GatewayClassName)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to get gatewayclass %s: %w", gw.Spec.GatewayClassName, err))
			} else {
				if err := objgc.EnsureFinalizer(ctx, r.client, gc); err != nil {
					errs = append(errs, fmt.Errorf("failed to finalize gatewayclass %s: %w", gc.Name, err))
				}
			}
			cntr, err := objgw.ContourForGateway(ctx, r.client, gw)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to get contour for gateway %s/%s: %w",
					gw.Namespace, gw.Name, err))
			} else {
				if err := objcontour.EnsureFinalizer(ctx, r.client, cntr); err != nil {
					errs = append(errs, fmt.Errorf("failed to finalize contour %s/%s: %w",
						cntr.Namespace, cntr.Name, err))
				}
			}
			if err := status.SyncGateway(ctx, r.client, gw); err != nil {
				errs = append(errs, fmt.Errorf("failed to sync status for gateway %s/%s: %w", gw.Namespace, gw.Name, err))
			}
		default:
			// Before doing anything with the gateway, ensure it has a finalizer
			// so it can cleaned-up later.
			if err := objgw.EnsureFinalizer(ctx, r.client, gw); err != nil {
				return ctrl.Result{}, err
			}
			r.log.Info("added finalizer to gateway", "namespace", gw.Namespace, "name", gw.Name)
		}
	} else {
		if err := r.ensureGatewayDeleted(ctx, gw); err != nil {
			switch e := err.(type) {
			case retryable.Error:
				r.log.Error(e, "got retryable error; requeueing", "after", e.After())
				return ctrl.Result{RequeueAfter: e.After()}, nil
			default:
				return ctrl.Result{}, err
			}
		}
	}
	if len(errs) != 0 {
		return ctrl.Result{}, retryable.NewMaybeRetryableAggregate(errs)
	}
	return ctrl.Result{}, nil
}

// ensureGateway ensures all necessary resources exist for the given gw.
func (r *reconciler) ensureGateway(ctx context.Context, gw *gatewayv1alpha1.Gateway) error {
	var errs []error
	cli := r.client
	contour, err := objgw.ContourForGateway(ctx, cli, gw)
	if err != nil {
		return fmt.Errorf("failed to get contour for gateway %s/%s", gw.Namespace, gw.Name)
	}
	if err := objns.EnsureNamespace(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to ensure namespace %s for contour %s/%s: %w",
			contour.Spec.Namespace.Name, contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("ensured namespace for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if err := objutil.EnsureRBAC(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to ensure rbac for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("ensured rbac for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if len(errs) == 0 {
		if err := objcm.EnsureConfigMap(ctx, cli, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure configmap for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("ensured configmap for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
		contourImage := r.config.ContourImage
		if err := objjob.EnsureJob(ctx, cli, contour, contourImage); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure job for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("ensured job for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
		if err := objdeploy.EnsureDeployment(ctx, cli, contour, contourImage); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure deployment for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("ensured deployment for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
		envoyImage := r.config.EnvoyImage
		if err := objds.EnsureDaemonSet(ctx, cli, contour, contourImage, envoyImage); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure daemonset for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("ensured daemonset for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
		if err := objsvc.EnsureContourService(ctx, cli, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure contour service for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("ensured contour service for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
		if contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.LoadBalancerServicePublishingType ||
			contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.NodePortServicePublishingType {
			if err := objsvc.EnsureEnvoyService(ctx, cli, contour); err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure envoy service for contour %s/%s: %w",
					contour.Namespace, contour.Name, err))
			} else {
				r.log.Info("ensured envoy service for contour", "namespace", contour.Namespace, "name", contour.Name)
			}
		}
	}
	return retryable.NewMaybeRetryableAggregate(errs)
}

// ensureGatewayDeleted ensures gw and all child resources have been deleted.
func (r *reconciler) ensureGatewayDeleted(ctx context.Context, gw *gatewayv1alpha1.Gateway) error {
	var errs []error
	cli := r.client
	contour, err := objgw.ContourForGateway(ctx, cli, gw)
	if err != nil {
		return fmt.Errorf("failed to get contour for gateway %s/%s", gw.Namespace, gw.Name)
	}
	if contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.LoadBalancerServicePublishingType ||
		contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.NodePortServicePublishingType {
		if err := objsvc.EnsureEnvoyServiceDeleted(ctx, cli, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete envoy service for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("deleted envoy service for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
	}
	if err := objsvc.EnsureContourServiceDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete service for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted contour service for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if err := objds.EnsureDaemonSetDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete daemonset from contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted daemonset for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if err := objdeploy.EnsureDeploymentDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete deployment from contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted deployment for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if err := objjob.EnsureJobDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete job from contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted job for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if err := objcm.EnsureConfigMapDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete configmap for contour %s/%s: %w",
			contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted configmap for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if err := objutil.EnsureRBACDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete rbac for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted rbac for contour", "namespace", contour.Namespace, "name", contour.Name)
	}

	if len(errs) == 0 {
		// Remove finalizer from dependent resources of gateway.
		cntr, err := objgw.ContourForGateway(ctx, r.client, gw)
		if err != nil {
			return fmt.Errorf("failed to get contour for gateway %s/%s: %w", gw.Namespace, gw.Name, err)
		}
		gc, err := objgc.Get(ctx, r.client, gw.Spec.GatewayClassName)
		if err != nil {
			return fmt.Errorf("failed to get gatewayclass %s: %w", gw.Spec.GatewayClassName, err)
		}
		otherClasses, err := objgc.OtherGatewayClassesRefContour(ctx, r.client, gc, cntr)
		switch {
		case err != nil:
			return fmt.Errorf("failed to verify if other gatewayclassess reference contour %s/%s: %w", gc.Namespace, gc.Name, err)
		case !otherClasses:
			// Remove the finalizer from the dependent contour since no other gatewayclasses reference it.
			if err := objcontour.EnsureFinalizerRemoved(ctx, r.client, cntr); err != nil {
				return fmt.Errorf("failed to remove finalizer from contour %s/%s: %w", cntr.Namespace, cntr.Name, err)
			}
			r.log.Info("removed finalizer from contour", "namespace", cntr.Namespace, "name", cntr.Name)
		}
		otherGateways, err := objgw.OtherGatewaysRefGatewayClass(ctx, r.client, gw)
		switch {
		case err != nil:
			return fmt.Errorf("failed to verify if other gateways reference gatewayclass %s: %w", gc.Name, err)
		case !otherGateways:
			// Remove the finalizer from the dependent gatewayclass since no other contours reference it.
			if err := objgc.EnsureFinalizerRemoved(ctx, r.client, gc); err != nil {
				return fmt.Errorf("failed to remove finalizer from gatewayclass %s: %w", gc.Name, err)
			}
			r.log.Info("removed finalizer from gatewayclass", "name", gc.Name)
		}
		// Remove finalizer from gateway.
		if err := objgw.EnsureFinalizerRemoved(ctx, cli, gw); err != nil {
			return fmt.Errorf("failed to remove finalizer from gateway %s/%s: %w", gw.Namespace, gw.Name, err)
		}
		r.log.Info("removed finalizer from gateway", "namespace", gw.Namespace, "name", gw.Name)
	}
	return utilerrors.NewAggregate(errs)
}
