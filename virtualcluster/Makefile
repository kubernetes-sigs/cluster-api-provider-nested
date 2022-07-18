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


# Explicitly opt into go modules, even though we're inside a GOPATH directory
export GO111MODULE=on

# Image URL to use all building/pushing image targets
DOCKER_REG ?= ${or ${VC_DOCKER_REGISTRY},"virtualcluster"}
IMG ?= ${DOCKER_REG}/manager-amd64 ${DOCKER_REG}/vn-agent-amd64 ${DOCKER_REG}/syncer-amd64

# TEST_FLAGS used as flags of go test.
TEST_FLAGS ?= -v

# COVERAGE_PACKAGES is the coverage we care about.
COVERAGE_PACKAGES=$(shell go list ./... | \
				  grep -v sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/client | \
				  grep -v sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis | \
				  grep -v sigs.k8s.io/cluster-api-provider-nested/virtualcluster/cmd | \
				  grep -v sigs.k8s.io/cluster-api-provider-nested/virtualcluster/test/e2e)

# CRD_OPTIONS ?= "crd:trivialVersions=true"
CRD_OPTIONS ?= "crd:trivialVersions=true,maxDescLen=0"

export KUBEBUILDER_ASSETS=/tmp/kubebuilder/bin/

# Build code.
#
# Args:
#   WHAT: Directory names to build.  If any of these directories has a 'main'
#     package, the build will produce executable files under $(OUT_DIR).
#     If not specified, "everything" will be built.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   GOLDFLAGS: Extra linking flags passed to 'go' when building.
#   GOGCFLAGS: Additional go compile flags passed to 'go' when building.
#
# Example:
#   make
#   make all
#   make all WHAT=cmd/kubelet GOFLAGS=-v
#   make all GOLDFLAGS=""
#     Note: Specify GOLDFLAGS as an empty string for building unstripped binaries, which allows
#           you to use code debugging tools like delve. When GOLDFLAGS is unspecified, it defaults
#           to "-s -w" which strips debug information. Other flags that can be used for GOLDFLAGS
#           are documented at https://golang.org/cmd/link/
.PHONY: all
all: test build

build: 
	hack/make-rules/build.sh $(WHAT)

# Run tests
.PHONY: test
PWD = $(CURDIR)
test: generate fmt vet manifests
	@mkdir -p coverage
	@( for pkg in ${COVERAGE_PACKAGES}; do \
		go test ${TEST_FLAGS} \
			-coverprofile=coverage/unit-test-`echo $$pkg | tr "/" "_"`.out \
			$$pkg || exit 1 ;\
	done )
	@( cd ./pkg/vn-agent/server/test; \
		go test ${TEST_FLAGS} \
			-coverprofile=${PWD}/coverage/unit-test-pkg_vn-agent_server_test.out )
	@cd ${PWD}


# run `make install-envtest` to refresh the required binaries in case the envTest fails.
K8S_VERSION ?= 1.19.2
FULL_K8S_VERSION ?= Kubernetes v$(K8S_VERSION)
install-envtest:
ifneq ($(FULL_K8S_VERSION), $(shell /usr/local/kubebuilder/bin/kube-apiserver --version))
	sudo mkdir -p /usr/local/kubebuilder
	curl -sSLo envtest-bins.tar.gz "https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-${K8S_VERSION}-$(shell go env GOOS)-$(shell go env GOARCH).tar.gz"
	sudo tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz && rm envtest-bins.tar.gz
endif

.PHONY: coverage
coverage: ## combine coverage after test
	@mkdir -p coverage
	@gocovmerge coverage/* > coverage/coverage.txt
	@go tool cover -html=coverage/coverage.txt -o coverage/coverage.html

.PHONY: clean
clean: ## clean to remove bin/* and files created by module
	@go mod tidy
	@rm -rf _output/*
	@rm -rf coverage/*

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crd
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role paths="./..." output:crd:none
	hack/make-rules/replace-null.sh

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate: controller-gen
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build release image.
#
# 1. run tests
# 2. build docker image
.PHONY: release-images
release-images: test build-images

# Build docker image.
#
# 1. build all binaries.
# 2. copy binaries to the corresponding docker image.
build-images:
	hack/make-rules/release-images.sh $(WHAT)

# Push the docker image
docker-push:
	$(foreach i,$(IMG),docker push $i;)

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Build and run kubernetes e2e tests.
#
# Args:
#  KUBECONFIG: kubeconfig to virtual cluster. If empty, create a virtual cluster.
#    Defaults to "".
#  FOCUS: Regexp that matches the tests to be run.  Defaults to "\[Conformance\]".
#  SKIP: Regexp that matches the tests that needs to be skipped.
#    Defaults to "\[Flaky\]|\[Slow\]|\[Serial\]"
#  BUILD_DEPENDENCIES: if true, build dependencies related to e2e test.
#    Defaults to true.
#
# Example:
#   make test-e2e-k8s KUBECONFIG=/path/to/vc-kubeconfig
.PHONY: test-e2e-k8s
test-e2e-k8s:
	hack/make-rules/test-e2e-k8s.sh
