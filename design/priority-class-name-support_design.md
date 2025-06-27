# PriorityClass Support Design Proposal

## Abstract
This design document outlines the implementation of priority class name support for Velero components, including the Velero server deployment, node agent daemonset, and maintenance jobs. This feature allows users to specify a priority class name for Velero components, which can be used to influence the scheduling and eviction behavior of these components.

## Background
Kubernetes allows users to define priority classes, which can be used to influence the scheduling and eviction behavior of pods. Priority classes are defined as cluster-wide resources, and pods can reference them by name. When a pod is created, the priority admission controller uses the priority class name to populate the priority value for the pod. The scheduler then uses this priority value to determine the order in which pods are scheduled.

Currently, Velero does not provide a way for users to specify a priority class name for its components. This can be problematic in clusters where resource contention is high, as Velero components may be evicted or not scheduled in a timely manner, potentially impacting backup and restore operations.

## Goals
- Add support for specifying priority class names for Velero components
- Update the Velero CLI to accept priority class name parameters for different components
- Update the Velero deployment, node agent daemonset, maintenance jobs, and data mover pods to use the specified priority class names

## Non Goals
- Creating or managing priority classes
- Automatically determining the appropriate priority class for Velero components

## High-Level Design
The implementation will add new fields to the Velero options struct to store the priority class names for the server deployment and node agent daemonset. The Velero CLI will be updated to accept new flags for these components. For data mover pods and maintenance jobs, priority class names will be configured through existing ConfigMap mechanisms (`node-agent-configmap` for data movers and `repo-maintenance-job-configmap` for maintenance jobs). The Velero deployment, node agent daemonset, maintenance jobs, and data mover pods will be updated to use their respective priority class names.

## Detailed Design

### CLI Changes
New flags will be added to the `velero install` command to specify priority class names for different components:

```go
flags.StringVar(
    &o.ServerPriorityClassName,
    "server-priority-class-name",
    o.ServerPriorityClassName,
    "Priority class name for the Velero server deployment. Optional.",
)

flags.StringVar(
    &o.NodeAgentPriorityClassName,
    "node-agent-priority-class-name",
    o.NodeAgentPriorityClassName,
    "Priority class name for the node agent daemonset. Optional.",
)
```

Note: Priority class names for data mover pods and maintenance jobs will be configured through their respective ConfigMaps (`--node-agent-configmap` for data movers and `--repo-maintenance-job-configmap` for maintenance jobs).

### Velero Options Changes
The `VeleroOptions` struct in `pkg/install/resources.go` will be updated to include new fields for priority class names:

```go
type VeleroOptions struct {
    // ... existing fields ...
    ServerPriorityClassName       string
    NodeAgentPriorityClassName    string
}
```

### Deployment Changes
The `podTemplateConfig` struct in `pkg/install/deployment.go` will be updated to include a new field for the priority class name:

```go
type podTemplateConfig struct {
    // ... existing fields ...
    priorityClassName string
}
```

A new function, `WithPriorityClassName`, will be added to set this field:

```go
func WithPriorityClassName(priorityClassName string) podTemplateOption {
    return func(c *podTemplateConfig) {
        c.priorityClassName = priorityClassName
    }
}
```

The `Deployment` function will be updated to use the priority class name:

```go
deployment := &appsv1api.Deployment{
    // ... existing fields ...
    Spec: appsv1api.DeploymentSpec{
        // ... existing fields ...
        Template: corev1api.PodTemplateSpec{
            // ... existing fields ...
            Spec: corev1api.PodSpec{
                // ... existing fields ...
                PriorityClassName: c.priorityClassName,
            },
        },
    },
}
```

### DaemonSet Changes
The `DaemonSet` function will use the priority class name passed via the podTemplateConfig (from the CLI flag):

```go
daemonSet := &appsv1api.DaemonSet{
    // ... existing fields ...
    Spec: appsv1api.DaemonSetSpec{
        // ... existing fields ...
        Template: corev1api.PodTemplateSpec{
            // ... existing fields ...
            Spec: corev1api.PodSpec{
                // ... existing fields ...
                PriorityClassName: c.priorityClassName,
            },
        },
    },
}
```

### Maintenance Job Changes
The `JobConfigs` struct in `pkg/repository/maintenance/maintenance.go` will be updated to include a field for the priority class name:

```go
type JobConfigs struct {
    // LoadAffinities is the config for repository maintenance job load affinity.
    LoadAffinities []*kube.LoadAffinity `json:"loadAffinity,omitempty"`

    // PodResources is the config for the CPU and memory resources setting.
    PodResources *kube.PodResources `json:"podResources,omitempty"`
    
    // PriorityClassName is the priority class name for the maintenance job pod
    // Note: This is only read from the global configuration, not per-repository
    PriorityClassName string `json:"priorityClassName,omitempty"`
}
```

The `buildJob` function will be updated to use the priority class name from the global job configuration:

