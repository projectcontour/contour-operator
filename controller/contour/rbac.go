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
	oputil "github.com/projectcontour/contour-operator/util"
	projectcontourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"

	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	// defaultContourRbacName is the default name used for Contour
	// RBAC resources.
	defaultContourRbacName = "contour"
	// defaultEnvoyRbacName is the default name used for Envoy RBAC resources.
	defaultEnvoyRbacName = "envoy"
	// defaultCertGenRbacName is the default name used for Contour
	// certificate generation RBAC resources.
	defaultCertGenRbacName = "contour-certgen"
)

// ensureRBAC ensures all the necessary RBAC resources exist for the
// provided contour.
func (r *Reconciler) ensureRBAC(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	ns := contour.Spec.Namespace.Name
	ctr := types.NamespacedName{Namespace: ns, Name: defaultContourRbacName}
	envoy := types.NamespacedName{Namespace: ns, Name: defaultEnvoyRbacName}
	certGen := types.NamespacedName{Namespace: ns, Name: defaultCertGenRbacName}
	names := []types.NamespacedName{ctr, envoy, certGen}
	certSvcAct := &corev1.ServiceAccount{}
	for _, name := range names {
		svcAct, err := r.ensureServiceAccount(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to ensure service account for contour %s/%s: %w", contour.Namespace, contour.Name, err)
		}
		if svcAct.Name == defaultCertGenRbacName {
			certSvcAct = svcAct
		}
	}
	cr, err := r.ensureClusterRole(ctx, defaultContourRbacName)
	if err != nil {
		return fmt.Errorf("failed to ensure cluster role for contour %s/%s: %w", contour.Namespace,
			contour.Name, err)
	}
	if err := r.ensureClusterRoleBinding(ctx, defaultContourRbacName, cr.Name, ctr); err != nil {
		return fmt.Errorf("failed to ensure cluster role binding for contour %s/%s: %w", contour.Namespace,
			contour.Name, err)
	}
	certRole, err := r.ensureRole(ctx, certGen)
	if err != nil {
		return fmt.Errorf("failed to ensure role for contour %s/%s: %w", contour.Namespace,
			contour.Name, err)
	}
	if err := r.ensureRoleBinding(ctx, ctr, certSvcAct, certRole); err != nil {
		return fmt.Errorf("failed to ensure role binding for contour %s/%s: %w", contour.Namespace,
			contour.Name, err)
	}

	return nil
}

// ensureServiceAccount ensures a ServiceAccount resource exists with the provided name.
func (r *Reconciler) ensureServiceAccount(ctx context.Context, name types.NamespacedName) (*corev1.ServiceAccount, error) {
	sa := oputil.NewServiceAccount(name.Namespace, name.Name)
	if err := r.Client.Create(context.TODO(), sa); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("service account exists; skipped adding", "namespace", sa.Namespace, "name", sa.Name)
			return sa, nil
		}
		return nil, fmt.Errorf("failed to create service account %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	r.Log.Info("created service account", "namespace", sa.Namespace, "name", sa.Name)
	return sa, nil
}

// ensureClusterRole ensures a ClusterRole resource exists with the
// provided name.
func (r *Reconciler) ensureClusterRole(ctx context.Context, name string) (*rbacv1.ClusterRole, error) {
	cr := desiredClusterRole(name)
	if err := r.Client.Create(ctx, cr); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("cluster role exists; skipped adding", "name", cr.Name)
			return cr, nil
		}
		return nil, fmt.Errorf("failed to create cluster role %s: %w", cr.Name, err)
	}
	r.Log.Info("created cluster role", "name", cr.Name)
	return cr, nil
}

// desiredClusterRole constructs an instance of the desired ClusterRole resource
// with the provided name.
func desiredClusterRole(name string) *rbacv1.ClusterRole {
	groupAll := []string{corev1.GroupName}
	groupNet := []string{networkingv1beta1.GroupName}
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
		// TODO [danehans]: Update roles when v1alpha1 Service APIs are released.
		Resources: []string{"gatewayclasses", "gateways", "httproutes", "tcproutes"},
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

	cr := oputil.NewClusterRole(name)
	cr.Rules = []rbacv1.PolicyRule{cfgMap, endPt, secret, svc, svcAPI, ing, ingStatus, cntr, cntrStatus, crd}
	return cr
}

