#!/bin/bash
if [[ $# -ne 1 ]]; then
  echo "usage: ${BASH_SOURCE} [VERSION]"
  exit 1
fi

VERSION=$1

if [[ $VERSION != "master" ]]; then
  GH_BASE_URL=https://github.com/heptio/ark/tags/$VERSION
else
  GH_BASE_URL=https://github.com/heptio/ark/branches/master
fi

svn export $GH_BASE_URL/docs/ $VERSION/ || (echo "Failed to copy docs for version $VERSION" && exit -1)
svn export --force $GH_BASE_URL/README.md $VERSION/README.md || (echo "Failed to copy README for version $VERSION" && exit -1)
