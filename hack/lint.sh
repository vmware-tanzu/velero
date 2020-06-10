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

LINTERS=$1

HACK_DIR=$(dirname "${BASH_SOURCE[0]}")

ADDITIONAL_LINTERS="gosec"
GOLANGCI_OPTIONS="-E $ADDITIONAL_LINTERS"

if [[ -n "${LINTERS}" ]]; then
  GOLANGCI_OPTIONS="-E ${LINTERS}"
fi

# Printing out cache status
golangci-lint cache status

echo "Running golangci-lint run $GOLANGCI_OPTIONS -c $HACK_DIR/../golangci.yaml"

# Enable GL_DEBUG line below for debug messages for golangci-lint
# export GL_DEBUG=loader,gocritic,env
golangci-lint run $GOLANGCI_OPTIONS -c $HACK_DIR/../golangci.yaml
