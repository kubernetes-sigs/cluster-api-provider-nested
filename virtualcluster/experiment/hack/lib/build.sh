#!/usr/bin/env bash

# Copyright 2020 The Kubernetes Authors.
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

readonly VC_EXPERIMENT_GO_PACKAGE=sigs.k8s.io/cluster-api-provider-nested/virtualcluster/experiment

readonly VC_EXPERIMENT_ALL_TARGETS=(
  cmd/scheduler
)
readonly VC_EXPERIMENT_ALL_BINARIES=("${VC_EXPERIMENT_ALL_TARGETS[@]##*/}")

# binaries_from_targets take a list of build targets and return the
# full go package to be built
binaries_from_targets() {
  local target
  for target; do
    # If the target starts with what looks like a domain name, assume it has a
    # fully-qualified package name rather than one that needs the Kubernetes
    # package prepended.
    if [[ "${target}" =~ ^([[:alnum:]]+".")+[[:alnum:]]+"/" ]]; then
      echo "${target}"
    else
      echo "${VC_EXPERIMENT_GO_PACKAGE}/${target}"
    fi
  done
}

version() {
  # GIT_COMMIT is used for daemon GitCommit in go build.
  GIT_COMMIT=$(git rev-parse HEAD)

  # BUILD_DATE is used for daemon BuildTime in go build.
  BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

  GIT_VERSION="v1.0.0"

  VERSION_PKG=sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/version
  echo "-X ${VERSION_PKG}.gitVersion=${GIT_VERSION}"
  echo "-X ${VERSION_PKG}.gitCommit=${GIT_COMMIT}"
  echo "-X ${VERSION_PKG}.buildDate=${BUILD_DATE}"
}

# Build binaries targets specified
#
# Input:
#   $@ - targets and go flags.  If no targets are set then all binaries targets
#     are built.
build_binaries() {
  local goflags goldflags gcflags
  goldflags="${GOLDFLAGS:-} -s -w $(version)"
  gcflags="${GOGCFLAGS:-}"
  goflags=${GOFLAGS:-}

  local -a targets=()
  local arg

  for arg; do
    if [[ "${arg}" == -* ]]; then
      # Assume arguments starting with a dash are flags to pass to go.
      goflags+=("${arg}")
    else
      targets+=("${arg}")
    fi
  done

  if [[ ${#targets[@]} -eq 0 ]]; then
    targets=("${VC_EXPERIMENT_ALL_TARGETS[@]}")
  fi

  local -a binaries
  while IFS="" read -r binary; do binaries+=("$binary"); done < <(binaries_from_targets "${targets[@]}")

  mkdir -p ${VC_EXPERIMENT_BIN_DIR}
  cd ${VC_EXPERIMENT_BIN_DIR}
  for binary in "${binaries[@]}"; do
    echo "Building ${binary}"
    GOOS=${GOOS:-linux} go build -ldflags "${goldflags:-}" -gcflags "${gcflags:-}" ${goflags} ${binary}
  done
}
