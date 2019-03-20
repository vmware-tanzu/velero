#!/bin/bash -e
#
# Copyright 2018 the Velero contributors.
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
HACK_DIR=$(dirname "${BASH_SOURCE}")
ISSUE_TEMPLATE_FILE=${VELERO_ROOT}/.github/ISSUE_TEMPLATE/bug_report.md
OUT_TMP_FILE="$(mktemp -d)"/bug_report.md


trap cleanup INT TERM HUP EXIT

cleanup() {
  rm -rf ${TMP_DIR}
}

echo "Verifying generated Github issue template"
${HACK_DIR}/update-generated-issue-template.sh ${OUT_TMP_FILE} > /dev/null
output=$(echo "`diff ${ISSUE_TEMPLATE_FILE} ${OUT_TMP_FILE}`")

if [[ -n "${output}" ]] ; then
    echo "FAILURE: verification of generated template failed:"
    echo "${output}"
    exit 1
fi

echo "Success!"
