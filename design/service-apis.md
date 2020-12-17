# Service APIs

This document outlines a design for implementing [Service APIs][1] in Contour Operator.

## Goals

- Manage Contour and dependent resources using Service APIs.

## Non Goals

- Support application-level Service APIs, i.e. xRoute. These APIs will be supported by Contour directly according to
  Issue [2809][2].

## Definitions

### Service APIs

Service APIs is an open source project managed by the Kubernetes SIG-NETWORK community. The project's goal is to evolve
service networking APIs within the Kubernetes ecosystem. Service APIs provide interfaces to expose Kubernetes
applications- Services, Ingress, and more.

## Background

Contour Operator manages Contour using the `contours.operator.projectcontour.io` Custom Resource Definition (CRD). The
Contour Custom Resource (CR) provides the user interface for managing an instance of Contour and is handled by the
Kubernetes API just like built-in objects. Contour Operator monitors the Contour CR and provides status on whether the
actual state matches the desired state of the object. Contour Operator will continue to support the Contour CRD. Adding
support for Service APIs will provide users an alternative approach to managing Contour.

## High-Level Design

Contour Operator will manage Contour using the `gatewayclasses.networking.x-k8s.io` and `gateways.networking.x-k8s.io`
CRDs. The combination of GatewayClass and Gateway CRs will provide the user interface for managing an instance of
Contour. Service APIs provides extension points to allow for custom resources to be linked at various layers of the API.
This provides more granular customization at the appropriate places within the API structure. These extension points
will be leveraged to expose Contour-specific configuration.

## Detailed Design

### GatewayClass

GatewayClass is a cluster-scoped resource that defines a set of Gateways sharing a common behavior. A GatewayClass
with `controller: projectcontour.io/contour-operator` will be managed by Contour Operator. There MUST be at least one
GatewayClass defined in order to be able to have a functional Gateway. This is similar to IngressClass for Ingress and
StorageClass for PersistentVolumes. The following is an example of a GatewayClass managed by Contour Operator:

```yaml
kind: GatewayClass
apiVersion: networking.x-k8s.io/v1alpha1
metadata:
  name: example-gatewayclass
spec:
  controller: projectcontour.io/contour-operator
  parametersRef:
    name: example-contour-config
    group: operator.projectcontour.io
    kind: ContourConfig
```

The `controller` field is a domain/path string that indicates the controller responsible for managing Gateways of this
class. The `projectcontour.io/contour-operator` string will be used to specify Contour Operator as the controller
responsible for handling the GatewayClass. This field is not mutable and cannot be empty. Versioning can be accomplished
by encoding the operator's version into the path. For example:

```
projectcontour.io/contour-operator/v1   // Use version 1
projectcontour.io/contour-operator/v2   // Use version 2
projectcontour.io/contour-operator      // Use the default version
```

The `parametersRef` field is a reference to an implementation-specific resource containing configuration parameters that
are associated to the GatewayClass. This field will be used to expose Contour-specific configuration. If the field is
unspecified, the operator will create an instance of Contour with default settings. Since `parametersRef` does not
contain a namespace field, the referenced resource must exist in a known namespace. The operator's namespace will be
used as the known namespace. If the referenced resource cannot be found, the operator will set the GatewayClass
`InvalidParameters` status condition to true.

### ContourConfig

A new ContourConfig CR will be introduced to provide a structured object for representing the Contour configuration.
This configuration-only resource will be consumed by a GatewayClass. ContourConfig is equivalent to spec of a Contour CR
to provide consistency and supportability for managing Contour using the Contour CR or Service APIs. For example:

```go
type ContourConfig struct {
	// Spec defines the desired state of ContourConfig.
	Spec ContourSpec `json:"spec,omitempty"`
}
```

Here is an example ContourConfig:

```yaml
apiVersion: operator.projectcontour.io/v1alpha1
kind: ContourConfig
metadata:
  name: example-contour-config
  namespace: contour-operator
spec:
  replicas: 2
  namespace:
    name: projectcontour
```
__Note:__ ContourConfig does not contain a status field. Status will be reflected in the associated GatewayClass.

### Gateway

A Gateway describes how traffic can be translated to Services within a Kubernetes cluster. That is, it defines
infrastructure-specific settings for translating traffic from somewhere that does not know about Kubernetes to somewhere
that does. For example, traffic sent from a client outside the cluster to a Service resource through a proxy such as
Contour. While many use-cases have client traffic originating “outside” the cluster, this is not a requirement.

