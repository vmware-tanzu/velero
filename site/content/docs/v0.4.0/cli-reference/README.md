---
title: "Command line reference"
layout: docs
---

The Ark client provides a CLI that allows you to initiate ad-hoc backups, scheduled backups, or restores.

*The files in this directory enumerate each of the possible `ark` commands and their flags. Note that you can also find this info with the CLI itself, using the `--help` flag.*

## Running the client

While it is possible to build and run the `ark` executable yourself, it is recommended to use the containerized version. Use the alias described in the quickstart:

```
alias ark='docker run --rm -u $(id -u) -v $(dirname $KUBECONFIG):/kubeconfig -e KUBECONFIG=/kubeconfig/$(basename $KUBECONFIG) gcr.io/heptio-images/ark:latest'
```

Assuming that your `KUBECONFIG` variable is set, this alias takes care of specifying the appropriate Kubernetes cluster credentials for you.

## Kubernetes cluster credentials
In general, Ark will search for your cluster credentials in the following order:
* `--kubeconfig` command line flag
* `$KUBECONFIG` environment variable
* In-cluster credentials--this only works when you are running Ark in a pod
