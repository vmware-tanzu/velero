# Requirements for installing and running Velero

## Supported Kubernetes Versions

- In general, Velero works on Kubernetes version 1.7 or later (when Custom Resource Definitions were introduced).
- Restic support requires Kubernetes version 1.10 or later, or an earlier version with the mount propagation feature enabled. See [restic integration][1].

## Velero resource requirements

By default, the Velero deployment requests 500m CPU, 128Mi memory and sets a limit of 1000m CPU, 256Mi.
Default requests and limits are not set for the restic pods as CPU/Memory usage can depend heavily on the size of volumes being backed up.

If you need to customize these resource requests and limits, you can set the following flags in your `velero install` command:

```bash
velero install \
    --provider <YOUR_PROVIDER> \
    --bucket <YOUR_BUCKET> \
    --secret-file <PATH_TO_FILE> \
    --velero-pod-cpu-request <CPU_REQUEST> \
    --velero-pod-mem-request <MEMORY_REQUEST> \
    --velero-pod-cpu-limit <CPU_LIMIT> \
    --velero-pod-mem-limit <MEMORY_LIMIT> \
    [--use-restic] \
    [--restic-pod-cpu-request <CPU_REQUEST>] \
    [--restic-pod-mem-request <MEMORY_REQUEST>] \
    [--restic-pod-cpu-limit <CPU_LIMIT>] \
    [--restic-pod-mem-limit <MEMORY_LIMIT>]
```

Values for these flags follow the same format as [Kubernetes resource requirements][2].

[1]: restic.md
[2]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
