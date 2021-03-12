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

package service

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/equality"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objds "github.com/projectcontour/contour-operator/internal/objects/daemonset"
	objdeploy "github.com/projectcontour/contour-operator/internal/objects/deployment"
	objgw "github.com/projectcontour/contour-operator/internal/objects/gateway"
	objcfg "github.com/projectcontour/contour-operator/internal/objects/sharedconfig"
	labelutil "github.com/projectcontour/contour-operator/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
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
	// EnvoyServiceHTTPPort is the HTTP port number of the Envoy service.
	EnvoyServiceHTTPPort = int32(80)
	// EnvoyServiceHTTPSPort is the HTTPS port number of the Envoy service.
	EnvoyServiceHTTPSPort = int32(443)
	// EnvoyNodePortHTTPPort is the NodePort port number for Envoy's HTTP service. For NodePort
	// details see: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
	EnvoyNodePortHTTPPort = int32(30080)
	// EnvoyNodePortHTTPSPort is the NodePort port number for Envoy's HTTPS service. For NodePort
	// details see: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
	EnvoyNodePortHTTPSPort = int32(30443)
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

// Config contains everything needed to manage a service.
type Config struct {
	// Namespace is the namespace of the Service.
	Namespace string
	// Labels are labels to apply to the ConfigMap.
	Labels map[string]string
	// Contour contains Contour service configuration parameters.
	Contour contourConfig
	// Envoy contains Envoy service configuration parameters.
	Envoy envoyConfig
}

// contourConfig contains contour service configuration parameters.
type contourConfig struct {
	// Name is the name of the contour service. Defaults to "contour".
	Name string
}

// envoyConfig contains Envoy service configuration parameters.
type envoyConfig struct {
	// Name is the name of the envoy service. Defaults to "envoy".
	Name string
	// HTTPServicePort is the http port number of envoy's service.
	HTTPServicePort int32
	// HTTPSServicePort is the https port number of envoy's service.
	HTTPSServicePort int32
	// NetworkPublishing defines the schema to publish Envoy to a network.
	NetworkPublishing operatorv1alpha1.EnvoyNetworkPublishing
	// TargetPorts is the schema to specify a target port for a service.
	TargetPorts []operatorv1alpha1.ContainerPort
}

// NewConfig returns a Config with default fields set.
func NewConfig() *Config {
	return &Config{
		Contour: contourConfig{Name: contourSvcName},
		Envoy:   envoyConfig{Name: envoySvcName},
	}
}

// NewCfgForContour returns a Config with default fields set for contour.
func NewCfgForContour(contour *operatorv1alpha1.Contour) *Config {
	cfg := NewConfig()
	cfg.Namespace = contour.Spec.Namespace.Name
	cfg.Labels = objcontour.OwnerLabels(contour)
	cfg.Envoy.HTTPServicePort = EnvoyServiceHTTPPort
	cfg.Envoy.HTTPSServicePort = EnvoyServiceHTTPSPort
	cfg.Envoy.TargetPorts = contour.Spec.NetworkPublishing.Envoy.ContainerPorts
	cfg.Envoy.NetworkPublishing = contour.Spec.NetworkPublishing.Envoy
	return cfg
}

// NewCfgForGateway returns a Config with default fields set for gw.
// It's expected that gw has been validated.
func NewCfgForGateway(gw *gatewayv1alpha1.Gateway) *Config {
	cfg := NewConfig()
	cfg.Namespace = gw.Namespace
	cfg.Labels = objgw.OwnerLabels(gw)
	cfg.Envoy.NetworkPublishing.Type = operatorv1alpha1.LoadBalancerServicePublishingType
	cfg.Envoy.NetworkPublishing.LoadBalancer.Scope = operatorv1alpha1.ExternalLoadBalancer
	cfg.Envoy.NetworkPublishing.LoadBalancer.ProviderParameters.Type = operatorv1alpha1.AWSLoadBalancerProvider
	listeners := gw.Spec.Listeners
	for _, listener := range listeners {
		if listener.Protocol == gatewayv1alpha1.HTTPProtocolType {
			cfg.Envoy.HTTPServicePort = int32(listener.Port)
		}
		if listener.Protocol == gatewayv1alpha1.HTTPSProtocolType {
			cfg.Envoy.HTTPSServicePort = int32(listener.Port)
		}
	}
	cfg.Envoy.TargetPorts = []operatorv1alpha1.ContainerPort{
		{
			Name:       "http",
			PortNumber: int32(8080),
		},
		{
			Name:       "https",
			PortNumber: int32(8443),
		},
	}
	return cfg
}

