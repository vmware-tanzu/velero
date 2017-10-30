#!/bin/bash -e
#
# Copyright 2017 the Heptio Ark contributors.
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

HACK_DIR=$(dirname "${BASH_SOURCE}")

echo "Verifying gofmt"
files=$(gofmt -l -s $(find . -type f -name "*.go" -not -path "./vendor/*" -not -path "./pkg/generated/*" -not -name "zz_generated*"))
if [[ -n "${files}" ]]; then
  echo "The following files need gofmt updating - please run 'make update'"
  echo "${files}"
  exit 1
fi
echo "Success!"

echo "Verifying goimports"
command -v goimports > /dev/null || go get golang.org/x/tools/cmd/goimports
goimports -l $(find . -type f -name "*.go" -not -path "./vendor/*" -not -path "./pkg/generated/*" -not -name "zz_generated*")
echo "Success!"

