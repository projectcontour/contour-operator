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
	"fmt"
	"strings"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilclock "k8s.io/apimachinery/pkg/util/clock"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// clock is to enable unit testing
var clock utilclock.Clock = utilclock.RealClock{}

// computeContourAvailableCondition computes the contour Available status condition
// type based on deployment, ds, set, exists and admitted.
func computeContourAvailableCondition(deployment *appsv1.Deployment, ds *appsv1.DaemonSet, set, exists, admitted bool) metav1.Condition {
	switch {
	case set:
		switch {
		case !exists:
			return metav1.Condition{
				Type:    operatorv1alpha1.ContourAvailableConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  "GatewayClassNonExistent",
				Message: "The referenced GatewayClass does not exist.",
			}
		case !admitted:
			return metav1.Condition{
				Type:    operatorv1alpha1.ContourAvailableConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  "GatewayClassNotAdmitted",
				Message: "The referenced GatewayClass is not admitted.",
			}
		default:
			// The referenced gatewayclass exists and is admitted.
			return metav1.Condition{
				Type:    operatorv1alpha1.ContourAvailableConditionType,
				Status:  metav1.ConditionTrue,
				Reason:  "GatewayClassAdmitted",
				Message: "The referenced GatewayClass is admitted.",
			}
		}
	default:
		if deployment == nil {
			return metav1.Condition{
				Type:    operatorv1alpha1.ContourAvailableConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  "ContourUnavailable",
				Message: "Contour deployment does not exist.",
			}
		}
		if ds == nil {
			return metav1.Condition{
				Type:    operatorv1alpha1.ContourAvailableConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  "ContourUnavailable",
				Message: "Envoy daemonset does not exist.",
			}
		}
		dsAvailable := ds.Status.NumberAvailable > 0
		for _, cond := range deployment.Status.Conditions {
			if cond.Type != appsv1.DeploymentAvailable {
				continue
			}
			switch {
			case cond.Status == corev1.ConditionTrue:
				if dsAvailable {
					return metav1.Condition{
						Type:    operatorv1alpha1.ContourAvailableConditionType,
						Status:  metav1.ConditionTrue,
						Reason:  "ContourAvailable",
						Message: "Contour has minimum availability.",
					}
				}
				return metav1.Condition{
					Type:    operatorv1alpha1.ContourAvailableConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  "ContourUnavailable",
					Message: "Envoy daemonset does not have minimum availability.",
				}
			case cond.Status == corev1.ConditionFalse:
				if dsAvailable {
					return metav1.Condition{
						Type:    operatorv1alpha1.ContourAvailableConditionType,
						Status:  metav1.ConditionFalse,
						Reason:  "ContourUnavailable",
						Message: fmt.Sprintf("Contour %s", strings.ToLower(cond.Message)),
					}
				}
				return metav1.Condition{
					Type:   operatorv1alpha1.ContourAvailableConditionType,
					Status: metav1.ConditionFalse,
					Reason: "ContourUnavailable",
					Message: fmt.Sprintf("Envoy daemonset does not have minimum availability. Contour %s",
						strings.ToLower(cond.Message)),
				}
			case cond.Status == corev1.ConditionUnknown:
				return metav1.Condition{
					Type:    operatorv1alpha1.ContourAvailableConditionType,
					Status:  metav1.ConditionUnknown,
					Reason:  fmt.Sprintf("ContourUnknown: %s", cond.Message),
					Message: fmt.Sprintf("Contour status unknown. %s", cond.Message),
				}
			}
		}
	}

	return metav1.Condition{
		Type:    operatorv1alpha1.ContourAvailableConditionType,
		Status:  metav1.ConditionUnknown,
		Reason:  "ContourUnknown",
		Message: "Contour status unknown.",
	}
}

// computeGatewayClassAdmittedCondition computes the Available status condition based
// upon the GatewayClass status specification.
func computeGatewayClassAdmittedCondition(owned, valid bool) metav1.Condition {
	c := metav1.Condition{
		Type:    string(gatewayv1alpha1.GatewayClassConditionStatusAdmitted),
		Status:  metav1.ConditionFalse,
		Reason:  "NotOwned",
		Message: "Not owned by Contour Operator.",
	}
	switch {
	case !valid:
		c.Status = metav1.ConditionFalse
		c.Reason = "Invalid"
		c.Message = "Invalid GatewayClass."
	case owned:
		c.Status = metav1.ConditionTrue
		c.Reason = "Owned"
		c.Message = "Owned by Contour Operator."
	}
	return c
}

// computeGatewayReadyCondition computes the Ready status condition based
// on the availability of the associated GatewayClass and Contour.
func computeGatewayReadyCondition(gcExists, gcAdmitted, cntrAvailable bool) metav1.Condition {
	c := metav1.Condition{
		Type:    string(gatewayv1alpha1.GatewayConditionReady),
		Status:  metav1.ConditionFalse,
		Reason:  "GatewayClassAndContourUnavailable",
		Message: "The associated GatewayClass and Contour are not available.",
	}
	switch {
	case !gcExists:
		c.Status = metav1.ConditionFalse
		c.Reason = "NonExistentGatewayClass"
		c.Message = "The GatewayClass does not exist."
	case !gcAdmitted:
		c.Status = metav1.ConditionFalse
		c.Reason = "GatewayClassNotAdmitted"
		c.Message = "The GatewayClass is not admitted."
	case !cntrAvailable:
		c.Status = metav1.ConditionFalse
		c.Reason = "ContourNotAvailable"
		c.Message = "The Contour is not available."
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "GatewayReady"
		c.Message = "The Gateway is ready to serve routes."
	}
	return c
}

// mergeConditions adds or updates matching conditions, and updates
// the transition time if details of a condition have changed. Returns
// the updated condition array.
func mergeConditions(conditions []metav1.Condition, updates ...metav1.Condition) []metav1.Condition {
	now := metav1.NewTime(clock.Now())
	var additions []metav1.Condition
	for i, update := range updates {
		add := true
		for j, cond := range conditions {
			if cond.Type == update.Type {
				add = false
				if conditionChanged(cond, update) {
					conditions[j].Status = update.Status
					conditions[j].Reason = update.Reason
					conditions[j].Message = update.Message
					if cond.Status != update.Status {
						conditions[j].LastTransitionTime = now
					}
					break
				}
			}
		}
		if add {
			updates[i].LastTransitionTime = now
			additions = append(additions, updates[i])
		}
	}
	conditions = append(conditions, additions...)
	return conditions
}

func conditionChanged(a, b metav1.Condition) bool {
	return a.Status != b.Status || a.Reason != b.Reason || a.Message != b.Message
}

// removeGatewayCondition returns a newly created []metav1.Condition that contains all items
// from conditions that are not equal to condition type t.
func removeGatewayCondition(conditions []metav1.Condition, t gatewayv1alpha1.GatewayConditionType) []metav1.Condition {
	var new []metav1.Condition
	if len(conditions) > 0 {
		for _, c := range conditions {
			if c.Type != string(t) {
				new = append(new, c)
			}
		}
	}
	return new
}
