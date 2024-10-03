---
title: "Run in a non-default namespace"
layout: docs
---

The Velero installation and backups by default are run in the `velero` namespace. However, it is possible to use a different namespace.

### 1) Customize the namespace during install 

Use the `--namespace` flag, in conjunction with the other flags in the `velero install` command (as shown in the [the Velero install instructions][0]). This will inform Velero where to install.

### 2) Customize the namespace for operational commands

To have namespace consistency, specify the namespace for all Velero operational commands to be the same as the namespace used to install Velero:

```bash
velero client config set namespace=<NAMESPACE_VALUE>
```

Alternatively, you may use the global `--namespace` flag with any operational command to tell Velero where to run.

[0]: basic-install.md#install-the-cli
