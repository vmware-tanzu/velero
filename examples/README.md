# Examples

This directory contains sample YAML config files for running Velero on each core provider. Starting with v0.10, these files are packaged into [the Velero release tarballs][2], and we highly recommend that you use the packaged versions of these files to ensure compatibility with the released code. 

* `common/`: Contains manifests to set up Velero. Can be used across cloud provider platforms. (Note that Azure requires its own deployment file due to its unique way of loading credentials).

* `minio/`: Used in the [Quickstart][1] to set up [Minio][0], a local S3-compatible object storage service. It provides a convenient way to test Velero without tying you to a specific cloud provider.

* `aws/`, `azure/`, `gcp/`, `ibm/`: Contains manifests specific to the given cloud provider's setup.

[0]: https://github.com/minio/minio
[1]: /README.md#quickstart
[2]: https://github.com/heptio/velero/releases