```go
func buildJob(cli client.Client, ctx context.Context, repo *velerov1api.BackupRepository, bslName string, config *JobConfigs,
    podResources kube.PodResources, logLevel logrus.Level, logFormat *logging.FormatFlag) (*batchv1.Job, error) {
    // ... existing code ...
    
    // Use the priority class name from the global job configuration if available
    // Note: Priority class is only read from global config, not per-repository
    priorityClassName := ""
    if config != nil && config.PriorityClassName != "" {
        priorityClassName = config.PriorityClassName
    }
    
    // ... existing code ...
    
    job := &batchv1.Job{
        // ... existing fields ...
        Spec: batchv1.JobSpec{
            // ... existing fields ...
            Template: corev1api.PodTemplateSpec{
                // ... existing fields ...
                Spec: corev1api.PodSpec{
                    // ... existing fields ...
                    PriorityClassName: priorityClassName,
                },
            },
        },
    }
    
    // ... existing code ...
}
```

Users will be able to configure the priority class name for all maintenance jobs by creating the repository maintenance job ConfigMap before installation. For example:

```bash
# Create the ConfigMap before running velero install
cat <<EOF | kubectl create configmap repo-maintenance-job-config -n velero --from-file=config.json=/dev/stdin
{
    "global": {
        "priorityClassName": "low-priority",
        "podResources": {
            "cpuRequest": "100m",
            "memoryRequest": "128Mi"
        }
    }
}
EOF

# Then install Velero referencing this ConfigMap
velero install --provider aws \
    --repo-maintenance-job-configmap repo-maintenance-job-config \
    # ... other flags
```

The ConfigMap can be updated after installation to change the priority class for future maintenance jobs. Note that only the "global" configuration is used for priority class - all maintenance jobs will use the same priority class regardless of which repository they are maintaining.

### Node Agent ConfigMap Changes
We'll update the `Configs` struct in `pkg/nodeagent/node_agent.go` to include a field for the priority class name in the node-agent-configmap:

```go
type Configs struct {
    // ... existing fields ...
    
    // PriorityClassName is the priority class name for both the node agent daemonset 
    // and the data mover pods it creates
    PriorityClassName string `json:"priorityClassName,omitempty"`
}
```

This will allow users to configure the priority class name for both the node agent daemonset and data mover pods through a single node-agent-configmap. For example:

```bash
# Create the ConfigMap before running velero install
cat <<EOF | kubectl create configmap node-agent-config -n velero --from-file=config.json=/dev/stdin
{
    "priorityClassName": "low-priority",
    "loadAffinity": [
        {
            "nodeSelector": {
                "matchLabels": {
                    "node-role.kubernetes.io/worker": "true"
                }
            }
        }
    ]
}
EOF

# Then install Velero referencing this ConfigMap
velero install --provider aws \
    --node-agent-configmap node-agent-config \
    --use-node-agent \
    # ... other flags
```

The `createBackupPod` function in `pkg/exposer/csi_snapshot.go` will be updated to accept and use the priority class name:

```go
func (e *csiSnapshotExposer) createBackupPod(
    ctx context.Context,
    ownerObject corev1api.ObjectReference,
    backupPVC *corev1api.PersistentVolumeClaim,
    operationTimeout time.Duration,
    label map[string]string,
    annotation map[string]string,
    affinity *kube.LoadAffinity,
    resources corev1api.ResourceRequirements,
    backupPVCReadOnly bool,
    spcNoRelabeling bool,
    nodeOS string,
    priorityClassName string, // New parameter
) (*corev1api.Pod, error) {
    // ... existing code ...
    
    pod := &corev1api.Pod{
        // ... existing fields ...
        Spec: corev1api.PodSpec{
            // ... existing fields ...
            PriorityClassName: priorityClassName,
            // ... existing fields ...
        },
    }
    
    // ... existing code ...
}
```

The call to `createBackupPod` in the `Expose` method will be updated to pass the priority class name retrieved from the node-agent-configmap:

```go
priorityClassName, _ := veleroutil.GetDataMoverPriorityClassName(ctx, namespace, kubeClient, configMapName)
backupPod, err := e.createBackupPod(
    ctx,
    ownerObject,
    backupPVC,
    csiExposeParam.OperationTimeout,
    csiExposeParam.HostingPodLabels,
    csiExposeParam.HostingPodAnnotations,
    csiExposeParam.Affinity,
    csiExposeParam.Resources,
    backupPVCReadOnly,
    spcNoRelabeling,
    csiExposeParam.NodeOS,
    priorityClassName, // Priority class name from node-agent-configmap
)
```

A new function, `GetDataMoverPriorityClassName`, will be added to the `veleroutil` package to retrieve the priority class name for data mover pods:

```go
func GetDataMoverPriorityClassName(ctx context.Context, namespace string, kubeClient kubernetes.Interface, configName string) (string, error) {
    // Get from node-agent-configmap
    configs, err := nodeagent.GetConfigs(ctx, namespace, kubeClient, configName)
    if err == nil && configs != nil && configs.PriorityClassName != "" {
        return configs.PriorityClassName, nil
    }
    
    // Return empty string if not found in configmap
    return "", nil
}
```

This function will get the priority class name from the node-agent-configmap. If it's not found, it will return an empty string.

