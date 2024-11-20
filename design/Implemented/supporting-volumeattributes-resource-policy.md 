# Adding Support For VolumeAttributes in Resource Policy
 
## Abstract
Currently [Velero Resource policies](https://velero.io/docs/main/resource-filtering/#creating-resource-policies) are only supporting "Driver" to be filtered for [CSI volume conditions](https://github.com/vmware-tanzu/velero/blob/8e23752a6ea83f101bd94a69dcf17f519a805388/internal/resourcepolicies/volume_resources_validator.go#L28)

If user want to skip certain CSI volumes based on other volume attributes like protocol or SKU, etc, they can't do it with the current Velero resource policies. It would be convenient if Velero resource policies could be extended to filter on volume attributes along with existing driver filter in the resource policies `conditions` to handle the backup of volumes just by `some specific volumes attributes conditions`.
 
## Background
As of Today, Velero resource policy already provides us the way to filter volumes based on the `driver` name. But it's not enough to handle the volumes based on other volume attributes like protocol, SKU, etc.

## Example:
  - Provision Azure NFS: Define the Storage class with `protocol: nfs` under storage class parameters to provision [CSI NFS Azure File Shares](https://learn.microsoft.com/en-us/azure/aks/azure-files-csi#nfs-file-shares).
  - User wants to back up AFS (Azure file shares) but only want to backup `SMB` type of file share volumes and not `NFS` file share volumes.

## Goals
- We are only bringing additional support in the resource policy to only handle volumes during backup.
- Introducing support for `VolumeAttributes` filter along with `driver` filter in CSI volume conditions to handle volumes.
 
## Non-Goals
- Currently, only handles volumes, and does not support other resources.
 
## Use-cases/Scenarios
### Skip backup volumes by some volume attributes:
Users want to skip PV with the requirements:
- option to skip specified PV on volume attributes type (like Protocol as NFS, SMB, etc)

### Sample Storage Class Used to create such Volumes
```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: azurefile-csi-nfs
provisioner: file.csi.azure.com
allowVolumeExpansion: true
parameters:
  protocol: nfs 
```

## High-Level Design
Modifying the existing Resource Policies code for [csiVolumeSource](https://github.com/vmware-tanzu/velero/blob/8e23752a6ea83f101bd94a69dcf17f519a805388/internal/resourcepolicies/volume_resources_validator.go#L28C6-L28C22) to add the new `VolumeAttributes` filter for CSI volumes and adding validations in existing [csiCondition](https://github.com/vmware-tanzu/velero/blob/8e23752a6ea83f101bd94a69dcf17f519a805388/internal/resourcepolicies/volume_resources.go#L150) to match with volume attributes in the conditions from Resource Policy config map and original persistent volume.

## Detailed Design
The volume resources policies should contain a list of policies which is the combination of conditions and related `action`, when target volumes meet the conditions, the related `action` will take effection.

Below is the API Design for the user configuration:

### API Design
```go
type csiVolumeSource struct {
	Driver string `yaml:"driver,omitempty"`
	// [NEW] CSI volume attributes
	VolumeAttributes map[string]string `yaml:"volumeAttributes,omitempty"`
}
```

The policies YAML config file would look like this:
```yaml
version: v1
volumePolicies:
  - conditions:
      csi:
        driver: disk.csi.azure.com
    action:
      type: skip
  - conditions:
      csi:
        driver: file.csi.azure.com
        volumeAttributes:
          protocol: nfs
    action:
      type: skip`
```
 
### New Supported Conditions
#### VolumeAttributes
Existing CSI Volume Condition can now add `volumeAttributes` which will be key and value pairs.  

 Specify details for the related volume source (currently only csi driver is supported filter)
     ```yaml
     csi: // match volume using `file.csi.azure.com` and with volumeAttributes protocol as nfs
       driver: file.csi.azure.com 
       volumeAttributes:
          protocol: nfs
     ```