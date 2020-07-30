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

# gen-docs.sh is used for the "make gen-docs" target. See additional
# documentation in the Makefile.

set -o errexit
set -o nounset
set -o pipefail

DOCS_DIRECTORY=site/content/docs
DATA_DOCS_DIRECTORY=site/data/docs
CONFIG_FILE=site/config.yaml

# don't run if there's already a directory for the target docs version
if [[ -d $DOCS_DIRECTORY/$NEW_DOCS_VERSION ]]; then
    echo "ERROR: $DOCS_DIRECTORY/$NEW_DOCS_VERSION already exists"
    exit 1
fi

# get the alphabetically last item in $DOCS_DIRECTORY to use as PREVIOUS_DOCS_VERSION
# if not explicitly specified by the user
if [[ -z "${PREVIOUS_DOCS_VERSION:-}" ]]; then
    echo "PREVIOUS_DOCS_VERSION was not specified, getting the latest version"
    PREVIOUS_DOCS_VERSION=$(ls -1 $DOCS_DIRECTORY/ | tail -n 1)
fi

# make a copy of the previous versioned docs dir
echo "Creating copy of docs directory $DOCS_DIRECTORY/$PREVIOUS_DOCS_VERSION in $DOCS_DIRECTORY/$NEW_DOCS_VERSION"
cp -r $DOCS_DIRECTORY/${PREVIOUS_DOCS_VERSION}/ $DOCS_DIRECTORY/${NEW_DOCS_VERSION}/

# 'git add' the previous version's docs as-is so we get a useful diff when we copy the master docs in
echo "Running 'git add' for previous version's doc contents to use as a base for diff"
git add -f $DOCS_DIRECTORY/${NEW_DOCS_VERSION}

# now copy the contents of $DOCS_DIRECTORY/master into the same directory so we can get a nice
# git diff of what changed since previous version
echo "Copying $DOCS_DIRECTORY/master/ to $DOCS_DIRECTORY/${NEW_DOCS_VERSION}/"
rm -rf $DOCS_DIRECTORY/${NEW_DOCS_VERSION}/ && cp -r $DOCS_DIRECTORY/master/ $DOCS_DIRECTORY/${NEW_DOCS_VERSION}/

# make a copy of the previous versioned ToC
NEW_DOCS_TOC="$(echo ${NEW_DOCS_VERSION} | tr . -)-toc"
PREVIOUS_DOCS_TOC="$(echo ${PREVIOUS_DOCS_VERSION} | tr . -)-toc"

echo "Creating copy of $DATA_DOCS_DIRECTORY/$PREVIOUS_DOCS_TOC.yml at $DATA_DOCS_DIRECTORY/$NEW_DOCS_TOC.yml"
cp $DATA_DOCS_DIRECTORY/$PREVIOUS_DOCS_TOC.yml $DATA_DOCS_DIRECTORY/$NEW_DOCS_TOC.yml

# 'git add' the previous version's ToC content as-is so we get a useful diff when we copy the master ToC in
echo "Running 'git add' for previous version's ToC to use as a base for diff"
git add $DATA_DOCS_DIRECTORY/$NEW_DOCS_TOC.yml

# now copy the master ToC so we can get a nice git diff of what changed since previous version
echo "Copying $DATA_DOCS_DIRECTORY/master-toc.yml to $DATA_DOCS_DIRECTORY/$NEW_DOCS_TOC.yml"
rm $DATA_DOCS_DIRECTORY/$NEW_DOCS_TOC.yml && cp $DATA_DOCS_DIRECTORY/master-toc.yml $DATA_DOCS_DIRECTORY/$NEW_DOCS_TOC.yml

# replace known version-specific links -- the sed syntax is slightly different in OS X and Linux,
# so check which OS we're running on.
if [[ $(uname) == "Darwin" ]]; then
    echo "[OS X] updating version-specific links"
    find $DOCS_DIRECTORY/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i '' "s|https://velero.io/docs/master|https://velero.io/docs/$VELERO_VERSION|g"
    find $DOCS_DIRECTORY/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i '' "s|https://github.com/vmware-tanzu/velero/blob/master|https://github.com/vmware-tanzu/velero/blob/$VELERO_VERSION|g"

    echo "[OS X] Updating latest version in $CONFIG_FILE"
    sed -i '' "s/latest: ${PREVIOUS_DOCS_VERSION}/latest: ${NEW_DOCS_VERSION}/" $CONFIG_FILE

    # newlines and lack of indentation are requirements for this sed syntax
    # which is doing an append
    echo "[OS X] Adding latest version to versions list in $CONFIG_FILE"
    sed -i '' "/- master/a\\
\ \ \ \ - ${NEW_DOCS_VERSION}
" $CONFIG_FILE

    echo "[OS X] Adding ToC mapping entry"
    sed -i '' "/master: master-toc/a\\
${NEW_DOCS_VERSION}: ${NEW_DOCS_TOC}
" $DATA_DOCS_DIRECTORY/toc-mapping.yml

else
    echo "[Linux] updating version-specific links"
    find $DOCS_DIRECTORY/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i'' "s|https://velero.io/docs/master|https://velero.io/docs/$VELERO_VERSION|g"
    find $DOCS_DIRECTORY/${NEW_DOCS_VERSION} -type f -name "*.md" | xargs sed -i'' "s|https://github.com/vmware-tanzu/velero/blob/master|https://github.com/vmware-tanzu/velero/blob/$VELERO_VERSION|g"

    echo "[Linux] Updating latest version in $CONFIG_FILE"
    sed -i'' "s/latest: ${PREVIOUS_DOCS_VERSION}/latest: ${NEW_DOCS_VERSION}/" $CONFIG_FILE
    
    echo "[Linux] Adding latest version to versions list in $CONFIG_FILE"
    sed -i'' "/- master/a - ${NEW_DOCS_VERSION}" $CONFIG_FILE
    
    echo "[Linux] Adding ToC mapping entry"
    sed -i'' "/master: master-toc/a ${NEW_DOCS_VERSION}: ${NEW_DOCS_TOC}" $DATA_DOCS_DIRECTORY/toc-mapping.yml
fi

echo "Success! $DOCS_DIRECTORY/$NEW_DOCS_VERSION has been created."
echo ""
echo "The next steps are:"
echo "  1. Consult site/README-HUGO.md for further manual steps required to finalize the new versioned docs generation."
echo "  2. Run a 'git diff' to review all changes made to the docs since the previous version."
echo "  3. Make any manual changes/corrections necessary."
echo "  4. Run 'git add' to stage all unstaged changes, then 'git commit'."
