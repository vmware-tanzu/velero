#!/bin/bash

# Copyright 2020 the Velero contributors.
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

if [[ -z "${BIN}" ]]; then
    echo "BIN must be set"
    exit 1
fi

if [[ "${BIN}" != "velero" ]]; then
    echo "${BIN} does not need the restic binary"
    exit 0
fi

if [[ -z "${GOOS}" ]]; then
    echo "GOOS must be set"
    exit 1
fi
if [[ -z "${GOARCH}" ]]; then
    echo "GOARCH must be set"
    exit 1
fi
if [[ -z "${RESTIC_VERSION}" ]]; then
    echo "RESTIC_VERSION must be set"
    exit 1
fi

# TODO: when the new restic version is released, make ppc64le to be also downloaded from their github releases.
#  This has been merged and will be applied to next release: https://github.com/restic/restic/pull/2342
if [[ "${GOARCH}" = "ppc64le" ]]; then
    wget --timeout=1 --tries=5 --quiet https://oplab9.parqtec.unicamp.br/pub/ppc64el/restic/restic-${RESTIC_VERSION} -O /output/usr/bin/restic
else
    wget --quiet https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_${GOOS}_${GOARCH}.bz2
    bunzip2 restic_${RESTIC_VERSION}_${GOOS}_${GOARCH}.bz2
    mv restic_${RESTIC_VERSION}_${GOOS}_${GOARCH} /output/usr/bin/restic
fi

chmod +x /output/usr/bin/restic
