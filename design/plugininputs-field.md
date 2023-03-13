# Proposal to add generic pluginInputs field in backup/restore/schedule CRs
- [Proposal to add generic pluginInputs field in backup/restore/schedule CRs](#proposal-to-add-generic-plugininputs-field-in-backuprestoreschedule-crs)
- [Abstract](#abstract)
- [Background](#background)
- [Goals](#goals)
- [Non Goals](#non-goals)
- [Use case examples for other plugins](#use-case-examples-for-other-plugins)
    - [Use case 1: Change StorageClass RIA (velero.io/change-storage-class)](#use-case-1-change-storageclass-ria-veleriochange-storage-class)
    - [Use case 2: Change Image Name (velero.io/change-image-name)](#use-case-2-change-image-name-veleriochange-image-name)
    - [Use case 3: Support for multiple CSI VolumeSnapshotClass (velero.io/csi)](#use-case-3-support-for-multiple-csi-volumesnapshotclass-veleriocsi)
    - [Use case 4:](#use-case-4)
- [Detailed Design](#detailed-design)
- [Alternatives Considered](#alternatives-considered)
- [Security Considerations](#security-considerations)
- [Compatibility](#compatibility)
- [Open Issues](#open-issues)

## Abstract
This proposal aims to add a generic pluginInputs field in the velero CRs (Backup, Schedule, Restore) to allow the user to specify the parameters which are specific to a plugin. This will allow plugin owners to expose their own settings in the velero CRs. 

## Background
Velero plugins (backup item action, restore item action)  provide flexibility to extend velero functionality. However, the current design of the (backup item action, restore item action) plugins does not enable the user to specify the settings specific to a plugin in the backup/restore CR. Plugin owners have to take the route of taking inputs through annotations, configmap with fixed naming/label conventions for global settings. This is not an ideal end use experience since we are bloating annotations, having limitations w.r.t global settings for all backups/schedules etc.

## Goals
- Add pluginInputs field in the velero CRs (Backup, Schedule, Restore) to allow the user to specify the parameters which are specific to a plugin.

# Precedence of this Design principle 
This approach is similar to all current velero / K8s storage design principles.
- **BackupStorageLocation** - we refer the name of the object store plugin provider(for eg "azure"), and below under config, we can specify the config specific to the plugin which is a generic property bag. Only the specific object store plugin can understand the config.
- **CSI based VolumeSnapshotClass** - we refer the name of the CSI driver(for eg "csi.cloud.disk.driver"), and below under parameters, we can specify the parameters specific to the plugin which is a generic property bag. Parameters such as SKU etc are only understood by the specific CSI driver.
- **VolumeSnapshotLocation** - we refer the name of the volume snapshotter plugin provider(for eg "aws"), and below under config, we can specify the config specific to the plugin which is a generic property bag. Properties such as region etc are only understood by the specific volume snapshotter plugin.


### Use case examples for other plugins

#### Use case 1: Change StorageClass RIA (velero.io/change-storage-class)
Change Storage Class plugin, uses a global configmap which is discovered through labels/ annotations which is not ideal customer experience and prohibhit usage of specific settings for each backup/ schedule setup.

The pluginInputs field can be used to pass in the reference the configmap/ secret to use for a particular plugin. This will allow the user to use different configmaps/ secrets for different backups/ schedules.

Example: 
```yaml
spec:
    pluginInputs:
    - name: velero.io/storageclass
    - properties:
        - key: storageclass-mapping-configmap
        - value: configMapNamespace/configMapName
```

#### Use case 2: Change Image Name (velero.io/change-image-name)
This plugin is similar to velero.io/change-storage-class plugin and uses a global configmap. 

Even the code uses below logic and fails if multiple configmaps.
```
return nil, errors.Errorf("found more than one ConfigMap matching label selector %q: %v", opts.LabelSelector, items)
```

Providing reference of configmap in the pluginInputs field will allow the user to use different configmaps for different restores.

#### Use case 3: Support for multiple CSI VolumeSnapshotClass (velero.io/csi)
Refer PR: https://github.com/vmware-tanzu/velero/pull/5774
Refer Issue: https://github.com/vmware-tanzu/velero/issues/5750

Currently the Velero CSI plugin chooses the VolumeSnapshotClass in the cluster that has the same driver name and also has the velero.io/csi-volumesnapshot-class label set on it. This global selection is not sufficient for many use cases.

Example: 
```yaml
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: backup-1
spec:
    pluginInputs:
    - name: velero.io/csi
    - properties:
        - key: csi.cloud.disk.driver
        - value: csi-diskdriver-snapclass
        - key: csi.cloud.file.driver
        - value: csi-filedriver-snapclass
```

#### Use case 4:
<Request for community inputs.>
Many companies currently have their own plugins for specific functionalities. Request to also contribute to proposal with use cases for your plugins.


## Detailed Design

### Plugin Inputs Contract Changes
Approach is to introduce a new field `pluginInputs` in the velero CRs (Backup, Schedule, Restore). This field can be leveraged by all plugins for sending plugin specific settings rather than relying on annotations or global settings which hold across backups.

```go
type PluginInput struct {
    Name string `json:"name"`
    Properties map[string][string] `json:"properties"`
}
```

## Alternatives Considered

## Security Considerations
No security impact.

## Compatibility
Since this is a generic property bag, it is the responsibility of plugin owners and the end users to ensure compatibility of the settings.

*Note*: The rule of thumb for plugin owners which leverage the pluginInputs field is to use the value specified in the pluginInputs field if present, else fallback to the global configmap/ secret / annotation based discovery which was existing behaviour or the plugin.

## Implementation
- Introduce new field in backup/restore / schedule CRDs as per design
- RIA plugins already have "backup/restore" object present in context of the function execution, they can simply change the logic from the current annotation based discovery to use the pluginInputs field and extract the settings from there.

## Open Issues
NA
