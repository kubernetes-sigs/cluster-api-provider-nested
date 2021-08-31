# syntax=docker/dockerfile:1.1-experimental

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

# Build the manager binary
FROM golang:1.16.2 as builder
WORKDIR /workspace

# Run this with docker build --build_arg $(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN  --mount=type=cache,target=/root/.local/share/golang \
     --mount=type=cache,target=/go/pkg/mod \
     go mod download

# Copy the sources
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY controlplane/ controlplane/

# Build
ARG package=.
ARG ARCH
ARG LDFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.local/share/golang \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -ldflags "${LDFLAGS} -extldflags '-static'"  -o manager ${package}
RUN curl -L https://dl.k8s.io/release/v1.22.0/bin/linux/amd64/kubeadm -o kubeadm && chmod +x kubeadm
ENTRYPOINT [ "/start.sh", "/workspace/manager" ]

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian10
# Copy the controller-manager into a thin image
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/kubeadm kubeadm
# USER 65532:65532
ENTRYPOINT ["/manager"]
