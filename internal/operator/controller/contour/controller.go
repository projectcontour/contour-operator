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
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objutil "github.com/projectcontour/contour-operator/internal/objects"
	objcm "github.com/projectcontour/contour-operator/internal/objects/configmap"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objds "github.com/projectcontour/contour-operator/internal/objects/daemonset"
	objdeploy "github.com/projectcontour/contour-operator/internal/objects/deployment"
	objjob "github.com/projectcontour/contour-operator/internal/objects/job"
	objns "github.com/projectcontour/contour-operator/internal/objects/namespace"
	objsvc "github.com/projectcontour/contour-operator/internal/objects/service"
	retryable "github.com/projectcontour/contour-operator/internal/retryableerror"
	"github.com/projectcontour/contour-operator/pkg/validation"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "contour_controller"
)

// Config holds all the things necessary for the controller to run.
type Config struct {
	// ContourImage is the name of the Contour container image.
	ContourImage string
	// EnvoyImage is the name of the Envoy container image.
	EnvoyImage string
}

// reconciler reconciles a Contour object.
type reconciler struct {
	config Config
	client client.Client
	log    logr.Logger
}

// +kubebuilder:rbac:groups=operator.projectcontour.io,resources=contours,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=operator.projectcontour.io,resources=contours/status,verbs=get;update;patch
// cert-gen needs create/update secrets.
// +kubebuilder:rbac:groups="",resources=namespaces;secrets;serviceaccounts;services,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.x-k8s.io,resources=gatewayclasses;gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.x-k8s.io,resources=gatewayclasses/status;gateways/status,verbs=create;get;update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;ingressclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=create;get;update
// +kubebuilder:rbac:groups=projectcontour.io,resources=httpproxies;tlscertificatedelegations;extensionservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=projectcontour.io,resources=httpproxies/status;extensionservices/status,verbs=create;get;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;delete;create;update;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;delete;create;update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list

// New creates the contour controller from mgr and cfg. The controller will be pre-configured
// to watch for Contour custom resources across all namespaces.
func New(mgr manager.Manager, cfg Config) (controller.Controller, error) {
	r := &reconciler{
		config: cfg,
		client: mgr.GetClient(),
		log:    ctrl.Log.WithName(controllerName),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &operatorv1alpha1.Contour{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}
	// Watch the Contour deployment and Envoy daemonset to properly surface Contour status conditions.
	if err := c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, r.enqueueRequestForOwningContour()); err != nil {
		return nil, err
	}
	if err := c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, r.enqueueRequestForOwningContour()); err != nil {
		return nil, err
	}
	return c, nil
}

// enqueueRequestForOwningContour returns an event handler that maps events to
// objects containing Contour owner labels.
func (r *reconciler) enqueueRequestForOwningContour() handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			labels := a.Meta.GetLabels()
			ns, nsFound := labels[operatorv1alpha1.OwningContourNsLabel]
			name, nameFound := labels[operatorv1alpha1.OwningContourNameLabel]
			if nsFound && nameFound {
				r.log.Info("queueing contour", "namespace", ns, "name", name, "related", a.Meta.GetSelfLink())
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

// Reconcile reconciles watched objects and attempts to make the current state of
// the object match the desired state.
func (r *reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.log.WithValues("contour", req.NamespacedName)
	r.log.Info("reconciling", "request", req)
	// Only proceed if we can get the state of contour.
	contour := &operatorv1alpha1.Contour{}
	if err := r.client.Get(ctx, req.NamespacedName, contour); err != nil {
		if errors.IsNotFound(err) {
			// This means the contour was already deleted/finalized and there are
			// stale queue entries (or something edge triggering from a related
			// resource that got deleted async).
			r.log.Info("contour not found; reconciliation will be skipped", "request", req)
			return ctrl.Result{}, nil
		}
		// Error reading the object, so requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get contour %q: %w", req, err)
	}

	// The contour is safe to process, so ensure current state matches desired state.
	desired := contour.ObjectMeta.DeletionTimestamp.IsZero()
	if desired {
		if err := validation.ValidateContour(contour); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to validate contour %s/%s: %w", contour.Namespace, contour.Name, err)
		}
		switch {
		case contour.IsFinalized():
			if err := r.ensureContour(ctx, contour); err != nil {
				switch e := err.(type) {
				case retryable.Error:
					r.log.Error(e, "got retryable error; requeueing", "after", e.After())
					return ctrl.Result{RequeueAfter: e.After()}, nil
				default:
					return ctrl.Result{}, err
				}
			}
			r.log.Info("ensured contour", "namespace", contour.Namespace, "name", contour.Name)
		default:
			// Before doing anything with the contour, ensure it has a finalizer
			// so it can cleaned-up later.
			if err := objcontour.EnsureFinalizer(ctx, r.client, contour); err != nil {
				return ctrl.Result{}, err
			}
			r.log.Info("finalized contour", "namespace", contour.Namespace, "name", contour.Name)
		}
	} else {
		if err := r.ensureContourDeleted(ctx, contour); err != nil {
			switch e := err.(type) {
			case retryable.Error:
				r.log.Error(e, "got retryable error; requeueing", "after", e.After())
				return ctrl.Result{RequeueAfter: e.After()}, nil
			default:
				return ctrl.Result{}, err
			}
		}
		r.log.Info("deleted contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Contour{}).
		Complete(r)
}

// ensureContour ensures all necessary resources exist for the given contour.
func (r *reconciler) ensureContour(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	var errs []error
	cli := r.client
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
	deploy := &appsv1.Deployment{}
	ds := &appsv1.DaemonSet{}
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
		var err error
		deploy, err = objdeploy.EnsureDeployment(ctx, cli, contour, contourImage)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure deployment for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("ensured deployment for contour", "namespace", contour.Namespace, "name", contour.Name)
		}
		envoyImage := r.config.EnvoyImage
		ds, err = objds.EnsureDaemonSet(ctx, cli, contour, contourImage, envoyImage)
		if err != nil {
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
	if err := r.syncContourStatus(ctx, contour, deploy, ds); err != nil {
		errs = append(errs, fmt.Errorf("failed to sync status for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("synced status for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	return retryable.NewMaybeRetryableAggregate(errs)
}

// ensureContourDeleted ensures contour and all child resources have been deleted.
func (r *reconciler) ensureContourDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	var errs []error
	cli := r.client
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
	if err := objns.EnsureNamespaceDeleted(ctx, cli, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete namespace %s for contour %s/%s: %w",
			contour.Spec.Namespace.Name, contour.Namespace, contour.Name, err))
	} else {
		r.log.Info("deleted namespace for contour", "namespace", contour.Namespace, "name", contour.Name)
	}
	if len(errs) == 0 {
		if err := objcontour.EnsureFinalizerRemoved(ctx, cli, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove finalizer from contour %s/%s: %w", contour.Namespace, contour.Name, err))
		} else {
			r.log.Info("removed finalizer from contour", "namespace", contour.Namespace, "name", contour.Name)
		}
	}
	return utilerrors.NewAggregate(errs)
}
