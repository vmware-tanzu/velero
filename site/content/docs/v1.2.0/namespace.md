---
title: "Run in custom namespace"
layout: docs
---

You can run Velero in any namespace.

First, ensure you've [downloaded & extracted the latest release][0].

Then, install Velero using the `--namespace` flag:

```bash
velero install --bucket <YOUR_BUCKET> --provider <YOUR_PROVIDER> --namespace <YOUR_NAMESPACE>
```



## Specify the namespace in client commands

To specify the namespace for all Velero client commands, run:

```bash
velero client config set namespace=<NAMESPACE_VALUE>
```

[0]: basic-install.md#install-the-cli
