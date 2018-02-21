# Troubleshooting

## heptio-ark namespace stuck terminating / unable to delete backups

Ark v0.7.0 added the ability to delete backups by adding what is called an Ark "finalizer" to each
backup. When you request the deletion of an object that has at least one finalizer, Kubernetes sets
the object's "deletion timestamp" (indicating the object has been marked for deletion), but it does
not immediately delete the object. Instead, Kubernetes only deletes the object when it no longer has
any finalizers. This means that something (Ark, in this case) must process the backup and then
remove the Ark finalizer from it.

Ark versions before v0.7.1 place the Ark server pod in the same namespace as backups, restores,
schedules, and the Ark config. If you try to delete the namespace (`kubectl delete
namespace/heptio-ark`), it's possible that the Ark server pod is deleted before the backups, because
the order of deletions is arbitrary. If this happens, the remaining bacukps will be "stuck"
deleting, because the Ark server pod no longer exists to remove their finalizers.

With v0.7.1, we strongly encourage you to run the Ark server pod in a different namespace than the
one used for backups, schedules, restores, and the Ark config. This is the default configuration as
of v0.7.1.

If you encounter this problem, here is how to fix it. First, make sure you have `jq` installed. Then
run:

```
bash <(kubectl -n heptio-ark get backup -o json | jq -c -r $'.items[] | "kubectl -n heptio-ark patch backup/" + .metadata.name + " -p \'" + (({metadata: {finalizers: ( (.metadata.finalizers // []) - ["gc.ark.heptio.com"]), resourceVersion: .metadata.resourceVersion}}) | tostring) + "\' --type=merge"')
```

This retrieves a list of backups and uses it to generate and run a list of commands that look like:

```
kubectl -n heptio-ark patch backup/my-backup -p '{"metadata":{"finalizers":[],"resourceVersion":"461343"}}' --type=merge
kubectl -n heptio-ark patch backup/some-other-backup -p '{"metadata":{"finalizers":[],"resourceVersion":"461718"}}' --type=merge
```

If you receive errors that patching backups is not allowed, it's possible that the Ark
CustomResourceDefinitions (CRDs) were deleted. You'll need to recreate them (they're in
`examples/common/00-prereqs.yaml`), then follow the steps above.
