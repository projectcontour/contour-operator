# Sets GIT_REF to a tag if it's present, otherwise the short git sha will be used.
GIT_REF = $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short=8 --verify HEAD)
VERSION ?= $(GIT_REF)

# Used as an argument to prepare a tagged release of the operator.
OLD_VERSION ?= main
NEW_VERSION ?= $(OLD_VERSION)

# Used as a go test argument for running e2e tests.
TEST ?= .*

# Image URL to use all building/pushing image targets
REGISTRY ?= ghcr.io/projectcontour
IMAGE := ${REGISTRY}/contour-operator

# Need v1 to support defaults in CRDs, unfortunately limiting us to k8s 1.16+
CRD_OPTIONS ?= "crd:crdVersions=v1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CONTROLLER_GEN := go run sigs.k8s.io/controller-tools/cmd/controller-gen

# Platforms to build the multi-arch image for.
IMAGE_PLATFORMS ?= linux/amd64,linux/arm64

# Stash the ISO 8601 date. Note that the GMT offset is missing the :
# separator, but there doesn't seem to be a way to do that without
# depending on GNU date.
ISO_8601_DATE = $(shell TZ=GMT date '+%Y-%m-%dT%R:%S%z')

# Sets the current Git sha.
BUILD_SHA = $(shell git rev-parse --verify HEAD)
# Sets the current branch. If we are on a detached header, filter it out so the
# branch will be empty. This is similar to --show-current.
BUILD_BRANCH = $(shell git branch | grep -v detached | awk '$$1=="*"{print $$2}')
# Sets the current tagged git version.
BUILD_VERSION = $(VERSION)

# Docker labels to be applied to the contour-operator image. We don't transform
# this with make because it's not worth pulling the tricks needed to handle
# the embedded whitespace.
#
# See https://github.com/opencontainers/image-spec/blob/master/annotations.md
DOCKER_BUILD_LABELS = \
	--label "org.opencontainers.image.created=${ISO_8601_DATE}" \
	--label "org.opencontainers.image.url=https://github.com/projectcontour/contour-operator/" \
	--label "org.opencontainers.image.documentation=https://github.com/projectcontour/contour-operator/" \
	--label "org.opencontainers.image.source=https://github.com/projectcontour/contour-operator/archive/${BUILD_VERSION}.tar.gz" \
	--label "org.opencontainers.image.version=${BUILD_VERSION}" \
	--label "org.opencontainers.image.revision=${BUILD_SHA}" \
	--label "org.opencontainers.image.vendor=Project Contour" \
	--label "org.opencontainers.image.licenses=Apache-2.0" \
	--label "org.opencontainers.image.title=contour-operator" \
	--label "org.opencontainers.image.description=Deploy and manage Contour using an operator."

TAG_LATEST ?= false

ifeq ($(TAG_LATEST), true)
	IMAGE_TAGS = \
		--tag $(IMAGE):$(VERSION) \
		--tag $(IMAGE):latest
else
	IMAGE_TAGS = \
		--tag $(IMAGE):$(VERSION)
endif

all: manager

# Run tests & validate against linters
.PHONY: check
check: test lint-golint lint-codespell

# Run tests
test: generate fmt vet manifests
	go test \
	  -race \
	  -mod=readonly \
	  -covermode=atomic \
	  -coverprofile coverage.out \
	  -coverpkg=./cmd/...,./internal/...,./pkg/... \
	  ./...

lint-golint:
	@echo Running Go linter ...
	@./hack/golangci-lint.sh run --build-tags=e2e

.PHONY: lint-codespell
lint-codespell: CODESPELL_SKIP := $(shell cat .codespell.skip | tr \\n ',')
lint-codespell:
	@echo Running Codespell ...
	@./hack/codespell.sh --skip $(CODESPELL_SKIP) --ignore-words .codespell.ignorewords --check-filenames --check-hidden -q2

# Build manager binary
manager: generate fmt vet
	go build -mod=readonly -o bin/contour-operator cmd/contour-operator.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests install
	go run ./cmd/contour-operator.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

deploy: ## Deploy the operator to a Kubernetes cluster. This assumes a kubeconfig in ~/.kube/config
deploy: manifests
	cd config/manager
	kustomize build config/default | kubectl apply -f -

load-image: ## Load the operator image to a kind cluster
load-image: container
	./hack/load-image.sh $(IMAGE) $(VERSION)

# Remove the operator deployment. This assumes a kubeconfig in ~/.kube/config
undeploy:
	cd config/manager
	kustomize build config/default | kubectl delete -f -

example: ## Generate the example operator manifest.
example:
	cd config/manager
	kustomize build config/default > examples/operator/operator.yaml

test-example: ## Test the example Contour.
.PHONY: test-example
test-example:
	go test -mod=readonly -timeout 20m -count 1 -v -tags e2e -run "$(TEST)" ./test/e2e/example

verify-image: ## Verifies operator image references and pull policy.
.PHONY: verify-image
verify-image:
	./hack/verify-image.sh $(NEW_VERSION)

reset-image: ## Resets operator image references and pull policy.
.PHONY: reset-image
reset-image:
	./hack/reset-image.sh $(IMAGE) $(OLD_VERSION)

# Generate Contour's rendered CRD manifest (i.e. HTTPProxy).
# Remove when https://github.com/projectcontour/contour-operator/issues/42 is fixed.
.PHONY: generate-contour-crds
generate-contour-crds:
	@./hack/generate-contour-crds.sh $(NEW_VERSION)

manifests: ## Generate manifests e.g. CRD, RBAC etc.
manifests: generate-contour-crds
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=contour-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...
	go fmt ./test/e2e/

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate:
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

multiarch-build-push: ## Build and push a multi-arch contour-operator container image to the Docker registry
	docker buildx build \
		--platform $(IMAGE_PLATFORMS) \
		--build-arg "BUILD_VERSION=$(BUILD_VERSION)" \
		--build-arg "BUILD_BRANCH=$(BUILD_BRANCH)" \
		--build-arg "BUILD_SHA=$(BUILD_SHA)" \
		$(DOCKER_BUILD_LABELS) \
		$(IMAGE_TAGS) \
		--push \
		.

container: ## Build the contour-operator container image
container: test
	docker build \
		--build-arg "BUILD_VERSION=$(BUILD_VERSION)" \
		--build-arg "BUILD_BRANCH=$(BUILD_BRANCH)" \
		--build-arg "BUILD_SHA=$(BUILD_SHA)" \
		$(DOCKER_BUILD_LABELS) \
		$(shell pwd) \
		--tag $(IMAGE):$(VERSION)

push: ## Push the contour-operator container image to the Docker registry
push: container
	docker push $(IMAGE):$(VERSION)
ifeq ($(TAG_LATEST), true)
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker push $(IMAGE):latest
endif

local-cluster: # Create a local kind cluster
	./hack/kind-dev-cluster.sh

release: ## Prepares a tagged release of the operator.
.PHONY: release
release:
	./hack/release/make-release-tag.sh $(OLD_VERSION) $(NEW_VERSION)

test-e2e: ## Runs e2e tests.
.PHONY: test-e2e
test-e2e: deploy
	go test -mod=readonly -timeout 20m -count 1 -v -tags e2e -run "$(TEST)" ./test/e2e
