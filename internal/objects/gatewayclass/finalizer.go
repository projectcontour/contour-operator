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

package gatewayclass

import (
	"context"
	"fmt"

	"github.com/projectcontour/contour-operator/pkg/slice"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const finalizer = gatewayv1alpha1.GatewayClassFinalizerGatewaysExist

// IsFinalized returns true if gc is finalized.
func IsFinalized(gc *gatewayv1alpha1.GatewayClass) bool {
	for _, f := range gc.Finalizers {
		if f == finalizer {
			return true
		}
	}
	return false
}

// EnsureFinalizer ensures the finalizer is added to the given gc.
func EnsureFinalizer(ctx context.Context, cli client.Client, gc *gatewayv1alpha1.GatewayClass) error {
	if !slice.ContainsString(gc.Finalizers, finalizer) {
		updated := gc.DeepCopy()
		updated.Finalizers = append(updated.Finalizers, finalizer)
		if err := cli.Update(ctx, updated); err != nil {
			return fmt.Errorf("failed to add finalizer %s: %w", finalizer, err)
		}
	}
	return nil
}

// EnsureFinalizerRemoved ensures the finalizer is removed for the given gc.
func EnsureFinalizerRemoved(ctx context.Context, cli client.Client, gc *gatewayv1alpha1.GatewayClass) error {
	if slice.ContainsString(gc.Finalizers, finalizer) {
		updated := gc.DeepCopy()
		updated.Finalizers = slice.RemoveString(updated.Finalizers, finalizer)
		if err := cli.Update(ctx, updated); err != nil {
			return fmt.Errorf("failed to remove finalizer %s: %w", finalizer, err)
		}
	}
	return nil
}
