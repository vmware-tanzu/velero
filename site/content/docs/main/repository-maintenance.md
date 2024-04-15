---
title: "Repository Maintenance"
layout: docs
---

From v1.14 on, Velero decouples repository maintenance from the Velero server by launching a k8s job to do maintenance when needed, to mitigate the impact on the Velero server during backups.

Before v1.14.0, Velero performs periodic maintenance on the repository within Velero server pod, this operation may consume significant CPU and memory resources in some cases, leading to Velero server being killed by OOM. Now Velero will launch independent k8s jobs to do the maintenance in Velero installation namespace.

For repository maintenance jobs, there's no limit on resources by default. You could configure the job resource limitation based on target data to be backed up.

## Settings
### Resource Limitation
You can customize the maintenance job resource requests and limit when using the [velero install][1] CLI command.

### Log
Maintenance job inherits the log level and log format settings from the Velero server, so if the Velero server enabled the debug log, the maintenance job will also open the debug level log.

### Num of Keeping Latest Maintenance Jobs
Velero will keep one specific number of the latest maintenance jobs for each repository. By default, we only keep 3 latest maintenance jobs for each repository, and Velero support configures this setting by the below command when Velero installs:

```bash
velero install --keep-latest-maintenance-jobs <NUM>
```

### Default Repository Maintenance Frequency
The frequency of running maintenance jobs could be set by the below command when Velero is installed:
```bash
velero install --default-repo-maintain-frequency <DURATION>
```
For Kopia the default maintenance frequency is 1 hour, and Restic is 7 * 24 hours.

### Others
Maintenance jobs will inherit the labels, annotations, tolerations, affinity, nodeSelector, service account, image, environment variables, cloud-credentials etc. from Velero deployment.

[1]: velero-install.md#usage