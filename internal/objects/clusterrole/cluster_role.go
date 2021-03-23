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

package clusterrole

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/equality"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	"github.com/projectcontour/contour-operator/pkg/labels"

	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// EnsureClusterRole ensures a ClusterRole resource exists with the provided name
// and contour namespace/name for the owning contour labels.
func EnsureClusterRole(ctx context.Context, cli client.Client, name string, contour *operatorv1alpha1.Contour) (*rbacv1.ClusterRole, error) {
	desired := desiredClusterRole(name, contour)
	current, err := CurrentClusterRole(ctx, cli, name)
	if err != nil {
		if errors.IsNotFound(err) {
			updated, err := createClusterRole(ctx, cli, desired)
			if err != nil {
				return nil, fmt.Errorf("failed to create cluster role %s: %w", desired.Name, err)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("failed to get cluster role %s: %w", desired.Name, err)
	}
	updated, err := updateClusterRoleIfNeeded(ctx, cli, contour, current, desired)
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
	groupGateway := []string{gatewayv1alpha1.GroupName}
	groupExt := []string{apiextensionsv1.GroupName}
	groupContour := []string{projectcontourv1.GroupName}
	verbCGU := []string{"create", "get", "update"}
	verbGLW := []string{"get", "list", "watch"}
	verbGLWU := []string{"get", "list", "watch", "update"}

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
	ns := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupAll,
		Resources: []string{"namespaces"},
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
	gateway := rbacv1.PolicyRule{
		Verbs:     verbGLWU,
		APIGroups: groupGateway,
		Resources: []string{"gatewayclasses", "gateways", "backendpolicies", "httproutes", "tlsroutes"},
	}
	gatewayStatus := rbacv1.PolicyRule{
		Verbs:     verbCGU,
		APIGroups: groupGateway,
		Resources: []string{"gatewayclasses/status", "gateways/status", "backendpolicies/status", "httproutes/status",
			"tlsroutes/status"},
	}
	ing := rbacv1.PolicyRule{
		Verbs:     verbGLW,
		APIGroups: groupNet,
		Resources: []string{"ingresses", "ingressclasses"},
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

	cr := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind: "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	cr.Labels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}
	cr.Rules = []rbacv1.PolicyRule{cfgMap, endPt, secret, svc, gateway, gatewayStatus, ing, ingStatus, cntr, cntrStatus, crd, ns}
	return cr
}

// CurrentClusterRole returns the current ClusterRole for the provided name.
func CurrentClusterRole(ctx context.Context, cli client.Client, name string) (*rbacv1.ClusterRole, error) {
	current := &rbacv1.ClusterRole{}
	key := types.NamespacedName{Name: name}
	err := cli.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// createClusterRole creates a ClusterRole resource for the provided cr.
func createClusterRole(ctx context.Context, cli client.Client, cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if err := cli.Create(ctx, cr); err != nil {
		return nil, fmt.Errorf("failed to create cluster role %s: %w", cr.Name, err)
	}
	return cr, nil
}

// updateClusterRoleIfNeeded updates a ClusterRole resource if current does not match desired,
// using contour to verify the existence of owner labels.
func updateClusterRoleIfNeeded(ctx context.Context, cli client.Client, contour *operatorv1alpha1.Contour, current, desired *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	if labels.Exist(current, objcontour.OwnerLabels(contour)) {
		cr, updated := equality.ClusterRoleConfigChanged(current, desired)
		if updated {
			if err := cli.Update(ctx, cr); err != nil {
				return nil, fmt.Errorf("failed to update cluster role %s: %w", cr.Name, err)
			}
			return cr, nil
		}
	}
	return current, nil
}
