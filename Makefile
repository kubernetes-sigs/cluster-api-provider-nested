# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

# Directories.
EXP_DIR := exp
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin
RELEASE_NOTES_BIN := bin/release-notes
RELEASE_NOTES := $(TOOLS_DIR)/$(RELEASE_NOTES_BIN)
GO_APIDIFF_BIN := bin/go-apidiff
GO_APIDIFF := $(TOOLS_DIR)/$(GO_APIDIFF_BIN)
ENVSUBST_BIN := bin/envsubst
ENVSUBST := $(TOOLS_DIR)/$(ENVSUBST_BIN)

export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

# Binaries.
# Need to use abspath so we can invoke these from subdirectories
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/kustomize)
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/controller-gen)
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/golangci-lint)
ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/envsubst)

# Define Docker related variables. Releases should modify and double check these vars.
REGISTRY ?= gcr.io
ifneq ($(shell gcloud config get-value project), )
REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
endif
STAGING_REGISTRY ?= gcr.io/k8s-staging-cluster-api-nested
PROD_REGISTRY ?= us.gcr.io/k8s-artifacts-prod/cluster-api-nested

# Infrastructure
IMAGE_NAME ?= cluster-api-nested-controller
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)

# Control Plane
CONTROLPLANE_IMAGE_NAME ?= nested-controlplane-controller
CONTROLPLANE_CONTROLLER_IMG ?= $(REGISTRY)/$(CONTROLPLANE_IMAGE_NAME)

TAG ?= dev
ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

# Set build time variables including version details
LDFLAGS := $(shell hack/version.sh)

all: test managers clusterctl

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: ## Run tests.
	source ./scripts/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v ./... $(TEST_ARGS)

## --------------------------------------
## Binaries
## --------------------------------------
.PHONY: binaries
binaries: managers

.PHONY: managers
managers: ## Build all managers
	$(MAKE) manager-nested-infrastructure
	$(MAKE) manager-nested-controlplane

.PHONY: manager-nested-infrastructure
manager-nested-infrastructure:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/manager sigs.k8s.io/cluster-api-provider-nested

.PHONY: manager-nested-controlplane
manager-nested-controlplane: ## Build manager binary
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/nested-controlplane-manager sigs.k8s.io/cluster-api-provider-nested/controlplane/nested

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT): # Download golanci-lint using hack script into tools folder.
	hack/ensure-golangci-lint.sh \
		-b $(TOOLS_DIR)/$(BIN_DIR) \
		v1.40.1

$(RELEASE_NOTES): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o $(RELEASE_NOTES_BIN) ./release

$(GO_APIDIFF): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o $(GO_APIDIFF_BIN) github.com/joelanford/go-apidiff

$(ENVSUBST): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o $(ENVSUBST_BIN) github.com/drone/envsubst/cmd/envsubst

$(KUSTOMIZE): # Build kustomize from tools folder.
	hack/ensure-kustomize.sh

envsubst: $(ENVSUBST) ## Build a local copy of envsubst.
kustomize: $(KUSTOMIZE) ## Build a local copy of kustomize.
controller-gen: $(CONTROLLER_GEN) ## Build a local copy of controller-gen.
golangci-lint: $(GOLANGCI_LINT) ## Build a local copy of golangci-lint.

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint lint-full
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false

apidiff: $(GO_APIDIFF) ## Check for API differences
	$(GO_APIDIFF) $(shell git rev-parse origin/main) --print-compatible

## --------------------------------------
## Generate / Manifests
## --------------------------------------

.PHONY: generate
generate:
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) ## Runs Go related generate targets
	go generate ./...
	$(MAKE) generate-go-infrastructure
	$(MAKE) generate-go-controlplane

.PHONY: generate-go-infrastructure
generate-go-infrastructure: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt \
		paths=./api/...

