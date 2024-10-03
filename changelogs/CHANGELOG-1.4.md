## v1.4.2
### 2020-07-13

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.4.2

### Container Image
`velero/velero:v1.4.2`

### Documentation
https://velero.io/docs/v1.4/

### Upgrading
https://velero.io/docs/v1.4/upgrade-to-1.4/

### All Changes
  * log a warning instead of erroring if an additional item returned from a plugin can't be found in the Kubernetes API (#2595, @skriss)
  * Adjust restic default time out to 4 hours and base pod resource requests to 500m CPU/512Mi memory. (#2696, @nrb)
  * capture version of the CRD prior before invoking the remap_crd_version backup item action (#2683, @ashish-amarnath)


## v1.4.1

This tag was created in code, but has no associated docker image due to misconfigured building infrastructure. v1.4.2 fixes this.

## v1.4.0
### 2020-05-26

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.4.0

### Container Image
`velero/velero:v1.4.0`

### Documentation
https://velero.io/docs/v1.4/

### Upgrading
https://velero.io/docs/v1.4/upgrade-to-1.4/

### Highlights

 * Added beta-level CSI support!
 * Added custom CA certificate support
 * Backup progress reporting
 * Changed backup tarball format to support all versions of a given resource

### All Changes
  * increment restic volumesnapshot count after successful pvb create (#2542, @ashish-amarnath)
  * Add details of CSI volumesnapshotcontents associated with a backup to `velero backup describe` when the `EnableCSI` feature flag is given on the velero client. (#2448, @nrb)
  * Allow users the option to retrieve all versions of a given resource (instead of just the preferred version) from the API server with the `EnableAPIGroupVersions` feature flag. (#2373, @brito-rafa)
  * Changed backup tarball format to store all versions of a given resource, updated backup tarball format to 1.1.0. (#2373, @brito-rafa)
  * allow feature flags to be passed from install CLI (#2503, @ashish-amarnath)
  * sync backups' CSI API objects into the cluster as part of the backup sync controller (#2496, @ashish-amarnath)
  * bug fix: in error location logging hook, if the item logged under the `error` key doesn't implement the `error` interface, don't return an error since this is a valid scenario (#2487, @skriss)
  * bug fix: in CRD restore plugin, don't use runtime.DefaultUnstructuredConverter.FromUnstructured(...) to avoid conversion issues when float64 fields contain int values (#2484, @skriss)
  * during backup deletion also delete CSI volumesnapshotcontents that were created as a part of the backup but the associated volumesnapshot object does not exist (#2480, @ashish-amarnath)
  * If plugins don't support the `--features` flag, don't pass it to them. Also, update the standard plugin server to ignore unknown flags. (#2479, @skriss)
  * At backup time, if a CustomResourceDefinition appears to have been created via the v1beta1 endpoint, retrieve it from the v1beta1 endpoint instead of simply changing the APIVersion. (#2478, @nrb)
  * update container base images from ubuntu:bionic to ubuntu:focal (#2471, @skriss)
  * bug fix: when a resource includes/excludes list contains unresolvable items, don't remove them from the list, so that the list doesn't inadvertently end up matching *all* resources. (#2462, @skriss)
  * Azure: add support for getting storage account key for restic directly from an environment variable (#2455, @jaygridley)
  * Support to skip VSL validation for the backup having SnapshotVolumes set to false or created with `--snapshot-volumes=false` (#2450, @mynktl)
  * report backup progress (number of items backed up so far out of an estimated total number of items) during backup in the logs and as status fields on the Backup custom resource (#2440, @skriss)
  * bug fix: populate namespace in logs for backup errors (#2438, @skriss)
  * during backup deletion also delete CSI volumesnapshots that were created as a part of the backup (#2411, @ashish-amarnath)
  * bump Kubernetes module dependencies to v0.17.4 to get fix for https://github.com/kubernetes/kubernetes/issues/86149 (#2407, @skriss)
  * bug fix: save PodVolumeBackup manifests to object storage even if the volume was empty, so that on restore, the PV is dynamically reprovisioned if applicable (#2390, @skriss)
  * Adding new restoreItemAction for PVC to update the selected-node annotation (#2377, @mynktl)
  * Added a --cacert flag to the install command to provide the CA bundle to use when verifying TLS connections to object storage (#2368, @mansam)
  * Added a `--cacert` flag to the velero client describe, download, and logs commands to allow passing a path to a certificate to use when verifying TLS connections to object storage. Also added a corresponding client config option called `cacert` which takes a path to a certificate bundle to use as a default when `--cacert` is not specified. (#2364, @mansam)
  * support setting a custom CA certificate on a BSL to use when verifying TLS connections (#2353, @mansam)
  * adding annotations on backup CRD for k8s major, minor and git versions (#2346, @brito-rafa)
  * When the EnableCSI feature flag is provided, upload CSI VolumeSnapshots and VolumeSnapshotContents to object storage as gzipped JSON. (#2323, @nrb)
  * add CSI snapshot API types into default restore priorities (#2318, @ashish-amarnath)
  * refactoring: wait for all informer caches to sync before running controllers (#2299, @skriss)
  * refactor restore code to lazily resolve resources via discovery and eliminate second restore loop for instances of restored CRDs (#2248, @skriss)
  * upgrade to go 1.14 and migrate from `dep` to go modules (#2214, @skriss)
  * clarify the wording for restore describe for namespaces included
