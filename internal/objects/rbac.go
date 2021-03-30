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

package objects

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objcr "github.com/projectcontour/contour-operator/internal/objects/clusterrole"
	objcrb "github.com/projectcontour/contour-operator/internal/objects/clusterrolebinding"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objrole "github.com/projectcontour/contour-operator/internal/objects/role"
	objrb "github.com/projectcontour/contour-operator/internal/objects/rolebinding"
	objsa "github.com/projectcontour/contour-operator/internal/objects/serviceaccount"
	"github.com/projectcontour/contour-operator/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ContourRbacName is the name used for Contour RBAC resources.
	ContourRbacName = "contour"
	// EnvoyRbacName is the name used for Envoy RBAC resources.
	EnvoyRbacName = "envoy"
	// CertGenRbacName is the name used for Contour certificate
	// generation RBAC resources.
	CertGenRbacName = "contour-certgen"
)

// EnsureRBAC ensures all the necessary RBAC resources exist for the
// provided contour.
func EnsureRBAC(ctx context.Context, cli client.Client, contour *operatorv1alpha1.Contour) error {
	ns := contour.Spec.Namespace.Name
	names := []string{ContourRbacName, EnvoyRbacName, CertGenRbacName}
	certSvcAct := &corev1.ServiceAccount{}
	for _, name := range names {
		svcAct, err := objsa.EnsureServiceAccount(ctx, cli, name, contour)
		if err != nil {
			return fmt.Errorf("failed to ensure service account %s/%s: %w", ns, name, err)
		}
		if svcAct.Name == CertGenRbacName {
			certSvcAct = svcAct
		}
	}
	// ClusterRole and ClusterRoleBinding resources are namespace-named to allow ownership
	// from individual instances of Contour.
	nsName := fmt.Sprintf("%s-%s", ContourRbacName, contour.Spec.Namespace.Name)
	cr, err := objcr.EnsureClusterRole(ctx, cli, nsName, contour)
	if err != nil {
		return fmt.Errorf("failed to ensure cluster role %s: %w", ContourRbacName, err)
	}
	if err := objcrb.EnsureClusterRoleBinding(ctx, cli, nsName, cr.Name, ContourRbacName, contour); err != nil {
		return fmt.Errorf("failed to ensure cluster role binding %s: %w", ContourRbacName, err)
	}
	certRole, err := objrole.EnsureRole(ctx, cli, CertGenRbacName, contour)
	if err != nil {
		return fmt.Errorf("failed to ensure role %s/%s: %w", ns, CertGenRbacName, err)
	}
	if err := objrb.EnsureRoleBinding(ctx, cli, ContourRbacName, certSvcAct.Name, certRole.Name, contour); err != nil {
		return fmt.Errorf("failed to ensure role binding %s/%s: %w", ns, ContourRbacName, err)
	}
	return nil
}

// EnsureRBACDeleted ensures all the necessary RBAC resources for the provided
// contour are deleted if Contour owner labels exist.
func EnsureRBACDeleted(ctx context.Context, cli client.Client, contour *operatorv1alpha1.Contour) error {
	var errs []error
	ns := contour.Spec.Namespace.Name
	objectsToDelete := []client.Object{}
	contoursExist, err := objcontour.OtherContoursExistInSpecNs(ctx, cli, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours contoursExist in namespace %s: %w",
			contour.Spec.Namespace.Name, err)
	}
	if !contoursExist {
		cntrRoleBind, err := objrb.CurrentRoleBinding(ctx, cli, ns, ContourRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if cntrRoleBind != nil {
			objectsToDelete = append(objectsToDelete, cntrRoleBind)
		}
		cntrRole, err := objrole.CurrentRole(ctx, cli, ns, ContourRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if cntrRole != nil {
			objectsToDelete = append(objectsToDelete, cntrRole)
		}
		certRole, err := objrole.CurrentRole(ctx, cli, ns, CertGenRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if certRole != nil {
			objectsToDelete = append(objectsToDelete, certRole)
		}
		names := []string{ContourRbacName, EnvoyRbacName, CertGenRbacName}
		for _, name := range names {
			svcAct, err := objsa.CurrentServiceAccount(ctx, cli, ns, name)
			if err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
			if svcAct != nil {
				objectsToDelete = append(objectsToDelete, svcAct)
			}
		}
	}
	contoursExist, _, err = objcontour.OtherContoursExist(ctx, cli, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours exist in any namespace: %w", err)
	}
	if !contoursExist {
		// ClusterRole and ClusterRoleBinding resources are namespace-named to allow ownership
		// from individual instances of Contour.
		nsName := fmt.Sprintf("%s-%s", ContourRbacName, contour.Spec.Namespace.Name)
		crb, err := objcrb.CurrentClusterRoleBinding(ctx, cli, nsName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if crb != nil {
			objectsToDelete = append(objectsToDelete, crb)
		}
		cr, err := objcr.CurrentClusterRole(ctx, cli, nsName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if cr != nil {
			objectsToDelete = append(objectsToDelete, cr)
		}
	}
	for _, object := range objectsToDelete {
		kind := object.GetObjectKind().GroupVersionKind().Kind
		namespace := object.(metav1.Object).GetNamespace()
		name := object.(metav1.Object).GetName()
		if labels.Exist(object, objcontour.OwnerLabels(contour)) {
			if err := cli.Delete(ctx, object); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("failed to delete %s %s/%s: %w", kind, namespace, name, err)
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}
