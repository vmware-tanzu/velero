#!/bin/bash -e

# Copyright 2023 the Velero contributors.

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BASE_DIR=./site/content/docs/main/cli-reference
WORKDIR=$(pwd)
go run ./cmd/doc-gen/doc-gen.go ${BASE_DIR}

# generate the cli-reference page for release branches
RELEASE_PREFIX="release-"
ORG="vmware-tanzu"
# ensure we have remote for vmware-tanzu/velero
git remote add ${ORG} https://github.com/${ORG}/velero.git || true
git fetch ${ORG}
RELEASE_BRANCHES=$(git branch --remotes | grep ${ORG}/release-1)
# for each release branch, generate the cli-reference page
for RELEASE_BRANCH in ${RELEASE_BRANCHES}; do
  # get the release branch name
  RELEASE_BRANCH_NAME=$(echo ${RELEASE_BRANCH} | sed 's/.*release-\([0-9]*\.[0-9]*\).*/\1/')
  echo "Generating cli-reference for v${RELEASE_BRANCH_NAME}"
  # copy current repo to a temp dir
  # change to temp dir
  # checkout to release branch
  # if no go.mod, skip
  # copy cmd/doc-gen/doc-gen.go from main
  # cleanup the temp dir
  TMP_DIR=$(mktemp -d)
  echo "Created temp dir: ${TMP_DIR} to use generate reference for v${RELEASE_BRANCH_NAME}" && \
  cd ${TMP_DIR} &&  \
  git clone https://github.com/${ORG}/velero.git --branch release-${RELEASE_BRANCH_NAME} --single-branch --depth 1 && \
  cd velero && \
  echo pwd is ${WORKDIR} && \
  if [[ ! -f go.mod ]]; then echo "no go.mod, skipping"; chmod -R 777 ${TMP_DIR} && rm -rf ${TMP_DIR}; continue; fi && \
  cp -r ${WORKDIR}/cmd/doc-gen/ cmd/doc-gen/ && \
  go get github.com/spf13/cobra/doc@v1.4.0 && \
  go run ./cmd/doc-gen/doc-gen.go ${WORKDIR}/site/content/docs/v${RELEASE_BRANCH_NAME}/cli-reference && \
  echo "successfully generated cli-reference for v${RELEASE_BRANCH_NAME}, cleaning up" && \
  chmod -R 777 ${TMP_DIR} && \
  rm -rf ${TMP_DIR} && echo "all clean!" || (echo "failed, cleaning ${TMP_DIR} then aborting" && chmod -R 777 ${TMP_DIR} && rm -rf ${TMP_DIR});
done

if [[ ${1:-} == '--verify' ]]; then
  # Use git diff to tell if ./site/content/docs/main/cli-reference needs updating
  cd ${WORKDIR} && git diff --exit-code ${BASE_DIR} && echo "cli-reference is up to date" || (echo "cli-reference is out of date, please run 'make update' and commit"; exit 1)
fi
