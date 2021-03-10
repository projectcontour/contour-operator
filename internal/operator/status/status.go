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

package status

import (
	"context"
	"fmt"
	"strings"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/equality"
	objds "github.com/projectcontour/contour-operator/internal/objects/daemonset"
	objdeploy "github.com/projectcontour/contour-operator/internal/objects/deployment"
	objgw "github.com/projectcontour/contour-operator/internal/objects/gateway"
	objgc "github.com/projectcontour/contour-operator/internal/objects/gatewayclass"
	retryable "github.com/projectcontour/contour-operator/internal/retryableerror"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// syncContourStatus computes the current status of contour based on whether contour
// is valid and updates status upon any changes since last sync.
func SyncContour(ctx context.Context, cli client.Client, contour *operatorv1alpha1.Contour, valid bool) error {
	var err error
	var errs []error

	latest := &operatorv1alpha1.Contour{}
	key := types.NamespacedName{
		Namespace: contour.Namespace,
		Name:      contour.Name,
	}
	if err := cli.Get(ctx, key, latest); err != nil {
		if errors.IsNotFound(err) {
			// The contour may have been deleted during status sync.
			return nil
		}
		return fmt.Errorf("failed to get contour %s/%s: %w", contour.Namespace, contour.Name, err)
	}

	updated := latest.DeepCopy()

	deploy, err := objdeploy.CurrentDeployment(ctx, cli, latest)
	if err != nil {
		updated.Status.AvailableContours = int32(0)
	} else {
		updated.Status.AvailableContours = deploy.Status.AvailableReplicas
	}
	ds, err := objds.CurrentDaemonSet(ctx, cli, latest)
	if err != nil {
		updated.Status.AvailableEnvoys = int32(0)
	} else {
		updated.Status.AvailableEnvoys = ds.Status.NumberAvailable
	}

	updated.Status.Conditions = mergeConditions(updated.Status.Conditions,
		computeContourAvailableCondition(deploy, ds, valid))
	gcSet := latest.GatewayClassSet()
	if gcSet {
		var gcExists, refsContour bool
		gcRef := *latest.Spec.GatewayClassRef
		if _, err := objgc.Get(ctx, cli, gcRef); err == nil {
			gcExists = true
			_, refsContour, err = objgc.ParameterRefExists(ctx, cli, contour.Name, contour.Namespace)
			if err != nil {
				return fmt.Errorf("failed to verify if gatewayclass %s exists: %w", gcRef, err)
			}
		}
		updated.Status.Conditions = mergeConditions(updated.Status.Conditions,
			computeContourAdmittedCondition(gcExists, refsContour))
	}

	if equality.ContourStatusChanged(latest.Status, updated.Status) {
		if err := cli.Status().Update(ctx, updated); err != nil {
			switch {
			case errors.IsNotFound(err):
				// The contour may have been deleted during status sync.
				return retryable.NewMaybeRetryableAggregate(errs)
			case strings.Contains(err.Error(), "the object has been modified"):
				// Retry if the object was modified during status sync.
				if err := SyncContour(ctx, cli, updated, valid); err != nil {
					errs = append(errs, fmt.Errorf("failed to update contour %s/%s status: %w", latest.Namespace,
						latest.Name, err))
				}
			default:
				errs = append(errs, fmt.Errorf("failed to update contour %s/%s status: %w", latest.Namespace,
					latest.Name, err))
			}
		}
	}

	return retryable.NewMaybeRetryableAggregate(errs)
}

// SyncGatewayClass computes the current status of gc based on whether gc is valid
// and updates status upon any changes since last sync.
func SyncGatewayClass(ctx context.Context, cli client.Client, gc *gatewayv1alpha1.GatewayClass, gcValid, cntrValid, cntrAvailable bool) error {
	var errs []error

	latest := &gatewayv1alpha1.GatewayClass{}
	key := types.NamespacedName{
		Namespace: gc.Namespace,
		Name:      gc.Name,
	}
	if err := cli.Get(ctx, key, latest); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to sync status for gateway class %s: %w", gc.Name, err)
	}

	updated := latest.DeepCopy()

	updated.Status.Conditions = mergeConditions(updated.Status.Conditions, computeGatewayClassAdmittedCondition(gcValid, cntrValid))
	updated.Status.Conditions = mergeConditions(updated.Status.Conditions, computeGatewayClassAvailableCondition(cntrAvailable))

	if equality.GatewayClassStatusChanged(latest.Status, updated.Status) {
		if err := cli.Status().Update(ctx, updated); err != nil {
			errs = append(errs, fmt.Errorf("failed to update gatewayclass %s status: %w", latest.Name, err))
		}
	}

	return retryable.NewMaybeRetryableAggregate(errs)
}

// SyncGateway computes the current status of gw and updates status based on
// any changes since last sync.
func SyncGateway(ctx context.Context, cli client.Client, gw *gatewayv1alpha1.Gateway) error {
	var errs []error

	latest := &gatewayv1alpha1.Gateway{}
	key := types.NamespacedName{
		Namespace: gw.Namespace,
		Name:      gw.Name,
	}
	if err := cli.Get(ctx, key, latest); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to sync status for gateway %s/%s: %w", gw.Namespace, gw.Name, err)
	}

	updated := latest.DeepCopy()

	gcName := latest.Spec.GatewayClassName
	gcExists := false
	gcAdmitted := false
	gc, err := objgc.Get(ctx, cli, gcName)
	if err == nil {
		gcExists = true
		if len(gc.Status.Conditions) > 0 {
			for _, c := range gc.Status.Conditions {
				if c.Type == string(gatewayv1alpha1.GatewayClassConditionStatusAdmitted) &&
					c.Status == metav1.ConditionTrue {
					gcAdmitted = true
					break
				}
			}
		}
	}

	cntrAvailable := false
	cntr, err := objgw.ContourForGateway(ctx, cli, gw)
	if err == nil {
		cntrAvailable = cntr.Available()
	}

	// Gateway's contain a default status condition that must be removed when reconciled by a controller.
	updated.Status.Conditions = removeGatewayCondition(updated.Status.Conditions, gatewayv1alpha1.GatewayConditionScheduled)
	updated.Status.Conditions = mergeConditions(updated.Status.Conditions,
		computeGatewayReadyCondition(gcExists, gcAdmitted, cntrAvailable))

	updated.Status.Addresses = []gatewayv1alpha1.GatewayAddress{}
	if equality.GatewayStatusChanged(latest.Status, updated.Status) {
		if err := cli.Status().Update(ctx, updated); err != nil {
			errs = append(errs, fmt.Errorf("failed to update gateway %s/%s status: %w", latest.Namespace,
				latest.Name, err))
		}
	}

	return retryable.NewMaybeRetryableAggregate(errs)
}
