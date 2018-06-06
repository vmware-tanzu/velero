#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

# resolve the wildcards in the directories
RESTORE_DIR=$(cd /restores/$1/host_pods/*/volumes/*/$2 && echo $PWD)
VOLUME_DIR=$(cd /host_pods/$1/volumes/*/$2 && echo $PWD)

# the mv command fails when the source directory is empty,
# so check first.
if [ -n "$(ls -A $RESTORE_DIR)" ]; then
    mv "$RESTORE_DIR"/* $VOLUME_DIR/
fi

# cleanup
rm -rf "$RESTORE_DIR"

# write the done file for the init container to pick up
mkdir -p "$VOLUME_DIR"/.ark && touch "$VOLUME_DIR"/.ark/$3
