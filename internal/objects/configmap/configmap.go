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

package configmap

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	objcontour "github.com/projectcontour/contour-operator/internal/objects/contour"
	objgw "github.com/projectcontour/contour-operator/internal/objects/gateway"
	"github.com/projectcontour/contour-operator/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

const (
	// ContourCfgMapName is the name of Contour's ConfigMap resource.
	// [TODO] danehans: Remove and use contour.Name when
	// https://github.com/projectcontour/contour/issues/2122 is fixed.
	ContourCfgMapName = "contour"
)

var contourCfgTemplate = template.Must(template.New("contour.yaml").Parse(`
#
# server:
#   determine which XDS Server implementation to utilize in Contour.
#   xds-server-type: contour
#
# Specify the service-apis Gateway Contour should watch.{{if and .GatewayName .GatewayNamespace}}
gateway:
  name: {{.GatewayName}}
  namespace: {{.GatewayNamespace}}{{else}}
# gateway:
#   name: contour
#   namespace: projectcontour{{end}}
#
# should contour expect to be running inside a k8s cluster
# incluster: true
#
# path to kubeconfig (if not running inside a k8s cluster)
# kubeconfig: /path/to/.kube/config
#
# Disable RFC-compliant behavior to strip "Content-Length" header if
# "Tranfer-Encoding: chunked" is also set.
# disableAllowChunkedLength: false
# Disable HTTPProxy permitInsecure field
disablePermitInsecure: false
tls:
# minimum TLS version that Contour will negotiate
# minimum-protocol-version: "1.2"
# TLS ciphers to be supported by Envoy TLS listeners when negotiating
# TLS 1.2.
# cipher-suites:
# - '[ECDHE-ECDSA-AES128-GCM-SHA256|ECDHE-ECDSA-CHACHA20-POLY1305]'
# - '[ECDHE-RSA-AES128-GCM-SHA256|ECDHE-RSA-CHACHA20-POLY1305]'
# - 'ECDHE-ECDSA-AES256-GCM-SHA384'
# - 'ECDHE-RSA-AES256-GCM-SHA384'
# Defines the Kubernetes name/namespace matching a secret to use
# as the fallback certificate when requests which don't match the
# SNI defined for a vhost.
  fallback-certificate:
#   name: fallback-secret-name
#   namespace: projectcontour
  envoy-client-certificate:
#   name: envoy-client-cert-secret-name
#   namespace: projectcontour
# The following config shows the defaults for the leader election.
# leaderelection:
#   configmap-name: leader-elect
#   configmap-namespace: projectcontour
### Logging options
# Default setting
accesslog-format: envoy
# To enable JSON logging in Envoy
# accesslog-format: json
# The default fields that will be logged are specified below.
# To customize this list, just add or remove entries.
# The canonical list is available at
# https://godoc.org/github.com/projectcontour/contour/internal/envoy#JSONFields
# json-fields:
#   - "@timestamp"
#   - "authority"
#   - "bytes_received"
#   - "bytes_sent"
#   - "downstream_local_address"
#   - "downstream_remote_address"
#   - "duration"
#   - "method"
#   - "path"
#   - "protocol"
#   - "request_id"
#   - "requested_server_name"
#   - "response_code"
#   - "response_flags"
#   - "uber_trace_id"
#   - "upstream_cluster"
#   - "upstream_host"
#   - "upstream_local_address"
#   - "upstream_service_time"
#   - "user_agent"
#   - "x_forwarded_for"
#
# default-http-versions:
# - "HTTP/2"
# - "HTTP/1.1"
#
# The following shows the default proxy timeout settings.
# timeouts:
#   request-timeout: infinity
#   connection-idle-timeout: 60s
#   stream-idle-timeout: 5m
#   max-connection-duration: infinity
#   delayed-close-timeout: 1s
#   connection-shutdown-grace-period: 5s
#
# Envoy cluster settings.
# cluster:
#   configure the cluster dns lookup family
#   valid options are: auto (default), v4, v6
#   dns-lookup-family: auto
#
# Envoy network settings.
# network:
#   Configure the number of additional ingress proxy hops from the
#   right side of the x-forwarded-for HTTP header to trust.
#   num-trusted-hops: 0
`))

