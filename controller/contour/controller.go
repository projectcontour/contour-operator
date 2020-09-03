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
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	retryable "github.com/projectcontour/contour-operator/util/retryableerror"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config holds all the things necessary for the controller to run.
type Config struct {
	// Image is the name of the Contour container image.
	Image string
}

// Reconciler reconciles a Contour object.
type Reconciler struct {
	Config Config
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.projectcontour.io,resources=contours,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=operator.projectcontour.io,resources=contours/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;delete;create

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("contour", req.NamespacedName)

	r.Log.Info("reconciling", "request", req)

	// Only proceed if we can get the state of contour.
	contour := &operatorv1alpha1.Contour{}
	if err := r.Client.Get(ctx, req.NamespacedName, contour); err != nil {
		if errors.IsNotFound(err) {
			// This means the contour was already deleted/finalized and there are
			// stale queue entries (or something edge triggering from a related
			// resource that got deleted async).
			r.Log.Info("contour not found; reconciliation will be skipped", "request", req)
			return ctrl.Result{}, nil
		}
		// Error reading the object, so requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get contour %q: %v", req, err)
	}

	// The contour is safe to process, so ensure current state matches desired state.
	desired := contour.ObjectMeta.DeletionTimestamp.IsZero()
	if desired {
		if err := r.ensureContour(ctx, contour, defaultContourNamespace); err != nil {
			switch e := err.(type) {
			case retryable.Error:
				r.Log.Error(e, "got retryable error; requeueing", "after", e.After())
				return ctrl.Result{RequeueAfter: e.After()}, nil
			default:
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.ensureContourRemoved(ctx, contour, defaultContourNamespace); err != nil {
			switch e := err.(type) {
			case retryable.Error:
				r.Log.Error(e, "got retryable error; requeueing", "after", e.After())
				return ctrl.Result{RequeueAfter: e.After()}, nil
			default:
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Contour{}).
		Complete(r)
}

// ensureContour ensures all necessary resources exist for the given contour
// in namespace ns.
func (r *Reconciler) ensureContour(ctx context.Context, contour *operatorv1alpha1.Contour, ns string) error {
	// Before doing anything with the contour, ensure it has a finalizer
	// so it can cleaned-up later.
	if err := r.ensureFinalizer(ctx, contour); err != nil {
		return fmt.Errorf("failed to finalize contour %s/%s: %v", contour.Namespace, contour.Name, err)
	}
	if err := r.ensureNamespace(ctx, ns); err != nil {
		return fmt.Errorf("failed to ensure namespace %s: %v", ns, err)
	}
	return nil
}

// ensureContourRemoved ensures all resources for the given contour do not exist
// in namespace ns.
func (r *Reconciler) ensureContourRemoved(ctx context.Context, contour *operatorv1alpha1.Contour, ns string) error {
	if err := r.ensureNamespaceRemoved(ctx, defaultContourNamespace); err != nil {
		return fmt.Errorf("failed to remove namespace %s: %v", defaultContourNamespace, err)
	}
	if err := r.ensureFinalizerRemoved(ctx, contour); err != nil {
		return fmt.Errorf("failed to remove finalizer from contour %s/%s: %v", contour.Namespace, contour.Name, err)
	}
	return nil
}
