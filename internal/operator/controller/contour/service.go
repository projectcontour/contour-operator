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
	equality "github.com/projectcontour/contour-operator/internal/equality"
	"github.com/projectcontour/contour-operator/internal/operator/config"

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
	// awsProviderType is the name of the Amazon Web Services provider.
	awsProviderType = "AWS"
	// azureProviderType is the name of the Microsoft Azure provider.
	azureProviderType = "Azure"
	// gcpProviderType is the name of the Google Cloud Platform provider.
	gcpProviderType = "GCP"
	// awsInternalLBAnnotation is the annotation used on a service to specify an AWS
	// load balancer as being internal.
	awsInternalLBAnnotation = "service.beta.kubernetes.io/aws-load-balancer-internal"
	// azureInternalLBAnnotation is the annotation used on a service to specify an Azure
	// load balancer as being internal.
	azureInternalLBAnnotation = "service.beta.kubernetes.io/azure-load-balancer-internal"
	// gcpLBTypeAnnotation is the annotation used on a service to specify a GCP load balancer
	// type.
	gcpLBTypeAnnotation = "cloud.google.com/load-balancer-type"
)

var (
	// LbAnnotations maps cloud providers to the provider's annotation
	// key/value pair used for managing a load balancer. For additional
	// details see:
	//  https://kubernetes.io/docs/concepts/services-networking/service/#internal-load-balancer
	//
	LbAnnotations = map[operatorv1alpha1.LoadBalancerProviderType]map[string]string{
		awsProviderType: {
			awsLbBackendProtoAnnotation: "tcp",
		},
	}

	// InternalLBAnnotations maps cloud providers to the provider's annotation
	// key/value pair used for managing an internal load balancer. For additional
	// details see:
	//  https://kubernetes.io/docs/concepts/services-networking/service/#internal-load-balancer
	//
	InternalLBAnnotations = map[operatorv1alpha1.LoadBalancerProviderType]map[string]string{
		awsProviderType: {
			awsInternalLBAnnotation: "0.0.0.0/0",
		},
		azureProviderType: {
			// Azure load balancers are not customizable and are set to (2 fail @ 5s interval, 2 healthy)
			azureInternalLBAnnotation: "true",
		},
		gcpProviderType: {
			gcpLBTypeAnnotation: "Internal",
		},
	}
)

// ensureContourService ensures that a Contour Service exists for the given contour.
func (r *reconciler) ensureContourService(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	desired := DesiredContourService(contour)

	current, err := r.currentContourService(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.createService(ctx, desired)
		}
		return fmt.Errorf("failed to get service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	if err := r.updateContourServiceIfNeeded(ctx, contour, current, desired); err != nil {
		return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return nil
}

// ensureEnvoyService ensures that an Envoy Service exists for the given contour.
func (r *reconciler) ensureEnvoyService(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	desired := DesiredEnvoyService(contour)

	current, err := r.currentEnvoyService(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.createService(ctx, desired)
		}
		return fmt.Errorf("failed to get service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	if err := r.updateEnvoyServiceIfNeeded(ctx, contour, current, desired); err != nil {
		return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return nil
}

// ensureContourServiceDeleted ensures that a Contour Service for the
// provided contour is deleted if Contour owner labels exist.
func (r *reconciler) ensureContourServiceDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	svc, err := r.currentContourService(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !ownerLabelsExist(svc, contour) {
		r.log.Info("service not labeled; skipping deletion", "namespace", svc.Namespace, "name", svc.Name)
	} else {
		if err := r.client.Delete(ctx, svc); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		r.log.Info("deleted service", "namespace", svc.Namespace, "name", svc.Name)
	}

	return nil
}

// ensureEnvoyServiceDeleted ensures that an Envoy Service for the
// provided contour is deleted.
func (r *reconciler) ensureEnvoyServiceDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	svc, err := r.currentEnvoyService(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !ownerLabelsExist(svc, contour) {
		r.log.Info("service not labeled; skipping deletion", "namespace", svc.Namespace, "name", svc.Name)
	} else {
		if err := r.client.Delete(ctx, svc); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		r.log.Info("deleted service", "namespace", svc.Namespace, "name", svc.Name)
	}

	return nil
}

// DesiredContourService generates the desired Contour Service for the given contour.
func DesiredContourService(contour *operatorv1alpha1.Contour) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: contour.Spec.Namespace.Name,
			Name:      contourSvcName,
			Labels: map[string]string{
				operatorv1alpha1.OwningContourNameLabel: contour.Name,
				operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
			},
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
			Selector:        contourDeploymentPodSelector().MatchLabels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}

	return svc
}

