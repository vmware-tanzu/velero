# Debugging Restores

* [Example][0]
* [Structure][1]

## Example

When Heptio Ark finishes a Restore, its status changes to "Completed" regardless of whether or not there are issues during the process. The number of warnings and errors are indicated in the output columns from `ark restore get`:

```
NAME                          BACKUP          STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
backup-test-20170726180512    backup-test     Completed   155        76        2017-07-26 11:41:14 -0400 EDT   <none>
backup-test-20170726180513    backup-test     Completed   121        14        2017-07-26 11:48:24 -0400 EDT   <none>
backup-test-2-20170726180514  backup-test-2   Completed   0          0         2017-07-26 13:31:21 -0400 EDT   <none>
backup-test-2-20170726180515  backup-test-2   Completed   0          1         2017-07-26 13:32:59 -0400 EDT   <none>
```

To delve into the warnings and errors into more detail, you can use the `-o` option:
```
kubectl restore get backup-test-20170726180512 -o yaml
```
The output YAML has a `status` field which may look like the following:
```
status:
  errors:
    ark: null
    cluster: null
    namespaces: null 
  phase: Completed
  validationErrors: null
  warnings:
    ark: null
    cluster: null
    namespaces:
      cm1:
      - secrets "default-token-t0slk" already exists
```

## Structure
The `status` field in a Restore's YAML has subfields for `errors` and `warnings`. `errors` appear for incomplete or partial restores. `warnings` appear for non-blocking issues (e.g. the restore looks "normal" and all resources referenced in the backup exist in some form, although some of them may have been pre-existing).

Both `errors` and `warnings` are structured in the same way:

* `ark`: A list of system-related issues encountered by the Ark server (e.g. couldn't read directory).

* `cluster`: A list of issues related to the restore of cluster-scoped resources.

* `namespaces`: A map of namespaces to the list of issues related to the restore of their respective resources.

[0]: #example
[1]: #structure
