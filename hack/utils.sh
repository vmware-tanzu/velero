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

# get_repo_root returns the root path of the project source tree
get_repo_root() {
    git rev-parse --show-toplevel
}

# cd_repo_root cds to the root path of the project source tree
cd_repo_root() {
    cd "$(get_root_path)" || exit
}
