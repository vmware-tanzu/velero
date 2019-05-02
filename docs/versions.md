# Upgrading Velero versions

Velero supports multiple concurrent versions. Whether you're setting up Velero for the first time or upgrading to a new version, you need to pay careful attention to versioning.

## Minor versions, patch versions

The documentation site provides docs for minor versions only, not for patch releases. Patch releases are guaranteed not to be breaking, but you should carefully read the [release notes][1] to make sure that you understand any relevant changes.

If you're upgrading from a patch version to a patch version, you only need to update the image tags in your configurations. No other steps are needed.

Breaking changes are documented in the release notes and in the documentation.

## Upgrading to 1.0 from an earlier version

- See [Upgrading to 1.0][2]

[1]: https://github.com/heptio/velero/releases
[2]: upgrade-to-1.0.md
