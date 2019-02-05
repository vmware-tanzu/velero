#!/bin/bash

# Copyright 2018 the Heptio Ark contributors.
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

set -o nounset
set -o errexit
set -o pipefail

GIT_TAG=$(git describe --tags --always)

# this script copies all of the files under examples/ into a new directory,
# config/ (which is gitignored so it doesn't result in the git state being 
# dirty, which would prevent goreleaser from running), and then updates all 
# of the image tags in those files to use $GIT_TAG.

rm -rf config/ && cp -r examples/ config/

# the "-i'.bak'" flag to sed is necessary, with no space between the flag
# and the value, for this to be compatible across BSD/OSX sed and GNU sed.
# remove the ".bak" files afterwards (they're copies of the originals).
find config/ -type f -name "*.yaml" | xargs sed -i'.bak' "s|gcr.io/heptio-images/velero:latest|gcr.io/heptio-images/velero:$GIT_TAG|g"
find config/ -type f -name "*.bak" | xargs rm

find config/ -type f -name "*.yaml" | xargs sed -i'.bak' "s|gcr.io/heptio-images/fsfreeze-pause:latest|gcr.io/heptio-images/fsfreeze-pause:$GIT_TAG|g"
find config/ -type f -name "*.bak" | xargs rm
