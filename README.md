![100]

[![Build Status][1]][2] [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3811/badge)](https://bestpractices.coreinfrastructure.org/projects/3811)


## Overview

Velero (formerly Heptio Ark) gives you tools to back up and restore your Kubernetes cluster resources and persistent volumes. You can run Velero with a public cloud platform or on-premises. Velero lets you:

* Take backups of your cluster and restore in case of loss.
* Migrate cluster resources to other clusters.
* Replicate your production cluster to development and testing clusters.

Velero consists of:

* A server that runs on your cluster
* A command-line client that runs locally

## Documentation

[The documentation][29] provides a getting started guide and information about building from source, architecture, extending Velero, and more.

Please use the version selector at the top of the site to ensure you are using the appropriate documentation for your version of Velero.

## Troubleshooting

If you encounter issues, review the [troubleshooting docs][30], [file an issue][4], or talk to us on the [#velero channel][25] on the Kubernetes Slack server.

## Contributing

If you are ready to jump in and test, add code, or help with documentation, follow the instructions on our [Start contributing][31] documentation for guidance on how to setup Velero for development.

## Changelog

See [the list of releases][6] to find out about feature changes.

### Velero compatibility matrix

The following is a list of the supported Kubernetes versions for each Velero version.

| Velero version | Expected Kubernetes version compatibility| Tested on Kubernetes version|
|----------------|--------------------|--------------------|
| 1.10           | 1.18-latest        |  1.22.5, 1.23.8, 1.24.6 and 1.25.1 |
| 1.9            | 1.18-latest        |  1.20.5, 1.21.2, 1.22.5, 1.23, and 1.24 |
| 1.8            | 1.18-latest        |  |
| 1.6.3-1.7.1    | 1.12-latest        ||
| 1.60-1.6.2     | 1.12-1.21          ||
| 1.5            | 1.12-1.21          ||
| 1.4            | 1.10-1.21          | |

Velero supports IPv4, IPv6, and dual stack environments. Support for this was tested against Velero v1.8.

The Velero maintainers are continuously working to expand testing coverage, but are not able to test every combination of Velero and supported Kubernetes versions for each Velero release. The table above is meant to track the current testing coverage and the expected supported Kubernetes versions for each Velero version. If you have a question about test coverage before v1.9, please reach out in the [#velero-users](https://kubernetes.slack.com/archives/C6VCGP4MT) Slack channel.

If you are interested in using a different version of Kubernetes with a given Velero version, we'd recommend that you perform testing before installing or upgrading your environment. For full information around capabilities within a release, also see the Velero [release notes](https://github.com/vmware-tanzu/velero/releases) or Kubernetes [release notes](https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG). See the Velero [support page](https://velero.io/docs/latest/support-process/) for information about supported versions of Velero.

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