generate-go-controlplane: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt \
		paths=./controlplane/nested/api/...

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(MAKE) generate-manifests-infrastructure
	$(MAKE) generate-manifests-controlplane
	## Copy files in CI folders.
	mkdir -p ./config/ci/{rbac,manager}
	cp -f ./config/rbac/*.yaml ./config/ci/rbac/
	cp -f ./config/manager/manager*.yaml ./config/ci/manager/

.PHONY: generate-manifests-infrastructure
generate-manifests-infrastructure:
	$(CONTROLLER_GEN) \
		paths=./api/... \
		paths=./controllers/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=./config/crd/bases \
		output:webhook:dir=./config/webhook \
		output:rbac:dir=./config/rbac \
		webhook

.PHONY: generate-manifests-controlplane
generate-manifests-controlplane:
	$(CONTROLLER_GEN) \
		paths=./controlplane/nested/api/... \
		paths=./controlplane/nested/controllers/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=./controlplane/nested/config/crd/bases \
		output:webhook:dir=./controlplane/nested/config/webhook \
		output:rbac:dir=./controlplane/nested/config/rbac \
		webhook

.PHONY: modules
modules: ## Runs go mod to ensure modules are up to date.
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-pull-prerequisites
docker-pull-prerequisites:
	docker pull docker.io/docker/dockerfile:experimental
	docker pull docker.io/library/golang:1.17.2
	docker pull gcr.io/distroless/static:latest

.PHONY: docker-infrastructure-build
docker-infrastructure-build: docker-pull-prerequisites ## Build the docker images for controller managers
	DOCKER_BUILDKIT=1 docker build --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) --build-arg ldflags="$(LDFLAGS)" . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

.PHONY: docker-controlplane-build
docker-controlplane-build: docker-pull-prerequisites ## Build the docker images for controller managers
	DOCKER_BUILDKIT=1 docker build --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) --build-arg ldflags="$(LDFLAGS)" --build-arg package=./controlplane/nested . -t $(CONTROLPLANE_CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLPLANE_CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./controlplane/nested/config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./controlplane/nested/config/default/manager_pull_policy.yaml"


.PHONY: docker-infrastructure-push
docker-infrastructure-push: ## Push the docker images
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

.PHONY: docker-controlplane-push
docker-controlplane-push: ## Push the docker images
	docker push $(CONTROLPLANE_CONTROLLER_IMG)-$(ARCH):$(TAG)

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-infrastructure-build-,$(ALL_ARCH)) $(addprefix docker-controlplane-build-,$(ALL_ARCH))

.PHONY: docker-build
docker-build:
	$(MAKE) docker-infrastructure-build
	$(MAKE) docker-controlplane-build

.PHONY: docker-infrastructure-build
docker-infrastructure-build-%:
	$(MAKE) ARCH=$* docker-infrastructure-build

.PHONY: docker-controlplane-build
docker-controlplane-build-%:
	$(MAKE) ARCH=$* docker-controlplane-build

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-infrastructure-push-,$(ALL_ARCH)) $(addprefix docker-controlplane-push-,$(ALL_ARCH))
	$(MAKE) docker-push-core-manifest
	$(MAKE) docker-push-nested-control-plane-manifest

.PHONY: docker-push
docker-push:
	$(MAKE) docker-infrastructure-push
	$(MAKE) docker-controlplane-push

.PHONY: docker-infrastructure-push
docker-infrastructure-push-%:
	$(MAKE) ARCH=$* docker-infrastructure-push

.PHONY: docker-controlplane-push
docker-controlplane-push-%:
	$(MAKE) ARCH=$* docker-controlplane-push

.PHONY: docker-push-core-manifest
docker-push-core-manifest: ## Push the fat manifest docker image for the core image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge $(CONTROLLER_IMG):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

.PHONY: docker-push-nested-control-plane-manifest
docker-push-nested-control-plane-manifest: ## Push the fat manifest docker image for the nested control plane image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLPLANE_CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLPLANE_CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLPLANE_CONTROLLER_IMG}:${TAG} ${CONTROLPLANE_CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge $(CONTROLPLANE_CONTROLLER_IMG):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLPLANE_CONTROLLER_IMG) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./controlplane/nested/config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./controlplane/nested/config/default/manager_pull_policy.yaml"

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resources)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(TARGET_RESOURCE)

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' $(TARGET_RESOURCE)

## --------------------------------------
## Release
## --------------------------------------

RELEASE_TAG := $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR := out

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release
release: clean-release ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the core manifest image to the production bucket.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG) \
		TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"
	# Set the control plane manifest image to the production bucket.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(PROD_REGISTRY)/$(CONTROLPLANE_IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG) \
		TARGET_RESOURCE="./controlplane/nested/config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent TARGET_RESOURCE="./controlplane/nested/config/default/manager_pull_policy.yaml"
	## Build the manifests
	$(MAKE) release-manifests clean-release-git

.PHONY: release-manifests
release-manifests: $(RELEASE_DIR) $(KUSTOMIZE) ## Builds the manifests to publish with a release
	# Build infrastructure-components.
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	# Build control-plane-components.
	$(KUSTOMIZE) build controlplane/nested/config/default > $(RELEASE_DIR)/control-plane-components.yaml

	## Build cluster-api-provider-nested-components (aggregate of all of the above).
	cat $(RELEASE_DIR)/infrastructure-components.yaml > $(RELEASE_DIR)/cluster-api-provider-nested-components.yaml
	echo "---" >> $(RELEASE_DIR)/cluster-api-provider-nested-components.yaml
	cat $(RELEASE_DIR)/control-plane-components.yaml >> $(RELEASE_DIR)/cluster-api-provider-nested-components.yaml
	# Add metadata to the release artifacts
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml
	cp templates/cluster-template*.yaml $(RELEASE_DIR)/


.PHONY: release-staging
release-staging: ## Builds and push container images to the staging bucket.
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-alias-tag

RELEASE_ALIAS_TAG=$(PULL_BASE_REF)

.PHONY: release-alias-tag
release-alias-tag: ## Adds the tag to the last build tag.
	gcloud container images add-tag $(CONTROLLER_IMG):$(TAG) $(CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)
	gcloud container images add-tag $(CONTROLPLANE_CONTROLLER_IMG):$(TAG) $(CONTROLPLANE_CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)

.PHONY: release-notes
release-notes: $(RELEASE_NOTES)  ## Generates a release notes template to be used with a release.
	$(RELEASE_NOTES)

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
	rm -rf hack/tools/bin

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_image_patch.yaml ./*manager_pull_policy.yaml

.PHONY: verify
verify:
	./hack/verify-boilerplate.sh
	./hack/verify-doctoc.sh
	./hack/verify-shellcheck.sh
	$(MAKE) verify-modules
	$(MAKE) verify-gen

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod hack/tools/go.mod hack/tools/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi
	@if (find . -name 'go.mod' | xargs -n1 grep -q -i 'k8s.io/client-go.*+incompatible'); then \
		find . -name "go.mod" -exec grep -i 'k8s.io/client-go.*+incompatible' {} \; -print; \
		echo "go module contains an incompatible client-go version"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi
