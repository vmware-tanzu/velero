#!/bin/sh

# Return value is written into LATEST
LATEST=false
function is_latest_release() {
    # If the tag has alpha, beta or rc in it, it's not "latest"
    if [[ "$TRAVIS_TAG" == *"beta"* || "$TRAVIS_TAG" == *"alpha"* || "$TRAVIS_TAG" == *"rc"* ]]; then
        LATEST=false
    else
        LATEST=true
    fi
}

# Always publish for master
if [ "$BRANCH" == "master" ]; then
    VERSION="$BRANCH" make all-containers all-push
fi

# Publish when TRAVIS_TAG is defined.
if [ ! -z "$TRAVIS_TAG" ]; then
    # Check if this is the latest release.
    is_latest_release

    VERSION="$TRAVIS_TAG" TAG_LATEST="$LATEST" make all-containers all-push
fi
