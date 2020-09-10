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


set +x

if [[ -z "$CI" ]]; then
   echo "This script is intended to be run only on Github Actions." >&2
   exit 1
fi

CHANGELOG_PATH='changelogs/unreleased'

# https://help.github.com/en/actions/reference/events-that-trigger-workflows#pull-request-event-pull_request
# GITHUB_REF is something like "refs/pull/:prNumber/merge"
pr_number=$(echo $GITHUB_REF | cut -d / -f 3)

change_log_file="${CHANGELOG_PATH}/${pr_number}-*"

if ls ${change_log_file} 1> /dev/null 2>&1; then
    echo "changelog for PR ${pr_number} exists"
    exit 0
else
    echo "PR ${pr_number} is missing a changelog. Please refer https://velero.io/docs/main/code-standards/#adding-a-changelog and add a changelog."
    exit 1
fi

