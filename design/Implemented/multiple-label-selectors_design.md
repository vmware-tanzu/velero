# Ensure support for backing up resources based on multiple labels
## Abstract
As of today Velero supports filtering of resources based on single label selector per backup. It is desired that Velero
support backing up of resources based on multiple labels (OR logic).

**Note:** This solution is required because kubernetes label selectors only allow AND logic of labels.

## Background
Currently, Velero's Backup/Restore API has a spec field `LabelSelector` which helps in filtering of resources based on
a **single** label value per backup/restore request. For instance, if the user specifies the `Backup.Spec.LabelSelector` as
`data-protection-app: true`, Velero will grab all the resources that possess this label and perform the backup
operation on them. The `LabelSelector` field does not accept more than one labels, and thus if the user want to take
backup for resources consisting of a label from a set of labels (label1 OR label2 OR label3) then the user needs to
create multiple backups per label rule. It would be really useful if Velero Backup API could respect a set of
labels (OR Rule) for a single backup request.

Related Issue: https://github.com/vmware-tanzu/velero/issues/1508

## Goals
- Enable support for backing up resources based on multiple labels (OR Logic) in a single backup config.
- Enable support for restoring resources based on multiple labels (OR Logic) in a single restore config.

## Use Case/Scenario
Let's say as a Velero user you want to take a backup of secrets, but all these secrets do not have one single consistent
label on them. We want to take backup of secrets having any one label in `app=gdpr`, `app=wpa` and `app=ccpa`. Here
we would have to create 3 instances of backup for each label rule. This can become cumbersome at scale.

## High-Level Design
### Addition of `OrLabelSelectors` spec to Velero Backup/Restore API
For Velero to back up resources if they consist of any one label from a set of labels, we would like to add a new spec
field `OrLabelSelectors` which would enable user to specify them. The Velero backup would somewhat look like:

```
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: backup-101
  namespace: openshift-adp
spec:
  includedNamespaces:
  - test
  storageLocation: velero-sample-1
  ttl: 720h0m0s
  orLabelSelectors:
  - matchLabels:
      app=gdpr
  - matchLabels:
      app=wpa
  - matchLabels:
      app=ccpa
```

**Note:** This approach will **not** be changing any current behavior related to Backup API spec `LabelSelector`. Rather we
propose that the label in `LabelSelector` spec and labels in `OrLabelSelectors` should be treated as different Velero functionalities.
Both these fields will be treated as separate Velero Backup API specs. If `LabelSelector` (singular) is present then just match that label.
And if `OrLabelSelectors` is present then match to any label in the set specified by the user. For backup case, if both the `LabelSelector` and `OrLabelSelectors` 
are specified (we do not anticipate this as a real world use-case) then the `OrLabelSelectors` will take precedence, `LabelSelector` will
only be used to filter only when `OrLabelSelectors` is not specified by the user. This helps to keep both spec behaviour independent and not confuse the users. 
This way we preserve the existing Velero behaviour and implement the new functionality in a much cleaner way.
For instance, let's take a look the following cases:

1. Only `LabelSelector` specified: Velero will create a backup with resources matching label `app=protect-db`
```
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: backup-101
  namespace: openshift-adp
spec:
  includedNamespaces:
  - test
  storageLocation: velero-sample-1
  ttl: 720h0m0s
  labelSelector:
  - matchLabels:
      app=gdpr
```
2. Only `OrLabelSelectors` specified: Velero will create a backup with resources matching any label from set `{app=gdpr, app=wpa, app=ccpa}`
```
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: backup-101
  namespace: openshift-adp
spec:
  includedNamespaces:
  - test
  storageLocation: velero-sample-1
  ttl: 720h0m0s
  orLabelSelectors:
  - matchLabels:
      app=gdpr
  - matchLabels:
      app=wpa
  - matchLabels:
      app=ccpa
```

Similar implementation will be done for the Restore API as well.

## Detailed Design
With the Introduction of `OrLabelSelectors` the BackupSpec and RestoreSpec will look like:

BackupSpec:
```
type BackupSpec struct {
[...]
// OrLabelSelectors is a set of []metav1.LabelSelector to filter with
// when adding individual objects to the backup. Resources matching any one
// label from the set of labels will be added to the backup. If empty
// or nil, all objects are included. Optional.
// +optional
OrLabelSelectors []\*metav1.LabelSelector
[...]
}
```

RestoreSpec:
```
type RestoreSpec struct {
[...]
// OrLabelSelectors is a set of []metav1.LabelSelector to filter with
// when restoring objects from the backup. Resources matching any one
// label from the set of labels will be restored from the backup. If empty
// or nil, all objects are included from the backup. Optional.
// +optional
OrLabelSelectors []\*metav1.LabelSelector
[...]
}
```

The logic to collect resources to be backed up for a particular backup will be updated in the `backup/item_collector.go`
around [here](https://github.com/vmware-tanzu/velero/blob/574baeb3c920f97b47985ec3957debdc70bcd5f8/pkg/backup/item_collector.go#L294).

And for filtering the resources to be restored, the changes will go [here](https://github.com/vmware-tanzu/velero/blob/d1063bda7e513150fd9ae09c3c3c8b1115cb1965/pkg/restore/restore.go#L1769)

**Note:**
- This feature will not be exposed via Velero CLI.