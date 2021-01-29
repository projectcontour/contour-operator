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

	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ensureClusterRole ensures a ClusterRole resource exists with the provided name
// and contour namespace/name for the owning contour labels.
func (r *reconciler) ensureClusterRole(ctx context.Context, name string, contour *operatorv1alpha1.Contour) (*rbacv1.ClusterRole, error) {
	desired := desiredClusterRole(name, contour)
	current, err := r.currentClusterRole(ctx, name)
	if err != nil {
		if errors.IsNotFound(err) {
			updated, err := r.createClusterRole(ctx, desired)
			if err != nil {
				return nil, fmt.Errorf("failed to create cluster role %s: %w", desired.Name, err)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("failed to get cluster role %s: %w", desired.Name, err)
	}

	updated, err := r.updateClusterRoleIfNeeded(ctx, contour, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update cluster role %s: %w", desired.Name, err)
	}

	return updated, nil
}

// desiredClusterRole constructs an instance of the desired ClusterRole resource with
// the provided name and contour namespace/name for the owning contour labels.
func desiredClusterRole(name string, contour *operatorv1alpha1.Contour) *rbacv1.ClusterRole {
	groupAll := []string{corev1.GroupName}
	groupNet := []string{networkingv1.GroupName}
	groupExt := []string{apiextensionsv1.GroupName}
	groupContour := []string{projectcontourv1.GroupName}
	verbCGU := []string{"create", "get", "update"}
	verbGLW := []string{"get", "list", "watch"}

	cfgMap := rbacv1.PolicyRule{
		Verbs:     verbCGU,
		APIGroups: groupAll,
		Resources: []string{"configmaps"},
	}
	endPt := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupAll,
		Resources: []string{"endpoints"},
	}
	secret := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupAll,
		Resources: []string{"secrets"},
	}
	svc := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupAll,
		Resources: []string{"services"},
	}
	crd := rbacv1.PolicyRule{
		Verbs:     []string{"list"},
		APIGroups: groupExt,
		Resources: []string{"customresourcedefinitions"},
	}
	svcAPI := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupNet,
		Resources: []string{"gatewayclasses", "gateways", "httproutes", "tcproutes", "backendpolicies"},
	}
	ing := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupNet,
		Resources: []string{"ingresses"},
	}
	ingStatus := rbacv1.PolicyRule{
		Verbs:     verbCGU,
		APIGroups: groupNet,
		Resources: []string{"ingresses/status"},
	}
	cntr := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupContour,
		Resources: []string{"httpproxies", "tlscertificatedelegations", "extensionservices"},
	}
	cntrStatus := rbacv1.PolicyRule{
		Verbs:     verbCGU,
		APIGroups: groupContour,
		Resources: []string{"httpproxies/status", "extensionservices/status"},
	}

	cr := objutil.NewClusterRole(name)
	cr.Labels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}
	cr.Rules = []rbacv1.PolicyRule{cfgMap, endPt, secret, svc, svcAPI, ing, ingStatus, cntr, cntrStatus, crd}
	return cr
}

// currentClusterRole returns the current ClusterRole for the provided name.
func (r *reconciler) currentClusterRole(ctx context.Context, name string) (*rbacv1.ClusterRole, error) {
	current := &rbacv1.ClusterRole{}
	key := types.NamespacedName{Name: name}
	err := r.client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// createClusterRole creates a ClusterRole resource for the provided cr.
func (r *reconciler) createClusterRole(ctx context.Context, cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if err := r.client.Create(ctx, cr); err != nil {
		return nil, fmt.Errorf("failed to create cluster role %s: %w", cr.Name, err)
	}
	r.log.Info("created cluster role", "name", cr.Name)

	return cr, nil
}

// updateClusterRoleIfNeeded updates a ClusterRole resource if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateClusterRoleIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("cluster role missing owner labels; skipped updating", "name", current.Name)
		return current, nil
	}

	cr, updated := equality.ClusterRoleConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, cr); err != nil {
			return nil, fmt.Errorf("failed to update cluster role %s: %w", cr.Name, err)
		}
		r.log.Info("updated cluster role", "name", cr.Name)
		return cr, nil
	}
	r.log.Info("cluster role unchanged; skipped updating", "name", current.Name)

	return current, nil
}
