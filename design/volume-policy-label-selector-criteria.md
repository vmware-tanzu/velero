# Add Label Selector as a criteria for Volume Policy

## Abstract
Velero’s volume policies currently support several criteria—such as capacity, storage class, and volume source type—for selecting volumes for backup operations. This update enhances that design by adding a label selector condition that uses Kubernetes native LabelSelector syntax to filter volumes based on the labels of their associated PVCs. This enables users to back up volumes more granularly (for example, by environment, application, or region) while preserving backward compatibility.
## Background 
PersistentVolumes(PVs) in Kubernetes are often bound to PersistentVolumeClaims(PVCs) that carry labels representing attributes like environment, application, or region. Matching backup policies against these PVC labels—provides greater flexibility and aligns with how many users manage their workloads. This design entails:
- Adopting the Kubernetes LabelSelector API for expressing selection criteria.
- Performing a runtime lookup of the PVC (when a PV has a ClaimRef) so that the PVC’s labels are used during policy evaluation.

## Goals
- Enable users to specify a PVC label selector in the volume policy so that only volumes whose associated PVCs meet the criteria are backed up.
- Allow backup policies to be scoped by criteria such as `environment=production` or `region=us-west` based on the labels on the PVC.
- Integrate the PVC label selector condition with the existing volume policy conditions (capacity, storage class, NFS, CSI, volume type) without breaking backward compatibility.

## Non-Goals
- No changes will be made to the actions (skip, snapshot, fs-backup) of the volume policy engine. This update focuses solely on how volumes are selected.
- We are not overhauling the entire volume policy engine. Instead, we are extending the condition matching logic to include PVC label selection using a native label selector.

## Use-cases/scenarios
1. Environment-Specific Backup:
- A user wishes to back up only those volumes whose associated PVCs have labels such as `environment=production` and `app=database`.
- The volume policy includes a PVC label selector condition that selects only PVCs matching these criteria.
2. Region-Specific Backup:
- A user operates across multiple regions and desires to back up only volumes in the `us-west` region.
- The policy specifies a PVC label selector for `region=us-west`, so only PVs bound to PVCs with that label are selected.
3. Automated Label-Based Backups:
- An external system/controller dynamically labels new PVCs (e.g., `backup=true`).
- A PVC label selector condition in the volume policy ensures that any new volume whose PVC carries the matching label is automatically included in the backup.

## High-Level Design

1. Extend Volume Policy Schema:
- The schema for volume conditions is extended to include an optional field (e.g., `pvcLabelSelector`) that accepts a Kubernetes `LabelSelector` object.
2. Implement New Condition Type:
- A new condition, `pvcLabelSelectorCondition`, is introduced that implements the `volumeCondition` interface. Its `match()` method converts the provided `LabelSelector` into a runtime selector and applies it to the PVC labels stored in the volume’s internal representation.
- Its `validate()` method ensures the selector is provided and well formed.
3. Update Structured Volume:
- The internal `structuredVolume` struct is updated with a new field (e.g., `pvcLabels`) to hold labels from the associated PVC.
- A new helper method (e.g. `parsePVWithPVC`) or modify the existing `parsePV`  is defined to look up the PVC (using the controller-runtime client) and populate the `pvcLabels` field.
4. Integrate with Policy Engine:
- The policy builder (in `BuildPolicy`) is updated to add a `pvcLabelSelectorCondition` whenever the volume policy contains a `pvcLabelSelector`.
- The matching logic (in `GetMatchAction`) uses the enhanced `structuredVolume` (populated via PVC lookup) to evaluate all conditions—including the new PVC label selector condition.
## Detailed Design

1. Extend Volume Policy conditions: Add `PVCLabelSelector` field 
```go
// volumeConditions defines the current format of conditions we parse.
type volumeConditions struct {
    Capacity         string            `yaml:"capacity,omitempty"`
    StorageClass     []string          `yaml:"storageClass,omitempty"`
    NFS              *nFSVolumeSource  `yaml:"nfs,omitempty"`
    CSI              *csiVolumeSource  `yaml:"csi,omitempty"`
    VolumeTypes      []SupportedVolume `yaml:"volumeTypes,omitempty"`
    // New field: pvcLabelSelector for matching PVC labels.
    PVCLabelSelector *metav1.LabelSelector `yaml:"pvcLabelSelector,omitempty"`
}

```
2. New Condition: `pvcLabelSelectorCondition`: create the new condition type which implements the `volumeCondition` interface:
```go
// pvcLabelSelectorCondition holds the Kubernetes label selector for PVCs.
type pvcLabelSelectorCondition struct {
    selector *metav1.LabelSelector 
}

// match converts the LabelSelector to a runtime selector and checks if the PVC’s labels match.
func (c *pvcLabelSelectorCondition) match(v *structuredVolume) bool {
    // If the volume’s associated PVC labels are not available, then no match.
    if v.pvcLabels == nil {
        return false
    }
    sel, err := metav1.LabelSelectorAsSelector(c.selector)
    if err != nil {
        return false
    }
    return sel.Matches(labels.Set(v.pvcLabels))
}

// validate ensures that the selector is non-nil.
func (c *pvcLabelSelectorCondition) validate() error {
    if c.selector == nil {
        return fmt.Errorf("PVCLabelSelector cannot be nil")
    }
    return nil
}

```
3. Update the `structuredVolume` Struct: Enhance the internal representation of a volume to include a field for PVC labels.
```go
type structuredVolume struct {
    capacity     resource.Quantity
    storageClass string
    // New field: pvcLabels contains labels from the associated PersistentVolumeClaim.
    pvcLabels    map[string]string
    nfs          *nFSVolumeSource
    csi          *csiVolumeSource
    volumeType   SupportedVolume
}
```

