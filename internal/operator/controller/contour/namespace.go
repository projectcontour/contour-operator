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
	"github.com/projectcontour/contour-operator/internal/equality"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// namespaceCoreList is a list of namespace names that should not be removed.
var namespaceCoreList = []string{"contour-operator", "default", "kube-system"}

// ensureNamespace ensures the namespace for the provided name exists.
func (r *reconciler) ensureNamespace(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	desired := DesiredNamespace(contour)
	current, err := r.currentSpecNsName(ctx, contour.Spec.Namespace.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.createNamespace(ctx, desired)
		}
		return fmt.Errorf("failed to get namespace %s: %w", desired.Name, err)
	}

	if err := r.updateNamespaceIfNeeded(ctx, contour, current, desired); err != nil {
		return fmt.Errorf("failed to update namespace %s: %w", desired.Name, err)
	}

	return nil
}

// ensureNamespaceRemoved ensures the namespace for the provided contour is removed,
// bypassing deletion if any of the following conditions apply:
//   - RemoveOnDeletion is unspecified or set to false.
//   - Another contour exists in the same namespace.
//   - The namespace of contour matches a name in namespaceCoreList.
//   - The namespace does not contain the Contour owner labels.
func (r *reconciler) ensureNamespaceRemoved(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	name := contour.Spec.Namespace.Name
	if !contour.Spec.Namespace.RemoveOnDeletion {
		r.log.Info("remove on deletion is not set for contour; skipping deletion of namespace",
			"contour_namespace", contour.Namespace, "contour_name", contour.Name, "namespace", name)
		return nil
	}
	for _, ns := range namespaceCoreList {
		if name == ns {
			r.log.Info("namespace of contour matches core list; skipping namespace deletion",
				"contour_namespace", contour.Namespace, "contour_name", contour.Name, "namespace", name)
			return nil
		}
	}
	ns, err := r.currentSpecNsName(ctx, name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !ownerLabelsExist(ns, contour) {
		r.log.Info("namespace not labeled; skipping deletion", "name", ns.Name)
		return nil
	}
	contoursExist, err := r.otherContoursExistInSpecNs(ctx, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours exist in namespace %s: %w", name, err)
	}
	if contoursExist {
		r.log.Info("other contours exist with same spec namespace; skipping namespace deletion", "name", name)
	} else {
		if err := r.client.Delete(ctx, ns); err != nil {
			if errors.IsNotFound(err) {
				r.log.Info("namespace does not exist", "name", ns.Name)
				return nil
			}
			return fmt.Errorf("failed to delete namespace %s: %w", ns.Name, err)
		}
		r.log.Info("deleted namespace", "name", ns.Name)
	}
	return nil
}

// DesiredNamespace returns the desired Namespace resource for the provided contour.
func DesiredNamespace(contour *operatorv1alpha1.Contour) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: contour.Spec.Namespace.Name,
			Labels: map[string]string{
				operatorv1alpha1.OwningContourNameLabel: contour.Name,
				operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
			},
		},
	}
}

// createNamespace creates a Namespace resource for the provided ns.
func (r *reconciler) createNamespace(ctx context.Context, ns *corev1.Namespace) error {
	if err := r.client.Create(ctx, ns); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", ns.Name, err)
	}
	r.log.Info("created namespace", "name", ns.Name)

	return nil
}

// currentSpecNsName returns the Namespace resource for spec.namespace.name of
// the provided contour.
func (r *reconciler) currentSpecNsName(ctx context.Context, name string) (*corev1.Namespace, error) {
	current := &corev1.Namespace{}
	key := types.NamespacedName{Name: name}
	err := r.client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// updateNamespaceIfNeeded updates a Namespace if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateNamespaceIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *corev1.Namespace) error {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("namespace missing owner labels; skipped updating", "name", current.Name)
		return nil
	}
	ns, updated := equality.NamespaceConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, ns); err != nil {
			return fmt.Errorf("failed to update namespace %s: %w", ns.Name, err)
		}
		r.log.Info("updated namespace", "namespace", ns.Name)
		return nil
	}
	r.log.Info("namespace unchanged; skipped updating", "name", current.Name)

	return nil
}
