# Upgrading Velero versions

Velero supports multiple concurrent versions. Whether you're setting up Velero for the first time or upgrading to a new version, you need to pay careful attention to versioning. This doc page is new as of version 0.10.0, and will be updated with information about subsequent releases.

## Minor versions, patch versions

The documentation site provides docs for minor versions only, not for patch releases. Patch releases are guaranteed not to be breaking, but you should carefully read the [release notes][1] to make sure that you understand any relevant changes.

If you're upgrading from a patch version to a patch version, you only need to update the image tags in your configurations. No other steps are needed.

Breaking changes are documented in the release notes and in the documentation.

## Breaking changes for version 0.10.0

- See [Upgrading to version 0.10.0][2]

## Velero versions and Kubernetes versions

Not all Velero versions support all versions of Kubernetes. You should be aware of the following known limitations:

- Velero version 0.9.0 requires Kubernetes version 1.8 or later. In version 0.9.1, Velero was updated to support earlier versions.
- Restic support requires Kubernetes version 1.10 or later, or an earlier version with the mount propagation feature enabled. See [Restic Integration][3].

[1]: https://github.com/heptio/velero/releases
[2]: https://velero.io/docs/v0.10.0/upgrading-to-v0.10
[3]: restic.md
