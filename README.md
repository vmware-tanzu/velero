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