We also need to changes related to PVC lookup and parsing. (Change `parsePV` method to `parsePVWithPVC`)
```go
func (s *structuredVolume) parsePVWithPVC(pv *corev1.PersistentVolume, client crclient.Client) error {
    s.capacity = *pv.Spec.Capacity.Storage()
    s.storageClass = pv.Spec.StorageClassName

    if pv.Spec.NFS != nil {
        s.nfs = &nFSVolumeSource{Server: pv.Spec.NFS.Server, Path: pv.Spec.NFS.Path}
    }
    if pv.Spec.CSI != nil {
        s.csi = &csiVolumeSource{Driver: pv.Spec.CSI.Driver, VolumeAttributes: pv.Spec.CSI.VolumeAttributes}
    }
    s.volumeType = getVolumeTypeFromPV(pv)

    // New changes: Lookup the PVC using the ClaimRef, if available.
    if pv.Spec.ClaimRef != nil {
        pvc := &corev1.PersistentVolumeClaim{}
        err := client.Get(context.Background(), crclient.ObjectKey{
            Namespace: pv.Spec.ClaimRef.Namespace,
            Name:      pv.Spec.ClaimRef.Name,
        }, pvc)
        if err != nil {
            return errors.Wrap(err, "failed to get PVC for PV")
        }
        s.pvcLabels = pvc.Labels
    }
    return nil
}

```
4. Update the Policy Builder: Update the `BuildPolicy` function to include the new PVC label selector condition when specified
```go
func (p *Policies) BuildPolicy(resPolicies *ResourcePolicies) error {
    for _, vp := range resPolicies.VolumePolicies {
        con, err := unmarshalVolConditions(vp.Conditions)
        if err != nil {
            return errors.WithStack(err)
        }
        volCap, err := parseCapacity(con.Capacity)
        if err != nil {
            return errors.WithStack(err)
        }
        var volP volPolicy
        volP.action = vp.Action
        volP.conditions = append(volP.conditions, &capacityCondition{capacity: *volCap})
        volP.conditions = append(volP.conditions, &storageClassCondition{storageClass: con.StorageClass})
        volP.conditions = append(volP.conditions, &nfsCondition{nfs: con.NFS})
        volP.conditions = append(volP.conditions, &csiCondition{csi: con.CSI})
        volP.conditions = append(volP.conditions, &volumeTypeCondition{volumeTypes: con.VolumeTypes})
        // If a PVC label selector is provided, add the corresponding condition.
        if con.PVCLabelSelector != nil {
            volP.conditions = append(volP.conditions, &pvcLabelSelectorCondition{selector: con.PVCLabelSelector})
        }
        p.volumePolicies = append(p.volumePolicies, volP)
    }
    p.version = resPolicies.Version
    return nil
}
```

5. Update the Matching Entry Point: Modify the `GetMatchAction` function so that when a PV is processed, it uses the new `parsePVWithPVC` method to populate the PVC labels before evaluating all conditions.
```go
func (p *Policies) GetMatchAction(res interface{}, client crclient.Client) (*Action, error) {
    volume := &structuredVolume{}
    switch obj := res.(type) {
    case *corev1.PersistentVolume:
		// New changes parsePV() <-> parsePVWithPVC()
        if err := volume.parsePVWithPVC(obj, client); err != nil {
            return nil, errors.Wrap(err, "failed to parse PV with PVC lookup")
        }
    case *corev1.Volume:
        volume.parsePodVolume(obj)
    default:
        return nil, errors.New("failed to convert object")
    }
    return p.match(volume), nil
}
```
The matching loop (which iterates over all conditions) remains unchanged. The new PVC label selector condition is now evaluated along with the other conditions.

