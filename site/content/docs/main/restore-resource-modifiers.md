---
title: "Restore Resource Modifiers"
layout: docs
---

## Resource Modifiers
Velero provides a generic ability to modify the resources during restore by specifying json patches. The json patches are applied to the resources before they are restored. The json patches are specified in a configmap and the configmap is referenced in the restore command. 

**Creating resource Modifiers**

Below is the two-step of using resource modifiers to modify the resources during restore.
1. Creating resource modifiers configmap

   You need to create one configmap in Velero install namespace from a YAML file that defined resource modifiers. The creating command would be like the below:
   ```bash
   kubectl create cm <configmap-name> --from-file <yaml-file> -n velero
   ```
2. Creating a restore reference to the defined resource policies

   You can create a restore with the flag `--resource-modifier-configmap`, which will apply the defined resource modifiers to the current restore. The creating command would be like the below:
   ```bash
   velero restore create --resource-modifier-configmap <configmap-name>
   ```

**YAML template**

- Yaml template:
```yaml
version: v1
resourceModifierRules:
- conditions:
     groupKind: persistentvolumeclaims
     resourceNameRegex: "^mysql.*$"
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
- You can specify multiple JSON Patches for a particular resource. The patches will be applied in the order specified in the configmap. A subsequent patch is applied in order and if multiple patches are specified for the same path, the last patch will override the previous patches.
- You can can specify multiple resourceModifierRules in the configmap. The rules will be applied in the order specified in the configmap. 

### Operations supported by the JSON Patch RFC: 
- add
- remove
- replace
- move
- copy
- test (covered below)

### Advanced scenarios
#### **Conditional patches using test operation**
 The `test` operation can be used to check if a particular value is present in the resource. If the value is present, the patch will be applied. If the value is not present, the patch will not be applied. This can be used to apply a patch only if a particular value is present in the resource. For example, if you wish to change the storage class of a PVC only if the PVC is using a particular storage class, you can use the following configmap.
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

#### **Other examples**
```yaml
version: v1
resourceModifierRules:
- conditions:
    groupKind: deployments.apps
    resourceNameRegex: "^test-.*$"
    namespaces:
    - bar
    - foo
  patches:
    # Dealing with complex values by escaping the yaml
  - operation: add
    path: "/spec/template/spec/containers/0"
    value: "{\"name\": \"nginx\", \"image\": \"nginx:1.14.2\", \"ports\": [{\"containerPort\": 80}]}"
    # Copy Operator
  - operation: copy
    from: "/spec/template/spec/containers/0"
    path: "/spec/template/spec/containers/1"
```

**Note:** 
- The design and approach is inspired from [kubectl patch command](https://github.com/kubernetes/kubectl/blob/0a61782351a027411b8b45b1443ec3dceddef421/pkg/cmd/patch/patch.go#L102C2-L104C1)
-  Update a container's image using a json patch with positional arrays
kubectl patch pod valid-pod -type='json' -p='[{"op": "replace", "path": "/spec/containers/0/image", "value":"new image"}]'
- Before creating the resource modifier yaml, you can try it out using kubectl patch command. The same commands should work as it is.