// EnsureContour ensures that a Contour Service exists for the given cfg.
func EnsureContour(ctx context.Context, cli client.Client, cfg *Config) error {
	desired := DesiredContour(cfg)
	current, err := currentContour(ctx, cli, cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			return create(ctx, cli, desired)
		}
		return fmt.Errorf("failed to get service %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	if err := updateContourIfNeeded(ctx, cli, cfg, current, desired); err != nil {
		return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	return nil
}

// EnsureEnvoy ensures that an Envoy Service exists for the given cfg.
func EnsureEnvoy(ctx context.Context, cli client.Client, cfg *Config) error {
	desired := DesiredEnvoy(cfg)
	current, err := currentEnvoy(ctx, cli, cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			return create(ctx, cli, desired)
		}
		return fmt.Errorf("failed to get service %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	if err := updateEnvoyIfNeeded(ctx, cli, cfg, current, desired); err != nil {
		return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	return nil
}

// EnsureContourDeleted ensures that a Contour Service for the provided cfg
// is deleted if owner labels exist.
func EnsureContourDeleted(ctx context.Context, cli client.Client, cfg *Config) error {
	svc, err := currentContour(ctx, cli, cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if labelutil.Exist(svc, cfg.Labels) {
		if err := cli.Delete(ctx, svc); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

// EnsureEnvoyDeleted ensures that an Envoy Service for the provided cfg
// is deleted if owner labels exist.
func EnsureEnvoyDeleted(ctx context.Context, cli client.Client, cfg *Config) error {
	svc, err := currentEnvoy(ctx, cli, cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if labelutil.Exist(svc, cfg.Labels) {
		if err := cli.Delete(ctx, svc); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

// DesiredContour generates the desired Contour Service for the given cfg.
func DesiredContour(cfg *Config) *corev1.Service {
	xdsPort := objcfg.XDSPort
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cfg.Namespace,
			Name:      cfg.Contour.Name,
			Labels:    cfg.Labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "xds",
					Port:       xdsPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{IntVal: xdsPort},
				},
			},
			Selector:        objdeploy.ContourDeploymentPodSelector().MatchLabels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}
	return svc
}

// DesiredEnvoy generates the desired Envoy Service for the given cfg.
func DesiredEnvoy(cfg *Config) *corev1.Service {
	var ports []corev1.ServicePort
	for _, port := range cfg.Envoy.TargetPorts {
		var p corev1.ServicePort
		httpFound := false
		httpsFound := false
		switch {
		case httpsFound && httpFound:
			break
		case port.Name == "http":
			httpFound = true
			p.Name = port.Name
			p.Port = cfg.Envoy.HTTPServicePort
			p.Protocol = corev1.ProtocolTCP
			p.TargetPort = intstr.IntOrString{IntVal: port.PortNumber}
			ports = append(ports, p)
		case port.Name == "https":
			httpsFound = true
			p.Name = port.Name
			p.Port = cfg.Envoy.HTTPSServicePort
			p.Protocol = corev1.ProtocolTCP
			p.TargetPort = intstr.IntOrString{IntVal: port.PortNumber}
			ports = append(ports, p)
		}
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   cfg.Namespace,
			Name:        cfg.Envoy.Name,
			Annotations: map[string]string{},
			Labels:      cfg.Labels,
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Ports:                 ports,
			Selector:              objds.EnvoyDaemonSetPodSelector().MatchLabels,
			SessionAffinity:       corev1.ServiceAffinityNone,
		},
	}
	pubType := cfg.Envoy.NetworkPublishing.Type
	switch pubType {
	case operatorv1alpha1.LoadBalancerServicePublishingType:
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		provider := cfg.Envoy.NetworkPublishing.LoadBalancer.ProviderParameters.Type
		lbAnnotations := LbAnnotations[provider]
		for name, value := range lbAnnotations {
			svc.Annotations[name] = value
		}
		isInternal := cfg.Envoy.NetworkPublishing.LoadBalancer.Scope == operatorv1alpha1.InternalLoadBalancer
		if isInternal {
			internalAnnotations := InternalLBAnnotations[provider]
			for name, value := range internalAnnotations {
				svc.Annotations[name] = value
			}
		}
	case operatorv1alpha1.NodePortServicePublishingType:
		svc.Spec.Type = corev1.ServiceTypeNodePort
		svc.Spec.Ports[0].NodePort = EnvoyNodePortHTTPPort
		svc.Spec.Ports[1].NodePort = EnvoyNodePortHTTPSPort
	}
	return svc
}

// currentContour returns the current Contour Service for the provided cfg.
func currentContour(ctx context.Context, cli client.Client, cfg *Config) (*corev1.Service, error) {
	current := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.Contour.Name,
	}
	err := cli.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// currentEnvoy returns the current Envoy Service for the provided cfg.
func currentEnvoy(ctx context.Context, cli client.Client, cfg *Config) (*corev1.Service, error) {
	current := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.Envoy.Name,
	}
	err := cli.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// create creates a Service resource for the provided svc.
func create(ctx context.Context, cli client.Client, svc *corev1.Service) error {
	if err := cli.Create(ctx, svc); err != nil {
		return fmt.Errorf("failed to create service %s/%s: %w", svc.Namespace, svc.Name, err)
	}
	return nil
}

// updateContourIfNeeded updates an Contour Service if current does not match desired,
// using cfg to verify the existence of owner labels.
func updateContourIfNeeded(ctx context.Context, cli client.Client, cfg *Config, current, desired *corev1.Service) error {
	if labelutil.Exist(current, cfg.Labels) {
		_, updated := equality.ClusterIPServiceChanged(current, desired)
		if updated {
			if err := cli.Update(ctx, desired); err != nil {
				return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return nil
		}
	}
	return nil
}

// updateEnvoyIfNeeded updates an Envoy Service if current does not match desired,
// using cfg to verify the existence of owner labels.
func updateEnvoyIfNeeded(ctx context.Context, cli client.Client, cfg *Config, current, desired *corev1.Service) error {
	if labelutil.Exist(current, cfg.Labels) {
		updated := false
		switch cfg.Envoy.NetworkPublishing.Type {
		case operatorv1alpha1.NodePortServicePublishingType:
			_, updated = equality.NodePortServiceChanged(current, desired)
		// Add additional network publishing types as they are introduced.
		default:
			// LoadBalancerService is the default network publishing type.
			_, updated = equality.LoadBalancerServiceChanged(current, desired)
		}
		if updated {
			if err := cli.Update(ctx, desired); err != nil {
				return fmt.Errorf("failed to update service %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return nil
		}
	}
	return nil
}
