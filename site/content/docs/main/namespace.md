---
title: "Run in a non-default namespace"
layout: docs
---

The Velero installation and backups by default are run in the `velero` namespace. However, it is possible to use a different namespace.

## Customize the namespace during install

Use the `--namespace` flag, in conjunction with the other flags in the `velero install` command (as shown in the [the Velero install instructions][0]). This will inform Velero where to install.

## Customize the namespace for operational commands

To have namespace consistency, specify the namespace for all Velero operational commands to be the same as the namespace used to install Velero:

```bash
velero client config set namespace=<NAMESPACE_VALUE>
```

Alternatively, you may use the global `--namespace` flag with any operational command to tell Velero where to run.

## Customize the targeted namespace

In the YAML deployment file using Velero's image, define the environment variable NAMESPACE_NAME to specify the namespace in which you want Velero to backup resources.

[0]: basic-install.md#install-the-cli
