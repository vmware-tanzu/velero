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
#	   directory, to establish a useful baseline to diff against.
#	2. Adds all copied content from step 1 to git's staging area via 'git add'.
#	3. Replaces the contents of the new docs directory with the contents of the
#	   'master' docs directory, updating any version-specific links (e.g. to a
#	   specific branch of the GitHub repository) to use the new version
#   4. Copies the previous version's ToC file and runs 'git add' to establish
#      a useful baseline to diff against.
#   5. Replaces the content of the new ToC file with the master ToC.
#   6. Update site/_config.yml and site/_data/toc-mapping.yml to include entries
#      for the new version.
#
# The unstaged changes in the working directory can now easily be diff'ed against the
# staged changes using 'git diff' to review all docs changes made since the previous 
# tagged version. Once the unstaged changes are ready, they can be added to the
# staging area using 'git add' and then committed.
#
# **NOTE**: there are additional manual steps required to finalize the process of generating
# a new versioned docs site. The full process is documented in site/README-JEKYLL.md.

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

# make a copy of the previous versioned docs dir
echo "Creating copy of docs directory site/docs/$PREVIOUS_DOCS_VERSION in site/docs/$NEW_DOCS_VERSION"
cp -r site/docs/${PREVIOUS_DOCS_VERSION}/ site/docs/${NEW_DOCS_VERSION}/

# 'git add' the previous version's docs as-is so we get a useful diff when we copy the master docs in
echo "Running 'git add' for previous version's doc contents to use as a base for diff"
git add site/docs/${NEW_DOCS_VERSION}

# now copy the contents of site/docs/master into the same directory so we can get a nice
# git diff of what changed since previous version
echo "Copying site/docs/master/ to site/docs/${NEW_DOCS_VERSION}/"
rm -rf site/docs/${NEW_DOCS_VERSION}/ && cp -r site/docs/master/ site/docs/${NEW_DOCS_VERSION}/

# make a copy of the previous versioned ToC
NEW_DOCS_TOC="$(echo ${NEW_DOCS_VERSION} | tr . -)-toc"
PREVIOUS_DOCS_TOC="$(echo ${PREVIOUS_DOCS_VERSION} | tr . -)-toc"

echo "Creating copy of site/_data/$PREVIOUS_DOCS_TOC.yml at site/_data/$NEW_DOCS_TOC.yml"
cp site/_data/$PREVIOUS_DOCS_TOC.yml site/_data/$NEW_DOCS_TOC.yml

# 'git add' the previous version's ToC content as-is so we get a useful diff when we copy the master ToC in
echo "Running 'git add' for previous version's ToC to use as a base for diff"
git add site/_data/$NEW_DOCS_TOC.yml

# now copy the master ToC so we can get a nice git diff of what changed since previous version
echo "Copying site/_data/master-toc.yml to site/_data/$NEW_DOCS_TOC.yml"
rm site/_data/$NEW_DOCS_TOC.yml && cp site/_data/master-toc.yml site/_data/$NEW_DOCS_TOC.yml

# replace known version-specific links -- the sed syntax is slightly different in OS X and Linux,
# so check which OS we're running on.
if [[ $(uname) == "Darwin" ]]; then
    echo "[OS X] updating version-specific links"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i '' "s|https://velero.io/docs/master|https://velero.io/docs/$NEW_DOCS_VERSION|g"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i '' "s|https://github.com/heptio/velero/blob/master|https://github.com/heptio/velero/blob/$NEW_DOCS_VERSION|g"

    echo "[OS X] Updating latest version in _config.yml"
    sed -i '' "s/latest: ${PREVIOUS_DOCS_VERSION}/latest: ${NEW_DOCS_VERSION}/" site/_config.yml

    # newlines and lack of indentation are requirements for this sed syntax
    # which is doing an append
    echo "[OS X] Adding latest version to versions list in _config.yml"
    sed -i '' "/- master/a\\
- ${NEW_DOCS_VERSION}
" site/_config.yml

    echo "[OS X] Adding ToC mapping entry"
    sed -i '' "/master: master-toc/a\\
${NEW_DOCS_VERSION}: ${NEW_DOCS_TOC}
" site/_data/toc-mapping.yml

else
    echo "[Linux] updating version-specific links"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i'' "s|https://velero.io/docs/master|https://velero.io/docs/$NEW_DOCS_VERSION|g"
    find site/docs/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i'' "s|https://github.com/heptio/velero/blob/master|https://github.com/heptio/velero/blob/$NEW_DOCS_VERSION|g"

    echo "[Linux] Updating latest version in _config.yml"
    sed -i'' "s/latest: ${PREVIOUS_DOCS_VERSION}/latest: ${NEW_DOCS_VERSION}/" site/_config.yml
    
    echo "[Linux] Adding latest version to versions list in _config.yml"
    sed -i'' "/- master/a - ${NEW_DOCS_VERSION}" site/_config.yml
    
    echo "[Linux] Adding ToC mapping entry"
    sed -i'' "/master: master-toc/a ${NEW_DOCS_VERSION}: ${NEW_DOCS_TOC}" site/_data/toc-mapping.yml
fi

echo "Success! site/docs/$NEW_DOCS_VERSION has been created."
echo ""
echo "The next steps are:"
echo "  1. Consult site/README-JEKYLL.md for further manual steps required to finalize the new versioned docs generation."
echo "  2. Run a 'git diff' to review all changes made to the docs since the previous version."
echo "  3. Make any manual changes/corrections necessary."
echo "  4. Run 'git add' to stage all unstaged changes, then 'git commit'."