// ensureRole ensures a Role resource with the provided name exists.
func (r *Reconciler) ensureRole(ctx context.Context, name types.NamespacedName) (*rbacv1.Role, error) {
	role := desiredRole(name)
	if err := r.Client.Create(ctx, role); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("role exists; skipped adding", "namespace", role.Namespace, "name", role.Name)
			return role, nil
		}
		return nil, fmt.Errorf("failed to create role %s/%s: %w", role.Namespace, role.Name, err)
	}
	r.Log.Info("created role", "namespace", role.Namespace, "name", role.Name)
	return role, nil
}

// desiredRole constructs an instance of the desired Role resource with
// the provided name.
func desiredRole(name types.NamespacedName) *rbacv1.Role {
	groupAll := []string{""}
	verbCU := []string{"create", "update"}
	secret := rbacv1.PolicyRule{
		Verbs:     verbCU,
		APIGroups: groupAll,
		Resources: []string{"secrets"},
	}
	role := oputil.NewRole(name.Namespace, name.Name)
	role.Rules = []rbacv1.PolicyRule{secret}
	return role
}

// ensureRoleBinding ensures a RoleBinding resource with the provided name exists.
// The RoleBinding will use svcAct for the subject and role for the role reference.
func (r *Reconciler) ensureRoleBinding(ctx context.Context, name types.NamespacedName, svcAct *corev1.ServiceAccount, role *rbacv1.Role) error {
	rb := oputil.NewRoleBinding(name.Namespace, name.Name)
	rb.Subjects = []rbacv1.Subject{
		rbacv1.Subject{
			Kind:      "ServiceAccount",
			APIGroup:  corev1.GroupName,
			Name:      svcAct.Name,
			Namespace: svcAct.Namespace,
		}}
	rb.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	if err := r.Client.Create(ctx, rb); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("role binding exists; skipped adding", "name", rb.Name)
			return nil
		}
		return fmt.Errorf("failed to create role binding %s: %w", rb.Name, err)
	}
	r.Log.Info("created role binding", "name", rb.Name)
	return nil
}

// ensureClusterRoleBinding ensures a ClusterRoleBinding resource with the provided
// name exists, using roleName for the role reference and svcAct for the subject.
func (r *Reconciler) ensureClusterRoleBinding(ctx context.Context, name, roleName string, svcAct types.NamespacedName) error {
	crb := oputil.NewClusterRoleBinding(name)
	crb.Subjects = []rbacv1.Subject{
		rbacv1.Subject{
			Kind:      "ServiceAccount",
			APIGroup:  corev1.GroupName,
			Name:      svcAct.Name,
			Namespace: svcAct.Namespace,
		},
	}
	crb.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleName,
	}

	if err := r.Client.Create(ctx, crb); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("cluster role binding exists; skipped adding", "name", crb.Name)
			return nil
		}
		return fmt.Errorf("failed to create cluster role binding %s: %w", crb.Name, err)
	}
	r.Log.Info("created cluster role binding", "name", crb.Name)
	return nil
}

