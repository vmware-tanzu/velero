---
title: "Velero Backup Storage Locations"
layout: docs
---

## Backup Storage Location

Velero can store backups in a number of locations. These are represented in the cluster via the `BackupStorageLocation` CRD.

Velero must have at least one `BackupStorageLocation`. By default, this is expected to be named `default`, however the name can be changed by specifying `--default-backup-storage-location` on `velero server`.  Backups that do not explicitly specify a storage location will be saved to this `BackupStorageLocation`.

A sample YAML `BackupStorageLocation` looks like the following:

```yaml
apiVersion: velero.io/v1
kind: BackupStorageLocation
metadata:
  name: default
  namespace: velero
spec:
  backupSyncPeriod: 2m0s
  provider: aws
  objectStorage:
    bucket: myBucket
  credential:
    name: secret-name
    key: key-in-secret
  config:
    region: us-west-2
    profile: "default"
```

### Parameter Reference

The configurable parameters are as follows:

#### Main config parameters

{{< table caption="Main config parameters" >}}
| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `provider` | String | Required Field | The name for whichever object storage provider will be used to store the backups. See [your object storage provider's plugin documentation](../supported-providers) for the appropriate value to use. |
| `objectStorage` | ObjectStorageLocation | Required Field | Specification of the object storage for the given provider. |
| `objectStorage/bucket` | String | Required Field | The storage bucket where backups are to be uploaded. |
| `objectStorage/prefix` | String | Optional Field | The directory inside a storage bucket where backups are to be uploaded. |
| `objectStorage/caCert` | String | Optional Field | A base64 encoded CA bundle to be used when verifying TLS connections |
| `config` | map[string]string | None (Optional) | Provider-specific configuration keys/values to be passed to the object store plugin. See [your object storage provider's plugin documentation](../supported-providers) for details. |
| `accessMode` | String | `ReadWrite` | How Velero can access the backup storage location. Valid values are `ReadWrite`, `ReadOnly`. |
| `backupSyncPeriod` | metav1.Duration | Optional Field | How frequently Velero should synchronize backups in object storage. Default is Velero's server backup sync period. Set this to `0s` to disable sync. |
| `validationFrequency` | metav1.Duration | Optional Field | How frequently Velero should validate the object storage . Default is Velero's server validation frequency. Set this to `0s` to disable validation. Default 1 minute. |
| `credential` | [corev1.SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#secretkeyselector-v1-core) | Optional Field | The credential information to be used with this location. |
| `credential/name` | String | Optional Field | The name of the secret within the Velero namespace which contains the credential information. |
| `credential/key` | String | Optional Field | The key to use within the secret. |
| `credential/key` | String | Optional Field | The key to use within the secret. |
| `encryption` | EncryptionSpec | Optional Field | EncryptionSpec holds configuration for a chosen encryption strategy. |
| `encryption/type` | Required Field | Encryption type to apply. Supported values: `none`, `age`. |
| `encryption/options` | map[string]string | Optional Field | Options contains strategy-specific key/value pairs. |
{{< /table >}}

### Encryption configuration

> **⚠️ Warning:** Encryption support is **experimental** and **NOT recommended** for production.

Client-side encryption allows Velero to encrypt backup data locally before sending it to the object store, providing an additional layer of security beyond server-side encryption. Two encryption types are supported:

* **none**: No client-side encryption (default). Options: none.
* **age**: Encryption using the [age](https://age-encryption.org/) tool. Options: recipient, privateKey.

Below is a brief overview of each encryption type and how to configure its options.

#### Type: none

No client-side encryption is performed. Omitting the `encryption` field or setting `type: none` results in standard behavior.

```yaml
spec:
  encryption:
    type: none
```

#### Type: age

`age` is a modern, simple, and secure encryption tool. When using `age`, Velero will encrypt objects with the specified recipient public key and decrypt them when restoring.

##### Fields

* `recipient` (string): The `age` recipient public key.
* `privateKey` (string): **Not recommended for production**. The literal private key value.

##### Private key format

An `age` private key typically looks like:

```
AGE-SECRET-KEY-14408X7C3QXCVPFR0XTSJZKM760LXMDH6HKDC4PYQMSAC74VXS6YQGPZZ0F
```

##### Example with Kubernetes secret and environment variables

In addition to mounting Kubernetes Secrets as volumes, you can also inject secret data directly into the Velero deployment as environment variables.

1. **Create a Kubernetes Secret**

```bash
kubectl create secret generic velero-encryption-secret \
  --namespace velero \
  --from-literal=AGE_RECIPIENT="age1jp7am0e5qzzw8ngtphw7rxwmltvzsw3emk4pa8avxm2vzl4xlfvqgc7puw" \
  --from-literal=AGE_PRIVATE_KEY="AGE-SECRET-KEY-14408X7C3QXCVPFR0XTSJZKM760LXMDH6HKDC4PYQMSAC74VXS6YQGPZZ0F"
```

2. **Update the Velero Deployment**

Edit the Velero Deployment to add the secret data as environment variables:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: velero
  namespace: velero
spec:
  template:
    spec:
      serviceAccountName: velero
      containers:
      - name: velero
        image: velero/velero:latest
        env:
        - name: AGE_RECIPIENT
          valueFrom:
            secretKeyRef:
              name: velero-encryption-secret
              key: AGE_RECIPIENT
        - name: AGE_PRIVATE_KEY
          valueFrom:
            secretKeyRef:
              name: velero-encryption-secret
              key: AGE_PRIVATE_KEY
        # ... other existing configuration ...
```

> **Note:** Values provided via environment variables (`AGE_RECIPIENT`, `AGE_PRIVATE_KEY`) take precedence over options specified directly in the BackupStorageLocation custom resource (`spec.encryption.options`). If a key is not set as an environment variable but is defined in the CR spec, Velero will use that value.

##### Direct private key (not recommended)

```yaml
spec:
  encryption:
    type: age
    options:
      recipient: age1q9s6f5z7x8v4y0d2j3k1l6m7n8o9p0q2r3s4t5
      privateKey: AGE-SECRET-KEY-14408X7C3QXCVPFR0XTSJZKM760LXMDH6HKDC4PYQMSAC74VXS6YQGPZZ0F
```
