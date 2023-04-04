#!/bin/bash
#
# Copyright the Velero contributors.
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

# this script expects to be run from the root of the Velero repo.

if [[ -z "${GOPATH}" ]]; then
  GOPATH=~/go
fi

if ! command -v controller-gen > /dev/null; then
  echo "controller-gen is missing"
  exit 1
fi

# get code-generation tools (for now keep in GOPATH since they're not fully modules-compatible yet)
mkdir -p ${GOPATH}/src/k8s.io
pushd ${GOPATH}/src/k8s.io
git clone -b v0.22.2 https://github.com/kubernetes/code-generator
popd

${GOPATH}/src/k8s.io/code-generator/generate-groups.sh \
  all \
  github.com/vmware-tanzu/velero/pkg/generated \
  github.com/vmware-tanzu/velero/pkg/apis \
  "velero:v1" \
  --go-header-file ./hack/boilerplate.go.txt \
  --output-base ../../.. \
  $@

# Generate apiextensions.k8s.io/v1
# Generate manifests e.g. CRD, RBAC etc.
controller-gen \
  crd:crdVersions=v1 \
  paths=./pkg/apis/velero/v1/... \
  rbac:roleName=velero-perms \
  paths=./pkg/controller/... \
  output:crd:artifacts:config=config/crd/v1/bases \
  object \
  paths=./pkg/apis/velero/v1/...

go generate ./config/crd/v1/crds
