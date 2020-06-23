# Ark version 0.7.0 and later: issue with deleting namespaces and backups 

Version 0.7.0 introduced the ability to delete backups. However, you may encounter an issue if you try to 
delete the `heptio-ark` namespace. The namespace can get stuck in a terminating state, and you cannot delete your backups. 
To fix:

1. If you don't have it, [install `jq`][0].

1. Run:
    
    ```bash
    bash <(kubectl -n heptio-ark get backup -o json | jq -c -r $'.items[] | "kubectl -n heptio-ark patch backup/" + .metadata.name + " -p \'" + (({metadata: {finalizers: ( (.metadata.finalizers // []) - ["gc.ark.heptio.com"]), resourceVersion: .metadata.resourceVersion}}) | tostring) + "\' --type=merge"')
    ```

This command retrieves a list of backups, then generates and runs another list of commands that look like:

```
kubectl -n heptio-ark patch backup/my-backup -p '{"metadata":{"finalizers":[],"resourceVersion":"461343"}}' --type=merge
kubectl -n heptio-ark patch backup/some-other-backup -p '{"metadata":{"finalizers":[],"resourceVersion":"461718"}}' --type=merge
```

If you encounter errors that tell you patching backups is not allowed, the Ark
CustomResourceDefinitions (CRDs) might have been deleted. To fix, recreate the CRDs using 
`examples/common/00-prereqs.yaml`, then follow the steps above.

## Mitigate the issue in Ark version 0.7.1 and later

In Ark version 0.7.1, the default configuration runs the Ark server in a different namespace from the namespace 
for backups, schedules, restores, and the Ark config. We strongly recommend that you keep this configuration. 
This approach can help prevent issues with deletes.

## For the curious: why the error occurs

The Ark team added the ability to delete backups by adding a **finalizer** to each
backup. When you request the deletion of an object that has at least one finalizer, Kubernetes sets
the object's deletion timestamp, which indicates that the object is marked for deletion. However, it does
not immediately delete the object. Instead, the object is deleted only when it no longer has
any finalizers. This means that something -- in this case, Ark -- must process the backup and then
remove the Ark finalizer from it.

Ark versions earlier than v0.7.1 place the Ark server pod in the same namespace as backups, restores,
schedules, and the Ark config. If you try to delete the namespace, with `kubectl delete
namespace/heptio-ark`, the Ark server pod might be deleted before the backups, because
the order of deletions is arbitrary. If this happens, the remaining bacukps are stuck in a 
deleting state, because the Ark server pod no longer exists to remove their finalizers.

[0]: https://stedolan.github.io/jq/