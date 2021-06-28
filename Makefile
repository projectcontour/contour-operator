# Sets GIT_REF to a tag if it's present, otherwise the short git sha will be used.
GIT_REF = $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short=8 --verify HEAD)
VERSION ?= $(GIT_REF)

# Used as an argument to prepare a tagged release of the operator.
OLD_VERSION ?= main
NEW_VERSION ?= $(OLD_VERSION)

# Used as a go test argument for running e2e tests.
TEST ?= .*
TEST_K8S_VERSION ?= 1.19.2

# Image URL to use all building/pushing image targets
IMAGE ?= docker.io/projectcontour/contour-operator
TAG_LATEST ?= false

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

ifeq ($(TAG_LATEST), true)
	IMAGE_TAGS = \
		--tag $(IMAGE):$(VERSION) \
		--tag $(IMAGE):latest
else
	IMAGE_TAGS = \
		--tag $(IMAGE):$(VERSION)
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

CONTROLLER_GEN := go run sigs.k8s.io/controller-tools/cmd/controller-gen
KUSTOMIZE := go run sigs.k8s.io/kustomize/kustomize/v3
SETUP_ENVTEST := go run sigs.k8s.io/controller-runtime/tools/setup-envtest

CODESPELL_SKIP := $(shell cat .codespell.skip | tr \\n ',')

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

##@ General

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

# Remove when https://github.com/projectcontour/contour-operator/issues/42 is fixed.
.PHONY: generate-contour-crds
generate-contour-crds: ## Generate Contour's rendered CRD manifest (i.e. HTTPProxy).
	@./hack/generate-contour-crds.sh $(NEW_VERSION)

manifests: generate-contour-crds ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=contour-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...
	go fmt ./test/e2e/

vet: ## Run go vet against code.
	go vet ./...

##@ Test

.PHONY: check
check: test lint-golint lint-codespell ## Run tests and validate against linters

test: manifests generate fmt vet ## Run tests.
	$(SETUP_ENVTEST) use -p path $(TEST_K8S_VERSION); KUBEBUILDER_ASSETS=${HOME}/.local/share/kubebuilder-envtest/k8s/$(TEST_K8S_VERSION)-$(shell go env GOHOSTOS)-$(shell go env GOHOSTARCH) go test ./... -coverprofile cover.out

lint-golint: ## Run golangci-lint against code.
	@echo Running Go linter ...
	@./hack/golangci-lint.sh run --build-tags=e2e

.PHONY: lint-codespell
lint-codespell: ## Run codespell against code.
	@echo Running Codespell ...
	@./hack/codespell.sh --skip $(CODESPELL_SKIP) --ignore-words .codespell.ignorewords --check-filenames --check-hidden -q2

.PHONY: test-e2e
test-e2e: deploy ## Run e2e tests.
	go test -mod=readonly -timeout 20m -count 1 -v -tags e2e -run "$(TEST)" ./test/e2e

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -mod=readonly -o bin/contour-operator cmd/contour-operator.go

run: manifests generate fmt vet install ## Run a controller from your host.
	go run ./cmd/contour-operator.go

docker-build: test ## Build the contour-operator container image.
	docker build \
		--build-arg "BUILD_VERSION=$(BUILD_VERSION)" \
		--build-arg "BUILD_BRANCH=$(BUILD_BRANCH)" \
		--build-arg "BUILD_SHA=$(BUILD_SHA)" \
		$(DOCKER_BUILD_LABELS) \
		$(shell pwd) \
		--tag $(IMAGE):$(VERSION)

docker-push: docker-build ## Push the contour-operator container image to the Docker registry.
	docker push ${IMAGE}:$(VERSION)
ifeq ($(TAG_LATEST), true)
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker push $(IMAGE):latest
endif

multiarch-build-push: ## Build and push a multi-arch contour-operator container image to the Docker registry.
	docker buildx build \
		--platform $(IMAGE_PLATFORMS) \
		--build-arg "BUILD_VERSION=$(BUILD_VERSION)" \
		--build-arg "BUILD_BRANCH=$(BUILD_BRANCH)" \
		--build-arg "BUILD_SHA=$(BUILD_SHA)" \
		$(DOCKER_BUILD_LABELS) \
		$(IMAGE_TAGS) \
		--push \
		.

##@ Deployment

install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMAGE}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

##@ Kind cluster

local-cluster: ## Create a local kind cluster
	./hack/kind-dev-cluster.sh

load-image: docker-build ## Load the operator image to a Kind cluster
	./hack/load-image.sh $(IMAGE) $(VERSION)

example: ## Generate example operator manifest.
	$(KUSTOMIZE) build config/default > config/samples/operator/operator.yaml

.PHONY: test-example
test-example: ## Test example operator.
	./hack/test-example.sh

##@ Release

.PHONY: verify-image
verify-image: ## Verifies operator image references and pull policy.
	./hack/verify-image.sh $(NEW_VERSION)

.PHONY: reset-image
reset-image: ## Resets operator image references and pull policy
	./hack/reset-image.sh $(IMAGE) $(OLD_VERSION)

.PHONY: release
release: ## Prepares a tagged release of the operator.
	./hack/release/make-release-tag.sh $(OLD_VERSION) $(NEW_VERSION)
