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
    
    // PriorityClassName is the priority class name for the data mover pods 
    // created by the node agent
    PriorityClassName string `json:"priorityClassName,omitempty"`
}
```

This will allow users to configure the priority class name for data mover pods through the node-agent-configmap. Note that the node agent daemonset itself gets its priority class from the `--node-agent-priority-class-name` CLI flag during installation, not from this configmap. For example:

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
priorityClassName, _ := kube.GetDataMoverPriorityClassName(ctx, namespace, kubeClient, configMapName)
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

A new function, `GetDataMoverPriorityClassName`, will be added to the `pkg/util/kube` package (in the same file as `ValidatePriorityClass`) to retrieve the priority class name for data mover pods:

```go
// In pkg/util/kube/priority_class.go

// GetDataMoverPriorityClassName retrieves the priority class name for data mover pods from the node-agent-configmap
func GetDataMoverPriorityClassName(ctx context.Context, namespace string, kubeClient kubernetes.Interface, configName string) (string, error) {
    // configData is a minimal struct to parse only the priority class name from the ConfigMap
    type configData struct {
        PriorityClassName string `json:"priorityClassName,omitempty"`
    }

    // Get the ConfigMap
    cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, configName, metav1.GetOptions{})
    if err != nil {
        if apierrors.IsNotFound(err) {
            // ConfigMap not found is not an error, just return empty string
            return "", nil
        }
        return "", errors.Wrapf(err, "error getting node agent config map %s", configName)
    }

    if cm.Data == nil {
        // No data in ConfigMap, return empty string
        return "", nil
    }

    // Extract the first value from the ConfigMap data
    jsonString := ""
    for _, v := range cm.Data {
        jsonString = v
        break // Use the first value found
    }

    if jsonString == "" {
        // No data to parse, return empty string
        return "", nil
    }

    // Parse the JSON to extract priority class name
    var config configData
    if err := json.Unmarshal([]byte(jsonString), &config); err != nil {
        // Invalid JSON is not a critical error for priority class
        // Just return empty string to use default behavior
        return "", nil
    }

    return config.PriorityClassName, nil
}
```

This function will get the priority class name from the node-agent-configmap. If it's not found, it will return an empty string.

### Validation and Logging

To improve observability and help with troubleshooting, the implementation will include:

1. **Optional Priority Class Validation**: A helper function to check if a priority class exists in the cluster. This function will be added to the `pkg/util/kube` package alongside other Kubernetes utility functions:

```go
// In pkg/util/kube/priority_class.go

// ValidatePriorityClass checks if the specified priority class exists in the cluster
// Returns true if the priority class exists or if priorityClassName is empty
// Returns false if the priority class doesn't exist or validation fails
// Logs warnings when the priority class doesn't exist
func ValidatePriorityClass(ctx context.Context, kubeClient kubernetes.Interface, priorityClassName string, logger logrus.FieldLogger) bool {
    if priorityClassName == "" {
        return true
    }
    
    _, err := kubeClient.SchedulingV1().PriorityClasses().Get(ctx, priorityClassName, metav1.GetOptions{})
    if err != nil {
        if apierrors.IsNotFound(err) {
            logger.Warnf("Priority class %q not found in cluster. Pod creation may fail if the priority class doesn't exist when pods are scheduled.", priorityClassName)
        } else {
            logger.WithError(err).Warnf("Failed to validate priority class %q", priorityClassName)
        }
        return false
    }
    logger.Infof("Validated priority class %q exists in cluster", priorityClassName)
    return true
}
```

2. **Debug Logging**: Add debug logs when priority classes are applied:

```go
// In deployment creation
if c.priorityClassName != "" {
    logger.Debugf("Setting priority class %q for Velero server deployment", c.priorityClassName)
}

// In daemonset creation
if c.priorityClassName != "" {
    logger.Debugf("Setting priority class %q for node agent daemonset", c.priorityClassName)
}

// In maintenance job creation
if priorityClassName != "" {
    logger.Debugf("Setting priority class %q for maintenance job %s", priorityClassName, job.Name)
}

// In data mover pod creation
if priorityClassName != "" {
    logger.Debugf("Setting priority class %q for data mover pod %s", priorityClassName, pod.Name)
}
```

These validation and logging features will help administrators:

- Identify configuration issues early (validation warnings)
- Troubleshoot priority class application issues
- Verify that priority classes are being applied as expected

The `ValidatePriorityClass` function should be called at the following points:

1. **During `velero install`**: Validate the priority classes specified via CLI flags:
   - After parsing `--server-priority-class-name` flag
   - After parsing `--node-agent-priority-class-name` flag

2. **When reading from ConfigMaps**: Validate priority classes when loading configurations:
   - In `GetDataMoverPriorityClassName` when reading from node-agent-configmap
   - In maintenance job controller when reading from repo-maintenance-job-configmap

3. **During pod/job creation** (optional, for runtime validation):
   - Before creating data mover pods (PVB/PVR/CSI snapshot data movement)
   - Before creating maintenance jobs

Example usage:

```go
// During velero install
if o.ServerPriorityClassName != "" {
    _ = kube.ValidatePriorityClass(ctx, kubeClient, o.ServerPriorityClassName, logger.WithField("component", "server"))
    // For install command, we continue even if validation fails (warnings are logged)
}