// ensureRBACRemoved ensures all the necessary RBAC resources for the provided
// contour do not exist.
func (r *Reconciler) ensureRBACRemoved(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	var errs []error
	ns := contour.Spec.Namespace.Name
	objectsToDelete := []runtime.Object{}
	exist, err := r.otherContoursExistInSpecNs(ctx, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours exist in namespace %s: %w",
			contour.Spec.Namespace.Name, err)
	}
	if !exist {
		cntrRoleBind := oputil.NewRoleBinding(ns, defaultContourRbacName)
		cntrRole := oputil.NewRole(ns, defaultContourRbacName)
		certRole := oputil.NewRole(ns, defaultCertGenRbacName)
		objectsToDelete = append(objectsToDelete, cntrRoleBind, cntrRole, certRole)
		names := []string{defaultContourRbacName, defaultEnvoyRbacName, defaultCertGenRbacName}
		for _, name := range names {
			svcAct := oputil.NewServiceAccount(ns, name)
			objectsToDelete = append(objectsToDelete, svcAct)
		}
	}
	exist, _, err = r.otherContoursExist(ctx, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours exist in any namespace: %w", err)
	}
	if !exist {
		crb := oputil.NewClusterRoleBinding(defaultContourRbacName)
		cr := oputil.NewClusterRole(defaultContourRbacName)
		objectsToDelete = append(objectsToDelete, crb, cr)
	}
	for _, object := range objectsToDelete {
		kind := object.GetObjectKind().GroupVersionKind().Kind
		namespace := object.(metav1.Object).GetNamespace()
		name := object.(metav1.Object).GetName()
		if err := r.Client.Delete(ctx, object); err != nil {
			if errors.IsNotFound(err) {
				r.Log.Info("object does not exist; skipping removal", "kind", kind, "namespace",
					namespace, "name", name)
				continue
			}
			return fmt.Errorf("failed to delete %s %s/%s: %w", kind, namespace, name, err)
		}
		r.Log.Info("deleted object", "kind", kind, "namespace", namespace, "name", name)
	}
	return utilerrors.NewAggregate(errs)
}

// ensureRoleRemoved ensures the Role resource with the provided name
// does not exist.
func (r *Reconciler) ensureRoleRemoved(ctx context.Context, name types.NamespacedName) error {
	role := oputil.NewRole(name.Namespace, name.Name)
	if err := r.Client.Delete(ctx, role); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("role does not exist; skipping removal", "namespace", role.Namespace,
				"name", role.Name)
			return nil
		}
		return fmt.Errorf("failed to remove role %s/%s: %w", role.Namespace, role.Name, err)
	}
	r.Log.Info("removed role", "namespace", role.Namespace, "name", role.Name)
	return nil
}

// ensureRoleBindingRemoved ensures the RoleBinding resource with the provided
// name does not exist.
func (r *Reconciler) ensureRoleBindingRemoved(ctx context.Context, name types.NamespacedName) error {
	rb := oputil.NewRoleBinding(name.Namespace, name.Name)
	if err := r.Client.Delete(ctx, rb); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("role binding does not exist; skipping removal", "name", rb.Name)
			return nil
		}
		return fmt.Errorf("failed to delete role binding %s: %w", rb.Name, err)
	}
	r.Log.Info("deleted role binding", "name", rb.Name)
	return nil
}

// ensureClusterRoleBindingRemoved ensures the ClusterRoleBinding resource with the
// provided name does not exist.
func (r *Reconciler) ensureClusterRoleBindingRemoved(ctx context.Context, name types.NamespacedName) error {
	crb := oputil.NewClusterRoleBinding(name.Name)
	if err := r.Client.Delete(ctx, crb); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("cluster role binding does not exist; skipping removal", "name", crb.Name)
			return nil
		}
		return fmt.Errorf("failed to delete cluster role binding %s: %w", crb.Name, err)
	}
	r.Log.Info("deleted cluster role binding", "name", crb.Name)
	return nil
}

// ensureClusterRoleRemoved ensures the ClusterRole resource with the provided
// name does not exist.
func (r *Reconciler) ensureClusterRoleRemoved(ctx context.Context, name types.NamespacedName) error {
	cr := oputil.NewClusterRole(name.Name)
	if err := r.Client.Delete(ctx, cr); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("cluster role does not exist; skipping removal", "name", cr.Name)
			return nil
		}
		return fmt.Errorf("failed to get cluster role %s: %w", cr.Name, err)
	}
	r.Log.Info("deleted cluster role", "name", cr.Name)
	return nil
}

// ensureServiceAccountRemoved ensures the ServiceAccount resource with the provided
// name does not exist.
func (r *Reconciler) ensureServiceAccountRemoved(ctx context.Context, name types.NamespacedName) error {
	sa := oputil.NewServiceAccount(name.Namespace, name.Name)
	if err := r.Client.Delete(ctx, sa); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("service account doesn't exist; skipping removal", "namespace", sa.Namespace,
				"name", sa.Name)
			return nil
		}
		return fmt.Errorf("failed to get service account %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	r.Log.Info("deleted service account", "namespace", sa.Namespace, "name", sa.Name)
	return nil
}
