# Repository maintenance job configuration design

## Abstract
Add this design to make the repository maintenance job can read configuration from a dedicate ConfigMap and make the Job's necessary parts configurable, e.g. `PodSpec.Affinity` and `PodSpec.Resources`.

## Background
Repository maintenance is split from the Velero server to a k8s Job in v1.14 by design [repository maintenance job](repository-maintenance.md).
The repository maintenance Job configuration was read from the Velero server CLI parameter, and it inherits the most of Velero server's Deployment's PodSpec to fill un-configured fields.

This design introduces a new way to let the user to customize the repository maintenance behavior instead of inheriting from the Velero server Deployment or reading from `velero server` CLI parameters.
The configurations added in this design including the resource limitations, node selection.
It's possible new configurations are introduced in future releases based on this design.

For the node selection, the repository maintenance Job also inherits from the Velero server deployment before, but the Job may last for a while and cost noneligible resources, especially memory.
The users have the need to choose which k8s node to run the maintenance Job.
This design reuses the data structure introduced by design [Velero Generic Data Path affinity configuration](node-agent-affinity.md) to make the repository maintenance job can choose which node running on.

## Goals
- Unify the repository maintenance Job configuration at one place.
- Let user can choose repository maintenance Job running on which nodes.

## Non Goals
- There was an [issue](https://github.com/vmware-tanzu/velero/issues/7911) to require the whole Job's PodSpec should be configurable. That's not in the scope of this design.
- Please notice this new configuration is dedicated for the repository maintenance. Repository itself configuration is not covered.


## Compatibility
v1.14 uses the `velero server` CLI's parameter to pass the repository maintenance job configuration.
In v1.15, those parameters are still kept, including `--maintenance-job-cpu-request`, `--maintenance-job-mem-request`, `--maintenance-job-cpu-limit`, `--maintenance-job-mem-limit`, and `--keep-latest-maintenance-jobs`.
But the parameters read from the ConfigMap specified by `velero server` CLI parameter `--repo-maintenance-job-configmap` introduced by this design have a higher priority.

If there `--repo-maintenance-job-configmap` is not specified, then the `velero server` parameters are used if provided.

If the `velero server` parameters are not specified too, then the default values are used.
* `--keep-latest-maintenance-jobs` default value is 3.
* `--maintenance-job-cpu-request` default value is 0.
* `--maintenance-job-mem-request` default value is 0.
* `--maintenance-job-cpu-limit` default value is 0.
* `--maintenance-job-mem-limit` default value is 0.

## Deprecation
Propose to deprecate the `velero server` parameters `--maintenance-job-cpu-request`, `--maintenance-job-mem-request`, `--maintenance-job-cpu-limit`, `--maintenance-job-mem-limit`, and `--keep-latest-maintenance-jobs` in release-1.15.
That means those parameters will be deleted in release-1.17.
After deletion, those resources-related parameters are replaced by the ConfigMap specified by `velero server` CLI's parameter `--repo-maintenance-job-configmap`.
`--keep-latest-maintenance-jobs` is deleted from `velero server` CLI. It turns into a non-configurable internal parameter, and its value is 3.
Please check [issue 7923](https://github.com/vmware-tanzu/velero/issues/7923) for more information why deleting this parameter.

## Design
This design introduces a new ConfigMap specified by `velero server` CLI parameter `--repo-maintenance-job-configmap` as the source of the repository maintenance job configuration. The specified ConfigMap is read from the namespace where Velero is installed.
If the ConfigMap doesn't exist, the internal default values are used.

Example of using the parameter `--repo-maintenance-job-configmap`:
```
velero server \
    ...
    --repo-maintenance-job-configmap repo-job-config
    ...
```

**Notice**
* Velero doesn't own this ConfigMap. If the user wants to customize the repository maintenance job, the user needs to create this ConfigMap.
* Velero reads this ConfigMap content at starting a new repository maintenance job, so the ConfigMap change will not take affect until the next created job.

### Structure
The data structure is as below:
```go
type Configs struct {
    // LoadAffinity is the config for data path load affinity.
    LoadAffinity []*LoadAffinity `json:"loadAffinity,omitempty"`    

    // PodResources is the config for the CPU and memory resources setting.
    PodResources *kube.PodResources `json:"podResources,omitempty"`
}

type LoadAffinity struct {
    // NodeSelector specifies the label selector to match nodes
    NodeSelector metav1.LabelSelector `json:"nodeSelector"`
}

type PodResources struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}
```

The ConfigMap content is a map.
If there is a key value as `global` in the map, the key's value is applied to all BackupRepositories maintenance jobs that cannot find their own specific configuration in the ConfigMap.
The other keys in the map is the combination of three elements of a BackupRepository:
* The namespace in which BackupRepository backs up volume data.
* The BackupRepository referenced BackupStorageLocation's name.
* The BackupRepository's type. Possible values are `kopia` and `restic`.

Those three keys can identify a [unique BackupRepository](https://github.com/vmware-tanzu/velero/blob/2fc6300f2239f250b40b0488c35feae59520f2d3/pkg/repository/backup_repo_op.go#L32-L37).

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

The `LoadAffinity` structure is reused from design [Velero Generic Data Path affinity configuration](node-agent-affinity.md).
It's possible that the users want to choose nodes that match condition A or condition B to run the job.
For example, the user want to let the nodes is in a specified machine type or the nodes locate in the us-central1-x zones to run the job.
This can be done by adding multiple entries in the `LoadAffinity` array.

### Affinity Example
A sample of the ConfigMap is as below:
``` bash
cat <<EOF > repo-maintenance-job-config.json
{
    "global": {
        podResources: {
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

### Value assigning rules
If the Velero BackupRepositoryController cannot find the introduced ConfigMap, the following default values are used for repository maintenance job:
``` go
config := Configs {
    // LoadAffinity is the config for data path load affinity.
    LoadAffinity: nil,

    // Resources is the config for the CPU and memory resources setting.
    PodResources: &kube.PodResources{
        // The repository maintenance job CPU request setting
	    CPURequest:   "0m",

        // The repository maintenance job memory request setting
	    MemoryRequest:   "0Mi",

        // The repository maintenance job CPU limit setting
	    CPULimit:     "0m",

        // The repository maintenance job memory limit setting
	    MemoryLimit:     "0Mi",
    },
}
```

If the Velero BackupRepositoryController finds the introduced ConfigMap with only `global` element, the `global` value is used.

If the Velero BackupRepositoryController finds the introduced ConfigMap with only element matches the BackupRepository, the matched element value is used.


If the Velero BackupRepositoryController finds the introduced ConfigMap with both `global` element and element matches the BackupRepository, the matched element defined values overwrite the `global` value, and the `global` value is still used for matched element undefined values.

For example, the ConfigMap content has two elements.
``` json
{
    "global": {
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
        ],
        "podResources": {
            "cpuRequest": "100m",
            "cpuLimit": "200m",
            "memoryRequest": "100Mi",
            "memoryLimit": "200Mi"
        }
    },
    "ns1-default-kopia": {
        "podResources": {
            "memoryRequest": "400Mi",
            "memoryLimit": "800Mi"
        }
    }
}
```
The config value used for BackupRepository backing up volume data in namespace `ns1`, referencing BSL `default`, and the type is `Kopia`:
``` go
config := Configs {
    // LoadAffinity is the config for data path load affinity.
    LoadAffinity: []*kube.LoadAffinity{
        {
			NodeSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "cloud.google.com/machine-family",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"e2"},
					},
				},
			},
		},
    },
    PodResources: &kube.PodResources{
        // The repository maintenance job CPU request setting
	    CPURequest:   "",
        // The repository maintenance job memory request setting
	    MemoryRequest:   "400Mi",
        // The repository maintenance job CPU limit setting
	    CPULimit:     "",
        // The repository maintenance job memory limit setting
	    MemoryLimit:     "800Mi",
    }
}
```


### Implementation
During the Velero repository controller starts to maintain a repository, it will call the repository manager's `PruneRepo` function to build the maintenance Job.
The ConfigMap specified by `velero server` CLI parameter `--repo-maintenance-job-configmap` is get to reinitialize the repository `MaintenanceConfig` setting.

``` go
	jobConfig, err := getMaintenanceJobConfig(
		context.Background(),
		m.client,
		m.log,
		m.namespace,
		m.repoMaintenanceJobConfig,
		repo,
	)
	if err != nil {
        log.Infof("Cannot find the ConfigMap %s with error: %s. Use default value.",
			m.namespace+"/"+m.repoMaintenanceJobConfig,
			err.Error(),
		)
	}

	log.Info("Start to maintenance repo")

	maintenanceJob, err := m.buildMaintenanceJob(
		jobConfig,
		param,
	)
	if err != nil {
		return errors.Wrap(err, "error to build maintenance job")
	}
```

## Alternatives Considered
An other option is creating each ConfigMap for a BackupRepository.
This is not ideal for scenario that has a lot of BackupRepositories in the cluster.