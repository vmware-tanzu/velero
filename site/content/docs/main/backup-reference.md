---
title: "Backup Reference"
layout: docs
---

## Exclude Specific Items from Backup

It is possible to exclude individual items from being backed up, even if they match the resource/namespace/label selectors defined in the backup spec. To do this, label the item as follows:

```bash
kubectl label -n <ITEM_NAMESPACE> <RESOURCE>/<NAME> velero.io/exclude-from-backup=true
```

## Specify Backup Orders of Resources of Specific Kind

To backup resources of specific Kind in a specific order, use option --ordered-resources to specify a mapping Kinds to an ordered list of specific resources of that Kind.  Resource names are separated by commas and their names are in format 'namespace/resourcename'. For cluster scope resource, simply use resource name. Key-value pairs in the mapping are separated by semi-colon.  Kind name is in plural form.

```bash
velero backup create backupName --include-cluster-resources=true --ordered-resources 'pods=ns1/pod1,ns1/pod2;persistentvolumes=pv4,pv8' --include-namespaces=ns1
velero backup create backupName --ordered-resources 'statefulsets=ns1/sts1,ns1/sts0' --include-namespaces=ns1
```
## Set Scheduled Backups

The **schedule** operation allows you to back up your data at recurring intervals. These intervals are specified by a Cron expression.

```
velero schedule create NAME --schedule [flags]
```

For example, to create a backup that runs every day at 3 am run

```
velero create schedule backupName --schedule="0 0 3 ? * *"
```

This command will create the backup within Velero, but it will not trigger until the next time it's specified interval, 3am. Scheduled backups are saved with the name `<SCHEDULE NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*.

Once you create the backup, you can then trigger it whenever you want by running

```
velero backup create --from-schedule backupName
```

This command will immediately trigger a new backup based on your template for `backupName`. This will not affect the backup schedule, and another backup will trigger at the scheduled time.
