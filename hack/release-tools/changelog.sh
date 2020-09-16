#!/bin/bash

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

set -o errexit
set -o nounset
set -o pipefail

function join { local IFS="$1"; shift; echo "$*"; }

CHANGELOG_PATH='changelogs/unreleased'
UNRELEASED=$(ls -t ${CHANGELOG_PATH})
echo -e "Generating CHANGELOG markdown from ${CHANGELOG_PATH}\n"
for entry in $UNRELEASED
do
    IFS=$'-' read -ra pruser <<<"$entry"
    contents=$(cat ${CHANGELOG_PATH}/${entry})
    pr=${pruser[0]}
    user=$(join '-' ${pruser[@]:1})
    echo "  * ${contents} (#${pr}, @${user})"
done
echo -e "\nCopy and paste the list above in to the appropriate CHANGELOG file."
echo "Be sure to run: git rm ${CHANGELOG_PATH}/*"
