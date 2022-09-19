#!/bin/bash -e
#
# Copyright 2018, 2019 the Velero contributors.
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

VELERO_ROOT=$(dirname ${BASH_SOURCE})/..
BIN=${VELERO_ROOT}/_output/bin

mkdir -p ${BIN}

echo "Updating generated Github issue template"
go build -o ${BIN}/issue-tmpl-gen ./hack/issue-template-gen/main.go

if [[ $# -gt 1 ]]; then
  echo "usage: ${BASH_SOURCE} [OUTPUT_FILE]"
  exit 1
fi

OUTPUT_ISSUE_FILE="$1"
if [[ -z "${OUTPUT_ISSUE_FILE}" ]]; then
  OUTPUT_ISSUE_FILE=${VELERO_ROOT}/.github/ISSUE_TEMPLATE/bug_report.md
fi

${BIN}/issue-tmpl-gen ${OUTPUT_ISSUE_FILE} 
echo "Success!"
