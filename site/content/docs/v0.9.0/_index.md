# Heptio Ark

**Maintainers:** [Heptio][0]

[![Build Status][1]][2]

## Overview

Ark gives you tools to back up and restore your Kubernetes cluster resources and persistent volumes. Ark lets you:

* Take backups of your cluster and restore in case of loss.
* Copy cluster resources across cloud providers. NOTE: Cloud volume migrations are not yet supported.
* Replicate your production environment for development and testing environments.

Ark consists of:

* A server that runs on your cluster
* A command-line client that runs locally

## More information

[The documentation][29] provides a getting started guide, plus information about building from source, architecture, extending Ark, and more.

## Troubleshooting

If you encounter issues, review the [troubleshooting docs][30], [file an issue][4], or talk to us on the [#ark-dr channel][25] on the Kubernetes Slack server. 

## Contributing

Thanks for taking the time to join our community and start contributing!

Feedback and discussion is available on [the mailing list][24].

### Before you start

* Please familiarize yourself with the [Code of Conduct][8] before contributing.
* See [CONTRIBUTING.md][5] for instructions on the developer certificate of origin that we require.

### Pull requests

* We welcome pull requests. Feel free to dig through the [issues][4] and jump in.

## Changelog

See [the list of releases][6] to find out about feature changes.

[0]: https://github.com/heptio
[1]: https://travis-ci.org/heptio/ark.svg?branch=main
[2]: https://travis-ci.org/heptio/ark

[4]: https://github.com/heptio/ark/issues
[5]: https://github.com/heptio/ark/blob/main/CONTRIBUTING.md
[6]: https://github.com/heptio/ark/releases

[8]: https://github.com/heptio/ark/blob/main/CODE_OF_CONDUCT.md
[9]: https://kubernetes.io/docs/setup/

[11]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#tabset-1
[12]: https://github.com/kubernetes/kubernetes/blob/main/cluster/addons/dns/README.md
[14]: https://github.com/kubernetes/kubernetes


[24]: http://j.hept.io/ark-list
[25]: https://kubernetes.slack.com/messages/ark-dr


[29]: https://velero.io/docs/v0.9.0/
[30]: /troubleshooting.md
