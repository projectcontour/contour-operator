
# Image URL to use all building/pushing image targets
IMAGE ?= docker.io/projectcontour/contour-operator
VERSION ?= main
NEW_VERSION ?= VERSION

# Need v1 to support defaults in CRDs, unfortunately limiting us to k8s 1.16+
CRD_OPTIONS ?= "crd:crdVersions=v1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Platforms to build the multi-arch image for.
IMAGE_PLATFORMS ?= linux/amd64,linux/arm64

# Sets GIT_REF to a tag if it's present, otherwise the short git sha will be used.
GIT_REF = $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short=8 --verify HEAD)
VERSION ?= $(GIT_REF)

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
	go test ./... -coverprofile cover.out

lint-golint:
	@echo Running Go linter ...
	@./hack/golangci-lint.sh run

.PHONY: lint-codespell
lint-codespell: CODESPELL_SKIP := $(shell cat .codespell.skip | tr \\n ',')
lint-codespell:
	@echo Running Codespell ...
	@./hack/codespell.sh --skip $(CODESPELL_SKIP) --ignore-words .codespell.ignorewords --check-filenames --check-hidden -q2

# Build manager binary
manager: generate fmt vet
	go build -o bin/contour-operator cmd/contour-operator.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./cmd/contour-operator.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy the operator to a Kubernetes cluster. This assumes a kubeconfig in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMAGE}:${VERSION}
	kustomize build config/default | kubectl apply -f -

# Remove the operator deployment. This assumes a kubeconfig in ~/.kube/config
undeploy:
	cd config/manager
	kustomize build config/default | kubectl delete -f -

# Generate the example operator manifest
example:
	cd config/manager
	kustomize build config/default > examples/operator/operator.yaml

# Generate Contour's rendered CRD manifest (i.e. HTTPProxy).
# Remove when https://github.com/projectcontour/contour-operator/issues/42 is fixed.
.PHONY: generate-contour-crds
generate-contour-crds:
	@./hack/generate-contour-crds.sh $(VERSION)

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen generate-contour-crds example
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
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

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

local-cluster: # Create a local kind cluster
	./hack/kind-dev-cluster.sh

release: ## Prepares a tagged release of the operator.
.PHONY: release
release:
	./hack/release/make-release-tag.sh $(VERSION) $(NEW_VERSION)
