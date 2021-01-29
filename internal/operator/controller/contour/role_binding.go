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
	equality "github.com/projectcontour/contour-operator/internal/equality"
	objutil "github.com/projectcontour/contour-operator/internal/object"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ensureRoleBinding ensures a RoleBinding resource exists with the provided
// ns/name and contour namespace/name for the owning contour labels.
// The RoleBinding will use svcAct for the subject and role for the role reference.
func (r *reconciler) ensureRoleBinding(ctx context.Context, name, svcAct, role string, contour *operatorv1alpha1.Contour) error {
	desired := desiredRoleBinding(name, svcAct, role, contour)
	current, err := r.currentRoleBinding(ctx, contour.Spec.Namespace.Name, name)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.createRoleBinding(ctx, desired); err != nil {
				return fmt.Errorf("failed to create role binding %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get role binding %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	if err := r.updateRoleBindingIfNeeded(ctx, contour, current, desired); err != nil {
		return fmt.Errorf("failed to update role binding %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return nil
}

// desiredRoleBinding constructs an instance of the desired RoleBinding resource
// with the provided name in Contour spec Namespace, using contour namespace/name
// for the owning contour labels. The RoleBinding will use svcAct for the subject
// and role for the role reference.
func desiredRoleBinding(name, svcAcctRef, roleRef string, contour *operatorv1alpha1.Contour) *rbacv1.RoleBinding {
	rb := objutil.NewRoleBinding(contour.Spec.Namespace.Name, name)
	rb.Labels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}
	rb.Subjects = []rbacv1.Subject{{
		Kind:      "ServiceAccount",
		APIGroup:  corev1.GroupName,
		Name:      svcAcctRef,
		Namespace: contour.Spec.Namespace.Name,
	}}
	rb.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     roleRef,
	}

	return rb
}

// currentRoleBinding returns the current RoleBinding for the provided ns/name.
func (r *reconciler) currentRoleBinding(ctx context.Context, ns, name string) (*rbacv1.RoleBinding, error) {
	current := &rbacv1.RoleBinding{}
	key := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	err := r.client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// createRoleBinding creates a RoleBinding resource for the provided rb.
func (r *reconciler) createRoleBinding(ctx context.Context, rb *rbacv1.RoleBinding) error {
	if err := r.client.Create(ctx, rb); err != nil {
		return fmt.Errorf("failed to create role binding %s/%s: %w", rb.Namespace, rb.Name, err)
	}
	r.log.Info("created role binding", "namespace", rb.Namespace, "name", rb.Name)

	return nil
}

// updateRoleBindingIfNeeded updates a RoleBinding resource if current does
// not match desired.
func (r *reconciler) updateRoleBindingIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *rbacv1.RoleBinding) error {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("role binding missing owner labels; skipped updating",
			"namespace", current.Namespace, "name", current.Name)
		return nil
	}

	rb, updated := equality.RoleBindingConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, rb); err != nil {
			return fmt.Errorf("failed to update role binding %s/%s: %w", rb.Namespace, rb.Name, err)
		}
		r.log.Info("updated role binding", "namespace", rb.Namespace, "name", rb.Name)
		return nil
	}
	r.log.Info("role binding unchanged; skipped updating", "namespace", current.Namespace,
		"name", current.Name)

	return nil
}
