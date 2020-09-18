---
title: Releasing Velero plugins
layout: docs
toc: "true"
---

# Releasing Velero plugins

Velero plugins maintained by the core maintainers do not have any shipped binaries, only container images, so there is no need to invoke a GoReleaser script.
Container images are built via a CI job on git push.

1. Tag the git version - `git tag v<version>`
1. Push the git tag - `git push --tags <upstream remote name>`
1. Wait for the container images to build
