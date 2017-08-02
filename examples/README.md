# Examples

The YAML config files in this directory can be used to quickly deploy a containerized Ark deployment.

* `common/`: Contains manifests to set up Ark. Can be used across cloud provider platforms. (Note that Azure requires its own deployment file due to its unique way of loading credentials).

* `minio/`: Used in the [Quickstart][1] to set up [Minio][0], a local S3-compatible object storage service. It provides a convenient way to test Ark without tying you to a specific cloud provider.

* `aws/`, `azure/`, `gcp/`: Contains manifests specific to the given cloud provider's setup.

[0]: https://github.com/minio/minio
[1]: /README.md#quickstart
