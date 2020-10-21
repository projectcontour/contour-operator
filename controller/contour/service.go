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
	utilequality "github.com/projectcontour/contour-operator/util/equality"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// contourSvcName is the name of Contour's Service.
	// [TODO] danehans: Update Contour name to contour.Name + "-contour" to support multiple
	// Contours/ns when https://github.com/projectcontour/contour/issues/2122 is fixed.
	contourSvcName = "contour"
	// [TODO] danehans: Update Envoy name to contour.Name + "-envoy" to support multiple
	// Contours/ns when https://github.com/projectcontour/contour/issues/2122 is fixed.
	// envoySvcName is the name of Envoy's Service.
	envoySvcName = "envoy"
	// awsLbBackendProtoAnnotation is a Service annotation that places the AWS ELB into
	// "TCP" mode so that it does not do HTTP negotiation for HTTPS connections at the
	// ELB edge. The downside of this is the remote IP address of all connections will
	// appear to be the internal address of the ELB.
	// TODO [danehans]: Make proxy protocol configurable or automatically enabled. See
	// https://github.com/projectcontour/contour-operator/issues/49 for details.
	awsLbBackendProtoAnnotation = "service.beta.kubernetes.io/aws-load-balancer-backend-protocol"
)

// ensureContourService ensures that a Contour Service exists for the given contour.
func (r *Reconciler) ensureContourService(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	desired := DesiredContourService(contour)

	current, err := r.currentContourService(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.createService(ctx, desired)
		}
		return fmt.Errorf("failed to get service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	if err := r.updateContourServiceIfNeeded(ctx, current, desired); err != nil {
		return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return nil
}

// ensureEnvoyService ensures that an Envoy Service exists for the given contour.
func (r *Reconciler) ensureEnvoyService(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	desired := DesiredEnvoyService(contour)

	current, err := r.currentEnvoyService(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.createService(ctx, desired)
		}
		return fmt.Errorf("failed to get service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	if err := r.updateEnvoyServiceIfNeeded(ctx, current, desired); err != nil {
		return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return nil
}

// ensureContourServiceDeleted ensures that a Contour Service for the
// provided contour is deleted.
func (r *Reconciler) ensureContourServiceDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: contour.Spec.Namespace.Name,
			Name:      contourSvcName,
		},
	}

	if err := r.Client.Delete(ctx, svc); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete service %s/%s: %w", svc.Namespace, svc.Name, err)
		}
		return nil
	}
	r.Log.Info("deleted service", "namespace", svc.Namespace, "name", svc.Name)

	return nil
}

// ensureEnvoyServiceDeleted ensures that an Envoy Service for the
// provided contour is deleted.
func (r *Reconciler) ensureEnvoyServiceDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: contour.Spec.Namespace.Name,
			Name:      envoySvcName,
		},
	}

	if err := r.Client.Delete(ctx, svc); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete service %s/%s: %w", svc.Namespace, svc.Name, err)
		}
		return nil
	}
	r.Log.Info("deleted service", "namespace", svc.Namespace, "name", svc.Name)

	return nil
}

// DesiredContourService generates the desired Contour Service for the given contour.
func DesiredContourService(contour *operatorv1alpha1.Contour) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: contour.Spec.Namespace.Name,
			Name:      contourSvcName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "xds",
					Port:       int32(xdsPort),
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{IntVal: int32(xdsPort)},
				},
			},
			Selector:        contourDeploymentPodSelector(contour).MatchLabels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}

	return svc
}

// DesiredEnvoyService generates the desired Envoy Service for the given contour.
func DesiredEnvoyService(contour *operatorv1alpha1.Contour) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   contour.Spec.Namespace.Name,
			Name:        envoySvcName,
			Annotations: map[string]string{awsLbBackendProtoAnnotation: "tcp"},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       int32(httpPort),
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{IntVal: int32(httpPort)},
				},
				{
					Name:       "https",
					Port:       int32(httpsPort),
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{IntVal: int32(httpsPort)},
				},
			},
			Selector:        envoyDaemonSetPodSelector(contour).MatchLabels,
			Type:            corev1.ServiceTypeLoadBalancer,
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}

	return svc
}

// currentContourService returns the current Contour Service for the provided contour.
func (r *Reconciler) currentContourService(ctx context.Context, contour *operatorv1alpha1.Contour) (*corev1.Service, error) {
	current := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      contourSvcName,
	}
	err := r.Client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// currentEnvoyService returns the current Envoy Service for the provided contour.
func (r *Reconciler) currentEnvoyService(ctx context.Context, contour *operatorv1alpha1.Contour) (*corev1.Service, error) {
	current := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      envoySvcName,
	}
	err := r.Client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// createService creates a Service resource for the provided svc.
func (r *Reconciler) createService(ctx context.Context, svc *corev1.Service) error {
	if err := r.Client.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create service %s/%s: %w", svc.Namespace, svc.Name, err)
	}
	r.Log.Info("created service", "namespace", svc.Namespace, "name", svc.Name)

	return nil
}

// updateContourServiceIfNeeded updates a Contour Service if current does not match desired.
func (r *Reconciler) updateContourServiceIfNeeded(ctx context.Context, current, desired *corev1.Service) error {
	svc, updated := utilequality.ClusterIpServiceChanged(current, desired)
	if updated {
		if err := r.Client.Update(ctx, svc); err != nil {
			return fmt.Errorf("failed to update service %s/%s: %w", svc.Namespace, svc.Name, err)
		}
		r.Log.Info("updated service", "namespace", svc.Namespace, "name", svc.Name)
		return nil
	}
	r.Log.Info("service unchanged; skipped updating service",
		"namespace", current.Namespace, "name", current.Name)

	return nil
}

// updateEnvoyServiceIfNeeded updates an Envoy Service if current does not match desired.
func (r *Reconciler) updateEnvoyServiceIfNeeded(ctx context.Context, current, desired *corev1.Service) error {
	svc, updated := utilequality.LoadBalancerServiceChanged(current, desired)
	if updated {
		if err := r.Client.Update(ctx, svc); err != nil {
			return fmt.Errorf("failed to update service %s/%s: %w", svc.Namespace, svc.Name, err)
		}
		r.Log.Info("updated service", "namespace", svc.Namespace, "name", svc.Name)
		return nil
	}
	r.Log.Info("service unchanged; skipped updating service",
		"namespace", current.Namespace, "name", current.Name)

	return nil
}
