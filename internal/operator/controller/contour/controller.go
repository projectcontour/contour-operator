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
	retryable "github.com/projectcontour/contour-operator/internal/retryableerror"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
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

var pointerTo = func(ios intstr.IntOrString) *intstr.IntOrString { return &ios }

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
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update
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
	return handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
		labels := a.GetLabels()
		ns, nsFound := labels[operatorv1alpha1.OwningContourNsLabel]
		name, nameFound := labels[operatorv1alpha1.OwningContourNameLabel]
		if nsFound && nameFound {
			r.log.Info("queueing contour", "namespace", ns, "name", name, "related", a.GetSelfLink())
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
	})
}

// Reconcile reconciles watched objects and attempts to make the current state of
// the object match the desired state.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
		if err := validateContour(contour); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to validate contour %s/%s: %w", contour.Namespace, contour.Name, err)
		}
		switch {
		case contourFinalized(contour):
			if err := r.ensureContour(ctx, contour); err != nil {
				switch e := err.(type) {
				case retryable.Error:
					r.log.Error(e, "got retryable error; requeueing", "after", e.After())
					return ctrl.Result{RequeueAfter: e.After()}, nil
				default:
					return ctrl.Result{}, err
				}
			}
		default:
			// Before doing anything with the contour, ensure it has a finalizer
			// so it can cleaned-up later.
			if err := r.ensureFinalizer(ctx, contour); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.ensureContourRemoved(ctx, contour); err != nil {
			switch e := err.(type) {
			case retryable.Error:
				r.log.Error(e, "got retryable error; requeueing", "after", e.After())
				return ctrl.Result{RequeueAfter: e.After()}, nil
			default:
				return ctrl.Result{}, err
			}
		}
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
	deploy := &appsv1.Deployment{}
	ds := &appsv1.DaemonSet{}
	if contour.Spec.GatewayClassRef == "None" {
		if err := r.ensureNamespace(ctx, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure namespace %s for contour %s/%s: %w",
				contour.Spec.Namespace.Name, contour.Namespace, contour.Name, err))
		}
		if err := r.ensureRBAC(ctx, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to ensure rbac for contour %s/%s: %w", contour.Namespace, contour.Name, err))
		}
		if len(errs) == 0 {
			if err := r.ensureConfigMap(ctx, contour); err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure configmap for contour %s/%s: %w", contour.Namespace, contour.Name, err))
			}
			if err := r.ensureJob(ctx, contour); err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure job for contour %s/%s: %w", contour.Namespace, contour.Name, err))
			}
			var err error
			deploy, err = r.ensureDeployment(ctx, contour)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure deployment for contour %s/%s: %w", contour.Namespace, contour.Name, err))
			}
			ds, err = r.ensureDaemonSet(ctx, contour)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure daemonset for contour %s/%s: %w", contour.Namespace, contour.Name, err))
			}
			if err := r.ensureContourService(ctx, contour); err != nil {
				errs = append(errs, fmt.Errorf("failed to ensure service for contour %s/%s: %w", contour.Namespace, contour.Name, err))
			}
			if contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.LoadBalancerServicePublishingType ||
				contour.Spec.NetworkPublishing.Envoy.Type == operatorv1alpha1.NodePortServicePublishingType {
				if err := r.ensureEnvoyService(ctx, contour); err != nil {
					errs = append(errs, fmt.Errorf("failed to ensure envoy service for contour %s/%s: %w",
						contour.Namespace, contour.Name, err))
				}
			}
		}
	}

	if err := r.syncContourStatus(ctx, contour, deploy, ds); err != nil {
		errs = append(errs, fmt.Errorf("failed to sync status for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}

	return retryable.NewMaybeRetryableAggregate(errs)
}

// ensureContourRemoved ensures all resources for the given contour do not exist.
func (r *reconciler) ensureContourRemoved(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	var errs []error
	if err := r.ensureEnvoyServiceDeleted(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove service for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}
	if err := r.ensureContourServiceDeleted(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove service for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}
	if err := r.ensureDaemonSetDeleted(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove daemonset from contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}
	if err := r.ensureDeploymentDeleted(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove deployment from contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}
	if err := r.ensureJobDeleted(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove job from contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}
	if err := r.ensureConfigMapDeleted(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove configmap for contour %s/%s: %w",
			contour.Namespace, contour.Name, err))
	}
	if err := r.ensureRBACRemoved(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove rbac for contour %s/%s: %w", contour.Namespace, contour.Name, err))
	}
	if err := r.ensureNamespaceRemoved(ctx, contour); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove namespace %s for contour %s/%s: %w",
			contour.Spec.Namespace.Name, contour.Namespace, contour.Name, err))
	}
	if len(errs) == 0 {
		if err := r.ensureFinalizerRemoved(ctx, contour); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove finalizer from contour %s/%s: %w", contour.Namespace, contour.Name, err))
		}
	}
	return utilerrors.NewAggregate(errs)
}