// When reading from ConfigMap in node-agent server
priorityClassName, err := kube.GetDataMoverPriorityClassName(ctx, namespace, kubeClient, configMapName)
if err == nil && priorityClassName != "" {
    // Validate the priority class exists in the cluster
    if kube.ValidatePriorityClass(ctx, kubeClient, priorityClassName, logger.WithField("component", "data-mover")) {
        dataMovePriorityClass = priorityClassName
        logger.WithField("priorityClassName", priorityClassName).Info("Using priority class for data mover pods")
    } else {
        logger.WithField("priorityClassName", priorityClassName).Warn("Priority class not found in cluster, data mover pods will use default priority")
        // Clear the priority class to prevent pod creation failures
        priorityClassName = ""
    }
}
```

Note: The validation function returns a boolean to allow callers to decide how to handle missing priority classes. For the install command, validation failures are ignored (only warnings are logged) to allow for scenarios where priority classes might be created after Velero installation. For runtime components like the node-agent server, the priority class is cleared if validation fails to prevent pod creation failures.

## Alternatives Considered

1. **Using a single flag for all components**: We could have used a single flag for all components, but this would not allow for different priority classes for different components. Since maintenance jobs and data movers typically require lower priority than the Velero server, separate flags provide more flexibility.

2. **Using a configuration file**: We could have added support for specifying the priority class names in a configuration file. However, this would have required additional changes to the Velero CLI and would have been more complex to implement.

3. **Inheriting priority class from parent components**: We initially considered having maintenance jobs inherit their priority class from the Velero server, and data movers inherit from the node agent. However, this approach doesn't allow for the appropriate prioritization of different components based on their importance and resource requirements.

## Security Considerations

There are no security considerations for this feature.

## Compatibility

This feature is compatible with all Kubernetes versions that support priority classes. The PodPriority feature became stable in Kubernetes 1.14. For more information, see the [Kubernetes documentation on Pod Priority and Preemption](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).

## ConfigMap Update Strategy

### Static ConfigMap Reading at Startup

The node-agent server reads and parses the ConfigMap once during initialization and passes configurations (like `podResources`, `loadAffinity`, and `priorityClassName`) directly to controllers as parameters. This approach ensures:

- Single ConfigMap read to minimize API calls
- Consistent configuration across all controllers
- Validation of priority classes at startup with fallback behavior
- No need for complex update mechanisms or watchers

ConfigMap changes require a restart of the node-agent to take effect.

### Implementation Approach

1. **Data Mover Controllers**: Receive priority class as a string parameter from node-agent server at initialization
2. **Maintenance Job Controller**: Read fresh configuration from repo-maintenance-job-configmap at job creation time
3. ConfigMap changes require restart of components to take effect
4. Priority class validation happens at startup with automatic fallback to prevent failures

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
10. Update the PodVolumeBackup controller to retrieve and apply priority class name from node-agent-configmap
11. Update the PodVolumeRestore controller to retrieve and apply priority class name from node-agent-configmap
12. Add the `GetDataMoverPriorityClassName` utility function to retrieve priority class from configmap
13. Add the priority class name flags for server and node agent to the `velero install` command
14. Add unit tests for:
    - `WithPriorityClassName` function
    - `GetDataMoverPriorityClassName` function
    - Priority class application in deployment, daemonset, and job specs
15. Add integration tests to verify:
    - Priority class is correctly applied to all component pods
    - ConfigMap updates are reflected in new pods
    - Empty/missing priority class names are handled gracefully
16. Update user documentation to include:
    - How to configure priority classes for each component
    - Examples of creating ConfigMaps before installation
    - Expected priority class hierarchy recommendations
    - Troubleshooting guide for priority class issues
17. Update CLI documentation for new flags (`--server-priority-class-name` and `--node-agent-priority-class-name`)

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

#### ConfigMap Pre-Creation Guide

For components that use ConfigMaps for priority class configuration, the ConfigMaps must be created before running `velero install`. Here's the recommended workflow:

```bash
# Step 1: Create priority classes in your cluster (if not already existing)
kubectl apply -f - <<EOF
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: velero-critical
value: 100
globalDefault: false
description: "Critical priority for Velero server"
---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: velero-standard
value: 50
globalDefault: false
description: "Standard priority for Velero node agent"
---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: velero-low
value: 10
globalDefault: false
description: "Low priority for Velero data movers and maintenance jobs"
EOF

# Step 2: Create the namespace
kubectl create namespace velero

# Step 3: Create ConfigMaps for data movers and maintenance jobs
kubectl create configmap node-agent-config -n velero --from-file=config.json=/dev/stdin <<EOF
{
    "priorityClassName": "velero-low"
}
EOF

kubectl create configmap repo-maintenance-job-config -n velero --from-file=config.json=/dev/stdin <<EOF
{
    "global": {
        "priorityClassName": "velero-low"
    }
}
EOF