## Alternatives Considered

1. **Using a single flag for all components**: We could have used a single flag for all components, but this would not allow for different priority classes for different components. Since maintenance jobs and data movers typically require lower priority than the Velero server, separate flags provide more flexibility.

2. **Using a configuration file**: We could have added support for specifying the priority class names in a configuration file. However, this would have required additional changes to the Velero CLI and would have been more complex to implement.

3. **Inheriting priority class from parent components**: We initially considered having maintenance jobs inherit their priority class from the Velero server, and data movers inherit from the node agent. However, this approach doesn't allow for the appropriate prioritization of different components based on their importance and resource requirements.

## Security Considerations

There are no security considerations for this feature.

## Compatibility

This feature is compatible with all Kubernetes versions that support priority classes. The PodPriority feature became stable in Kubernetes 1.14. For more information, see the [Kubernetes documentation on Pod Priority and Preemption](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).

## Implementation

The implementation will involve the following steps:

1. Add the priority class name fields for server and node agent to the `VeleroOptions` struct
2. Add the priority class name field to the `podTemplateConfig` struct
3. Add the `WithPriorityClassName` function for the server deployment and daemonset
4. Update the `Deployment` function to use the server priority class name
5. Update the `DaemonSet` function to use the node agent priority class name
6. Update the `JobConfigs` struct to include `PriorityClassName` field
7. Update the `buildJob` function in maintenance job to use the priority class name from JobConfigs (global config only)
8. Update the `Configs` struct in node agent to include `PriorityClassName` field for data mover pods
9. Update the data mover pod creation to use the priority class name from node-agent-configmap
10. Add the priority class name flags for server and node agent to the `velero install` command
11. Update documentation to explain how to configure priority classes

Note: The server deployment and node agent daemonset will have CLI flags for priority class. Data mover pods and maintenance jobs will use their respective ConfigMaps for priority class configuration.

This approach ensures that different Velero components can use different priority class names based on their importance and resource requirements:

1. The Velero server deployment can use a higher priority class to ensure it continues running even under resource pressure.
2. The node agent daemonset can use a medium priority class.
3. Maintenance jobs can use a lower priority class since they should not run when resources are limited.
4. Data mover pods can use a lower priority class since they should not run when resources are limited.

### Implementation Considerations

Priority class names are configured through different mechanisms:

1. **Server Deployment**: Uses the `--server-priority-class-name` CLI flag during installation.

2. **Node Agent DaemonSet**: Uses the `--node-agent-priority-class-name` CLI flag during installation.

3. **Data Mover Pods**: Will use the node-agent-configmap (specified via the `--node-agent-configmap` flag). This ConfigMap controls priority class for all data mover pods (including PVB and PVR) created by the node agent.

4. **Maintenance Jobs**: Will use the repository maintenance job ConfigMap (specified via the `--repo-maintenance-job-configmap` flag). Users should create this ConfigMap before running `velero install` with the desired priority class configuration. The ConfigMap can be updated after installation to change priority classes for future maintenance jobs. While the ConfigMap structure supports per-repository configuration for resources and affinity, priority class is intentionally only read from the global configuration to ensure all maintenance jobs have the same priority.

This approach has several advantages:

- Leverages existing configuration mechanisms, minimizing new CLI flags
- Provides a single point of configuration for related components (node agent and its pods)
- Allows dynamic configuration updates without requiring Velero reinstallation
- Maintains backward compatibility with existing installations
- Enables administrators to set up priority classes during initial deployment
- Keeps configuration simple by using the same priority class for all maintenance jobs

The priority class name for data mover pods will be determined by checking the node-agent-configmap. This approach provides a centralized way to configure priority class names for all data mover pods. The same approach will be used for PVB (PodVolumeBackup) and PVR (PodVolumeRestore) pods, which will also retrieve their priority class name from the node-agent-configmap.

For PVB and PVR pods specifically, the controllers will need to be updated to retrieve the priority class name from the node-agent-configmap and pass it to the pod creation functions. For example, in the PodVolumeBackup controller:

```go
// In pkg/controller/pod_volume_backup_controller.go
priorityClassName, _ := veleroutil.GetDataMoverPriorityClassName(ctx, namespace, kubeClient, configMapName)

// Add priorityClassName to the pod spec
pod := &corev1api.Pod{
    // ... existing fields ...
    Spec: corev1api.PodSpec{
        // ... existing fields ...
        PriorityClassName: priorityClassName,
    },
}
```

Similarly, in the PodVolumeRestore controller:

```go
// In pkg/controller/pod_volume_restore_controller.go
priorityClassName, _ := veleroutil.GetDataMoverPriorityClassName(ctx, namespace, kubeClient, configMapName)

// Add priorityClassName to the pod spec
pod := &corev1api.Pod{
    // ... existing fields ...
    Spec: corev1api.PodSpec{
        // ... existing fields ...
        PriorityClassName: priorityClassName,
    },
}
```

This ensures that all pods created by Velero (data movers, PVB, and PVR) use a consistent approach for priority class name configuration.

## Open Issues

None.
