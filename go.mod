module github.com/projectcontour/contour-operator

go 1.15

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/opencontainers/go-digest v1.0.0 // indirect
	k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/controller-runtime v0.9.0-beta.0
	sigs.k8s.io/controller-tools v0.5.0
	sigs.k8s.io/gateway-api v0.3.0
)
