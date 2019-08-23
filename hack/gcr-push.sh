#!/bin/sh

# Return value is written into LATEST
LATEST=""
function latest_release() {
# Loop through the tags since pre-release versions come before the actual versions.
# Iterate til we find the first non-pre-release
    for t in $(git tag -l --sort=-v:refname);
    do
        # If the tag has alpha, beta or rc in it, it's not "latest"
        if [[ "$t" == *"beta"* || "$t" == *"alpha"* || "$t" == *"rc"* ]]; then
            continue
        fi
        LATEST="$t"
        break
    done
}


# Always publish for master
if [ "$BRANCH" == "master" ]; then
    VERSION="$BRANCH" make all-containers all-push
fi

# Publish when TRAVIS_TAG is defined.
if [ ! -z "$TRAVIS_TAG" ]; then
    # Calculate the latest release
    latest_release

    # Default to pre-release
    TAG_LATEST=false
    if [[ "$TRAVIS_TAG" == "$LATEST" ]]; then
        TAG_LATEST=true
    fi

    VERSION="$TRAVIS_TAG" TAG_LATEST="$TAG_LATEST" make all-containers all-push
fi
