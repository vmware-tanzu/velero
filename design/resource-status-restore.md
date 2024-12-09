# Allow Object-Level Resource Status Restore in Velero

## Abstract
This design proposes a way to enhance Velero’s restore functionality by enabling object-level resource status restoration through annotations. 
Currently, Velero allows restoring resource statuses only at a resource type level, which lacks granularity of restoring the status of specific resources. 
By introducing an annotation that controllers can set on individual resource objects, this design aims to improve flexibility and autonomy for users/resource-controllers, providing a more way
to enable resource status restore.


## Background
Velero provides the `restoreStatus` field in the Restore custom resource to specify resource types for status restoration. However, this feature is limited to resource types as a whole, lacking the granularity needed to restore specific objects of a resource type. Resource controllers, especially those managing custom resources with external dependencies, may need to restore status on a per-object basis based on internal logic and dependencies.

This design adds an annotation-based approach to allow controllers to specify status restoration at the object level, enabling Velero to handle status restores more flexibly.

## Goals
- Provide a mechanism to specify the restoration of a resource’s status at an object level.
- Maintain backwards compatibility with existing functionality, allowing gradual adoption of this feature.
- Integrate the new annotation-based objects-level status restore with Velero’s existing resource-type-level `restoreStatus` configuration.

## Non-Goals
- Alter Velero’s existing resource type-level status restoration mechanism.

## Use-Cases/Scenarios

1. Controller managing specific Resources
  - A resource controller identifies that a specific object of a resource should have its status restored due to particular dependencies
  - The controller automatically sets the `velero.io/restore-status: true` annotation on the resource.
  - During restore, Velero restores the status of this object, while leaving other resources unaffected.

2. Resource-Type level Restore
  - A user specifies a resource type (e.g., workflows) in the restoreStatus.includedResources field within the Restore custom resource.
  - Velero restores the status for all objects of the specified resource type, regardless of whether they have the `velero.io/restore-status` annotation.

3. Default Behavior for objects Without the Annotation
  - objects without the `velero.io/restore-status` annotation behave as they currently do: Velero skips their status restoration unless the resource type is specified in the `restoreStatus.includedResources` field.

## High-Level Design

- Object-Level Status Restore Annotation: We are introducing the `velero.io/restore-status` annotation at the resource object level to mark specific objects for status restoration.
  - `true`: Indicates that the status should be restored for this object, even if the resource type is not listed in `restoreStatus.includedResources`.
  - Absence: Indicates no special treatment, and the object will only restore status if its resource type is in `restoreStatus.includedResources`.

- Velero Restore Logic Update: During a restore operation, Velero will:
  - Check the restoreStatus.includedResources field for resource types that should have status restored.
  - For instances in these types, restore status for all objects, regardless of the annotation.
  - For other resource types, check each object for the `velero.io/restore-status: true` annotation, restoring status only for those marked objects.


## Detailed Design

1. Annotation for object-Level Status Restore: The `velero.io/restore-status` annotation will be set on individual resource objects by users/controllers as needed:
```yaml
metadata:
  annotations:
    velero.io/restore-status: "true"
```

2. Restore Logic Modifications: During the restore operation, the restore controller will follow these steps:
   -  Check `restoreStatus.includedResources` to get a list of resource types that should have their statuses restored.
   -  For each object in the specified types, restore the status regardless of any annotation.
   -  For other resource types (not in `restoreStatus.includedResources`), check each object’s annotations.
   -  If an object has `velero.io/restore-status: true`, restore the status for that object.

3. Error Handling: If an invalid annotation value or format is encountered, Velero logs a warning and continues without restoring the status for that object.


## Implementation

We are targeting the implementation of this design for Velero 1.16 release.

Current restoreStatus logic resides here: https://github.com/vmware-tanzu/velero/blob/32a8c62920ad96c70f1465252c0197b83d5fa6b6/pkg/restore/restore.go#L1652

The modified logic would look somewhat like:

```go
// Determine whether to restore status based on resource type configuration and object-level annotation
shouldRestoreStatus := ctx.resourceStatusIncludesExcludes != nil && ctx.resourceStatusIncludesExcludes.ShouldInclude(groupResource.String())

// Check for the object-level annotation on the resource object
objectAnnotation := obj.GetAnnotations()["velero.io/restore-status"]
shouldRestoreStatusForObject := objectAnnotation == "true"

// If either the resource type is included or the object-level annotation indicates restoration
if (shouldRestoreStatus || shouldRestoreStatusForObject) && statusFieldErr != nil {
    err := fmt.Errorf("could not get status to be restored %s: %v", kube.NamespaceAndName(obj), statusFieldErr)
    ctx.log.Errorf(err.Error())
    errs.Add(namespace, err)
    return warnings, errs, itemExists
}

ctx.log.Debugf("status field for %s: exists: %v, should restore: %v, should restore by annotation: %v", newGR, statusFieldExists, shouldRestoreStatus, shouldRestoreStatusForObject)

// Proceed with status restoration if required by either resource type or annotation
if statusFieldExists && (shouldRestoreStatus || shouldRestoreStatusForObject) {
    if err := unstructured.SetNestedField(obj.Object, objStatus, "status"); err != nil {
        ctx.log.Errorf("could not set status field %s: %v", kube.NamespaceAndName(obj), err)
        errs.Add(namespace, err)
        return warnings, errs, itemExists
    }
    obj.SetResourceVersion(createdObj.GetResourceVersion())
    updated, err := resourceClient.UpdateStatus(obj, metav1.UpdateOptions{})
    if err != nil {
        ctx.log.Infof("status field update failed %s: %v", kube.NamespaceAndName(obj), err)
        warnings.Add(namespace, err)
    } else {
        createdObj = updated
    }
}
```

