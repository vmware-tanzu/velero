---
title: "Node-agent Configuration"
layout: docs
---

The Velero node-agent is a DaemonSet that hosts modules for completing backup and restore operations, including file system backup/restore and CSI snapshot data movement. This document provides comprehensive configuration options for the ConfigMap provisioned by node-agent's `--node-agent-configmap` parameter.

## Overview

Node-agent puts advanced configurations of data movement and PodVolume operations into a ConfigMap that contains JSON configuration. The ConfigMap should be created in the same namespace where Velero is installed, and its name is specified using the `--node-agent-configmap` parameter.

### Creating and Managing the ConfigMap
For detailed information, see [Node-agent Concurrency](../node-agent-concurrency.md).

**Notice**: The ConfigMap's life cycle control is out of the scope of Velero.
User need to create and maintain the ConfigMap themselves.

The ConfigMap name can be specified during Velero installation:
```bash
velero install --node-agent-configmap=<ConfigMap-Name>
```

To create the ConfigMap:
1. Save your configuration to a JSON file
2. Create the ConfigMap:
```bash
kubectl create cm <ConfigMap-Name> -n velero --from-file=<json-file-name>
```

To apply the ConfigMap to the node-agent DaemonSet:
```bash
kubectl edit ds node-agent -n velero
```

Add the ConfigMap reference to the container arguments:
```yaml
spec:
  template:
    spec:
      containers:
      - args:
        - --node-agent-configmap=<ConfigMap-Name>
```
**Important**: The node-agent server checks configurations at startup time. After editing the ConfigMap, restart the node-agent DaemonSet for changes to take effect.
`kubectl rollout restart -n <velero-namespace> daemonset/node-agent`

## Configuration Sections
### Load Concurrency (`loadConcurrency`)

Controls the concurrent number of data movement operations per node to optimize resource usage and performance.

For detailed information, see [Node-agent Prepare Queue Length](../node-agent-prepare-queue-length.md).

#### Global Configuration
Sets a default concurrent number applied to all nodes:
```json
{
  "loadConcurrency": {
    "globalConfig": 2
  }
}
```

#### Per-node Configuration
Specify different concurrent numbers for specific nodes using label selectors:
```json
{
  "loadConcurrency": {
    "globalConfig": 2,
    "perNodeConfig": [
      {
        "nodeSelector": {
          "matchLabels": {
            "kubernetes.io/hostname": "node1"
          }
        },
        "number": 3
      },
      {
        "nodeSelector": {
          "matchLabels": {
            "beta.kubernetes.io/instance-type": "Standard_B4ms"
          }
        },
        "number": 5
      }
    ]
  }
}
```

- **Range**: Starts from 1 (no concurrency per node), no upper limit
- **Priority**: Per-node configuration overrides global configuration
- **Conflicts**: If a node matches multiple rules, the smallest number is used
- **Default**: 1 if not specified

**Use Cases:**
- Increase concurrency on nodes with more resources
- Reduce concurrency on nodes with limited resources or critical workloads
- Prevent OOM kills and resource contention

#### PrepareQueueLength
Control the maximum number of backup/restore operations that can be in preparation phases simultaneously.
The concurrency numbers controls how many backup/restore operations can run at the same time. 
The prepare queue length controls how many backup/restore operations can create the workload pods that are pending for start.
If there are thousands of volume B/R operations, and without this control, thousands of B/R pods will be created at the same time, then causing a big burden on the k8s API server.

```json
{
  "loadConcurrency": {
    "prepareQueueLength": 10
  }
}
```

- **Range**: Starts from 1 (for all node-agent pods), no upper limit
- **Scope**: This parameter controls all PVB, PVR, DataUpload, and DataDownload pods pending number. It applies to all node-agent pods.
- **Default**: No limitation if not specified

**Use Cases:**
- Prevent too much workload pods are created, but cannot start.
- Limit resource consumption from intermediate objects (PVCs, VolumeSnapshots, etc.)
- Prevent resource exhaustion when backup/restore concurrency is limited
- Balance between parallelism and resource usage

