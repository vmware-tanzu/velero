# Two stage Backup API: CreateSnapshot and UploadSnapshot

Status: {Draft}

To minimize the time between pre-hook and post-hook, there should be a separate upload API (`UploadSnapshot`) for the backup. This will ensure that application consitency is retained in a better way by providing a mechanism to minimize the total time required between execution of `Pre-Hook` and `Post-Hook`.

## Goals

- Application consistency should be maintained and downtime of application should be reduced.
- Implement `UploadSnapshot` API as a part of `VolumeSnapshotter` interface.

## Non Goals

- None

## Background

As a Velero user as well as Velero plugin developer, we face challenges, when we try to retain application consistency in case of stateful applications. Velero provides a mechanism to apply `Pre-hooks` and `Post-hooks`. However, when snapshot size is more, in that case Velero takes more time to upload a backup to ObjectStore and unless backup upload operation is finished, it would not release the Post hook on Pod. Hence, application ios are blocked due to upload operation (in some cases this introduces application downtime).
To solve this problem, we have to split the existing backup flow and introduce one more API to upload backup independently.

## High-Level Design

Existing backup flow:    `Pre-hook` -> `CreateSnapshot` -> `Post-hook`

```acsii

+--------------------+
|                    |
|     Pre-Hook       |
|                    |
+---------+----------+
          |
          |
          | < 1 >
          |
          v
+---------+----------+
|   CreateSnapshot   |
|         +          |
|   UploadSnapshot   |
+----------+---------+
           |
           |
           | < 2 >
           |
           v
+----------+---------+
|                    |
|    Post-Hook       |
|                    |
+--------------------+




```

In the above flow, `CreateSnapshot` method creates snapshot and starts upload too. Hence, split this into two APIs: `CreateSnapshot` and `UploadSnapshot`.

Proposed design: `Pre-hook` -> `CreateSnapshot` -> `Post-hook` -> `UploadSnapshot` (Background)

```acsii

+--------------------+
|                    |
|     Pre-Hook       |
|                    |
+---------+----------+
          |
          |
          | < 1 >
          |
          v
+---------+----------+
|                    |
|   CreateSnapshot   |
|                    |
+----------+---------+
           |
           |
           | < 2 >
           |
           v
+----------+---------+
|                    |
|    Post-Hook       |
|                    |
+----------+---------+
           |
           | < 3 >
           |
           v
 +---------+---------+
 |                   |
 |  UploadSnapshot   |
 |   (Background)    |
 +-------------------+


```

## Detailed Design

Velero has a pretty neat feature to maintain the application consistent backups/snapshot called `Pre-hooks` and `Post-Hooks`. However, there's a small change in Velero which needs to be addressed in order to achieve minimum downtime for an application or more consistent application backups. Existing backup flow for a typical stateful application looks like: `Pod` -> `Pre-Hook` --> `PVC` -> `PV` -> `Post-Hook`. This is a mechanism which is followed by Velero block store plugins. In most of the plugins, `CreateSnapshot` method of `VolumeSnapshotter` interface implements logic to upload the snapshot and this is a blocking call for `Post-Hook`. This will introduce more time and will keep application frozen for time equals to snapshot upload time.


To solve the above problem, we can split existing backup flow and introduce one more method called `UploadSnapshot` to `VolumeSnapshotter` interface, which will execute once `Post-Hook` is executed and will keep running in background. This will reduce the time between `Pre-Hook` and `Post-Hook` significantly as well as it will not impact application consistency or downtime.

### Implementation

This change should be implemented in both Velero as well as Velero plugins. However, Velero plugins can continue to rely on current implementation. Only way for Velero plugins to get benifitted from this approach is to implement `UploadSnapshot` method.

Volumesnapshotter is designed for snapshot, this can be either cloud snapshot or local snapshot. As of now, most plugin use volumesnapshotlocations to decide the snapshot type.
We can add new config parameter in velero command line to define snapshot type, through this, velero will decide whether to invoke `UploadSnapshot` or not.

**Implementation plan for Velero:**

- Add `UploadSnapshot` method to `VolumeSnapshotter` interface.
- Generate GRPC code using codegen.
- Implement `UploadSnapshot` method in client and server.
- Call `UploadSnapshot` method from `itemBackupper` as soon as `CreateSnapshot` call returns. `UploadSnapshot` should run as go routine and should be tracked with backup process.
- If Velero plugin doesn't implement `UploadSnapshot` method then handle `Unimplemented` error.

**Implementation plan for Velero Plugin:**

- Update vendor with latest Velero code and make sure that GRPC generated code is present in vendor.
- Move the logic of upload backup to separate `UploadSnapshot` method.

**Method signature:**

```go

func UploadSnapshot(snapshotID string) error {
...
}

```

```Note: Backup progress should not be updated until `UploadSnapshot` is returned.```

More design discussion on this can be found in [issue.](https://github.com/heptio/velero/issues/1519)

## Alternatives Considered

Let plugin developers implement their own upload mechanism on storage orchastrator side. Once `CreateSnapshot` of plugin is called, plugin would take snapshot of Volume, trigger own upload method in background and return the `CreateSnapshot` call as a success to Velero. This is not as per Velero norm and will not be implemented as a part of Velero. In general, this way will not benefit larger part of Velero users who are using restic plugin. Hence, it would be good to have a implementation to trigger upload mechanism from Velero.

## Security Considerations

Implementation proposed in this proposal will not have any security impact on Velero and Velero plugins. Since, this will involve splitting existing APIs.
