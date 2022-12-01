module github.com/projectcontour/contour-operator

go 1.15

require (
	github.com/docker/distribution v2.8.0+incompatible
	github.com/go-logr/logr v1.2.3
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.25.0
	k8s.io/apiextensions-apiserver v0.25.0
	k8s.io/apimachinery v0.25.0
	k8s.io/client-go v0.25.0
	k8s.io/utils v0.0.0-20220728103510-ee6ede2d64ed
	sigs.k8s.io/controller-runtime v0.12.1
	sigs.k8s.io/controller-tools v0.10.0
	sigs.k8s.io/gateway-api v0.5.1
)