**Affected CR Phases:**
- DataUpload/DataDownload CRs in `Accepted` or `Prepared` phases
- PodVolumeBackup/PodVolumeRestore CRs in preparation phases

### Node Selection (`loadAffinity`)
Constrains which nodes can run data movement operations using affinity and anti-affinity rules.

For detailed information, see [Node Selection for Data Movement](../data-movement-node-selection.md).

```json
{
  "loadAffinity": [
    {
      "nodeSelector": {
        "matchLabels": {
          "beta.kubernetes.io/instance-type": "Standard_B4ms"
        },
        "matchExpressions": [
          {
            "key": "kubernetes.io/hostname",
            "values": ["node-1", "node-2", "node-3"],
            "operator": "In"
          },
          {
            "key": "critical-workload",
            "operator": "DoesNotExist"
          }
        ]
      }
    }
  ]
}
```

#### Storage Class Specific Selection
Configure different node selection rules for specific storage classes:
* For StorageClass `fast-ssd`, the first match is chosen, which is nodes with label `"environment": "production"`.
* For StorageClass `hdd`, the nodes with label `"environment": "backup"` are chosen. 

```json
{
  "loadAffinity": [
    {
      "nodeSelector": {
        "matchLabels": {
          "environment": "production"
        }
      },
      "storageClass": "fast-ssd"
    },
    {
      "nodeSelector": {
        "matchLabels": {
          "environment": "staging"
        }
      },
      "storageClass": "fast-ssd"
    },
    {
      "nodeSelector": {
        "matchLabels": {
          "environment": "backup"
        }
      },
      "storageClass": "hdd"
    }
  ]
}
```

**Important Limitations:**
- Only the first element in the `loadAffinity` array is used for general node selection
- Additional elements are only considered if they have a `storageClass` field
- To combine multiple conditions, use both `matchLabels` and `matchExpressions` in a single element

**Use Cases:**
- Prevent data movement on nodes with critical workloads
- Run data movement only on nodes with sufficient resources
- Ensure data movement runs only on nodes where storage is accessible
- Comply with topology constraints

### Pod Resources (`podResources`)
Configure CPU and memory resources for data mover pods to optimize performance and prevent resource conflict.
The configurations work for PodVolumeBackup, PodVolumeRestore, DataUpload, and DataDownload pods.

```json
{
  "podResources": {
    "cpuRequest": "1000m",
    "cpuLimit": "2000m",
    "memoryRequest": "1Gi",
    "memoryLimit": "4Gi"
  }
}
```

**Use Cases:**
- Limit resource consumption in resource-constrained clusters
- Guarantee resources for time-critical backup/restore operations
- Prevent OOM kills during large data transfers
- Control scheduling priority relative to production workloads

**Values**: Must be valid Kubernetes Quantity expressions
**Validation**: Request values must not exceed limit values
**Default**: BestEffort QoS if not specified
**Failure Handling**: Invalid values cause the entire `podResources` section to be ignored

For detailed information, see [Data Movement Pod Resource Configuration](../data-movement-pod-resource-configuration.md).


### Priority Class (`priorityClassName`)
Configure the node-agent created pod's PriorityClass.
The configurations work for PodVolumeBackup, PodVolumeRestore, DataUpload, and DataDownload pods.

Configure pod priority to control scheduling behavior:

**High Priority** (e.g., `system-cluster-critical`):
- ✅ Faster scheduling and less likely to be preempted
- ❌ May impact production workload performance

**Low Priority** (e.g., `low-priority`):
- ✅ Protects production workloads from resource competition
- ❌ May delay backup operations or cause preemption

Example:

``` json
{
  "priorityClassName": "low-priority"
}
```

### Backup PVC Configuration (`backupPVC`)

Configure intermediate PVCs used during data movement backup operations for optimal performance.

