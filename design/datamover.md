# Add Datamover to Velero
## Problem Statement
Today, Velero supports snapshotting of volumes using a supported plugin. For some volume/plugin types, these snapshots do not provide durability and portability. Velero would like to implement a generic data mover interface that allows vendors to supply their own data mover to be used for compatible storage types.
- Specifically, CSI snapshots are not guaranteed to be durable or portable, and this is preventing the CSI plugin from going GA.
- Kubernetes is moving in such a way that every volume would be backed by a CSI driver. CSI snapshotting allows for a way for a user to take a snapshot, but does not guarantee durability nor portability. We want to provide a generic way for users to guarantee portability and durability of these snapshots for a given provider.
- We also want to allow vendors to provide their own volume data management strategies to be able to work within the velero backup/restore workflow. 

## What is a Data Mover ?
Data Mover defines an interface that enables movement of kubernetes data from cluster to backup storage(e.g. object storage) and vice-versa.

## Goals
- The data mover should be **Pluggable**. This means that a user can leverage different types of movers for the same set of resources as long as the movers conform to a similar interface.
- The interface for Data Mover should support moving data associated with resources **other than** Persistent Volumes
  - Volume Snapshots are an immediate need, and will likely drive most of the original design discussions.
  - Ex. a Data Mover for Kubevirt would move VM data to backup storage. The interface should be generic enough to support this use case.
- The interface for Data Mover should enable publishing its specific configuration parameters for users to have fine-grained control over the movement workflow
  - Examples:
    - Data mover for vsphere volume can configure block vs file mount usage
    - Kubevirt mover can configure export file type
    - Image mover can specify image format type
- End-user interaction with data mover should be exposed through the existing backup/restore operations, additionally 2 new CRDs will be introduced - DataMoverBackup
and DataMoverRestore which would help facilitate data movement and coupling with Velero.

## Non-Goals
- This implementation will **NOT** change the existing Velero backup/restore workflow.
  - It is possible in the future that this interface can be used to replace some legacy Velero implementations, but that is **NOT** a goal for this design/implementation.
- This design of the interface should not include any specific details about (aside from samples):
  - A mover type
    - Although we are focused on volumes for phase 1, the interface should have zero knowledge of the underlying data that needs to be moved.
    - We may provide a way to set the default data mover but still allow users to choose which data mover to user.
  - Mover implementations
    - Ex. How a mover handles volumes at the block vs filesystem level is not relevant to the overarching interface design
    - On the other hand the user will set some mover specific parameters in the interface, thus the user will be aware about some implementation knowledge

## Use Cases
- Data Mover for Persistent Volume Snapshots
  - For a given snapshot type, a capable data mover should be able to clone the snapshot data into some backup target. Of course,  we will be supporting the BSL targets but
  the design will be extensible enough to support other kinds of backup targets as well. 
  - The most urgent use case for this today is CSI snapshots of volumes where the given CSI driver does not guarantee durability and portability. A user creates a CSI snapshot of a volume and wishes to clone this snapshot data into some backup target. The user then would like to be able to restore a volume from this snapshot that exists in the backup target.
- User wants to consume third party vendor data movers
  - Example: Dell Power Protect has their own data mover to handle PVs and would like to integrate this into the native Velero backup workflow as opposed to it being a separate and disconnected process.To do this, Velero’s data mover interface has to support the ability for a vendor to bring their own data mover and have the Velero backup/restore workflow trigger its process.
- User wants to move data associated with resources other than Persistent Volumes
  - Example: User takes a backup of a kubevirt VM. A Kubevirt data mover would be capable of exporting the VM and pushing it to backup storage.
  - Data mover interface should be capable of defining mover strategies for any type of kubernetes resource.
- Data movement lifecycle will be tied up with Velero Backup/Restore.
  - Currently, for a given backup if deleted, all the associated snapshots, restores etc. get deleted, we will follow similar behavior and also delete the moved data when backup is deleted.
- Impact of datamovement on backup/restore status
  - The status of snapshot upload/download will be cascaded to the backup/restore CRs
- Datamovement monitoring
  - Velero will poll the datamover plugins for status on the snapshot operations
- Datamover cancel operation
  - Velero can attempt to cancel a datamover operation by calling the Cancel API call on the BIA/RIA (new version). The plugin can then take any appropriate action as needed. Cancel will be called on backup deletion, and possibly upon reaching timeouts, if timeout support is included in this feature. This can even be a No-Op.
- Datamover usage/registration will be enabled via the Velero install command as a plugin.


