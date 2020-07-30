# Image tagging policy

This document describes Velero's image tagging policy.

## Released versions

`gcr.io/heptio-images/velero:<SemVer>`

Velero follows the [Semantic Versioning](http://semver.org/) standard for releases. Each tag in the `github.com/heptio/velero` repository has a matching image, e.g. `gcr.io/heptio-images/velero:v0.11.0`.

### Latest

`gcr.io/heptio-images/velero:latest`

The `latest` tag follows the most recently released version of Velero.

## Development

`gcr.io/heptio-images/velero:main`

The `main` tag follows the latest commit to land on the `main` branch.