A Gateway is 1:1 with the lifecycle of the infrastructure configuration. When a Gateway is created, the operator will
create an instance of Contour. Gateway is the resource that triggers actions in Service APIs. Other Service API
resources represent structured configuration that are linked together by a Gateway.

A Gateway MAY contain one or more Service APIs *Route references which serve to direct traffic for a subset of traffic
to a specific service. Note that Contour Operator is NOT responsible for managing *Route Service API resources. The
following is an example of a Gateway that listens for HTTP traffic on TCP port 80: 

```yaml
kind: Gateway
apiVersion: networking.x-k8s.io/v1alpha1
metadata:
  name: example-gateway
spec:
  gatewayClassName: example-gatewayclass
  listeners:
    - protocol: HTTP
      port: 80
  ...
```

The `port` or `protocol` fields are required and specify the network port/protocol pair that Envoy will listen for
connection requests on. TLS configuration must be included in the Gateway listener when "HTTPS" or "TLS" is specified
for `protocol`. `certificateRef` of a TLS configuration is a reference to a Secret in the operator's namespace
containing a TLS certificate and private key. The secret MUST contain the following keys and data:

- tls.crt: certificate file contents
- tls.key: key file contents
  
For example:

```yaml
kind: Gateway
...
listeners:
- protocol: HTTPS
  port: 443
  tls:
    mode: Terminate
    certificateRef:
      kind: Secret
      group: core
      name: default-cert
---
kind: Secret
apiVersion: v1
metadata:
  name: default-cert
  namespace: contour-operator
data:
  tls.crt: <CERT>
  tls.key: <KEY>
```

Gateway contains a `routes` field that specifies a schema for associating routes with the `listener` using Kubernetes
selectors. A Route is a Service APIs resource capable of servicing a request and allows a user to expose a resource
(i.e. Service) by an externally-reachable URL, proxy traffic to the resource, and terminate SSL/TLS connections. The
following example will bind the Gateway to Routes that include label "app: example".The `port` or `protocol` fields are required when

```yaml
kind: Gateway
apiVersion: networking.x-k8s.io/v1alpha1
metadata:
  name: example-gateway
spec:
  gatewayClassName: example-gatewayclass
  listeners:
    - protocol: HTTP
      port: 80
      routes:
        kind: HTTPRoute
        selector:
          matchLabels:
            app: example
---
kind: HTTPRoute
apiVersion: networking.x-k8s.io/v1alpha1
metadata:
  name: example-httproute
  labels:
    app: example
spec:
  rules:
  - forwardTo:
    - serviceName: example-svc
      port: 8080
```

The `addresses` field specifies one or more network addresses requested for this Gateway. If unspecified, an IP address
will be automatically assigned. If the specified address is invalid or unavailable, the operator MUST indicate this in
Gateway status. The following is a Gateway example that specifies IP address "1.2.3.4". The address will be used for
each Gateway listener:

```yaml
kind: Gateway
apiVersion: networking.x-k8s.io/v1alpha1
metadata:
  name: example-gateway
spec:
  gatewayClassName: example-gatewayclass
  addresses:
    - type: IPAddress
      value: 1.2.3.4
```

Although Gateway supports multiple address types, only "IPAddress" will be supported by the Contour Operator.

## Implementation Details

### Controller

Create a controller that reconciles the GatewayClass resource and performs the following:

- Only reconcile a GatewayClass that specifies `controller: projectcontour.io/contour-operator`.
- Ensure the ContourConfig referenced by a GatewayClass `parametersRef` exists and is valid.
- Update the GatewayClass status conditions accordingly.

Create a controller that reconciles the Gateway resource and performs the following:

- Ensure the GatewayClass specified by `gatewayClassName` exists and add the
  `gateway-exists-finalizer.networking.x-k8s.io` finalizer to a GatewayClass whenever an associated Gateway is running.
- Validate the `spec` configuration.
- Ensure an instance of Contour exists with the desired configuration.  
- Update the Gateway status conditions accordingly.

### RBAC

Existing Contour Operator RBAC policies must be modified to allow the `update` verb for GatewayClass and Gateway
resources.

### Test Plan

- Add e2e, integration, and unit tests.
- Create a CI job to run tests.

[1]: https://github.com/kubernetes-sigs/service-apis
[2]: https://github.com/projectcontour/contour/issues/2809
