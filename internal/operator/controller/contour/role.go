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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ensureRole ensures a Role resource exists with the provided name/ns
// and contour namespace/name for the owning contour labels.
func (r *reconciler) ensureRole(ctx context.Context, name string, contour *operatorv1alpha1.Contour) (*rbacv1.Role, error) {
	desired := desiredRole(name, contour)
	current, err := r.currentRole(ctx, contour.Spec.Namespace.Name, name)
	if err != nil {
		if errors.IsNotFound(err) {
			updated, err := r.createRole(ctx, desired)
			if err != nil {
				return nil, fmt.Errorf("failed to create role %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("failed to get role %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	updated, err := r.updateRoleIfNeeded(ctx, contour, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update role %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return updated, nil
}

// desiredRole constructs an instance of the desired ClusterRole resource with the
// provided ns/name and contour namespace/name for the owning contour labels.
func desiredRole(name string, contour *operatorv1alpha1.Contour) *rbacv1.Role {
	role := objutil.NewRole(contour.Spec.Namespace.Name, name)
	groupAll := []string{""}
	verbCU := []string{"create", "update"}
	secret := rbacv1.PolicyRule{
		Verbs:     verbCU,
		APIGroups: groupAll,
		Resources: []string{"secrets"},
	}
	role.Rules = []rbacv1.PolicyRule{secret}
	role.Labels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}

	return role
}

// currentRole returns the current Role for the provided ns/name.
func (r *reconciler) currentRole(ctx context.Context, ns, name string) (*rbacv1.Role, error) {
	current := &rbacv1.Role{}
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

// createRole creates a Role resource for the provided role.
func (r *reconciler) createRole(ctx context.Context, role *rbacv1.Role) (*rbacv1.Role, error) {
	if err := r.client.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("failed to create role %s/%s: %w", role.Namespace, role.Name, err)
	}
	r.log.Info("created role", "namespace", role.Namespace, "name", role.Name)

	return role, nil
}

// updateRoleIfNeeded updates a Role resource if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateRoleIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *rbacv1.Role) (*rbacv1.Role, error) {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("role missing owner labels; skipped updating", "namespace", current.Namespace,
			"name", current.Name)
		return current, nil
	}

	role, updated := equality.RoleConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, role); err != nil {
			return nil, fmt.Errorf("failed to update cluster role %s/%s: %w", role.Namespace, role.Name, err)
		}
		r.log.Info("updated cluster role", "namespace", role.Namespace, "name", role.Name)
		return role, nil
	}
	r.log.Info("role unchanged; skipped updating", "namespace", current.Namespace, "name", current.Name)

	return current, nil
}
