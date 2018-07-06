# Command line reference

The Ark client provides a CLI that allows you to initiate ad-hoc backups, scheduled backups, or restores.

[The files in the CLI reference directory][1] in the repository enumerate each of the possible `ark` commands and their flags. 
This information is available in the CLI, using the `--help` flag.

## Running the client

We recommend that you [download a pre-built release][26], but you can also build and run the `ark` executable. 

## Kubernetes cluster credentials

In general, Ark will search for your cluster credentials in the following order:
* `--kubeconfig` command line flag
* `$KUBECONFIG` environment variable
* In-cluster credentials--this only works when you are running Ark in a pod

[1]: https://github.com/heptio/ark/tree/master/docs/cli-reference
[26]: https://github.com/heptio/ark/releases