// otherContoursExist lists Contour objects in all namespaces, returning the list
// and true if any exist other than contour.
func (r *reconciler) otherContoursExist(ctx context.Context, contour *operatorv1alpha1.Contour) (bool, *operatorv1alpha1.ContourList, error) {
	contours := &operatorv1alpha1.ContourList{}
	if err := r.client.List(ctx, contours); err != nil {
		return false, nil, fmt.Errorf("failed to list contours: %w", err)
	}
	if len(contours.Items) == 0 || len(contours.Items) == 1 && contours.Items[0].Name == contour.Name {
		return false, nil, nil
	}
	return true, contours, nil
}

// otherContoursExistInSpecNs lists Contour objects in the same spec.namespace.name as contour,
// returning true if any exist.
func (r *reconciler) otherContoursExistInSpecNs(ctx context.Context, contour *operatorv1alpha1.Contour) (bool, error) {
	exist, contours, err := r.otherContoursExist(ctx, contour)
	if err != nil {
		return false, err
	}
	if exist {
		for _, c := range contours.Items {
			if c.Spec.Namespace.Name == contour.Spec.Namespace.Name {
				return true, nil
			}
		}
	}
	return false, nil
}

// contourOwningSelector returns a label selector using "contour.operator.projectcontour.io/owning-contour-name"
// and "contour.operator.projectcontour.io/owning-contour-namespace" labels.
func contourOwningSelector(contour *operatorv1alpha1.Contour) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			operatorv1alpha1.OwningContourNameLabel: contour.Name,
			operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
		},
	}
}

// contourDeploymentPodSelector returns a label selector using "app: contour" as the
// key/value pair.
//
// TODO [danehans]: Update to use "contour.operator.projectcontour.io/deployment-contour"
// when https://github.com/projectcontour/contour/issues/1821 is fixed.
func contourDeploymentPodSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "contour",
		},
	}
}

// envoyDaemonSetPodSelector returns a label selector using "app: envoy" as the
// key/value pair.
//
// TODO [danehans]: Update to use "contour.operator.projectcontour.io/daemonset-envoy"
// when https://github.com/projectcontour/contour/issues/1821 is fixed.
func envoyDaemonSetPodSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "envoy",
		},
	}
}

// contourFinalized returns true if contour is finalized.
func contourFinalized(contour *operatorv1alpha1.Contour) bool {
	for _, f := range contour.Finalizers {
		if f == ContourFinalizer {
			return true
		}
	}

	return false
}

// ownerLabelsExist returns true if obj contains Contour owner labels.
func ownerLabelsExist(obj metav1.Object, contour *operatorv1alpha1.Contour) bool {
	labels := obj.GetLabels()
	nameFound := false
	nsFound := false
	if labels == nil {
		return false
	}
	for l, v := range labels {
		switch {
		case nameFound && nsFound:
			return true
		case l == operatorv1alpha1.OwningContourNameLabel && v == contour.Name:
			nameFound = true
		case l == operatorv1alpha1.OwningContourNsLabel && v == contour.Namespace:
			nsFound = true
		}
	}
	if nameFound && nsFound {
		return true
	}
	// no contour owning name and ns labels found.
	return false
}
