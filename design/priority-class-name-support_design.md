# PriorityClass Support Design Proposal

## Abstract
This design document outlines the implementation of priority class name support for Velero components, including the Velero server deployment, node agent daemonset, and maintenance jobs. This feature allows users to specify a priority class name for Velero components, which can be used to influence the scheduling and eviction behavior of these components.

## Background
Kubernetes allows users to define priority classes, which can be used to influence the scheduling and eviction behavior of pods. Priority classes are defined as cluster-wide resources, and pods can reference them by name. When a pod is created, the priority admission controller uses the priority class name to populate the priority value for the pod. The scheduler then uses this priority value to determine the order in which pods are scheduled.

Currently, Velero does not provide a way for users to specify a priority class name for its components. This can be problematic in clusters where resource contention is high, as Velero components may be evicted or not scheduled in a timely manner, potentially impacting backup and restore operations.

## Goals
- Add support for specifying a priority class name for Velero components
- Update the Velero CLI to accept a priority class name parameter
- Update the Velero deployment, node agent daemonset, and maintenance jobs to use the specified priority class name

## Non Goals
- Creating or managing priority classes
- Automatically determining the appropriate priority class for Velero components

## High-Level Design
The implementation will add a new field to the Velero options struct to store the priority class name. The Velero CLI will be updated to accept a new flag, `--priority-class-name`, which will be used to set this field. The Velero deployment, node agent daemonset, and maintenance jobs will be updated to use the specified priority class name.

## Detailed Design

### CLI Changes
A new flag, `--priority-class-name`, will be added to the `velero install` command. This flag will accept a string value, which will be the name of the priority class to use for Velero components.

```go
flags.StringVar(
    &o.PriorityClassName,
    "priority-class-name",
    o.PriorityClassName,
    "Priority class name for the Velero deployment, node agent daemonset, and maintenance jobs. Optional.",
)
```

### Velero Options Changes
The `VeleroOptions` struct in `pkg/install/resources.go` will be updated to include a new field for the priority class name:

```go
type VeleroOptions struct {
    // ... existing fields ...
    PriorityClassName string
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
The `DaemonSet` function will be updated to use the priority class name:

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
The maintenance job creation in `pkg/repository/maintenance/maintenance.go` will be updated to use the priority class name from the Velero server deployment:

```go
job := &batchv1.Job{
    // ... existing fields ...
    Spec: batchv1.JobSpec{
        // ... existing fields ...
        Template: corev1api.PodTemplateSpec{
            // ... existing fields ...
            Spec: corev1api.PodSpec{
                // ... existing fields ...
                PriorityClassName: veleroutil.GetPriorityClassNameFromVeleroServer(deployment),
            },
        },
    },
}
```

A new function, `GetPriorityClassNameFromVeleroServer`, will be added to the `veleroutil` package to retrieve the priority class name from the Velero server deployment:

```go
func GetPriorityClassNameFromVeleroServer(deployment *appsv1api.Deployment) string {
    return deployment.Spec.Template.Spec.PriorityClassName
}
```

## Alternatives Considered
1. **Using separate flags for each component**: We could have added separate flags for the Velero deployment, node agent daemonset, and maintenance jobs. However, this would have increased the complexity of the CLI and would have been more difficult to maintain.

2. **Using a configuration file**: We could have added support for specifying the priority class name in a configuration file. However, this would have required additional changes to the Velero CLI and would have been more complex to implement.

## Security Considerations
There are no security considerations for this feature.

## Compatibility
This feature is compatible with all Kubernetes versions that support priority classes. The PodPriority feature became stable in Kubernetes 1.14. For more information, see the [Kubernetes documentation on Pod Priority and Preemption](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).

## Implementation
The implementation will involve the following steps:
1. Add the `PriorityClassName` field to the `VeleroOptions` struct
2. Add the `priorityClassName` field to the `podTemplateConfig` struct
3. Add the `WithPriorityClassName` function
4. Update the `Deployment` function to use the priority class name
5. Update the `DaemonSet` function to use the priority class name
6. Add the `GetPriorityClassNameFromVeleroServer` function to the `veleroutil` package
7. Update the maintenance job creation to use the priority class name from the Velero server deployment
8. Add the `--priority-class-name` flag to the `velero install` command

Note: We're only adding the flag to the CLI install command, not to the server configuration, as the priority class name is only needed during installation. This is because:

1. The priority class name is set on the Velero deployment and node agent daemonset during installation.
2. For maintenance jobs, the priority class name is inherited from the Velero server deployment through the `GetPriorityClassNameFromVeleroServer` function in the `veleroutil` package.
3. For data mover pods created by the exposer, the priority class name is inherited from the node agent daemonset through the `getInheritedPodInfo` function, which calls `nodeagent.GetPodSpec` to get the pod spec from the node agent daemonset.

This approach ensures that all Velero components use the same priority class name without requiring additional configuration after installation.

## Open Issues
None.
