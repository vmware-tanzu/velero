# PodVolumeRestore API Status Signaling Design

## Abstract

This design proposes replacing the current file-based signaling mechanism for PodVolumeRestore completion with a Kubernetes API-based approach. Instead of writing "done files" to restored volumes, the PodVolumeRestore controller will create a ConfigMap in the user's namespace that the init container can read to determine when volume restoration is complete.

**Key Security Principle**: This design avoids granting user-namespaced workloads any cross-namespace access. The init container only needs to read ConfigMaps in its own namespace, maintaining proper security boundaries.

## Background

Currently, Velero's File System Backup restore process uses a file-based signaling mechanism:

1. The PodVolumeRestore controller restores volume data
2. Upon completion, it writes a "done file" to `.velero/<restore-UID>` in the restored volume
3. An init container (`velero-restore-helper`) polls for these done files
4. When all done files are found, the init container exits, allowing the pod to proceed

This approach fails when volumes are 100% full, as documented in [issue #2812](https://github.com/vmware-tanzu/velero/issues/2812), resulting in errors like:
```
error creating .velero directory for done file: mkdir /host_pods/.../mount/.velero: no space left on device
```

## Goals

- Eliminate dependency on writing files to restored volumes for signaling
- Provide a more reliable completion signaling mechanism
- Maintain backward compatibility where possible
- Support scenarios where restored volumes are completely full
- Avoid requiring cross-namespace access from user workloads

## Non-Goals

- Changing the overall PodVolumeRestore workflow
- Modifying how volume data is actually restored
- Changing the PodVolumeRestore CRD structure
- Granting user workloads access to resources in the velero namespace

## High-Level Design

Replace the file-based signaling with ConfigMap-based status checking:

1. **PodVolumeRestore Controller**: Creates/updates a ConfigMap in the user's namespace with restore status
2. **Init Container**: Reads the ConfigMap in its own namespace to check restoration status
3. **RBAC**: Init container only needs read access to ConfigMaps in its own namespace (no cross-namespace access)

## Detailed Design

### PodVolumeRestore Controller Changes

The controller will create or update a ConfigMap in the user's namespace to signal completion:

```go
// pkg/controller/pod_volume_restore_controller.go
func (c *PodVolumeRestoreReconciler) OnDataPathCompleted(ctx context.Context, pvr *velerov1api.PodVolumeRestore, result *datapath.Result) {
    // ... existing restoration logic ...
    
    // Try to write done file for backward compatibility
    doneFilePath := filepath.Join(volumePath, ".velero", string(restoreUID))
    if err := os.MkdirAll(filepath.Dir(doneFilePath), 0755); err != nil {
        log.WithError(err).Warn("Failed to create .velero directory, will use ConfigMap signaling")
    } else if err := os.WriteFile(doneFilePath, nil, 0644); err != nil {
        log.WithError(err).Warn("Failed to write done file, will use ConfigMap signaling")
    }
    
    // Create/update ConfigMap in user namespace for signaling
    if err := c.updateRestoreStatusConfigMap(ctx, pvr); err != nil {
        log.WithError(err).Error("Failed to update restore status ConfigMap")
        // Don't fail the restore, but log the error
    }
    
    // Always update PVR status
    pvr.Status.Phase = velerov1api.PodVolumeRestorePhaseCompleted
    pvr.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
    // ... update CR ...
}

func (c *PodVolumeRestoreReconciler) updateRestoreStatusConfigMap(ctx context.Context, pvr *velerov1api.PodVolumeRestore) error {
    restoreUID := string(pvr.OwnerReferences[0].UID)
    namespace := pvr.Spec.Pod.Namespace
    podName := pvr.Spec.Pod.Name
    
    // ConfigMap name based on restore UID and pod name
    cmName := fmt.Sprintf("velero-restore-status-%s-%s", restoreUID, podName)
    
    // Get or create the ConfigMap
    cm := &corev1.ConfigMap{}
    err := c.Client.Get(ctx, types.NamespacedName{
        Namespace: namespace,
        Name:      cmName,
    }, cm)
    
    if err != nil && !apierrors.IsNotFound(err) {
        return err
    }
    
    if apierrors.IsNotFound(err) {
        // Create new ConfigMap
        cm = &corev1.ConfigMap{
            ObjectMeta: metav1.ObjectMeta{
                Name:      cmName,
                Namespace: namespace,
                Labels: map[string]string{
                    "velero.io/restore-uid": restoreUID,
                    "velero.io/pod-name":    podName,
                    "velero.io/type":        "restore-status",
                },
                // Set owner reference to the pod so ConfigMap is cleaned up
                OwnerReferences: []metav1.OwnerReference{
                    {
                        APIVersion: "v1",
                        Kind:       "Pod",
                        Name:       podName,
                        UID:        pvr.Spec.Pod.UID,
                    },
                },
            },
            Data: make(map[string]string),
        }
    }
    
    // Update the status for this PVR
    if cm.Data == nil {
        cm.Data = make(map[string]string)
    }
    
    cm.Data[pvr.Name] = string(pvr.Status.Phase)
    cm.Data["updated"] = time.Now().Format(time.RFC3339)
    
    // Create or update the ConfigMap
    if apierrors.IsNotFound(err) {
        return c.Client.Create(ctx, cm)
    }
    return c.Client.Update(ctx, cm)
}
```

### Init Container Implementation

Replace the current `velero-restore-helper` with an enhanced version that reads ConfigMaps in its own namespace:

```go
// cmd/velero-restore-helper/velero-restore-helper.go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func main() {
    restoreUID := os.Args[1]
    
    // Try file-based check first for backward compatibility
    if checkDoneFiles(restoreUID) {
        cleanup()
        return
    }
    
    // Fall back to ConfigMap-based checking
    config, err := rest.InClusterConfig()
    if err != nil {
        // If we can't get in-cluster config, fall back to file-only mode
        waitForDoneFiles(restoreUID)
        return
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        waitForDoneFiles(restoreUID)
        return
    }
    
    waitForConfigMapCompletion(clientset, restoreUID)
}

func waitForConfigMapCompletion(client kubernetes.Interface, restoreUID string) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    namespace := os.Getenv("POD_NAMESPACE")
    podName := os.Getenv("POD_NAME")
    
    // ConfigMap name that the controller will create
    cmName := fmt.Sprintf("velero-restore-status-%s-%s", restoreUID, podName)
    
    // Get expected volume count from environment (set by PodVolumeRestoreAction)
    expectedVolumes := os.Getenv("VELERO_EXPECTED_VOLUMES")
    if expectedVolumes == "" {
        // Fall back to counting done files if we don't know expected count
        waitForDoneFiles(restoreUID)
        return
    }
    
    expectedCount := 0
    for _, vol := range strings.Split(expectedVolumes, ",") {
        if vol != "" {
            expectedCount++
        }
    }
    
    for {
        <-ticker.C
        
        // Get the ConfigMap
        cm, err := client.CoreV1().ConfigMaps(namespace).Get(
            context.TODO(),
            cmName,
            metav1.GetOptions{},
        )
        
        if err != nil {
            fmt.Printf("Waiting for restore status ConfigMap: %v\n", err)
            continue
        }
        
        // Check if all expected PVRs are complete
        completedCount := 0
        failedCount := 0
        
        for key, value := range cm.Data {
            if key == "updated" {
                continue
            }
            
            if value == "Completed" {
                completedCount++
            } else if value == "Failed" {
                failedCount++
                fmt.Printf("PodVolumeRestore %s failed\n", key)
            }
        }
        
        totalProcessed := completedCount + failedCount
        
        fmt.Printf("Restore progress: %d/%d volumes restored (%d completed, %d failed)\n", 
            totalProcessed, expectedCount, completedCount, failedCount)
        
        if totalProcessed >= expectedCount {
            if failedCount > 0 {
                fmt.Printf("WARNING: %d volume restore(s) failed\n", failedCount)
            }
            fmt.Println("All PodVolumeRestores processed")
            cleanup()
            return
        }
    }
}

func checkDoneFiles(restoreUID string) bool {
    // Check if all expected done files exist
    volumeDirs, err := os.ReadDir("/restores")
    if err != nil {
        return false
    }
    
    for _, volumeDir := range volumeDirs {
        if !volumeDir.IsDir() {
            continue
        }
        
        doneFile := filepath.Join("/restores", volumeDir.Name(), ".velero", restoreUID)
        if _, err := os.Stat(doneFile); os.IsNotExist(err) {
            return false
        }
    }
    
    return len(volumeDirs) > 0
}

func waitForDoneFiles(restoreUID string) {
    // Existing file-based waiting logic
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    for {
        <-ticker.C
        if checkDoneFiles(restoreUID) {
            cleanup()
            return
        }
    }
}

func cleanup() {
    // Clean up /restores directory as before
    os.RemoveAll("/restores")
}
```

### RBAC Configuration

The init container only needs minimal permissions in its own namespace:

```yaml
# No additional RBAC required for init containers
# Init containers run with the pod's service account and only need:
# - Read access to ConfigMaps in their own namespace
# This is typically available by default or through existing pod service accounts

# The PodVolumeRestore controller needs permission to create ConfigMaps in user namespaces:
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: velero-restore-controller
rules:
# Existing rules...
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["create", "update", "get", "list"]
```

No cross-namespace permissions are required for user workloads. The init container reads ConfigMaps in its own namespace using the pod's service account.

### PodVolumeRestoreAction Changes

Update the restore action to pass required environment variables to the init container:

```go
// pkg/restore/actions/pod_volume_restore_action.go
func (a *PodVolumeRestoreAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
    // ... existing code ...
    
    initContainerBuilder := newRestoreInitContainerBuilder(image, string(input.Restore.UID))
    initContainerBuilder.Resources(&resourceReqs)
    initContainerBuilder.SecurityContext(&securityContext)
    
    // Collect volume names being restored
    volumeNames := []string{}
    for _, vol := range pod.Spec.Volumes {
        if vol.PersistentVolumeClaim != nil {
            volumeNames = append(volumeNames, vol.Name)
        }
    }
    
    // Add environment variables for ConfigMap-based checking
    initContainerBuilder.Env(
        &corev1api.EnvVar{
            Name: "POD_NAMESPACE",
            ValueFrom: &corev1api.EnvVarSource{
                FieldRef: &corev1api.ObjectFieldSelector{
                    FieldPath: "metadata.namespace",
                },
            },
        },
        &corev1api.EnvVar{
            Name: "POD_NAME",
            ValueFrom: &corev1api.EnvVarSource{
                FieldRef: &corev1api.ObjectFieldSelector{
                    FieldPath: "metadata.name",
                },
            },
        },
        &corev1api.EnvVar{
            Name:  "VELERO_EXPECTED_VOLUMES",
            Value: strings.Join(volumeNames, ","),
        },
    )
    
    // No special service account needed - uses pod's default service account
    // which should have read access to ConfigMaps in its own namespace
    
    // ... rest of existing code ...
}
```

## Backward Compatibility

The design maintains backward compatibility through:

1. **Dual-mode operation**: The init container first checks for done files (existing behavior), then falls back to API checking
2. **Best-effort done file creation**: Controllers still attempt to create done files but don't fail if unable
3. **Graceful degradation**: If API access fails, the system falls back to file-based waiting

## Migration Path

1. **Phase 1**: Deploy new init container with dual-mode support
2. **Phase 2**: Update controllers to handle done file creation failures gracefully
3. **Phase 3**: Monitor and validate API-based signaling in production
4. **Phase 4**: (Optional) Deprecate file-based signaling in future release

## Security Considerations

- **No cross-namespace access required**: Init containers only read ConfigMaps in their own namespace
- **Principle of least privilege**: User workloads don't need access to velero namespace resources
- **Automatic cleanup**: ConfigMaps have owner references to pods, ensuring cleanup when pods are deleted
- **Controller permissions**: Only the PodVolumeRestore controller (running in velero namespace) needs permission to create ConfigMaps in user namespaces
- **No elevated permissions for user workloads**: Pods use their default service accounts with standard namespace-scoped permissions

## Performance Implications

- **Reduced I/O**: Eliminates file write operations on restored volumes
- **API load**: Adds API queries (1 per second per restoring pod)
- **Network traffic**: Minimal - only status checks, not data transfer

## Alternatives Considered

1. **Direct PodVolumeRestore API access** (Rejected due to security concerns):
   - Would require user workloads to have cross-namespace access to velero namespace
   - Violates principle of least privilege
   - Creates potential security risk

2. **Skip done file on error**: Simply skip done file creation when disk is full
   - Simpler but less reliable
   - No positive confirmation mechanism

3. **EmptyDir volume**: Use separate volume for signaling files
   - Requires pod spec modifications
   - More complex volume management

4. **Annotations on Pod**: Update pod annotations with restore status
   - Would require init container to have write access to pod
   - More complex permission model
   - Pod annotations have size limits

## Open Questions

1. Should we make API-based signaling the default or keep file-based as primary?
2. Should we add a configuration option to disable done file creation entirely?
3. How long should we maintain backward compatibility with file-based signaling?

## Implementation Plan

1. **Implement dual-mode init container** with API query capability
2. **Update controllers** to handle done file failures gracefully  
3. **Add RBAC resources** for restore helper service account
4. **Update PodVolumeRestoreAction** to configure init container
5. **Add unit and integration tests** for both signaling modes
6. **Document the new behavior** and migration steps
7. **Performance testing** to validate API load impact

## References

- [Issue #2812: Volumes 100% full cannot be restored](https://github.com/vmware-tanzu/velero/issues/2812)
- [PR #7141: Add --write-sparse-files flag](https://github.com/vmware-tanzu/velero/pull/7141)
- [File System Backup Documentation](https://velero.io/docs/main/file-system-backup/)