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
	"github.com/projectcontour/contour-operator/util/slice"
)

const (
	// contourFinalizer is the name of the finalizer used
	// for Contour.
	contourFinalizer = "contour.operator.projectcontour.io/finalizer"
)

// ensureFinalizer ensures the finalizer is added to the given contour.
func (r *Reconciler) ensureFinalizer(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	if !slice.ContainsString(contour.Finalizers, contourFinalizer) {
		updated := contour.DeepCopy()
		updated.Finalizers = append(updated.Finalizers, contourFinalizer)
		if err := r.Client.Update(ctx, updated); err != nil {
			return fmt.Errorf("failed to add finalizer %s: %v", contourFinalizer, err)
		}
		r.Log.Info("added finalizer to contour", "finalizer", contourFinalizer,
			"namespace", contour.Namespace, "name", contour.Name)
	}
	return nil
}

// ensureFinalizerRemoved ensures the finalizer is removed for the given contour.
func (r *Reconciler) ensureFinalizerRemoved(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	if slice.ContainsString(contour.Finalizers, contourFinalizer) {
		updated := contour.DeepCopy()
		updated.Finalizers = slice.RemoveString(updated.Finalizers, contourFinalizer)
		if err := r.Client.Update(ctx, updated); err != nil {
			return fmt.Errorf("failed to remove finalizer %s: %v", contourFinalizer, err)
		}
		r.Log.Info("removed finalizer from contour", "finalizer", contourFinalizer,
			"namespace", contour.Namespace, "name", contour.Name)
	}
	return nil
}
