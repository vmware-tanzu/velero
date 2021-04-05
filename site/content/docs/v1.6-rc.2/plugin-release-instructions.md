---
title: Releasing Velero plugins
layout: docs
toc: "true"
---

Velero plugins maintained by the core maintainers do not have any shipped binaries, only container images, so there is no need to invoke a GoReleaser script.
Container images are built via a CI job on git push.

Plugins the Velero core team is responsible include all those listed in [the Velero-supported providers list](supported-providers.md) _except_ the vSphere plugin.

1. Update the README.md file to update the compatibility matrix and `velero install` instructions with the expected version number and open a PR.
  1. Determining the version number is based on semantic versioning and whether the plugin uses any newly introduced, changed, or removed methods or variables from Velero.
1. Once the updated README.md PR is merged, checkout the upstream `main` branch. `git checkout upstream/main`.
1. Tag the git version - `git tag v<version>`.
1. Push the git tag - `git push --tags <upstream remote name>`
1. Wait for the container images to build
