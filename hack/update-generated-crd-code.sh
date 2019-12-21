#!/bin/bash
#
# Copyright 2017 the Velero contributors.
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

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

source "$(dirname "$0")/utils.sh"

TOOLS_DIR=$(get_repo_root)/hack/tools


# GO111MODULE=on ${GOPATH}/src/k8s.io/code-generator/generate-groups.sh \
#   client \
#   github.com/vmware-tanzu/velero/pkg/generated \
#   github.com/vmware-tanzu/velero/pkg/apis \
#   "velero:v1" \
#   --go-header-file $(get_repo_root)/hack/boilerplate.go.txt \
#   $@

echo "# Generate deepcopy funcs"
${TOOLS_DIR}/bin/controller-gen \
  object:headerFile=$(get_repo_root)/hack/boilerplate.go.txt \
  paths=$(get_repo_root)/pkg/apis/velero/v1/...

echo "# Generate client for types"
${TOOLS_DIR}/bin/client-gen \
 --clientset-name "versioned" \
 --input-base '' \
 --input github.com/vmware-tanzu/velero/pkg/apis/velero/v1 \
 --output-base '' \
 --output-package 'pkg/generated/clientset' \
 --go-header-file $(get_repo_root)/hack/boilerplate.go.txt

 echo "# Generate listers for types"
 ${TOOLS_DIR}/bin/lister-gen \
  --input-dirs github.com/vmware-tanzu/velero/pkg/apis/velero/v1 \
  --output-base '' \
  --output-package 'pkg/generated/listers' \
  --go-header-file $(get_repo_root)/hack/boilerplate.go.txt

 echo "# Generate informers for types"
 ${TOOLS_DIR}/bin/informer-gen \
  --input-dirs github.com/vmware-tanzu/velero/pkg/apis/velero/v1 \
  --versioned-clientset-package 'pkg/generated/clientset/versioned' \
  --listers-package "pkg/generated/listers" \
  --output-base '' \
  --output-package 'pkg/generated/informers' \
  --go-header-file $(get_repo_root)/hack/boilerplate.go.txt \
  --v 5

echo "# Generate CRD manifests"
${TOOLS_DIR}/bin/controller-gen \
  crd \
  output:dir=$(get_repo_root)/pkg/generated/crds/manifests \
  paths=$(get_repo_root)/pkg/apis/velero/v1/...


# go generate $(get_repo_root)/pkg/generated/crds
