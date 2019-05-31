#!/usr/bin/env bash

# If we're doing push build, as opposed to a PR, always run make ci
if [ "$TRAVIS_PULL_REQUEST" == "false" ]; then
    make ci
    # Exit script early, returning make ci's error
    exit $?
fi

# Only run `make ci` if files outside of the site directory changed in the branch
# In a PR build, $TRAVIS_BRANCH is the destination branch.
if [[ $(git diff --name-only $TRAVIS_BRANCH | grep --invert-match site/) ]]; then
  make ci
else
  echo "Skipping make ci since nothing outside of site directory changed."
  exit 0
fi
