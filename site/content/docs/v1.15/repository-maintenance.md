---
title: "Repository Maintenance"
layout: docs
---

From v1.14 on, Velero decouples repository maintenance from the Velero server by launching a k8s job to do maintenance when needed, to mitigate the impact on the Velero server during backups.

Before v1.14.0, Velero performs periodic maintenance on the repository within Velero server pod, this operation may consume significant CPU and memory resources in some cases, leading to Velero server being killed by OOM. Now Velero will launch independent k8s jobs to do the maintenance in Velero installation namespace.

For repository maintenance jobs, there's no limit on resources by default. You could configure the job resource limitation based on target data to be backed up.

From v1.15 and on, Velero introduces a new ConfigMap, specified by `velero server --repo-maintenance-job-configmap` parameter, to set repository maintenance Job configuration, including Node Affinity and resources. The old `velero server` parameters ( `--maintenance-job-cpu-request`, `--maintenance-job-mem-request`, `--maintenance-job-cpu-limit`, `--maintenance-job-mem-limit`, and `--keep-latest-maintenance-jobs`) introduced in v1.14 are deprecated, and will be deleted in v1.17.

The users can specify the ConfigMap name during velero installation by CLI:
`velero install --repo-maintenance-job-configmap=<ConfigMap-Name>`

## Settings
### Resource Limitation and Node Affinity
Those are specified by the ConfigMap specified by `velero server --repo-maintenance-job-configmap` parameter.

This ConfigMap content is a Map.
If there is a key value as `global` in the map, the key's value is applied to all BackupRepositories maintenance jobs that cannot find their own specific configuration in the ConfigMap.
The other keys in the map is the combination of three elements of a BackupRepository, because those three keys can identify a unique BackupRepository:
* The namespace in which BackupRepository backs up volume data.
* The BackupRepository referenced BackupStorageLocation's name.
* The BackupRepository's type. Possible values are `kopia` and `restic`.

If there is a key match with BackupRepository, the key's value is applied to the BackupRepository's maintenance jobs.
By this way, it's possible to let user configure before the BackupRepository is created.
This is especially convenient for administrator configuring during the Velero installation.
For example, the following BackupRepository's key should be `test-default-kopia`.

``` yaml
- apiVersion: velero.io/v1
  kind: BackupRepository
  metadata:
    generateName: test-default-kopia-
    labels:
      velero.io/repository-type: kopia
      velero.io/storage-location: default
      velero.io/volume-namespace: test
    name: test-default-kopia-kgt6n
    namespace: velero
  spec:
    backupStorageLocation: default
    maintenanceFrequency: 1h0m0s
    repositoryType: kopia
    resticIdentifier: gs:jxun:/restic/test
    volumeNamespace: test
```

You can still customize the maintenance job resource requests and limit when using the [velero install][1] CLI command.

The `LoadAffinity` structure is reused from design [node-agent affinity configuration](2).

### Affinity Example
It's possible that the users want to choose nodes that match condition A or condition B to run the job.
For example, the user want to let the nodes is in a specified machine type or the nodes locate in the us-central1-x zones to run the job.
This can be done by adding multiple entries in the `LoadAffinity` array.

The sample of the ```repo-maintenance-job-configmap``` ConfigMap for the above scenario is as below:
``` bash
cat <<EOF > repo-maintenance-job-config.json
{
    "global": {
        "podResources": {
            "cpuRequest": "100m",
            "cpuLimit": "200m",
            "memoryRequest": "100Mi",
            "memoryLimit": "200Mi"
        },
        "loadAffinity": [
            {
                "nodeSelector": {
                    "matchExpressions": [
                        {
                            "key": "cloud.google.com/machine-family",
                            "operator": "In",
                            "values": [
                                "e2"
                            ]
                        }
                    ]          
                }
            },
            {
                "nodeSelector": {
                    "matchExpressions": [
                        {
                            "key": "topology.kubernetes.io/zone",
                            "operator": "In",
                            "values": [
                                "us-central1-a",
                                "us-central1-b",
                                "us-central1-c"
                            ]
                        }
                    ]          
                }
            }
        ]
    }
}
EOF
```
This sample showcases two affinity configurations:
- matchLabels: maintenance job runs on nodes with label key `cloud.google.com/machine-family` and value `e2`.
- matchLabels: maintenance job runs on nodes located in `us-central1-a`, `us-central1-b` and `us-central1-c`.
The nodes matching one of the two conditions are selected.

To create the configMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm repo-maintenance-job-config -n velero --from-file=repo-maintenance-job-config.json
```

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
Maintenance jobs will inherit the labels, annotations, toleration, nodeSelector, service account, image, environment variables, cloud-credentials etc. from Velero deployment.

[1]: velero-install.md#usage
[2]: node-agent-concurrency.md