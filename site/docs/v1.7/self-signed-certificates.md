# Use Velero with a storage provider secured by a self-signed certificate

If you are using an S3-Compatible storage provider that is secured with a self-signed certificate, connections to the object store may fail with a `certificate signed by unknown authority` message.
In order to proceed, a certificate bundle may be provided when adding the storage provider.

## Trusting a self-signed certificate during installation

When using the `velero install` command, you can use the `--cacert` flag to provide a path
to a PEM-encoded certificate bundle to trust.

```bash
velero install \
    --plugins <PLUGIN_CONTAINER_IMAGE [PLUGIN_CONTAINER_IMAGE]>
    --provider <YOUR_PROVIDER> \
    --bucket <YOUR_BUCKET> \
    --secret-file <PATH_TO_FILE> \
    --cacert <PATH_TO_CA_BUNDLE>
```

Velero will then automatically use the provided CA bundle to verify TLS connections to
that storage provider when backing up and restoring.

## Trusting a self-signed certificate with the Velero client

To use the describe, download, or logs commands to access a backup or restore contained
in storage secured by a self-signed certificate as in the above example, you must use
the `--cacert` flag to provide a path to the certificate to be trusted.

```bash
velero backup describe my-backup --cacert <PATH_TO_CA_BUNDLE>
```
