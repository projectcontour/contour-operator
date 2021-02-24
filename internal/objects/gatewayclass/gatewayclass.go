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

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// Get returns a GatewayClass named name, if it exists.
func Get(ctx context.Context, cli client.Client, name string) (*gatewayv1alpha1.GatewayClass, error) {
	gc := &gatewayv1alpha1.GatewayClass{}
	key := types.NamespacedName{Name: name}
	if err := cli.Get(ctx, key, gc); err != nil {
		return nil, fmt.Errorf("failed to get gatewayclass %s: %w", name, err)
	}
	return gc, nil
}

// Admitted return true if the GatewayClass specified by name is admitted.
func Admitted(ctx context.Context, cli client.Client, name string) (bool, error) {
	gc, err := Get(ctx, cli, name)
	if err != nil {
		return false, fmt.Errorf("failed to verify admission for gatewayclass %s: %w", name, err)
	}
	if gc != nil {
		for _, c := range gc.Status.Conditions {
			if c.Type == string(gatewayv1alpha1.ConditionRouteAdmitted) && c.Status == metav1.ConditionTrue {
				return true, nil
			}
		}
	}
	return false, nil
}

// IsController returns true if the operator is the controller for gc.
func IsController(gc *gatewayv1alpha1.GatewayClass) bool {
	return gc.Spec.Controller == operatorv1alpha1.GatewayClassControllerRef
}

// ParameterRefExists returns true if a GatewayClass exists with a parametersRef
// ns/name that matches the provided ns/name.
func ParameterRefExists(ctx context.Context, cli client.Client, name, ns string) (*gatewayv1alpha1.GatewayClass, bool, error) {
	gcList := &gatewayv1alpha1.GatewayClassList{}
	if err := cli.List(ctx, gcList); err != nil {
		return nil, false, err
	}
	for _, gc := range gcList.Items {
		if gc.Spec.ParametersRef.Name == name && gc.Spec.ParametersRef.Namespace == ns {
			return &gc, true, nil
		}
	}
	return nil, false, nil
}

// OtherGatewayClassesRefContour returns true if GatewayClasses other than gc reference contour.
func OtherGatewayClassesRefContour(ctx context.Context, cli client.Client, gc *gatewayv1alpha1.GatewayClass, contour *operatorv1alpha1.Contour) (bool, error) {
	gcList := &gatewayv1alpha1.GatewayClassList{}
	if err := cli.List(ctx, gcList); err != nil {
		return false, fmt.Errorf("failed to list gateways")
	}
	if gcList != nil {
		for _, g := range gcList.Items {
			if g.Spec.ParametersRef != nil {
				switch {
				case g.Name == gc.Name && g.Namespace == gc.Namespace:
					continue
				case g.Spec.ParametersRef.Namespace == contour.Namespace &&
					g.Spec.ParametersRef.Name == contour.Name &&
					g.Spec.ParametersRef.Scope == "Namespace" &&
					g.Spec.ParametersRef.Kind == contour.Kind:
					return true, nil
				}
			}
		}
	}
	return false, nil
}
