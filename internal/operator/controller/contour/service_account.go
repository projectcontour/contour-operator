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
	utilequality "github.com/projectcontour/contour-operator/internal/equality"
	objutil "github.com/projectcontour/contour-operator/internal/object"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ensureServiceAccount ensures a ServiceAccount resource exists with the provided name
// and contour namespace/name for the owning contour labels.
func (r *reconciler) ensureServiceAccount(ctx context.Context, name string, contour *operatorv1alpha1.Contour) (*corev1.ServiceAccount, error) {
	desired := DesiredServiceAccount(name, contour)
	current, err := r.currentServiceAccount(ctx, contour.Spec.Namespace.Name, name)
	if err != nil {
		if errors.IsNotFound(err) {
			updated, err := r.createServiceAccount(ctx, desired)
			if err != nil {
				return nil, fmt.Errorf("failed to create service account %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("failed to get service account %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	updated, err := r.updateSvcAcctIfNeeded(ctx, contour, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update service account %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return updated, nil
}

// DesiredServiceAccount generates the desired ServiceAccount resource for the
// given contour.
func DesiredServiceAccount(name string, contour *operatorv1alpha1.Contour) *corev1.ServiceAccount {
	sa := objutil.NewServiceAccount(contour.Spec.Namespace.Name, name)
	sa.Labels = map[string]string{
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}

	return sa
}

// currentServiceAccount returns the current ServiceAccount for the provided ns/name.
func (r *reconciler) currentServiceAccount(ctx context.Context, ns, name string) (*corev1.ServiceAccount, error) {
	current := &corev1.ServiceAccount{}
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

// createServiceAccount creates a ServiceAccount resource for the provided sa.
func (r *reconciler) createServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	if err := r.client.Create(ctx, sa); err != nil {
		return nil, fmt.Errorf("failed to create service account %s/%s: %w", sa.Namespace, sa.Name, err)
	}
	r.log.Info("created service account", "namespace", sa.Namespace, "name", sa.Name)

	return sa, nil
}

// updateSvcAcctIfNeeded updates a ServiceAccount resource if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateSvcAcctIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("service account missing owner labels; skipped updating", "namespace", current.Namespace,
			"name", current.Name)
		return current, nil
	}

	sa, updated := utilequality.ServiceAccountConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, sa); err != nil {
			return nil, fmt.Errorf("failed to update service account %s/%s: %w", sa.Namespace, sa.Name, err)
		}
		r.log.Info("updated service account", "namespace", sa.Namespace, "name", sa.Name)
		return sa, nil
	}
	r.log.Info("service account unchanged; skipped updating", "namespace", current.Namespace, "name", current.Name)

	return current, nil
}