// DesiredEnvoyService generates the desired Envoy Service for the given contour.
func DesiredEnvoyService(contour *operatorv1alpha1.Contour) *corev1.Service {
	var ports []corev1.ServicePort
	for _, port := range contour.Spec.NetworkPublishing.Envoy.ContainerPorts {
		var p corev1.ServicePort
		httpFound := false
		httpsFound := false
		switch {
		case httpsFound && httpFound:
			break
		case port.Name == "http":
			httpFound = true
			p.Name = port.Name
			p.Port = config.EnvoyServiceHTTPPort
			p.Protocol = corev1.ProtocolTCP
			p.TargetPort = intstr.IntOrString{IntVal: port.PortNumber}
			ports = append(ports, p)
		case port.Name == "https":
			httpsFound = true
			p.Name = port.Name
			p.Port = config.EnvoyServiceHTTPSPort
			p.Protocol = corev1.ProtocolTCP
			p.TargetPort = intstr.IntOrString{IntVal: port.PortNumber}
			ports = append(ports, p)
		}
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   contour.Spec.Namespace.Name,
			Name:        envoySvcName,
			Annotations: map[string]string{},
			Labels: map[string]string{
				operatorv1alpha1.OwningContourNameLabel: contour.Name,
				operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
			},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Ports:                 ports,
			Selector:              envoyDaemonSetPodSelector().MatchLabels,
			SessionAffinity:       corev1.ServiceAffinityNone,
		},
	}

	epType := contour.Spec.NetworkPublishing.Envoy.Type
	switch epType {
	case operatorv1alpha1.LoadBalancerServicePublishingType:
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		provider := contour.Spec.NetworkPublishing.Envoy.LoadBalancer.ProviderParameters.Type
		lbAnnotations := LbAnnotations[provider]
		for name, value := range lbAnnotations {
			svc.Annotations[name] = value
		}
		isInternal := contour.Spec.NetworkPublishing.Envoy.LoadBalancer.Scope == operatorv1alpha1.InternalLoadBalancer
		if isInternal {
			internalAnnotations := InternalLBAnnotations[provider]
			for name, value := range internalAnnotations {
				svc.Annotations[name] = value
			}
		}
	case operatorv1alpha1.NodePortServicePublishingType:
		svc.Spec.Type = corev1.ServiceTypeNodePort
		svc.Spec.Ports[0].NodePort = config.EnvoyNodePortHTTPPort
		svc.Spec.Ports[1].NodePort = config.EnvoyNodePortHTTPSPort
	}

	return svc
}

// currentContourService returns the current Contour Service for the provided contour.
func (r *reconciler) currentContourService(ctx context.Context, contour *operatorv1alpha1.Contour) (*corev1.Service, error) {
	current := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      contourSvcName,
	}
	err := r.client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// currentEnvoyService returns the current Envoy Service for the provided contour.
func (r *reconciler) currentEnvoyService(ctx context.Context, contour *operatorv1alpha1.Contour) (*corev1.Service, error) {
	current := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      envoySvcName,
	}
	err := r.client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// createService creates a Service resource for the provided svc.
func (r *reconciler) createService(ctx context.Context, svc *corev1.Service) error {
	if err := r.client.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create service %s/%s: %w", svc.Namespace, svc.Name, err)
	}
	r.log.Info("created service", "namespace", svc.Namespace, "name", svc.Name)

	return nil
}

// updateContourServiceIfNeeded updates a Contour Service if current does not match desired.
func (r *reconciler) updateContourServiceIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *corev1.Service) error {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("service missing owner labels; skipped updating", "namespace", current.Namespace,
			"name", current.Name)
		return nil
	}
	_, updated := equality.ClusterIPServiceChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, desired); err != nil {
			return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
		}
		r.log.Info("updated service", "namespace", desired.Namespace, "name", desired.Name)
		return nil
	}
	r.log.Info("service unchanged; skipped updating",
		"namespace", current.Namespace, "name", current.Name)

	return nil
}

// updateEnvoyServiceIfNeeded updates an Envoy Service if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateEnvoyServiceIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *corev1.Service) error {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("service missing owner labels; skipped updating", "namespace", current.Namespace,
			"name", current.Name)
		return nil
	}
	updated := false
	switch contour.Spec.NetworkPublishing.Envoy.Type {
	case operatorv1alpha1.NodePortServicePublishingType:
		_, updated = equality.NodePortServiceChanged(current, desired)
	// Add additional network publishing types as they are introduced.
	default:
		// LoadBalancerService is the default network publishing type.
		_, updated = equality.LoadBalancerServiceChanged(current, desired)
	}
	if updated {
		if err := r.client.Update(ctx, desired); err != nil {
			return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
		}
		r.log.Info("updated service", "namespace", desired.Namespace, "name", desired.Name)
		return nil
	}
	r.log.Info("service unchanged; skipped updating",
		"namespace", current.Namespace, "name", current.Name)

	return nil
}
