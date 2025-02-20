# Add Label Selector as a criteria for Volume Policy

## Abstract
Velero’s volume policies currently support several criteria (such as capacity, storage class, and volume source type) to select volumes for backup. This update extends the design by allowing users to specify required labels on the associated PersistentVolumeClaim (PVC) via a simple key/value map. At runtime, Velero looks up the PVC (when a PV has a ClaimRef), extracts its labels, and compares them with the user-specified map. If all key/value pairs match, the volume qualifies for backup.

## Background 
PersistentVolumes (PVs) in Kubernetes are typically bound to PersistentVolumeClaims (PVCs) that include labels (for example, indicating environment, application, or region). Basing backup policies on these PVC labels enables more precise control over which volumes are processed.

## Goals
- Allow users to specify a simple key/value mapping in the volume policy YAML so that only volumes whose associated PVCs contain those labels are selected.
- Support policies that target volumes based on criteria such as environment=production or region=us-west.

## Non-Goals
- No changes will be made to the actions (skip, snapshot, fs-backup) of the volume policy engine. This update focuses solely on how volumes are selected.
- The design does not support other label selector operations (e.g., NotIn, Exists, DoesNotExist) and only allows for exact key/value matching.

## Use-cases/scenarios
1. Environment-Specific Backup:
- A user wishes to back up only those volumes whose associated PVCs have labels such as `environment=production` and `app=database`.
- The volume policy specifies a pvcLabels map with those key/value pairs; only volumes whose PVCs match are processed.
```yaml
volumePolicies:
  - conditions:
      pvcLabels:
        environment: production
        app: database
    action:
      type: snapshot
```
2. Region-Specific Backup:
- A user operating in multiple regions wants to back up only volumes in the `us-west` region.
- The policy includes `pvcLabels: { region: us-west }`, so only PVs bound to PVCs with that label are selected.
```yaml
volumePolicies:
  - conditions:
      pvcLabels:
        region: us-west
    action:
      type: snapshot
```
3. Automated Label-Based Backups:
- An external system automatically labels new PVCs (for example, `backup: true`).
- A volume policy with `pvcLabels: { backup: true }` ensures that any new volume whose PVC contains that label is included in backup operations.
```yaml
version: v1
volumePolicies:
  - conditions:
      pvcLabels:
        backup: true
    action:
      type: snapshot
```
## High-Level Design

1. Extend Volume Policy Schema:
- The YAML schema for volume conditions is extended to include an optional field pvcLabels of type `map[string]string`.
2. Implement New Condition Type:
- A new condition, `pvcLabelsCondition`, is created. It implements the `volumeCondition` interface and simply compares the user-specified key/value pairs with the actual PVC labels (populated at runtime).
3. Update Structured Volume:
- The internal representation of a volume (`structuredVolume`) is extended with a new field `pvcLabels map[string]string` to store the labels from the associated PVC.
- A new helper function (or an updated parsing function) is used to perform a PVC lookup when a PV has a ClaimRef, populating the pvcLabels field.
4. Integrate with Policy Engine:
- The policy builder is updated to create and add a `pvcLabelsCondition` if the policy YAML contains a `pvcLabels` entry.
- The matching entry point uses the updated `structuredVolume` (populated with PVC labels) to evaluate all conditions, including the new PVC labels condition.
## Detailed Design

1. Update Volume Conditions Schema: Define the conditions struct with a simple map for PVC labels:
```go
// volumeConditions defines the current format of conditions we parse.
type volumeConditions struct {
    Capacity     string            `yaml:"capacity,omitempty"`
    StorageClass []string          `yaml:"storageClass,omitempty"`
    NFS          *nFSVolumeSource  `yaml:"nfs,omitempty"`
    CSI          *csiVolumeSource  `yaml:"csi,omitempty"`
    VolumeTypes  []SupportedVolume `yaml:"volumeTypes,omitempty"`
    // New field: pvcLabels for simple exact-match filtering.
    PVCLabels map[string]string `yaml:"pvcLabels,omitempty"`
}
```
2. New Condition: `pvcLabelsCondition`: Implement a condition that compares expected labels with those on the PVC:
```go
// pvcLabelsCondition defines a condition that matches if the PVC's labels contain all the specified key/value pairs.
type pvcLabelsCondition struct {
    labels map[string]string
}

func (c *pvcLabelsCondition) match(v *structuredVolume) bool {
    if len(c.labels) == 0 {
        return true // No label condition specified; always match.
    }
    if v.pvcLabels == nil {
        return false // No PVC labels found.
    }
    for key, expectedVal := range c.labels {
        if actualVal, exists := v.pvcLabels[key]; !exists || actualVal != expectedVal {
            return false
        }
    }
    return true
}

func (c *pvcLabelsCondition) validate() error {
    // No extra validation needed for a simple map.
    return nil
}
```
3. Update `structuredVolume`: Extend the internal volume representation with a field for PVC labels:
```go
// structuredVolume represents a volume with parsed fields.
type structuredVolume struct {
    capacity     resource.Quantity
    storageClass string
    // New field: pvcLabels stores labels from the associated PVC.
    pvcLabels    map[string]string
    nfs          *nFSVolumeSource
    csi          *csiVolumeSource
    volumeType   SupportedVolume
}
```
4. Update PVC Lookup – `parsePVWithPVC`: Modify the PV parsing function to perform a PVC lookup:
```go
func (s *structuredVolume) parsePVWithPVC(pv *corev1.PersistentVolume, client crclient.Client) error {
    s.capacity = *pv.Spec.Capacity.Storage()
    s.storageClass = pv.Spec.StorageClassName

    if pv.Spec.NFS != nil {
        s.nfs = &nFSVolumeSource{
            Server: pv.Spec.NFS.Server,
            Path:   pv.Spec.NFS.Path,
        }
    }
    if pv.Spec.CSI != nil {
        s.csi = &csiVolumeSource{
            Driver:           pv.Spec.CSI.Driver,
            VolumeAttributes: pv.Spec.CSI.VolumeAttributes,
        }
    }
    s.volumeType = getVolumeTypeFromPV(pv)

    // If the PV is bound to a PVC, look it up and store its labels.
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
5. Update the Policy Builder: Add the new condition to the policy if pvcLabels is provided:
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
        // If a pvcLabels map is provided, add the pvcLabelsCondition.
        if con.PVCLabels != nil && len(con.PVCLabels) > 0 {
            volP.conditions = append(volP.conditions, &pvcLabelsCondition{labels: con.PVCLabels})
        }
        p.volumePolicies = append(p.volumePolicies, volP)
    }
    p.version = resPolicies.Version
    return nil
}
```
6. Update the Matching Entry Point: Use the updated PV parsing that performs a PVC lookup:
```go
func (p *Policies) GetMatchAction(res interface{}, client crclient.Client) (*Action, error) {
    volume := &structuredVolume{}
    switch obj := res.(type) {
    case *corev1.PersistentVolume:
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

Note: The matching loop (p.match(volume)) iterates over all conditions (including our new pvcLabelsCondition) and returns the corresponding action if all conditions match.