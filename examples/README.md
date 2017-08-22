# Examples

This directory contains two sub-directories:
* `/ksonnet` - The [ksonnet][0] files in this directory can be used to autogenerate your YAML manifests with the `make generate-examples` command.

* `/yaml` - The YAML manifests in this directory can be used to quickly deploy a containerized Ark deployment.

## /ksonnet

Ark uses the [official ksonnet Docker image][1] (which comes with a [`kubecfg`][2] executable) to compile `*.jsonnet` files into YAML. Because ksonnet can read in external variables, you can easily generate YAML manifests that work your setup, *regardless of its particulars* (e.g. cloud providers, RBAC settings, etc.). For detailed instructions, see the [guide for Cloud Provider Specifics][3].

This sub-directory is itself broken down into:
* `00-prereqs.jsonnet` - This YAML file sets up the Heptio Ark namespace and service account.

* `10-ark.jsonnet` - Other than the prereq components, this YAML file contains everything that you need to run Ark.

* `20-nginx-example.jsonnet` - This YAML file runs an Nginx server, which can be used to demonstrate Ark's backup and restore capabilities.

* `/components` - These files contain most of the ksonnet logic--they determine which Kubernetes objects are created.

* `/conf` - These files provide the raw data to define objects in `/components.` They that should be pretty consistent across Ark deployments.

Each of the files in `/conf` and `/components` covers a fairly distinct part of the values that you can configure. They're separated out to provide a more human-friendly way of viewing how Ark's constituent resources are set up.

## /yaml

This subdirectory contains both pre-created and auto-generated YAML manifests.

* `generated/`: As described above, the `*.yaml` files here are compiled from the files in `../ksonnet/` by running `make generate-examples`. `env.txt` contains a recording of the Heptio Ark-related environment variable at the time of generation.

* `quickstart/`: Used in the [Quickstart][4] to set up [Minio][5], a local S3-compatible object storage service. It provides a convenient way to test Ark without tying you to a specific cloud provider.

[0]: http://ksonnet.heptio.com
[1]: https://hub.docker.com/r/ksonnet/ksonnet-lib/
[2]: https://github.com/ksonnet/kubecfg
[3]: /docs/cloud-provider-specifics.md
[4]: /README.md#quickstart
[5]: https://github.com/minio/minio
