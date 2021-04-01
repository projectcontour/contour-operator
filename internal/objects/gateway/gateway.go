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

package gateway

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objgc "github.com/projectcontour/contour-operator/internal/objects/gatewayclass"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// OtherGatewaysExist lists Gateway objects in all namespaces, returning the list
// if any exist other than gw.
func OtherGatewaysExist(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (*gatewayv1alpha1.GatewayList, error) {
	gwList := &gatewayv1alpha1.GatewayList{}
	if err := cli.List(ctx, gwList); err != nil {
		return nil, fmt.Errorf("failed to list gateways: %w", err)
	}
	if len(gwList.Items) == 0 || len(gwList.Items) == 1 && gwList.Items[0].Name == gw.Name {
		return nil, nil
	}
	return gwList, nil
}

// OtherGatewaysExistInNs lists Gateway objects in the same namespace as gw,
// returning true if any exist.
func OtherGatewaysExistInNs(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (bool, error) {
	gwList, err := OtherGatewaysExist(ctx, cli, gw)
	if err != nil {
		return false, err
	}
	if len(gwList.Items) > 0 {
		for _, item := range gwList.Items {
			if item.Namespace == gw.Namespace {
				return true, nil
			}
		}
	}
	return false, nil
}

// OwningSelector returns a label selector using "contour.operator.projectcontour.io/owning-gateway-name"
// and "contour.operator.projectcontour.io/owning-gateway-namespace" labels.
func OwningSelector(gw *gatewayv1alpha1.Gateway) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			operatorv1alpha1.OwningGatewayNameLabel: gw.Name,
			operatorv1alpha1.OwningGatewayNsLabel:   gw.Namespace,
		},
	}
}

// ContourForGateway returns the Contour associated to gw, if one exists and is
// managed by the operator.
func ContourForGateway(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (*operatorv1alpha1.Contour, error) {
	gc, err := ClassForGateway(ctx, cli, gw)
	if err != nil {
		return nil, err
	}
	if gc == nil {
		return nil, nil
	}

	cntr, err := objcontour.CurrentContour(ctx, cli, gc.Spec.ParametersRef.Namespace, gc.Spec.ParametersRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get contour for gateway %s/%s", gw.Namespace, gw.Name)
	}
	return cntr, nil
}

// ClassForGateway returns the GatewayClass referenced by gw, if one exists and is
// managed by the operator.
func ClassForGateway(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (*gatewayv1alpha1.GatewayClass, error) {
	gc, err := objgc.Get(ctx, cli, gw.Spec.GatewayClassName)
	if err != nil {
		return nil, fmt.Errorf("failed to verify if gatewayclass %s exists for gateway %s/%s",
			gw.Spec.GatewayClassName, gw.Namespace, gw.Name)
	}
	if objgc.IsController(gc) {
		return gc, nil
	}
	return nil, nil
}

// OtherGatewaysRefGatewayClass returns true if other gateways have the same
// gatewayClassName as gw.
func OtherGatewaysRefGatewayClass(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) (bool, error) {
	gwList, err := OtherGatewaysExist(ctx, cli, gw)
	if err != nil {
		return false, fmt.Errorf("failed to verify if gateways other than %s/%s exist: %v", gw.Namespace, gw.Name, err)
	}
	if gwList != nil {
		for _, g := range gwList.Items {
			switch {
			case g.Namespace == gw.Namespace && g.Name == gw.Name:
				continue
			case g.Spec.GatewayClassName == gw.Spec.GatewayClassName:
				return true, nil
			}
		}
	}
	return false, nil
}

// OwnerLabels returns owner labels for the provided gw.
func OwnerLabels(gw *gatewayv1alpha1.Gateway) map[string]string {
	return map[string]string{
		operatorv1alpha1.OwningGatewayNameLabel: gw.Name,
		operatorv1alpha1.OwningGatewayNsLabel:   gw.Namespace,
	}
}