# Step 4: Install Velero with priority class configuration
velero install \
    --provider aws \
    --server-priority-class-name velero-critical \
    --node-agent-priority-class-name velero-standard \
    --node-agent-configmap node-agent-config \
    --repo-maintenance-job-configmap repo-maintenance-job-config \
    --use-node-agent
```

#### Recommended Priority Class Hierarchy

When configuring priority classes for Velero components, consider the following hierarchy based on component criticality:

1. **Velero Server (Highest Priority)**:
   - Example: `velero-critical` with value 100
   - Rationale: The server must remain running to coordinate backup/restore operations

2. **Node Agent DaemonSet (Medium Priority)**:
   - Example: `velero-standard` with value 50
   - Rationale: Node agents need to be available on nodes but are less critical than the server

3. **Data Mover Pods & Maintenance Jobs (Lower Priority)**:
   - Example: `velero-low` with value 10
   - Rationale: These are temporary workloads that can be delayed during resource contention

This hierarchy ensures that core Velero components remain operational even under resource pressure, while allowing less critical workloads to be preempted if necessary.

This approach has several advantages:

- Leverages existing configuration mechanisms, minimizing new CLI flags
- Provides a single point of configuration for related components (node agent and its pods)
- Allows dynamic configuration updates without requiring Velero reinstallation
- Maintains backward compatibility with existing installations
- Enables administrators to set up priority classes during initial deployment
- Keeps configuration simple by using the same priority class for all maintenance jobs

The priority class name for data mover pods will be determined by checking the node-agent-configmap. This approach provides a centralized way to configure priority class names for all data mover pods. The same approach will be used for PVB (PodVolumeBackup) and PVR (PodVolumeRestore) pods, which will also retrieve their priority class name from the node-agent-configmap.

For PVB and PVR pods specifically, the implementation follows this approach:

1. **Controller Initialization**: Both PodVolumeBackup and PodVolumeRestore controllers are updated to accept a priority class name as a string parameter. The node-agent server reads the priority class from the node-agent-configmap once at startup:

```go
// In node-agent server startup (pkg/cmd/cli/nodeagent/server.go)
dataMovePriorityClass := ""
if s.config.nodeAgentConfig != "" {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
    defer cancel()
    priorityClass, err := kube.GetDataMoverPriorityClassName(ctx, s.namespace, s.kubeClient, s.config.nodeAgentConfig)
    if err != nil {
        s.logger.WithError(err).Warn("Failed to get priority class name from node-agent-configmap, using empty value")
    } else if priorityClass != "" {
        // Validate the priority class exists in the cluster
        if kube.ValidatePriorityClass(ctx, s.kubeClient, priorityClass, s.logger.WithField("component", "data-mover")) {
            dataMovePriorityClass = priorityClass
            s.logger.WithField("priorityClassName", priorityClass).Info("Using priority class for data mover pods")
        } else {
            s.logger.WithField("priorityClassName", priorityClass).Warn("Priority class not found in cluster, data mover pods will use default priority")
        }
    }
}

// Pass priority class to controllers
pvbReconciler := controller.NewPodVolumeBackupReconciler(
    s.mgr.GetClient(), s.mgr, s.kubeClient, ..., dataMovePriorityClass)
pvrReconciler := controller.NewPodVolumeRestoreReconciler(
    s.mgr.GetClient(), s.mgr, s.kubeClient, ..., dataMovePriorityClass)
```

2. **Controller Structure**: Controllers store the priority class name as a field:

```go
type PodVolumeBackupReconciler struct {
    // ... existing fields ...
    dataMovePriorityClass string
}
```

3. **Pod Creation**: The priority class is included in the pod spec when creating data mover pods.

### VGDP Micro-Service Considerations

With the introduction of VGDP micro-services (as described in the VGDP micro-service design), data mover pods are created as dedicated pods for volume snapshot data movement. These pods will also inherit the priority class configuration from the node-agent-configmap. Since VGDP-MS pods (backupPod/restorePod) inherit their configurations from the node-agent, they will automatically use the priority class name specified in the node-agent-configmap.

This ensures that all pods created by Velero for data movement operations (CSI snapshot data movement, PVB, and PVR) use a consistent approach for priority class name configuration through the node-agent-configmap.

### How Exposers Receive Configuration

CSI Snapshot Exposer and Generic Restore Exposer do not directly watch or read ConfigMaps. Instead, they receive configuration through their parent controllers:

1. **Controller Initialization**: Controllers receive the priority class name as a parameter during initialization from the node-agent server.

2. **Configuration Propagation**: During reconciliation of resources:
   - The controller calls `setupExposeParam()` which includes the `dataMovePriorityClass` value
   - For CSI operations: `CSISnapshotExposeParam.PriorityClassName` is set
   - For generic restore: `GenericRestoreExposeParam.PriorityClassName` is set
   - The controller passes these parameters to the exposer's `Expose()` method

3. **Pod Creation**: The exposer creates pods with the priority class name provided by the controller.

This design keeps exposers stateless and ensures:
- Exposers remain simple and focused on pod creation
- All configuration flows through controllers consistently
- No complex state synchronization between components
- Configuration changes require component restart to take effect

## Open Issues

None.
