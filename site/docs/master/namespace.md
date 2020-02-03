# Run in a custom namespace

The Velero installation and backups by default are run in the `velero` namespace. However, it is possible to customize it.

### 1) Customize the namespace on the cluster 

During [the Velero installation][0], use the `--namespace` flag to inform Velero where to run.

```bash
velero install --bucket <YOUR_BUCKET> --provider <YOUR_PROVIDER> --namespace <YOUR_NAMESPACE>
```

### 2) Customize the namespace for operational commands

To have namespace consistency, specify the namespace for all Velero operational commands same as the namespace used to install Velero:

```bash
velero client config set namespace=<NAMESPACE_VALUE>
```

Alternatively, you may use the glogal `--namespace` flag with any operational command to tell Velero where to run.

[0]: basic-install.md#install-the-cli
