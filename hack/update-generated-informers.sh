#!/bin/bash -e
#
# Copyright 2017 Heptio Inc.
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

ARK_ROOT=$(dirname ${BASH_SOURCE})/..
BIN=${ARK_ROOT}/_output/bin
mkdir -p ${BIN}

echo "Updating generated informers"

go build -o ${BIN}/informer-gen ./vendor/k8s.io/kubernetes/cmd/libs/go2idl/informer-gen

OUTPUT_BASE=""
if [[ -z "${GOPATH}" ]]; then
  OUTPUT_BASE="${HOME}/go/src"
else
  OUTPUT_BASE="${GOPATH}/src"
fi

verify=""
for i in "$@"; do
  if [[ $i == "--verify-only" ]]; then
    verify=1
    break
  fi
done

if [[ -z ${verify} ]]; then
  rm -rf ${ARK_ROOT}/pkg/generated/informers
fi

${BIN}/informer-gen \
  --logtostderr \
  --go-header-file /dev/null \
  --output-base ${OUTPUT_BASE} \
  --input-dirs github.com/heptio/ark/pkg/apis/ark/v1 \
  --output-package github.com/heptio/ark/pkg/generated/informers \
  --listers-package github.com/heptio/ark/pkg/generated/listers \
  --internal-clientset-package github.com/heptio/ark/pkg/generated/clientset \
  --versioned-clientset-package github.com/heptio/ark/pkg/generated/clientset \
  $@

echo "Success!"
