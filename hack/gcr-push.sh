#!/bin/bash

set +x

if [[ -z "$TRAVIS" ]]; then
    echo "This script is intended to be run only on Travis." >&2
    exit 1
fi

# Return value is written into HIGHEST
HIGHEST=""
function highest_release() {
    # Loop through the tags since pre-release versions come before the actual versions.
    # Iterate til we find the first non-pre-release

    # This is not necessarily the most recently made tag; instead, we want it to be the highest semantic version.
    # The most recent tag could potentially be a lower semantic version, made as a point release for a previous series.
    # As an example, if v1.3.0 exists and we create v1.2.2, v1.3.0 should still be `latest`.
    # `git describe --tags $(git rev-list --tags --max-count=1)` would return the most recently made tag.

    for t in $(git tag -l --sort=-v:refname);
    do
        # If the tag has alpha, beta or rc in it, it's not "latest"
        if [[ "$t" == *"beta"* || "$t" == *"alpha"* || "$t" == *"rc"* ]]; then
            continue
        fi
        HIGHEST="$t"
        break
    done
}

if [ "$BRANCH" == "master" ]; then
    VERSION="$BRANCH"
elif [ ! -z "$TRAVIS_TAG" ]; then
    VERSION="$TRAVIS_TAG"
else
    # If we're not on master and we're not building a tag, exit early.
    exit 0
fi

# Calculate the latest release
highest_release

# Assume we're not tagging `latest` by default.
TAG_LATEST=false
if [[ "$TRAVIS_TAG" == "$HIGHEST" ]]; then
    TAG_LATEST=true
fi

echo "DIAGNOSTICS"
ls -al .
ls -al /home/travis/.docker
cat /home/travis/.docker/config.json
docker --version
gcloud --version
which gcloud
echo "END DIAGNOSTICS"


openssl aes-256-cbc -K $encrypted_f58ab4413c21_key -iv $encrypted_f58ab4413c21_iv -in nolanb-vmware-55ce4993acec.json.enc -out nolanb-vmware-55ce4993acec.json -d
gcloud auth activate-service-account --key-file nolanb-vmware-55ce4993acec.json
#openssl aes-256-cbc -K $encrypted_f58ab4413c21_key -iv $encrypted_f58ab4413c21_iv -in heptio-images-fac92d2303ac.json.enc -out heptio-images-fac92d2303ac.json -d
#gcloud auth activate-service-account --key-file heptio-images-fac92d2303ac.json
yes | gcloud auth configure-docker
unset GIT_HTTP_USER_AGENT


echo "Building and pushing container images."

VERSION="$VERSION" TAG_LATEST="$TAG_LATEST" make all-containers all-push