// Config contains everything needed to manage a ConfigMap.
type Config struct {
	// Namespace is the namespace of the ConfigMap.
	Namespace string
	// Name is the name of the ConfigMap. Defaults to "contour".
	Name string
	// Labels are labels to apply to the ConfigMap.
	Labels map[string]string
	// Contour contains Contour configuration parameters.
	Contour contourConfig
}

// contourConfig contains Contour configuration parameters.
type contourConfig struct {
	// GatewayNamespace is the Gateway namespace Contour should watch.
	GatewayNamespace string
	// GatewayName is the Gateway name Contour should watch.
	GatewayName string
}

// NewConfig returns a Config with default fields set.
func NewConfig() *Config {
	return &Config{Name: ContourCfgMapName}
}

// NewCfgForContour returns a ConfigMap Config with default fields set for contour.
func NewCfgForContour(contour *operatorv1alpha1.Contour) *Config {
	cfg := NewConfig()
	cfg.Namespace = contour.Namespace
	labels := objcontour.OwnerLabels(contour)
	cfg.Labels = labels
	return cfg
}

// NewCfgForGateway returns a ConfigMap Config with default fields set for gw.
func NewCfgForGateway(gw *gatewayv1alpha1.Gateway) *Config {
	cfg := NewConfig()
	cfg.Namespace = gw.Namespace
	labels := objgw.OwnerLabels(gw)
	cfg.Labels = labels
	cfg.Contour.GatewayNamespace = gw.Namespace
	cfg.Contour.GatewayName = gw.Name
	return cfg
}

// Ensure ensures that a ConfigMap exists for the given cfg.
func Ensure(ctx context.Context, cli client.Client, cfg *Config) error {
	desired, err := desired(cfg)
	if err != nil {
		return fmt.Errorf("failed to build configmap: %w", err)
	}
	current, err := current(ctx, cli, cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			return create(ctx, cli, desired)
		}
		return fmt.Errorf("failed to get configmap %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	if err := updateIfNeeded(ctx, cli, cfg, current, desired); err != nil {
		return fmt.Errorf("failed to update configmap %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	return nil
}

// Delete deletes a ConfigMap for the provided cfg, if the configured owner labels exist.
func Delete(ctx context.Context, cli client.Client, cfg *Config) error {
	cfgMap, err := current(ctx, cli, cfg)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if labels.Exist(cfgMap, cfg.Labels) {
		if err := cli.Delete(ctx, cfgMap); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

// current gets the ConfigMap for the provided cfg from the api server.
func current(ctx context.Context, cli client.Client, cfg *Config) (*corev1.ConfigMap, error) {
	current := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.Name,
	}
	err := cli.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// desired generates the desired ConfigMap for the given cfg.
func desired(cfg *Config) (*corev1.ConfigMap, error) {
	cfgFile := new(bytes.Buffer)
	if err := contourCfgTemplate.Execute(cfgFile, cfg.Contour); err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
			Labels:    cfg.Labels,
		},
		Data: map[string]string{
			"contour.yaml": cfgFile.String(),
		},
	}

	return cm, nil
}

// create creates a ConfigMap resource for the provided cm.
func create(ctx context.Context, cli client.Client, cm *corev1.ConfigMap) error {
	if err := cli.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create configmap %s/%s: %w", cm.Namespace, cm.Name, err)
	}
	return nil
}

// updateIfNeeded updates a ConfigMap if current does not match desired,
// using cfg to verify the existence of owner labels.
func updateIfNeeded(ctx context.Context, cli client.Client, cfg *Config, current, desired *corev1.ConfigMap) error {
	if labels.Exist(current, cfg.Labels) {
		changed, updated := cfgMapChanged(current, desired)
		if !changed {
			return nil
		}

		if err := cli.Update(ctx, updated); err != nil {
			return fmt.Errorf("failed to update configmap: %w", err)
		}
	}
	return nil
}

// cfgMapChanged compares current and expected returning true and an
// updated ConfigMap if they don't match.
func cfgMapChanged(current, expected *corev1.ConfigMap) (bool, *corev1.ConfigMap) {
	changed := false
	updated := current.DeepCopy()

	if !apiequality.Semantic.DeepEqual(current.Data, expected.Data) {
		changed = true
		updated.Data = expected.Data
	}

	return changed, updated
}
