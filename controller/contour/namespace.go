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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// defaultContourNamespace is the name of the namespace
	// used by Contour.
	defaultContourNamespace = "projectcontour"
)

// ensureNamespace ensures the namespace for the provided name exists.
func (r *Reconciler) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := r.Client.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
	switch {
	case err == nil:
		r.Log.Info("namespace exists; skipped adding", "name", ns.Name)
		return nil
	case errors.IsNotFound(err):
		if err := r.Client.Create(context.TODO(), ns); err != nil {
			return fmt.Errorf("failed to create namespace %s: %v", ns.Name, err)
		}
		r.Log.Info("created namespace", "name", ns.Name)
		return nil
	}
	return fmt.Errorf("failed to get namespace %s: %v", ns.Name, err)
}

// ensureNamespaceRemoved ensures the namespace for the provided name
// does not exist.
func (r *Reconciler) ensureNamespaceRemoved(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := r.Client.Get(ctx, types.NamespacedName{Name: ns.Name}, ns)
	switch {
	case err == nil:
		if err := r.Client.Delete(ctx, ns); err != nil {
			return fmt.Errorf("failed to delete namespace %s: %v", ns.Name, err)
		}
		r.Log.Info("deleted namespace", "name", ns.Name)
		return nil
	case errors.IsNotFound(err):
		r.Log.Info("namespace does not exist; skipping removal", "name", ns.Name)
		return nil
	}
	return fmt.Errorf("failed to get namespace %s: %v", ns.Name, err)
}
