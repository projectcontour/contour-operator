module github.com/projectcontour/contour-operator

go 1.15

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.2.0 //indirect
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/projectcontour/contour v1.9.0
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.4.0
	sigs.k8s.io/service-apis v0.1.1-0.20210114050902-2b2bf1b8fdf5
)
