---
title: Releasing Velero plugins
layout: docs
toc: "true"
---

# Releasing Velero plugins

Velero plugins maintained by the core maintainers do not have any shipped binaries, only container images, so there is no need to invoke a GoReleaser script.
Container images are built via a CI job on git push.

1. Update the README.md file to update the compatibility matrix and `velero install` instructions with the expected version number and open a PR.
1. Once the updated README.md PR is merged, checkout the upstream `main` branch. `git checkout upstream/main`.
1. Tag the git version - `git tag v<version>`
1. Push the git tag - `git push --tags <upstream remote name>`
1. Wait for the container images to build
