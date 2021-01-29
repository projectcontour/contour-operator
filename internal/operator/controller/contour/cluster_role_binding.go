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
	objutil "github.com/projectcontour/contour-operator/internal/object"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ensureClusterRoleBinding ensures a ClusterRoleBinding resource with the provided
// name exists, using roleRef for the role reference, svcAct for the subject and
// the contour namespace/name for the owning contour labels.
func (r *reconciler) ensureClusterRoleBinding(ctx context.Context, name, roleRef, svcAct string, contour *operatorv1alpha1.Contour) error {
	desired := desiredClusterRoleBinding(name, roleRef, svcAct, contour)
	current, err := r.currentClusterRoleBinding(ctx, name)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.createClusterRoleBinding(ctx, desired); err != nil {
				return fmt.Errorf("failed to create cluster role binding %s: %w", desired.Name, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get cluster role binding %s: %w", desired.Name, err)
	}

	if err := r.updateClusterRoleBindingIfNeeded(ctx, contour, current, desired); err != nil {
		return fmt.Errorf("failed to update cluster role binding %s: %w", desired.Name, err)
	}

	return nil
}

// desiredClusterRoleBinding constructs an instance of the desired ClusterRoleBinding
// resource with the provided name, contour namespace/name for the owning contour
// labels, roleRef for the role reference, and svcAcctRef for the subject.
func desiredClusterRoleBinding(name, roleRef, svcAcctRef string, contour *operatorv1alpha1.Contour) *rbacv1.ClusterRoleBinding {
	crb := objutil.NewClusterRoleBinding(name)
	crb.Labels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}
	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			APIGroup:  corev1.GroupName,
			Name:      svcAcctRef,
			Namespace: contour.Spec.Namespace.Name,
		},
	}
	crb.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleRef,
	}

	return crb
}

// currentClusterRoleBinding returns the current ClusterRoleBinding for the
// provided name.
func (r *reconciler) currentClusterRoleBinding(ctx context.Context, name string) (*rbacv1.ClusterRoleBinding, error) {
	current := &rbacv1.ClusterRoleBinding{}
	key := types.NamespacedName{Name: name}
	err := r.client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// createClusterRoleBinding creates a ClusterRoleBinding resource for the provided crb.
func (r *reconciler) createClusterRoleBinding(ctx context.Context, crb *rbacv1.ClusterRoleBinding) error {
	if err := r.client.Create(ctx, crb); err != nil {
		return fmt.Errorf("failed to create cluster role binding %s: %w", crb.Name, err)
	}
	r.log.Info("created cluster role binding", "name", crb.Name)

	return nil
}

// updateClusterRoleBindingIfNeeded updates a ClusterRoleBinding resource if current
// does not match desired, using contour to verify the existence of owner labels.
func (r *reconciler) updateClusterRoleBindingIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *rbacv1.ClusterRoleBinding) error {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("cluster role binding missing owner labels; skipped updating", "name", current.Name)
		return nil
	}

	crb, updated := equality.ClusterRoleBindingConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, crb); err != nil {
			return fmt.Errorf("failed to update cluster role binding %s: %w", crb.Name, err)
		}
		r.log.Info("updated cluster role binding", "name", crb.Name)
		return nil
	}
	r.log.Info("cluster role binding unchanged; skipped updating", "name", current.Name)

	return nil
}
