# Proposal to add support for Multiple VolumeSnapshotClasses in CSI Plugin

- [Proposal to add support for Multiple VolumeSnapshotClasses in CSI Plugin](#proposal-to-add-support-for-multiple-volumesnapshotclasses-in-csi-plugin)
	- [Abstract](#abstract)
	- [Background](#background)
	- [Goals](#goals)
	- [Non Goals](#non-goals)
    - [User Stories](#user-stories)
        - [Scenario 1](#scenario-1)
        - [Scenario 2](#scenario-2)
    - [Detailed Design](#detailed-design)
        - [Plugin Inputs Contract Changes](#plugin-inputs-contract-changes)
        - [Using Plugin Inputs for CSI Plugin](#using-plugin-inputs-for-csi-plugin)
        - [Annotations overrides on PVC for CSI Plugin](#annotations-overrides-on-pvc-for-csi-plugin)
        - [Using Plugin Inputs for Other Plugins](#using-plugin-inputs-for-other-plugins)
    - [Alternatives Considered](#alternatives-considered)
    - [Security Considerations](#security-considerations)
    - [Compatibility](#compatibility)
    - [Implementation](#implementation)
    - [Open Issues](#open-issues)


## Abstract
Currently the Velero CSI plugin chooses the VolumeSnapshotClass in the cluster that has the same driver name and also has the velero.io/csi-volumesnapshot-class label set on it. This global selection is not sufficient for many use cases. This proposal is to add support for multiple VolumeSnapshotClasses in CSI Plugin where the user can specify the VolumeSnapshotClass to use for a particular driver and backup.


## Background
The Velero CSI plugin chooses the VolumeSnapshotClass in the cluster that has the same driver name and also has the velero.io/csi-volumesnapshot-class label set on it. This global selection is not sufficient for many use cases. For example, if a cluster has multiple VolumeSnapshotClasses for the same driver, the user may want to use a VolumeSnapshotClass that is different from the default one. The user might also have different schedules set up for backing up different parts of the cluster and might wish to use different VolumeSnapshotClasses for each of these backups. 

## Goals
- Allow the user to specify the VolumeSnapshotClass to use for a particular driver and backup.

## Non Goals
- Deprecating existing VSC selection behaviour. (The current behaviour will remain the default behaviour if the user does not specify the VolumeSnapshotClass to use for a particular driver and backup.)


## User Stories

### Scenario 1
- Consider Alice is a cluster admin and has a cluster with multiple VolumeSnapshotClasses for the same driver. Each VSC stores the snapshots taken in different ResourceGroup(Azure equivalent). 
- Alice has configured multiple scheduled backups each covering a different set of namespaces, representing different apps owned by different teams. 
- Alice wants to use a different VolumeSnapshotClass for each backup such that each snapshot goes in it's respective ResourceGroup to simply management of snapshots(COGS, RBAC etc).
- In current velero, Alice can't achieve this as the CSI plugin will use the default VolumeSnapshotClass for the driver and all snapshots will go in the same ResourceGroup.
- Proposed design will allow Alice to achieve this by specifying the VolumeSnapshotClass to use for a particular driver and backup/schedule.

## Scenario 2
- Bob is a cluster admin has PVCs storing different types of data. 
- Most of the PVCs are used for storing non-sensitive application data. But certain PVCs store critical financial data.
- For such PVCs Bob wants to use a VolumeSnapshotClass with certain encryption related parameters set. 
- In current velero, Bob can't achieve this as the CSI plugin will use the default VolumeSnapshotClass for the driver and all snapshots will be taken using the same VolumeSnapshotClass.
- Proposed design will allow Bob to achieve this by overriding the VolumeSnapshotClass to use for a particular driver and backup/schedule using annotations on those specific PVCs.


## Detailed Design

### Staged Approach: 

### Stage 1 Approach
#### Through Annotations
    1. **Support VolumeSnapshotClass selection at backup/schedule level**
    The user can  annotate the backup/ schedule with driver and VolumeSnapshotClass name. The CSI plugin will use the VolumeSnapshotClass specified in the annotation. If the annotation is not present, the CSI plugin will use the default VolumeSnapshotClass for the driver.

        *example annotation on backup/schedule:*
        ```yaml
        apiVersion: velero.io/v1
        kind: Backup
        metadata:
        name: backup-1
        annotations:
            velero.io/csi-volumesnapshot-class_csi.cloud.disk.driver: csi-diskdriver-snapclass
            velero.io/csi-volumesnapshot-class_csi.cloud.file.driver: csi-filedriver-snapclass
            velero.io/csi-volumesnapshot-class_<driver name>: csi-snapclass
        ```

         To query the annotations on a backup: "velero.io/csi-volumesnapshot-class_'driver name'" - where driver names comes from the PVC's driver.

    2. **Support VolumeSnapshotClass selection at PVC level**
    The user can annotate the PVCs with driver and VolumeSnapshotClass name. The CSI plugin will use the VolumeSnapshotClass specified in the annotation. If the annotation is not present, the CSI plugin will use the default VolumeSnapshotClass for the driver. If the VolumeSnapshotClass provided is of a different driver, the CSI plugin will use the default VolumeSnapshotClass for the driver.

        *example annotation on PVC:*
        ```yaml
        apiVersion: v1
        kind: PersistentVolumeClaim
        metadata:
        name: pvc-1
        annotations:
            velero.io/csi-volumesnapshot-class: csi-diskdriver-snapclass
            
        ```
    
    Consider this as a override option in conjunction to part 1.

**Note**: The user has to annotate the PVCs or backups with the VolumeSnapshotClass to use for each driver. This is not ideal for the user experience.
    - **Mitigation**: We can extend Velero CLI to also annotate backups/schedules with the VolumeSnapshotClass to use for each driver. This will make it easier for the user to annotate the backups/schedules. This mitigation is not for the PVCs though, since PVCs is anyways a specific use case. Similar to : " kubectl run --image myimage --annotations="foo=bar" --annotations="another=one" mypod"
    We can add support for  - velero backup create my-backup --annotations "velero.io/csi:csi.cloud.disk.driver=csi-diskdriver-snapclass"

### Stage 2 Approach
The above annotations route is to get started and for initial design closure/ implementation, north star is to either introduce CSI specific fields (considering that CSI might be a very core part of velero going forward) in the backup/restore CR OR leverage the pluginInputs field as being tracked in: https://github.com/vmware-tanzu/velero/pull/5981

Refer section Alternatives 2. **Through generic property bag in the velero contracts**: in the design doc for more details on the pluginInputs field.


## Alternatives Considered
1. **Through CSI Specific Fields in Velero contracts**

    **Considerations**
    - Since CSI snapshotting is done through the plugin, we don't intend to bloat up the Backup Spec with CSI specific fields. 
    - But considering that CSI Snapshotting is the way forward, we can debate if we should add a CSI section to the Backup Spec.


    **Approach**: Similar to VolumeSnapshotLocation param in the Backup Spec, we can add a VolumeSnapshotClass param in the Backup Spec. This will allow the user to specify the VolumeSnapshotClass to use for the backup. The CSI plugin will use the VolumeSnapshotClass specified in the Backup Spec. If the VolumeSnapshotClass is not specified, the CSI plugin will use the default VolumeSnapshotClass for the driver.

    *example of VolumeSnapshotClass param in the Backup Spec:*
    ```yaml
    apiVersion: velero.io/v1
    kind: Backup
    metadata:
    name: backup-1
    spec:
        csiParameters: 
            volumeSnapshotClasses: 
                driver: csi.cloud.disk.driver
                snapClass: csi-diskdriver-snapclass
            timeout: 10m
    ```

1. **Through changes in velero contracts**
    1. **Through configmap references.**
    Currently even the storageclass mapping plugin expects the user to create a configmap which is used globally, and fetched through labels. This behaviour has same issue as the VolumeSnapshotClass selection. We can introduce a field in the velero contracts which allow passing configmap references for each plugin. And then the plugin can honour the configmap passed in as reference. The configmap can be used to pass the VolumeSnapshotClass to use for the backup, and also other parameters to tweak. This can help in making plugins more flexible while not depending on global behaviour.
    

        *example of configmap reference in the velero contracts:*
        ```yaml
        apiVersion: velero.io/v1
        kind: Backup
        metadata:
        name: backup-1
        spec:
            configmapRefs:
            - name: csi-volumesnapshotclass-configmap
            - namespace: velero
            - plugin: velero.io/csi
         ```

    2. **Through generic property bag in the velero contracts**: We can introduce a field in the velero contracts which allow passing a generic property bag for each plugin. And then the plugin can honour the property bag passed in.


        *example of property bag in the velero contracts:*
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

    **Note**: Both these approaches can also be used to tweak other parameters such as CSI Snapshotting Timeout/intervals. And further can be used by other plugins.


## Security Considerations
No security impact.

## Compatibility
Existing behaviour of csi plugin will be retained where it fetches the VolumeSnapshotClass through the label. This will be the default behaviour if the user does not specify the VolumeSnapshotClass.

## Implementation
TBD based on closure of high level design proposals.

## Open Issues
NA
