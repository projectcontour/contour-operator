module github.com/projectcontour/contour-operator

go 1.15

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/go-logr/logr v1.2.0
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.1
	k8s.io/apiextensions-apiserver v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/controller-tools v0.6.2
	sigs.k8s.io/gateway-api v0.4.0
)
