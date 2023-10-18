# Proposal to Support JSON Merge Patch and Strategic Merge Patch in Resource Modifiers

- [Proposal to Support JSON Merge Patch and Strategic Merge Patch in Resource Modifiers](#proposal-to-support-json-merge-patch-and-strategic-merge-patch-in-resource-modifiers)
    - [Abstract](#abstract)
    - [Goals](#goals)
    - [Non Goals](#non-goals)
    - [User Stories](#user-stories)
        - [Scenario 1](#scenario-1)
        - [Scenario 2](#scenario-2)
    - [Detailed Design](#detailed-design)
        - [How to choose the right patch type](#how-to-choose-the-right-patch-type)
        - [New Field MergePatches](#new-field-mergepatches)
        - [New Field StrategicPatches](#new-field-strategicpatches)
        - [Conditional Patches in ALL Patch Types](#conditional-patches-in-all-patch-types)
        - [Wildcard Support for GroupResource](#wildcard-support-for-groupresource)
        - [Helper Command to Generate Merge Patch and Strategic Merge Patch](#helper-command-to-generate-merge-patch-and-strategic-merge-patch)
    - [Security Considerations](#security-considerations)
    - [Compatibility](#compatibility)
    - [Implementation](#implementation)
    - [Future Enhancements](#future-enhancements)
    - [Open Issues](#open-issues)

## Abstract
Velero introduced the concept of Resource Modifiers in v1.12.0. This feature allows the user to specify a configmap with a set of rules to modify the resources during restore. The user can specify the filters to select the resources and then specify the JSON Patch to apply on the resource. This feature is currently limited to the operations supported by JSON Patch RFC.
This proposal is to add support for JSON Merge Patch and Strategic Merge Patch in the Resource Modifiers. This will allow the user to use the same configmap to apply JSON Merge Patch and Strategic Merge Patch on the resources during restore.

## Goals
- Allow the user to specify a JSON patch, JSON Merge Patch or Strategic Merge Patch for modification.
- Allow the user to specify multiple JSON Patch, JSON Merge Patch or Strategic Merge Patch.
- Allow the user to specify mixed JSON Patch, JSON Merge Patch and Strategic Merge Patch in the same configmap.

## Non Goals
- Deprecating the existing RestoreItemAction plugins for standard substitutions(like changing the namespace, changing the storage class, etc.)

## User Stories

### Scenario 1
- Alice has some Pods and part of them have an annotation `{"for": "bar"}`.
- Alice wishes to restore these Pods to a different cluster without this annotation.
- Alice can use this feature to remove this annotation during restore.

### Scenario 2
- Bob has a Pod with several containers and one container with name nginx has an image `repo1/nginx`.
- Bob wishes to restore this Pod to a different cluster, but new cluster can not access repo1, so he pushes the image to repo2.
- Bob can use this feature to update the image of container nginx to `repo2/nginx` during restore.

## Detailed Design
- The design and approach is inspired by kubectl patch command and [this doc](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/).
- New fields `MergePatches` and `StrategicPatches` will be added to the `ResourceModifierRule` struct to support all three patch types.
- Only one of the three patch types can be specified in a single `ResourceModifierRule`.
- Add wildcard support for `groupResource` in `conditions` struct.
- The workflow to create Resource Modifier ConfigMap and reference it in RestoreSpec will remain the same as described in document [Resource Modifiers](https://github.com/vmware-tanzu/velero/blob/main/site/content/docs/main/restore-resource-modifiers.md).

### How to choose the right patch type
- [JSON Merge Patch](https://datatracker.ietf.org/doc/html/rfc7386) is a naively simple format, with limited usability. Probably it is a good choice if you are building something small, with very simple JSON Schema.
- [JSON Patch](https://datatracker.ietf.org/doc/html/rfc6902) is a more complex format, but it is applicable to any JSON documents. For a comparison of JSON patch and JSON merge patch, see [JSON Patch and JSON Merge Patch](https://erosb.github.io/post/json-patch-vs-merge-patch/).
- Strategic Merge Patch is a Kubernetes defined patch type, mainly used to process resources of type list. You can replace/merge a list, add/remove items from a list by key, change the order of items in a list, etc. Strategic merge patch is not supported for custom resources. For more details, see [this doc](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/).

### New Field MergePatches
MergePatches is a list to specify the merge patches to be applied on the resource. The merge patches will be applied in the order specified in the configmap. A subsequent patch is applied in order and if multiple patches are specified for the same path, the last patch will override the previous patches.

Example of MergePatches in ResourceModifierRule
```yaml
version: v1
resourceModifierRules:
- conditions:
    groupResource: pods
    namespaces:
    - ns1
  mergePatches:
  - patchData: |
      {
        "metadata": {
          "annotations": {
            "foo": null
          }
        }
      }
```
- The above configmap will apply the Merge Patch to all the pods in namespace ns1 and remove the annotation `foo` from the pods.
- Both json and yaml format are supported for the patchData.

### New Field StrategicPatches
StrategicPatches is a list to specify the strategic merge patches to be applied on the resource. The strategic merge patches will be applied in the order specified in the configmap. A subsequent patch is applied in order and if multiple patches are specified for the same path, the last patch will override the previous patches.

Example of StrategicPatches in ResourceModifierRule
```yaml
version: v1
resourceModifierRules:
- conditions:
    groupResource: pods
    resourceNameRegex: "^my-pod$"
    namespaces:
    - ns1
  strategicPatches:
  - patchData: |
      {
        "spec": {
          "containers": [
            {
              "name": "nginx",
              "image": "repo2/nginx"
            }
          ]
        }
      }
```
- The above configmap will apply the Strategic Merge Patch to the pod with name my-pod in namespace ns1 and update the image of container nginx to `repo2/nginx`.
- Both json and yaml format are supported for the patchData.

### Conditional Patches in ALL Patch Types
Since JSON Merge Patch and Strategic Merge Patch do not support conditional patches, we will use the `test` operation of JSON Patch to support conditional patches in all patch types by adding it to `Conditions` struct in `ResourceModifierRule`.

Example of test in conditions
```yaml
version: v1
resourceModifierRules:
- conditions:
    groupResource: persistentvolumeclaims.storage.k8s.io
    matches:
    - path: "/spec/storageClassName"
      value: "premium"
  mergePatches:
  - patchData: |
      {
        "metadata": {
          "annotations": {
            "foo": null
          }
        }
      }
```
- The above configmap will apply the Merge Patch to all the PVCs in all namespaces with storageClassName premium and remove the annotation `foo` from the PVCs.
- You can specify multiple rules in the `matches` list. The patch will be applied only if all the matches are satisfied.

### Wildcard Support for GroupResource
The user can specify a wildcard for groupResource in the conditions' struct. This will allow the user to apply the patches for all the resources of a particular group or all resources in all groups. For example, `*.apps` will apply to all the resources in the `apps` group, `*` will apply to all the resources in all groups.

### Helper Command to Generate Merge Patch and Strategic Merge Patch
The patchData of Strategic Merge Patch is sometimes a bit complex for user to write. We can provide a helper command to generate the patchData for Strategic Merge Patch. The command will take the original resource and the modified resource as input and generate the patchData.
It can also be used in JSON Merge Patch.

Here is a sample code snippet to achieve this:
```go
package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "web",
					Image: "nginx",
				},
			},
		},
	}
	newPod := pod.DeepCopy()
	patch := client.StrategicMergeFrom(pod)
	newPod.Spec.Containers[0].Image = "nginx1"

	data, _ := patch.Data(newPod)
	fmt.Println(string(data))
	// Output:
	// {"spec":{"$setElementOrder/containers":[{"name":"web"}],"containers":[{"image":"nginx1","name":"web"}]}}
}
```

## Security Considerations
No security impact.

## Compatibility
Compatible with current Resource Modifiers.

## Implementation
- Use "github.com/evanphx/json-patch" to support JSON Merge Patch.
- Use "k8s.io/apimachinery/pkg/util/strategicpatch" to support Strategic Merge Patch.
- Use glob to support wildcard for `groupResource` in `conditions` struct.
- Use `test` operation of JSON Patch to calculate the `matches` in `conditions` struct.

## Future enhancements
- add a Velero subcommand to generate/validate the patchData for Strategic Merge Patch and JSON Merge Patch.
- add jq support for more complex conditions or patches, to meet the situations that the current conditions or patches can not handle. like [this issue](https://github.com/vmware-tanzu/velero/issues/6344)

## Open Issues
N/A