For detailed information, see [BackupPVC Configuration for Data Movement Backup](../data-movement-backup-pvc-configuration.md).

#### Configuration Options
- **`storageClass`**: Alternative storage class for backup PVCs (defaults to source PVC's storage class)
- **`readOnly`**: This is a boolean value. If set to `true` then `ReadOnlyMany` will be the only value set to the backupPVC's access modes. Otherwise `ReadWriteOnce` value will be used.
- **`spcNoRelabeling`**: This is a boolean value. If set to true, then `pod.Spec.SecurityContext.SELinuxOptions.Type` will be set to `spc_t`. From the SELinux point of view, this will be considered a `Super Privileged Container` which means that selinux enforcement will be disabled and volume relabeling will not occur. This field is ignored if `readOnly` is `false`.

**Use Cases:**
- Use read-only volumes for faster snapshot-to-volume conversion
- Use dedicated storage classes optimized for backup operations
- Reduce replica count for intermediate backup volumes
- Comply with SELinux requirements in secured environments

**Important Notes:**
- Ensure specified storage classes exist and support required access modes
- In SELinux environments, always set `spcNoRelabeling: true` when using `readOnly: true`
- Failures result in DataUpload CR staying in `Accepted` phase until timeout (30m default)

#### Storage Class Mapping
Configure different backup PVC settings per source storage class:
```json
{
  "backupPVC": {
    "fast-storage": {
      "storageClass": "backup-storage-1"
    },
    "slow-storage": {
      "storageClass": "backup-storage-2"
    }
  }
}
```

#### ReadOnly and SPC configuration
Create BackupPVC in ReadOnly mode, which can avoid full data clone during backup process in some storage providers, such as Ceph RBD.

In an `SELinux-enabled` cluster, any time users set `readOnly=true` they must also set `spcNoRelabeling=true`. 

```json
{
  "backupPVC": {
    "source-storage-class": {
      "storageClass": "backup-optimized-class",
      "readOnly": true,
      "spcNoRelabeling": true
    }
  }
}
```

### Restore PVC Configuration (`restorePVC`)

Configure intermediate PVCs used during data movement restore operations.

```json
{
  "restorePVC": {
    "ignoreDelayBinding": true
  }
}
```

#### Configuration Options
- **`ignoreDelayBinding`**: Ignore `WaitForFirstConsumer` binding mode constraints

**Use Cases:**
- Improve restore parallelism by not waiting for pod scheduling
- Enable volume restore without requiring a pod to be mounted
- Work around topology constraints when you know the environment setup

**Important Notes:**
- Use only when you understand your cluster's topology constraints
- May result in volumes provisioned on nodes where workload pods cannot be scheduled
- Works best with node selection to ensure proper node targeting

For detailed information, see [RestorePVC Configuration for Data Movement Restore](../data-movement-restore-pvc-configuration.md).

### Privileged FS Backup and Restore (`privilegedFsBackup`)
Add `privileged` permission in PodVolumeBackup and PodVolumeRestore created pod's `SecurityContext`, because in some k8s environments, mounting HostPath volume needs privileged permission to work.

In v1.17, the PodVolumeBackup and PodVolumeRestore are micro-serviced into independent pods, but they still mount the target volume by HostPath way. As a result, `privileged` permission are needed.

``` json
{
  "privilegedFsBackup": true
}
```

For detailed information, see [Enable file system backup document](../customize-installation.md#enable-file-system-backup)

### Cache PVC Configuration (`cachePVCConfig`)

Configure intermediate PVCs used for data movement restore operations to cache the downloaded data.

For detailed information, see [Cache PVC Configuration for Data Movement Restore](../data-movement-cache-volume.md).

#### Configuration Options
- **`thresholdInGB`**: Minimum backup data size (in GB) to trigger cache PVC creation during restore
- **`storageClass`**: Storage class used to create cache PVCs.

**Use Cases:**
- Improve restore performance by caching downloaded data locally
- Reduce repeated data downloads from object storage
- Optimize restore operations for large volumes

**Important Notes:**
- Cache PVC is only created when restored data size exceeds the threshold
- Ensure specified storage class exists and has sufficient capacity
- Cache PVCs are temporary and cleaned up after restore completion

```json
{
  "cachePVCConfig": {
    "thresholdInGB": 1,
    "storageClass": "cache-optimized-storage"
  }
}
```

## Complete Configuration Example
Here's a comprehensive example showing how all configuration sections work together:

```json
{
  "loadConcurrency": {
    "globalConfig": 2,
    "prepareQueueLength": 15,
    "perNodeConfig": [
      {
        "nodeSelector": {
          "matchLabels": {
            "kubernetes.io/hostname": "node1"
          }
        },
        "number": 3
      }
    ]
  },
  "loadAffinity": [
    {
      "nodeSelector": {
        "matchLabels": {
          "node-type": "backup"
        },
        "matchExpressions": [
          {
            "key": "critical-workload",
            "operator": "DoesNotExist"
          }
        ]
      }
    },
    {
      "nodeSelector": {
        "matchLabels": {
          "environment": "staging"
        }
      },
      "storageClass": "fast-ssd"
    }
  ],
  "podResources": {
    "cpuRequest": "500m",
    "cpuLimit": "1000m",
    "memoryRequest": "1Gi",
    "memoryLimit": "2Gi"
  },
  "priorityClassName": "backup-priority",
  "backupPVC": {
    "fast-storage": {
      "storageClass": "backup-optimized-class",
      "readOnly": true,
      "spcNoRelabeling": true
    },
    "slow-storage": {
      "storageClass": "backup-storage-2"
    }
  },
  "restorePVC": {
    "ignoreDelayBinding": true
  },
  "privilegedFsBackup": true,
  "cachePVC": {
      "thresholdInGB": 1,
      "storageClass": "cache-optimized-storage"
  }
}
```

This configuration:
- Allows 2 concurrent operations globally, 3 on worker `node1`
- Allows up to 15 operations in preparation phases
- Runs data movement only on backup nodes without critical workloads
- Uses fast storage nodes for fast-ssd storage class operations
- Limits pod resources to prevent cluster overload
- Uses high priority for backup operations
- Optimizes backup PVCs with read-only access and dedicated storage classes
- Ignores delay binding for faster restores
- Enable privileged permission for PodVolume pods
- Enable cache PVC for FS restore
- The cache threshold is 1GB and use dedicated StorageClass

## Troubleshooting

### Common Issues

1. **ConfigMap not taking effect**: Restart node-agent DaemonSet after changes
2. **Invalid resource values**: Check logs for validation errors; entire section ignored on failure
3. **Storage class not found**: Ensure specified storage classes exist in the cluster
4. **SELinux issues**: Set `spcNoRelabeling: true` when using `readOnly: true`
5. **Node selection not working**: Verify node labels and check only first loadAffinity element is used

### Validation

To verify your configuration is loaded correctly:
```bash
kubectl logs -n velero -l app=node-agent | grep -i config
```

To check current node-agent configuration:
```bash
kubectl get cm <ConfigMap-Name> -n velero -o yaml
```

## Related Documentation
For detailed information on specific configuration sections:
- [Node-agent Concurrency](../node-agent-concurrency.md)
- [Node Selection for Data Movement](../data-movement-node-selection.md)
- [Data Movement Pod Resource Configuration](../data-movement-pod-resource-configuration.md)
- [BackupPVC Configuration for Data Movement Backup](../data-movement-backup-pvc-configuration.md)
- [RestorePVC Configuration for Data Movement Restore](../data-movement-restore-pvc-configuration.md)
- [Node-agent Prepare Queue Length](../node-agent-prepare-queue-length.md)
- [Cache PVC Configuration for Data Movement Restore](../data-movement-cache-volume.md)