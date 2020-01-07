#!/bin/bash
#
# Copyright 2019 the Velero contributors.
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

if [ -z "${RESTIC_VERSION}" ]; then
    echo "RESTIC_VERSION must be set"
    exit 1
fi

if [ ! -d "_output/bin/linux/ppc64le/" ]; then
    mkdir -p _output/bin/linux/ppc64le/
fi

wget --quiet https://oplab9.parqtec.unicamp.br/pub/ppc64el/restic/restic-${RESTIC_VERSION}
mv restic-${RESTIC_VERSION} _output/bin/linux/ppc64le/restic
chmod +x _output/bin/linux/ppc64le/restic

