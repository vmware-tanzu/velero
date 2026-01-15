# Allow Object-Level Resource Status Restore in Velero

## Abstract
This design proposes a way to enhance Velero’s restore functionality by enabling object-level resource status restoration through annotations. 
Currently, Velero allows restoring resource statuses only at a resource type level, which lacks granularity of restoring the status of specific resources. 
By introducing an annotation that controllers can set on individual resource objects, this design aims to improve flexibility and autonomy for users/resource-controllers, providing a more way
to enable resource status restore.


## Background
Velero provides the `restoreStatus` field in the Restore API to specify resource types for status restoration. However, this feature is limited to resource types as a whole, lacking the granularity needed to restore specific objects of a resource type. Resource controllers, especially those managing custom resources with external dependencies, may need to restore status on a per-object basis based on internal logic and dependencies.

This design adds an annotation-based approach to allow controllers to specify status restoration at the object level, enabling Velero to handle status restores more flexibly.

## Goals
- Provide a mechanism to specify the restoration of a resource’s status at an object level.
- Maintain backwards compatibility with existing functionality, allowing gradual adoption of this feature.
- Integrate the new annotation-based objects-level status restore with Velero’s existing resource-type-level `restoreStatus` configuration.

## Non-Goals
- Alter Velero’s existing resource type-level status restoration mechanism for resources without annotations.

## Use-Cases/Scenarios

1. Controller managing specific Resources
  - A resource controller identifies that a specific object of a resource should have its status restored due to particular dependencies
  - The controller automatically sets the `velero.io/restore-status: true` annotation on the resource.
  - During restore, Velero restores the status of this object, while leaving other resources unaffected.
  - The status for the annotated object will be restored regardless of its inclusion/exclusion in `restoreStatus.includedResources`

2. A specific object must not have its status restored even if its included in `restoreStatus.includedResources`
  - A user specifies a resource type in the `restoreStatus.includedResources` field within the Restore custom resource.
  - A particular object of that resource type is annotated with `velero.io/restore-status: false` by the user.
  - The status of the annotated object will not restored even though its included in `restoreStatus.includedResources` because annotation is `false` and it takes precedence.

4. Default Behavior for objects Without the Annotation
  - Objects without the `velero.io/restore-status` annotation behave as they currently do: Velero skips their status restoration unless the resource type is specified in the `restoreStatus.includedResources` field.

## High-Level Design

- Object-Level Status Restore Annotation: We are introducing the `velero.io/restore-status` annotation at the resource object level to mark specific objects for status restoration.
  - `true`: Indicates that the status should be restored for this object
  - `false`: Skip restoring status for this specific object
  - Invalid or missing annotations defer to the meaning of existing resource type-level logic.

- Restore logic precedence: 
  - Annotations take precedence when they exist with valid values (`true` or `false`).
  - Restore spec `restoreStatus.includedResources` is only used when annotations are invalid or missing.

- Velero Restore Logic Update: During a restore operation, Velero will:
  - Extend the existing restore logic to parse and prioritize annotations introduced in this design.
  - Update resource objects accordingly based on their annotation values or fallback configuration.


## Detailed Design

- Annotation for object-Level Status Restore: The `velero.io/restore-status` annotation will be set on individual resource objects by users/controllers as needed:
```yaml
metadata:
  annotations:
    velero.io/restore-status: "true"
```

- Restore Logic Modifications: During the restore operation, the restore controller will follow these steps:
   - Parse the `restoreStatus.includedResources` spec to determine resource types eligible for status restoration.
   - For each resource object:
     - Check for the `velero.io/restore-status` annotation.
     - If the annotation value is:
       - `true`: Restore the status of the object
       - `false`: Skip restoring the status of the object
     - If the annotation is invalid or missing:
       - Default to the `restoreStatus.includedResources` configuration


## Implementation

We are targeting the implementation of this design for Velero 1.16 release.

Current restoreStatus logic resides here: https://github.com/vmware-tanzu/velero/blob/32a8c62920ad96c70f1465252c0197b83d5fa6b6/pkg/restore/restore.go#L1652

The modified logic would look somewhat like:

```go
// Determine whether to restore status from resource type configuration
shouldRestoreStatus := ctx.resourceStatusIncludesExcludes != nil && ctx.resourceStatusIncludesExcludes.ShouldInclude(groupResource.String())

// Check for object-level annotation
annotations := obj.GetAnnotations()
objectAnnotation := annotations["velero.io/restore-status"]
annotationValid := objectAnnotation == "true" || objectAnnotation == "false"

// Determine restore behavior based on annotation precedence
shouldRestoreStatus = (annotationValid && objectAnnotation == "true") || (!annotationValid && shouldRestoreStatus)

ctx.log.Debugf("status field for %s: exists: %v, should restore: %v (by annotation: %v)", newGR, statusFieldExists, shouldRestoreStatus, annotationValid)

if shouldRestoreStatus && statusFieldExists {
    if err := unstructured.SetNestedField(obj.Object, objStatus, "status"); err != nil {
        ctx.log.Errorf("Could not set status field %s: %v", kube.NamespaceAndName(obj), err)
        errs.Add(namespace, err)
        return warnings, errs, itemExists
    }
    obj.SetResourceVersion(createdObj.GetResourceVersion())
    updated, err := resourceClient.UpdateStatus(obj, metav1.UpdateOptions{})
    if err != nil {
        ctx.log.Infof("Status field update failed %s: %v", kube.NamespaceAndName(obj), err)
        warnings.Add(namespace, err)
	} else {
        createdObj = updated
    }
}
```

