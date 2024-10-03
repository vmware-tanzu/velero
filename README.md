![100]

[![Build Status][1]][2] [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3811/badge)](https://bestpractices.coreinfrastructure.org/projects/3811)
![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/vmware-tanzu/velero)

## Overview

Velero (formerly Heptio Ark) gives you tools to back up and restore your Kubernetes cluster resources and persistent volumes. You can run Velero with a public cloud platform or on-premises. 

Velero lets you:

* Take backups of your cluster and restore in case of loss.
* Migrate cluster resources to other clusters.
* Replicate your production cluster to development and testing clusters.

Velero consists of:

* A server that runs on your cluster
* A command-line client that runs locally

## Documentation

[The documentation][29] provides a getting started guide and information about building from source, architecture, extending Velero and more.

Please use the version selector at the top of the site to ensure you are using the appropriate documentation for your version of Velero.

## Troubleshooting

If you encounter issues, review the [troubleshooting docs][30], [file an issue][4], or talk to us on the [#velero channel][25] on the Kubernetes Slack server.

## Contributing

If you are ready to jump in and test, add code, or help with documentation, follow the instructions on our [Start contributing][31] documentation for guidance on how to setup Velero for development.

## Changelog

See [the list of releases][6] to find out about feature changes.

### Velero compatibility matrix

The following is a list of the supported Kubernetes versions for each Velero version.

| Velero version | Expected Kubernetes version compatibility | Tested on Kubernetes version        |
|----------------|-------------------------------------------|-------------------------------------|
| 1.14           | 1.18-latest                               | 1.27.9, 1.28.9, and 1.29.4          |
| 1.13           | 1.18-latest                               | 1.26.5, 1.27.3, 1.27.8, and 1.28.3  |
| 1.12           | 1.18-latest                               | 1.25.7, 1.26.5, 1.26.7, and 1.27.3  |
| 1.11           | 1.18-latest                               | 1.23.10, 1.24.9, 1.25.5, and 1.26.1 |
| 1.10           | 1.18-latest                               | 1.22.5, 1.23.8, 1.24.6 and 1.25.1   |

Velero supports IPv4, IPv6, and dual stack environments. Support for this was tested against Velero v1.8.

The Velero maintainers are continuously working to expand testing coverage, but are not able to test every combination of Velero and supported Kubernetes versions for each Velero release. The table above is meant to track the current testing coverage and the expected supported Kubernetes versions for each Velero version.

If you are interested in using a different version of Kubernetes with a given Velero version, we'd recommend that you perform testing before installing or upgrading your environment. For full information around capabilities within a release, also see the Velero [release notes](https://github.com/vmware-tanzu/velero/releases) or Kubernetes [release notes](https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG). See the Velero [support page](https://velero.io/docs/latest/support-process/) for information about supported versions of Velero.

For each release, Velero maintainers run the test to ensure the upgrade path from n-2 minor release.  For example, before the release of v1.10.x, the test will verify that the backup created by v1.9.x and v1.8.x can be restored using the build to be tagged as v1.10.x.

[1]: https://github.com/vmware-tanzu/velero/workflows/Main%20CI/badge.svg
[2]: https://github.com/vmware-tanzu/velero/actions?query=workflow%3A"Main+CI"
[4]: https://github.com/vmware-tanzu/velero/issues
[6]: https://github.com/vmware-tanzu/velero/releases
[9]: https://kubernetes.io/docs/setup/
[10]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-with-homebrew-on-macos
[11]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#tabset-1
[12]: https://github.com/kubernetes/kubernetes/blob/master/cluster/addons/dns/README.md
[14]: https://github.com/kubernetes/kubernetes
[24]: https://groups.google.com/forum/#!forum/projectvelero
[25]: https://kubernetes.slack.com/messages/velero
[29]: https://velero.io/docs/
[30]: https://velero.io/docs/troubleshooting
[31]: https://velero.io/docs/start-contributing
[100]: https://velero.io/docs/main/img/velero.png
