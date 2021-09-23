## v1.7.0
### 2021-09-07

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.7.0

### Container Image
`velero/velero:v1.7.0`

### Documentation
https://velero.io/docs/v1.7/

### Upgrading
https://velero.io/docs/v1.7/upgrade-to-1.7/

### Highlights

#### Distroless images

The Velero container images now use [distroless base images](https://github.com/GoogleContainerTools/distroless).
Using distroless images as the base ensures that only the packages and programs necessary for running Velero are included.
Unrelated libraries and OS packages, that often contain security vulnerabilities, are now excluded.
This change reduces the size of both the server and restic restore helper image by approximately 62MB.

As the [distroless](https://github.com/GoogleContainerTools/distroless) images do not contain a shell, it will no longer be possible to exec into Velero containers using these images.

#### New "debug" command

This release introduces the new `velero debug` command.
This command collects information about a Velero installation, such as pod logs and resources managed by Velero, in a tarball which can be provided to the Velero maintainer team to help diagnose issues.

### All changes

  * Distinguish between different unnamed node ports when preserving (#4026, @sseago)
  * Validate namespace in Velero backup create command (#4057, @codegold79)
  * Empty the "ClusterIPs" along with "ClusterIP" when "ClusterIP" isn't "None" (#4101, @ywk253100)
  * Add a RestoreItemAction plugin (`velero.io/apiservice`) which skips the restore of any `APIService` which is managed by Kubernetes. These are identified using the `kube-aggregator.kubernetes.io/automanaged` label. (#4028, @zubron)
  * Change the base image to distroless (#4055, @ywk253100)
  * Updated the version of velero/velero-plugin-for-aws version from v1.2.0 to v1.2.1 (#4064, @kahirokunn)
  * Skip the backup and restore of DownwardAPI volumes when using restic. (#4076, @zubron)
  * Bump up Go to 1.16 (#3990, @reasonerjt)
  * Fix restic error when volume is emptyDir and Pod not running (#3993, @mahaupt)
  * Select the velero deployment with both label and container name (#3996, @ywk253100)
  * Wait for the namespace to be deleted before removing the CRDs during uninstall. This deprecates the `--wait` flag of the `uninstall` command (#4007, @ywk253100)
  * Use the cluster preferred CRD API version when polling for Velero CRD readiness. (#4015, @zubron)
  * Implement velero debug (#4022, @reasonerjt)
  * Skip the restore of volumes that originally came from a projected volume when using restic. (#3877, @zubron)
  * Run the E2E test with kind(provision various versions of k8s cluster) and MinIO on Github Action (#3912, @ywk253100)
  * Fix -install-velero flag for e2e tests (#3919, @jaidevmane)
  * Upgrade Velero ClusterRoleBinding to use v1 API (#3926, @jenting)
  * enable e2e tests to choose crd apiVersion (#3941, @sseago)
  * Fixing multipleNamespaceTest bug - Missing expect statement in test (#3983, @jaidevmane)
  * Add --client-page-size flag to server to allow chunking Kubernetes API LIST calls across multiple requests on large clusters (#3823, @dharmab)
  * Fix CR restore regression introduced in 1.6 restore progress. (#3845, @sseago)
  * Use region specified in the BackupStorageLocation spec when getting restic repo identifier. Originally fixed by @jala-dx in #3617. (#3857, @zubron)
  * skip backuping projected volume when using restic (#3866, @alaypatel07)
  * Install Kubernetes preferred CRDs API version (v1beta1/v1). (#3614, @jenting)
  * Add Label to BackupSpec so that labels can explicitly be provided to Schedule.Spec.Template.Metadata.Labels which will be reflected on the backups created. (#3641, @arush-sal)
  * Add PVC UID label to PodVolumeRestore (#3792, @sseago)
  * Support pulling plugin images by digest (#3803, @2uasimojo)
  * Added BackupPhaseUploading and BackupPhaseUploadingPartialFailure backup phases as part of Upload Progress Monitoring. (#3805, @dsmithuchida)

    Uploading (new)
    The "Uploading" phase signifies that the main part of the backup, including 
    snapshotting has completed successfully and uploading is continuing. In 
    the event of an error during uploading, the phase will change to 
    UploadingPartialFailure. On success, the phase changes to Completed. The 
    backup cannot be restored from when it is in the Uploading state.

    UploadingPartialFailure (new)
    The "UploadingPartialFailure" phase signifies that the main part of the backup,
    including snapshotting has completed, but there were partial failures either 
    during the main part or during the uploading. The backup cannot be restored 
    from when it is in the UploadingPartialFailure state.
  * 🐛 Fix plugin name derivation from image name (#3711, @ashish-amarnath)
  * ✨ ⚠️ Remove CSI volumesnapshot artifact deletion

This change requires https://github.com/vmware-tanzu/velero-plugin-for-csi/pull/86 for Velero to continue
deleting of CSI volumesnapshots when the corresponding backups are deleted. (#3734, @ashish-amarnath)
  * use unstructured to marshal selective fields for service restore action (#3789, @alaypatel07)
