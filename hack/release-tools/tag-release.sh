#!/usr/bin/env bash

# This script will do the necessary checks and actions to create a release of Velero.
# It will first validate that all prerequisites are met, then verify the version string is what the user expects.
# A git tag will be created and pushed to GitHub, and GoReleaser will be invoked.

# This script is meant to be a combination of documentation and executable.
# If you have questions at any point, please stop and ask!

# Directory in which the script itself resides, so we can use it for calling programs that are in the same directory.
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

function tag_and_push() {
    echo "Tagging and pushing $VELERO_VERSION"
    git tag $VELERO_VERSION
    git push $VELERO_VERSION
}

# For now, have the person doing the release pass in the VELERO_VERSION variable as an environment variable.
# In the future, we might be able to inspect git via `git describe --abbrev=0` to get a hint for it.
if [[ -z "$VELERO_VERSION" ]]; then
    printf "The \$VELERO_VERSION environment variable is not set. Please set it with\n\texport VELERO_VERSION=v<version.to.release>\nthen try again."
    exit 1
fi

# Make sure the user's provided their github token, so we can give it to goreleaser.
if [[ -z "$GITHUB_TOKEN" ]]; then
    printf "The GITHUB_TOKEN environment variable is not set. Please set it with\n\t export GITHUB_TOKEN=<your github token>\n then try again."
    exit 1
fi

# Ensure that we have a clean working tree before we let any changes happen, especially important for cutting release branches.
#if [[ -n $(git status --short) ]]; then 
#    echo "Your git working directory is dirty! Please clean up untracked files and stash any changes before proceeding."
#    exit 3
#fi

# Make sure that there's no issue with the environment variable's format before trying to eval the parsed version.
if ! go run $DIR/chk_version.go --verify;  then
    exit 2
fi
# Since we're past the validation of the VELERO_VERSION, parse the version's individual components.
eval $(go run $DIR/chk_version.go)


printf "To clarify, you've provided a version string of $VELERO_VERSION.\n"
printf "Based on this, the following assumptions have been made: \n"

[[ "$VELERO_PATCH" != 0 ]] && printf "*\t This is a patch release.\n"

# -n is "string is non-empty"
[[ -n $VELERO_PRERELEASE ]] && printf "*\t This is a pre-release.\n"

# -z is "string is empty"
[[ -z $VELERO_PRERELEASE ]] && printf "*\t This is a GA release.\n"

echo "If this is all correct, press enter/return to proceed to TAG THE RELEASE and UPLOAD THE TAG TO GITHUB."
echo "Otherwise, press ctrl-c to CANCEL the process without making any changes."

read -p "Ready to continue? "

echo "Alright, let's go."

echo "Pulling down all git tags and branches before doing any work."
git fetch upstream --all --tags

# If we've got a patch release, we'll need to create a release branch for it.
if [[ "$VELERO_PATCH" > 0 ]]; then
    release_branch_name=release-$VELERO_MAJOR.$VELERO_MINOR
   
    # TODO: check to see if upstream exists.
    if [[ -z $(git branch | grep $release_branch_name) ]]; then
        git checkout -b $release_branch_name
        echo "Release branch made."
    else
        echo "Release branch $release_branch_name exists already."
    fi

    echo "Pushing $release_branch_name to upstream remote"
    git push --set-upstream upstream/$release_branch_name $release_branch_name
    
    tag_and_push
fi

echo "Checking out upstream/master."
git checkout upstream/master

tag_and_push


echo "Ivoking Goreleaser."
RELEASE_NOTES_FILE=changelogs/CHANGELOG-$VELERO_MAJOR.$VELERO_MINOR.md \
    PUBLISH=TRUE \
    make release


