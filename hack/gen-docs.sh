#!/bin/bash

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

# gen-docs.sh generates a new versioned docs directory under site/docs. It follows
# the following process:
#	1. Copies the contents of the most recently tagged docs directory into the new
#	   directory, to establish a useful baseline to diff against
#	2. Adds all copied content from step 1 to git's staging area via 'git add'.
#	3. Replaces the contents of the new docs directory with the contents of the
#	   'master' docs directory, updating any version-specific links (e.g. to a
#	   specific branch of the GitHub repository) to use the new version.
#
# The unstaged changes in the working directory can now be diff'ed against the
# staged changes using 'git diff' to review all docs changes made since the previous 
# tagged version. Once the unstaged changes are ready, they can be added to the
# staging area using 'git add' and then committed.

set -o errexit
set -o nounset
set -o pipefail

# don't run if there's already a directory for the target docs version
if [[ -d site/docs/$NEW_DOCS_VERSION ]]; then
    echo "ERROR: site/docs/$NEW_DOCS_VERSION already exists"
    exit 1
fi

# get the alphabetically last item in site/docs to use as PREVIOUS_DOCS_VERSION
# if not explicitly specified by the user
if [[ -z "${PREVIOUS_DOCS_VERSION:-}" ]]; then
    echo "PREVIOUS_DOCS_VERSION was not specified, getting the latest version"
    PREVIOUS_DOCS_VERSION=$(ls -1 site/docs/ | tail -n 1)
fi

# make a copy of the previous versioned docs
echo "Creating copy of docs directory site/docs/$PREVIOUS_DOCS_VERSION in site/docs/$NEW_DOCS_VERSION"
cp -r site/docs/${PREVIOUS_DOCS_VERSION}/ site/docs/${NEW_DOCS_VERSION}/

# 'git add' the previous version's docs as-is so we get a useful diff when we copy the master docs in
echo "Running 'git add' for previous version's doc contents to use as a base for diff"
git add site/docs/${NEW_DOCS_VERSION}


# now copy the contents of site/docs/master into the same directory so we can get a nice
# git diff of what changed since previous version
rm -rf site/docs/${NEW_DOCS_VERSION}/ && cp -r site/docs/master/ site/docs/${NEW_DOCS_VERSION}/

# replace known version-specific links -- the sed syntax is slightly different in OS X and Linux,
# so check which OS we're running on.
if [[ $(uname) == "Darwin" ]]; then
    echo "[OS X] updating version-specific links"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i '' "s|https://velero.io/docs/master|https://velero.io/docs/$NEW_DOCS_VERSION|g"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i '' "s|https://github.com/heptio/velero/blob/master|https://github.com/heptio/velero/blob/$NEW_DOCS_VERSION|g"
else
    echo "[Linux] updating version-specific links"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i'' "s|https://velero.io/docs/master|https://velero.io/docs/$NEW_DOCS_VERSION|g"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i'' "s|https://github.com/heptio/velero/blob/master|https://github.com/heptio/velero/blob/$NEW_DOCS_VERSION|g"
fi

echo "Success! site/docs/$NEW_DOCS_VERSION has been created. You can now run 'git diff' to review all docs changes made since the previous tagged version."
