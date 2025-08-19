---
title: "Node-agent Configuration"
layout: docs
---

The Velero node-agent is a DaemonSet that hosts modules for completing backup and restore operations, including file system backup/restore and CSI snapshot data movement. This document provides comprehensive configuration options for the node-agent through a ConfigMap.

## Overview

Node-agent configuration is provided through a ConfigMap that contains JSON configuration for various aspects of data movement operations. The ConfigMap should be created in the same namespace where Velero is installed, and its name is specified using the `--node-agent-configmap` parameter.

### Creating and Managing the ConfigMap

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

## Configuration Sections

### Load Concurrency (`loadConcurrency`)

Controls the concurrent number of data movement operations per node to optimize resource usage and performance.

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

- **Range**: Starts from 1 (no concurrency), no upper limit
- **Priority**: Per-node configuration overrides global configuration
- **Conflicts**: If a node matches multiple rules, the smallest number is used
- **Default**: 1 if not specified

**Use Cases:**
- Increase concurrency on nodes with more resources
- Reduce concurrency on nodes with limited resources or critical workloads
- Prevent OOM kills and resource contention

For detailed information, see [Node-agent Concurrency](node-agent-concurrency.md).

### Node Selection (`loadAffinity`)

Constrains which nodes can run data movement operations using affinity and anti-affinity rules.

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
          "environment": "backup"
        }
      }
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

For detailed information, see [Node Selection for Data Movement](data-movement-node-selection.md).

### Pod Resources (`podResources`)

Configure CPU and memory resources for data mover pods to optimize performance and prevent resource conflicts.

```json
{
  "podResources": {
    "cpuRequest": "1000m",
    "cpuLimit": "2000m",
    "memoryRequest": "1Gi",
    "memoryLimit": "4Gi"
  },
  "priorityClassName": "backup-priority"
}
```

#### Resource Configuration
- **Values**: Must be valid Kubernetes Quantity expressions
- **Validation**: Request values must not exceed limit values
- **Default**: BestEffort QoS if not specified
- **Failure Handling**: Invalid values cause the entire `podResources` section to be ignored

#### Priority Class Configuration
Configure pod priority to control scheduling behavior:

**High Priority** (e.g., `system-cluster-critical`):
- ✅ Faster scheduling and less likely to be preempted
- ❌ May impact production workload performance

**Low Priority** (e.g., `low-priority`):
- ✅ Protects production workloads from resource competition
- ❌ May delay backup operations or cause preemption

**Use Cases:**
- Limit resource consumption in resource-constrained clusters
- Guarantee resources for time-critical backup/restore operations
- Prevent OOM kills during large data transfers
- Control scheduling priority relative to production workloads

For detailed information, see [Data Movement Pod Resource Configuration](data-movement-pod-resource-configuration.md).

### Backup PVC Configuration (`backupPVC`)

Configure intermediate PVCs used during data movement backup operations for optimal performance.

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

#### Configuration Options
- **`storageClass`**: Alternative storage class for backup PVCs (defaults to source PVC's storage class)
- **`readOnly`**: Use `ReadOnlyMany` access mode for faster volume creation from snapshots
- **`spcNoRelabeling`**: Required in SELinux clusters when using `readOnly` mode

#### Storage Class Mapping
Configure different backup PVC settings per source storage class:
```json
{
  "backupPVC": {
    "fast-storage": {
      "storageClass": "backup-storage",
      "readOnly": true
    },
    "slow-storage": {
      "storageClass": "backup-storage"
    }
  }
}
```

**Use Cases:**
- Use read-only volumes for faster snapshot-to-volume conversion
- Use dedicated storage classes optimized for backup operations
- Reduce replica count for intermediate backup volumes
- Comply with SELinux requirements in secured environments

**Important Notes:**
- Ensure specified storage classes exist and support required access modes
- In SELinux environments, always set `spcNoRelabeling: true` when using `readOnly: true`
- Failures result in DataUpload CR staying in `Accepted` phase until timeout (30m default)

For detailed information, see [BackupPVC Configuration for Data Movement Backup](data-movement-backup-pvc-configuration.md).

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

For detailed information, see [RestorePVC Configuration for Data Movement Restore](data-movement-restore-pvc-configuration.md).

### Prepare Queue Length (`prepareQueueLength`)

Control the maximum number of backup/restore operations that can be in preparation phases simultaneously.

```json
{
  "prepareQueueLength": 10
}
```

**Use Cases:**
- Limit resource consumption from intermediate objects (PVCs, VolumeSnapshots, etc.)
- Prevent resource exhaustion when backup/restore concurrency is limited
- Balance between parallelism and resource usage

**Affected CR Phases:**
- DataUpload/DataDownload CRs in `Accepted` or `Prepared` phases
- PodVolumeBackup/PodVolumeRestore CRs in preparation phases

For detailed information, see [Node-agent Prepare Queue Length](node-agent-prepare-queue-length.md).

## Complete Configuration Example

Here's a comprehensive example showing how all configuration sections work together:

```json
{
  "loadConcurrency": {
    "globalConfig": 2,
    "perNodeConfig": [
      {
        "nodeSelector": {
          "matchLabels": {
            "node-type": "backup"
          }
        },
        "number": 4
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
          "storage-tier": "fast"
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
    "fast-ssd": {
      "storageClass": "backup-optimized",
      "readOnly": true
    },
    "standard": {
      "storageClass": "backup-standard"
    }
  },
  "restorePVC": {
    "ignoreDelayBinding": true
  },
  "prepareQueueLength": 15
}
```

This configuration:
- Allows 2 concurrent operations globally, 4 on backup nodes
- Runs data movement only on backup nodes without critical workloads
- Uses fast storage nodes for fast-ssd storage class operations
- Limits pod resources to prevent cluster overload
- Uses high priority for backup operations
- Optimizes backup PVCs with read-only access and dedicated storage classes
- Ignores delay binding for faster restores
- Allows up to 15 operations in preparation phases

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
- [Node-agent Concurrency](node-agent-concurrency.md)
- [Node Selection for Data Movement](data-movement-node-selection.md)
- [Data Movement Pod Resource Configuration](data-movement-pod-resource-configuration.md)
- [BackupPVC Configuration for Data Movement Backup](data-movement-backup-pvc-configuration.md)
- [RestorePVC Configuration for Data Movement Restore](data-movement-restore-pvc-configuration.md)
- [Node-agent Prepare Queue Length](node-agent-prepare-queue-length.md)