# Backup Reference

## Exclude Specific Items from Backup

It is possible to exclude individual items from being backed up, even if they match the resource/namespace/label selectors defined in the backup spec. To do this, label the item as follows:

```bash
kubectl label -n <ITEM_NAMESPACE> <RESOURCE>/<NAME> velero.io/exclude-from-backup=true
```

Please note, in v1.1.0 the label will not be honoured for "additional items" eg; PVCs/PVs that are in use by a pod that is being backed up. If this scenario is required, consider upgrading to v1.2.0 or later.
