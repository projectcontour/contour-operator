module github.com/projectcontour/contour-operator

go 1.15

require (
	github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr v0.2.0
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/projectcontour/contour v1.9.0
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.3
)
