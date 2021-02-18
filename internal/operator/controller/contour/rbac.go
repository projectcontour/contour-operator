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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// contourRbacName is the name used for Contour RBAC resources.
	contourRbacName = "contour"
	// envoyRbacName is the name used for Envoy RBAC resources.
	envoyRbacName = "envoy"
	// certGenRbacName is the name used for Contour certificate
	// generation RBAC resources.
	certGenRbacName = "contour-certgen"
)

// ensureRBAC ensures all the necessary RBAC resources exist for the
// provided contour.
func (r *reconciler) ensureRBAC(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	ns := contour.Spec.Namespace.Name
	names := []string{contourRbacName, envoyRbacName, certGenRbacName}
	certSvcAct := &corev1.ServiceAccount{}
	for _, name := range names {
		svcAct, err := r.ensureServiceAccount(ctx, name, contour)
		if err != nil {
			return fmt.Errorf("failed to ensure service account %s/%s: %w", ns, name, err)
		}
		if svcAct.Name == certGenRbacName {
			certSvcAct = svcAct
		}
	}
	cr, err := r.ensureClusterRole(ctx, contourRbacName, contour)
	if err != nil {
		return fmt.Errorf("failed to ensure cluster role %s: %w", contourRbacName, err)
	}
	if err := r.ensureClusterRoleBinding(ctx, contourRbacName, cr.Name, contourRbacName, contour); err != nil {
		return fmt.Errorf("failed to ensure cluster role binding %s: %w", contourRbacName, err)
	}
	certRole, err := r.ensureRole(ctx, certGenRbacName, contour)
	if err != nil {
		return fmt.Errorf("failed to ensure role %s/%s: %w", ns, certGenRbacName, err)
	}
	if err := r.ensureRoleBinding(ctx, contourRbacName, certSvcAct.Name, certRole.Name, contour); err != nil {
		return fmt.Errorf("failed to ensure role binding %s/%s: %w", ns, contourRbacName, err)
	}

	return nil
}

// ensureRBACRemoved ensures all the necessary RBAC resources for the provided
// contour are deleted if Contour owner labels exist.
func (r *reconciler) ensureRBACRemoved(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	var errs []error
	ns := contour.Spec.Namespace.Name
	objectsToDelete := []client.Object{}
	contoursExist, err := r.otherContoursExistInSpecNs(ctx, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours contoursExist in namespace %s: %w",
			contour.Spec.Namespace.Name, err)
	}
	if !contoursExist {
		cntrRoleBind, err := r.currentRoleBinding(ctx, ns, contourRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if cntrRoleBind != nil {
			objectsToDelete = append(objectsToDelete, cntrRoleBind)
		}
		cntrRole, err := r.currentRole(ctx, ns, contourRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if cntrRole != nil {
			objectsToDelete = append(objectsToDelete, cntrRole)
		}
		certRole, err := r.currentRole(ctx, ns, certGenRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if certRole != nil {
			objectsToDelete = append(objectsToDelete, certRole)
		}
		names := []string{contourRbacName, envoyRbacName, certGenRbacName}
		for _, name := range names {
			svcAct, err := r.currentServiceAccount(ctx, ns, name)
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
	contoursExist, _, err = r.otherContoursExist(ctx, contour)
	if err != nil {
		return fmt.Errorf("failed to verify if contours exist in any namespace: %w", err)
	}
	if !contoursExist {
		crb, err := r.currentClusterRoleBinding(ctx, contourRbacName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
		if crb != nil {
			objectsToDelete = append(objectsToDelete, crb)
		}
		cr, err := r.currentClusterRole(ctx, contourRbacName)
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
		if !ownerLabelsExist(object.(metav1.Object), contour) {
			r.log.Info("object not labeled; skipping deletion", "kind", kind, "namespace", namespace,
				"name", name)
		} else {
			if err := r.client.Delete(ctx, object); err != nil {
				if errors.IsNotFound(err) {
					r.log.Info("object does not exist; skipping removal", "kind", kind, "namespace",
						namespace, "name", name)
					continue
				}
				return fmt.Errorf("failed to delete %s %s/%s: %w", kind, namespace, name, err)
			}
			r.log.Info("deleted object", "kind", kind, "namespace", namespace, "name", name)
		}
	}
	return utilerrors.NewAggregate(errs)
}
