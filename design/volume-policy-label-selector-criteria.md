# Add Label Selector as a criteria for Volume Policy

## Abstract
The volume policies in Velero currently support several criteria for selecting volumes for backup operations, such as capacity, storage class, and volume source type. However, there is a lack of flexibility in managing volumes based on custom label strategies. This design proposes an enhancement to Velero’s volume policies by adding label selectors as a condition for selecting PersistentVolumes (PVs). This will allow users to select volumes more granularly, based on Kubernetes labels, enabling more effective resource management and automation in backup operations.

## Background 
In Kubernetes, PersistentVolumes (PVs) can be tagged with labels that help identify and group resources based on various attributes (e.g., environment, application, region). However, Velero’s existing volume policy engine lacks the ability to use these labels for filtering PVs during backup operations. This gap in functionality limits the flexibility and granularity with which backup policies can be applied.

With the integration of label selectors into Velero’s volume policy system, users will have the ability to back up volumes based on the labels they have assigned, making backup operations more flexible, automated, and aligned with their resource management strategies.

## Goals
- Add support for label selectors as a condition to filter PersistentVolumes during backup operations.
- Enable users to select PVs for backup based on labels such as environment, application, or any custom label they define.
- Ensure that the new feature integrates smoothly with the existing volume policy conditions (such as storage class, capacity, NFS, and CSI).
- Ensure that existing volume policies continue to work without modification, and the label selector is an optional condition.

## Non-Goals
-  The goal is to add a single condition (label selector) to the existing policy framework, not a complete redesign.
- No changes to the action aspect of volume policy engine

## Use-cases/scenarios
1.	Use-Case 1: Environment-Specific Backup
- A User wants to back up only the PersistentVolumes that are labeled with environment=production and app=database to ensure that only production database volumes are backed up.
- Scenario: The user configures a volume policy with a label selector condition that matches the labels environment=production and app=database. Only those PVs that have these labels are included in the backup operation.
2. Use-Case 2: Region-Specific Backup
- A user has multiple regions, and PVs are labeled with region-specific labels (e.g., region=us-west, region=us-east). The user wants to back up only the volumes from the us-west region.
- Scenario: The user configures a policy with a label selector condition for region=us-west. This ensures that only PVs from the us-west region are backed up.
3. Use-Case 3: Automatic Label-Based Backups
- The user wants to automate backups based on labels assigned dynamically by an external system, ensuring that all newly created PVs with specific labels are backed up automatically.
- Scenario: The user sets up a label selector condition for a set of labels (e.g., backup=true). Whenever a new PV is created with these labels, it is automatically included in the backup without manual intervention.

## High-Level Design

1.	Volume Condition Interface:
- The `volumeCondition` interface will be extended to include the labelSelectorCondition which will match PVs based on their labels.

2. Label Selector Condition:
- A new struct, `labelSelectorCondition`, will be introduced to hold the Kubernetes label selector and implement the `match` method to evaluate whether a PV’s labels satisfy the selector.

3. Structured Volume:
- The `structuredVolume` struct will be updated to parse and store labels from PVs. This data will be used by the `labelSelectorCondition` to perform the matching.

4. Volume Policy:
- The `volumePolicy` struct will hold a list of conditions, including the labelSelectorCondition. Each condition will be evaluated to determine whether a PV matches the policy.

## Detailed Design

1.	`volumeCondition` Interface:
The existing `volumeCondition` interface will be extended to support the new `labelSelectorCondition`:
```go
type volumeCondition interface {
    match(v *structuredVolume) bool
    validate() error
}
```

2. `labelSelectorCondition` Struct:
This struct will hold a LabelSelector and implement the `match` method to check if a volume’s labels match the specified selector. It will also implement the `validate` method to validate label selector.
```go
type labelSelectorCondition struct {
    selector *metav1.LabelSelector // Kubernetes LabelSelector
}

func (c *labelSelectorCondition) match(v *structuredVolume) bool {
    if c.selector == nil {
        return true // If no selector is provided, match all volumes
    }

    labels := v.labels
    if labels == nil {
        return false // No labels in the volume, can't match
    }

    // Convert LabelSelector to a Selector and check for matching labels
    labelSelector, err := metav1.LabelSelectorAsSelector(c.selector)
    if err != nil {
        return false // Handle invalid selector
    }

    return labelSelector.Matches(labels)
}

func (c *labelSelectorCondition) validate() error {
    if c.selector == nil {
        return fmt.Errorf("LabelSelector cannot be nil")
    }
    return nil
}
```
3.	`structuredVolume` Struct:
The `structuredVolume` struct will be updated to include labels as a field. This will allow the volume to be checked against label selector conditions.
```go
type structuredVolume struct {
    capacity     resource.Quantity
    storageClass string
    labels       map[string]string  // Labels field
    nfs          *nFSVolumeSource
    csi          *csiVolumeSource
    volumeType   SupportedVolume
}

func (s *structuredVolume) parsePV(pv *corev1api.PersistentVolume) {
    s.capacity = *pv.Spec.Capacity.Storage()
    s.storageClass = pv.Spec.StorageClassName
    s.labels = pv.Labels // Extracting labels from PersistentVolume

    nfs := pv.Spec.NFS
    if nfs != nil {
        s.nfs = &nFSVolumeSource{Server: nfs.Server, Path: nfs.Path}
    }

    csi := pv.Spec.CSI
    if csi != nil {
        s.csi = &csiVolumeSource{Driver: csi.Driver, VolumeAttributes: csi.VolumeAttributes}
    }

    s.volumeType = getVolumeTypeFromPV(pv)
}
```

