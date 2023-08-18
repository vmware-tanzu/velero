# Proposal to add support for Resource Modifiers (AKA JSON Substitutions) in Restore Workflow

- [Proposal to add support for Resource Modifiers (AKA JSON Substitutions) in Restore Workflow](#proposal-to-add-support-for-resource-modifiers-aka-json-substitutions-in-restore-workflow)
    - [Abstract](#abstract)
    - [Goals](#goals)
    - [Non Goals](#non-goals)
    - [User Stories](#user-stories)
        - [Scenario 1](#scenario-1)
        - [Scenario 2](#scenario-2)
    - [Detailed Design](#detailed-design)
        - [Reference in velero API](#reference-in-velero-api)
        - [ConfigMap Structure](#configmap-structure)
        - [Operations supported by the JSON Patch library:](#operations-supported-by-the-json-patch-library)
        - [Advance scenarios](#advance-scenarios)
            - [Conditional patches using test operation](#conditional-patches-using-test-operation)
    - [Alternatives Considered](#alternatives-considered)
    - [Security Considerations](#security-considerations)
    - [Compatibility](#compatibility)
    - [Implementation](#implementation)
    - [Future Enhancements](#future-enhancements)
    - [Open Issues](#open-issues)

## Abstract
Currently velero supports substituting certain values in the K8s resources during restoration like changing the namespace, changing the storage class, etc. This proposal is to add generic support for JSON substitutions in the restore workflow. This will allow the user specify filters for particular resources and then specify a JSON patch (operator, path, value) to apply on a resource. This will allow the user to substitute any value in the K8s resource without having to write a new RestoreItemAction plugin for each kind of substitution.

<!-- ## Background -->

## Goals
- Allow the user to specify a GroupKind, Name(optional), JSON patch for modification.
- Allow the user to specify multiple JSON patch.

## Non Goals
- Deprecating the existing RestoreItemAction plugins for standard substitutions(like changing the namespace, changing the storage class, etc.)

## User Stories

### Scenario 1
- Alice has a PVC which is encrypted using a DES(Disk Encryption Set - Azure example) mentioned in the PVC YAML through the StorageClass YAML. 
- Alice wishes to restore this snapshot to a different cluster. The new cluster does not have access to the same DES to provision disk's out of the snapshot.
- She wishes to use a different DES for all the PVCs which use the certain DES.
- She can use this feature to substitute the DES in all StorageClass YAMLs with the new DES without having to create a fresh storageclass, or understanding the name of the storageclass.

### Scenario 2
- Bob has multi zone cluster where nodes are spread across zones. 
- Bob has pinned certain pods to a particular zone using nodeSelector/ nodeaffinity on the pod spec.
- In case of zone outage of the cloudprovider, Bob wishes to restore the workload to a different namespace in the same cluster, but change the zone pinning of the workload.
- Bob can use this feature to substitute the nodeSelector/ nodeaffinity in the pod spec with the new zone pinning to quickly failover the workload to a different zone's nodes.

## Detailed Design
- The design and approach is inspired from [kubectl patch command](https://github.com/kubernetes/kubectl/blob/0a61782351a027411b8b45b1443ec3dceddef421/pkg/cmd/patch/patch.go#L102C2-L104C1)
```bash
# Update a container's image using a json patch with positional arrays
kubectl patch pod valid-pod -type='json' -p='[{"op": "replace", "path": "/spec/containers/0/image", "value":"new image"}]'
```
- The user is expected to create a configmap with the desired Resource Modifications. Then the reference of the configmap will be provided in the RestoreSpec.
- The core restore workflow before creating/updating a particular resource in the cluster will be checked against the filters provided and respective substitutions will be applied on it.

### Reference in velero API 
> Example of Reference to configmap in RestoreSpec
```yaml
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: restore-1
spec:
    resourceModifier:
      refType: Configmap
      ref: resourcemodifierconfigmap
```
> Example CLI Command
```bash
velero restore create --from-backup backup-1 --resource-modifier-configmap resourcemodifierconfigmap
```

### Resource Modifier ConfigMap Structure
- User first needs to provide details on which resources the JSON Substitutions need to be applied. 
    - For this the user will provide 4 inputs - Namespaces(for NS Scoped resources), GroupKind (kind.group format similar to includeResources field in velero) and Name Regex(optional).
    - If the user does not provide the Name, the JSON Substitutions will be applied to all the resources of the given Group and Kind under the given namespaces.

- Further the use will specify the JSON Patch using the structure of kubectl's "JSON Patch" based inputs.
- Sample data in ConfigMap
```yaml
version: v1
resourceModifierRules:
- conditions:
    groupKind: persistentvolumeclaims
    resourceNameRegex: "mysql.*"
    namespaces:
    - bar
    - foo
  patches:
  - operation: replace
    path: "/spec/storageClassName"
    value: "premium"
  - operation: remove
    path: "/metadata/labels/test"
```
- The above configmap will apply the JSON Patch to all the PVCs in the namespaces bar and foo with name starting with mysql. The JSON Patch will replace the storageClassName with "premium" and remove the label "test" from the PVCs.
- The user can specify multiple JSON Patches for a particular resource. The patches will be applied in the order specified in the configmap. A subsequent patch is applied in order and if multiple patches are specified for the same path, the last patch will override the previous patches.
- The user can specify multiple resourceModifierRules in the configmap. The rules will be applied in the order specified in the configmap. 

> Users need to create one configmap in Velero install namespace from a YAML file that defined resource modifiers. The creating command would be like the below:
```bash
kubectl create cm <configmap-name> --from-file <yaml-file> -n velero
```

### Operations supported by the JSON Patch library: 
- add
- remove
- replace
- move
- copy
- test (covered below)

### Advance scenarios
#### **Conditional patches using test operation**
 The `test` operation can be used to check if a particular value is present in the resource. If the value is present, the patch will be applied. If the value is not present, the patch will not be applied. This can be used to apply a patch only if a particular value is present in the resource. For example, if the user wishes to change the storage class of a PVC only if the PVC is using a particular storage class, the user can use the following configmap.
```yaml
version: v1
resourceModifierRules:
- conditions:
    groupKind: persistentvolumeclaims.storage.k8s.io
    resourceNameRegex: ".*"
    namespaces:
    - bar
    - foo
  patches:
  - operation: test
    path: "/spec/storageClassName"
    value: "premium"
  - operation: replace
    path: "/spec/storageClassName"
    value: "standard"
```

## Alternatives Considered
1. JSON Path based addressal of json fields in the resource
    - This was the initial planned approach, but there is no open source library which gives satisfactory edit functionality with support for all operators supported by the JsonPath RFC.
    - We attempted modifying the  [https://kubernetes.io/docs/reference/kubectl/jsonpath/](https://kubernetes.io/docs/reference/kubectl/jsonpath/) but given the complexity of the code it did not make sense to change it since it would become a long term maintainability problem.
1. RestoreItemAction for each kind of standard substitution
    - Not an extensible design. If a new kind of substitution is required, a new RestoreItemAction needs to be written.
1. RIA for JSON Substitution: The approach of doing JSON Substitution through a RestoreItemAction plugin was considered. But it is likely to have performance implications as the plugin will be invoked for all the resources.

## Security Considerations
No security impact.

## Compatibility
Compatibility with existing StorageClass mapping RestoreItemAction and similar plugins needs to be evaluated. 

## Implementation
- Changes in Restore CRD. Add a new field to the RestoreSpec to reference the configmap.
- One example of where code will be modified:  https://github.com/vmware-tanzu/velero/blob/eeee4e06d209df7f08bfabda326b27aaf0054759/pkg/restore/restore.go#L1266 On the obj before Creation, we can apply the conditions to check if the resource is filtered out using given parameters. Then using JsonPatch provided, we can update the resource.
- For Jsonpatch - https://github.com/evanphx/json-patch library is used.
- JSON Patch RFC https://datatracker.ietf.org/doc/html/rfc6902

## Future enhancements
- Additional features such as wildcard support in path, regex match support in value, etc. can be added in future. This would involve forking the https://github.com/evanphx/json-patch library and adding the required features, since those features are not supported by the library currently and are not part of jsonpatch RFC.

## Open Issues
NA
