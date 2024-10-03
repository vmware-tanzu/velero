---
title: "Use Velero with a storage provider secured by a self-signed certificate"
layout: docs
---

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

## Error with client certificate with custom S3 server

In case you are using a custom S3-compatible server, you may encounter that the backup fails with an error similar to one below.

```
rpc error: code = Unknown desc = RequestError: send request failed caused by:
Get https://minio.com:3000/k8s-backup-bucket?delimiter=%2F&list-type=2&prefix=: remote error: tls: alert(116)
```

Error 116 represents certificate required as seen here in [error codes](https://datatracker.ietf.org/doc/html/rfc8446#appendix-B.2).
Velero as a client does not include its certificate while performing SSL handshake with the server.
From [TLS 1.3 spec](https://tools.ietf.org/html/rfc8446), verifying client certificate is optional on the server.
You will need to change this setting on the server to make it work.
