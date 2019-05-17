![100]

[![Build Status][1]][2]  

## Overview

Velero (formerly Heptio Ark) gives you tools to back up and restore your Kubernetes cluster resources and persistent volumes. Velero lets you:

* Take backups of your cluster and restore in case of loss.
* Copy cluster resources to other clusters.
* Replicate your production environment for development and testing environments.

Velero consists of:

* A server that runs on your cluster
* A command-line client that runs locally

You can run Velero in clusters on a cloud provider or on-premises. For detailed information, see [Compatible Storage Providers][99].

## Installation

We strongly recommend that you use an [official release][6] of Velero. The tarballs for each release contain the
command-line client **and** version-specific sample YAML files for deploying Velero to your cluster.
Follow the instructions under the **Install** section of [our documentation][29] to get started.

_The code and sample YAML files in the master branch of the Velero repository are under active development and are not guaranteed to be stable. Use them at your own risk!_

## More information

[The documentation][29] provides a getting started guide, plus information about building from source, architecture, extending Velero, and more.

Please use the version selector at the top of the site to ensure you are using the appropriate documentation for your version of Velero.

## Troubleshooting

If you encounter issues, review the [troubleshooting docs][30], [file an issue][4], or talk to us on the [#velero channel][25] on the Kubernetes Slack server.

## Contributing

Thanks for taking the time to join our community and start contributing!

Feedback and discussion are available on [the mailing list][24].

### Before you start

* Please familiarize yourself with the [Code of Conduct][8] before contributing.
* See [CONTRIBUTING.md][5] for instructions on the developer certificate of origin that we require.
* Read how [we're using ZenHub][26] for project and roadmap planning

### Pull requests

* We welcome pull requests. Feel free to dig through the [issues][4] and jump in.

## Changelog

See [the list of releases][6] to find out about feature changes.

[1]: https://travis-ci.org/heptio/velero.svg?branch=master
[2]: https://travis-ci.org/heptio/velero

[4]: https://github.com/heptio/velero/issues
[5]: https://github.com/heptio/velero/blob/master/CONTRIBUTING.md
[6]: https://github.com/heptio/velero/releases

[8]: https://github.com/heptio/velero/blob/master/CODE_OF_CONDUCT.md
[9]: https://kubernetes.io/docs/setup/
[10]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-with-homebrew-on-macos
[11]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#tabset-1
[12]: https://github.com/kubernetes/kubernetes/blob/master/cluster/addons/dns/README.md
[14]: https://github.com/kubernetes/kubernetes

[24]: https://groups.google.com/forum/#!forum/projectvelero
[25]: https://kubernetes.slack.com/messages/velero
[26]: https://github.com/heptio/velero/blob/master/docs/zenhub.md


[29]: https://velero.io/docs/v0.11.0/install-overview
[30]: /troubleshooting.md

[99]: /support-matrix.md
[100]: /img/velero.png
