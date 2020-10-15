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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// namespaceCoreList is a list of namespace names that should not be removed.
var namespaceCoreList = []string{"contour-operator", "default", "kube-system"}

// ensureNamespace ensures the namespace for the provided name exists.
func (r *Reconciler) ensureNamespace(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	name := contour.Spec.Namespace.Name
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := r.Client.Create(context.TODO(), ns); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("namespace exists", "name", ns.Name)
			return nil
		}
		return fmt.Errorf("failed to create namespace %s: %w", ns.Name, err)
	}
	r.Log.Info("created namespace", "name", ns.Name)
	return nil
}

// ensureNamespaceRemoved ensures the namespace for the provided contour is removed,
// bypassing deletion if any of the following conditions apply:
//   - RemoveOnDeletion is unspecified or set to false.
//   - Another contour exists in the same namespace.
//   - The namespace of contour matches a name in namespaceCoreList.
func (r *Reconciler) ensureNamespaceRemoved(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	name := contour.Spec.Namespace.Name
	if !contour.Spec.Namespace.RemoveOnDeletion {
		r.Log.Info("remove on deletion is not set for contour; skipping removal of namespace",
			"contour_namespace", contour.Namespace, "contour_name", contour.Name, "namespace", name)
		return nil
	}
	for _, ns := range namespaceCoreList {
		if name == ns {
			r.Log.Info("namespace of contour matches core list; skipping namespace removal",
				"contour_namespace", contour.Namespace, "contour_name", contour.Name, "namespace", name)
			return nil
		}
	}
	exist, err := r.otherContoursExistInSpecNs(ctx, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours exist in namespace %s: %w", contour.Namespace, err)
	}
	if !exist {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err := r.Client.Delete(ctx, ns); err != nil {
			if errors.IsNotFound(err) {
				r.Log.Info("namespace does not exist", "name", ns.Name)
				return nil
			}
			return fmt.Errorf("failed to delete namespace %s: %w", ns.Name, err)
		}
		r.Log.Info("deleted namespace", "name", ns.Name)
		return nil
	}
	r.Log.Info("other contours with same spec namespace; skipping namespace removal", "name", name)
	return nil
}
