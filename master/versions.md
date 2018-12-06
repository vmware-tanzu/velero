# Upgrading Ark versions

Ark supports multiple concurrent versions. Whether you're setting up Ark for the first time or upgrading to a new version, you need to pay careful attention to versioning. This doc page is new as of version 0.10.0, and will be updated with information about subsequent releases.

## Minor versions, patch versions

The documentation site provides docs for minor versions only, not for patch releases. Patch releases are guaranteed not to be breaking, but you should carefully read the [release notes][1] to make sure that you understand any relevant changes.

If you're upgrading from a patch version to a patch version, you only need to update the image tags in your configurations. No other steps are needed.

Breaking changes are documented in the release notes and in the documentation.

## Breaking changes for version 0.10.0

- See [Upgrading to version 0.10.0][2]

## Ark versions and Kubernetes versions

Not all Ark versions support all versions of Kubernetes. You should be aware of the following known limitations:

- Ark version 0.9.0 requires Kubernetes version 1.8 or later. In version 0.9.1, Ark was updated to support earlier versions.
- Restic support requires Kubernetes version 1.10 or later, or an earlier version with the mount propagation feature enabled. See [Restic Integration][3].

[1]: https://github.com/heptio/ark/releases
[2]: upgrading-to-v0.10.md
[3]: restic.md
