## v1.3.2
### 2020-04-03

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.3.2

### Container Image
`velero/velero:v1.3.2`

### Documentation
https://velero.io/docs/v1.3.2/

### Upgrading
https://velero.io/docs/v1.3.2/upgrade-to-1.3/

### All Changes
* Allow `plugins/` as a valid top-level directory within backup storage locations. This directory is a place for plugin authors to store arbitrary data as needed. It is recommended to create an additional subdirectory under `plugins/` specifically for your plugin, e.g. `plugins/my-plugin-data/`. (#2350, @skriss)
* bug fix: don't panic in `velero restic repo get` when last maintenance time is `nil` (#2315, @skriss)

## v1.3.1
### 2020-03-10

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.3.1

### Container Image
`velero/velero:v1.3.1`

### Documentation
https://velero.io/docs/v1.3.1/

### Upgrading
https://velero.io/docs/v1.3.1/upgrade-to-1.3/

### Highlights

Fixed a bug that caused failures when backing up CustomResourceDefinitions with whole numbers in numeric fields.

### All Changes
 * Fix CRD backup failures when fields contained a whole number. (#2322, @nrb)


## v1.3.0
#### 2020-03-02

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.3.0

### Container Image
`velero/velero:v1.3.0`

### Documentation
https://velero.io/docs/v1.3.0/

### Upgrading
https://velero.io/docs/v1.3.0/upgrade-to-1.3/

### Highlights

#### Custom Resource Definition Backup and Restore Improvements

This release includes a number of related bug fixes and improvements to how Velero backs up and restores custom resource definitions (CRDs) and instances of those CRDs.

We found and fixed three issues around restoring CRDs that were originally created via the `v1beta1` CRD API.  The first issue affected CRDs that  had the `PreserveUnknownFields` field set to `true`.  These CRDs could not be restored into 1.16+ Kubernetes clusters, because the `v1` CRD API does not allow this field to be set to `true`. We added code to the restore process to check for this scenario, to set the `PreserveUnknownFields` field to `false`, and to instead set `x-kubernetes-preserve-unknown-fields` to `true` in the OpenAPIv3 structural schema, per Kubernetes guidance. For more information on this, see the [Kubernetes documentation](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#pruning-versus-preserving-unknown-fields). The second issue affected CRDs without structural schemas. These CRDs need to be backed up/restored through the `v1beta1` API, since all CRDs created through the `v1` API must have structural schemas. We added code to detect these CRDs and always back them up/restore them through the `v1beta1` API. Finally, related to the previous issue, we found that our restore code was unable to handle backups with multiple API versions for a given resource type, and we’ve remediated this as well.

We also improved the CRD restore process to enable users to properly restore CRDs and instances of those CRDs in a single restore operation. Previously, users found that they needed to run two separate restores: one to restore the CRD(s), and another to restore instances of the CRD(s).  This was due to two deficiencies in the Velero code. First, Velero did not wait for a CRD to be fully accepted by the Kubernetes API server and ready for serving before moving on; and second, Velero did not refresh its cached list of available APIs in the target cluster after restoring CRDs, so it was not aware that it could restore instances of those CRDs.

We fixed both of these issues by (1) adding code to wait for CRDs to be “ready” after restore before moving on, and (2) refreshing the cached list of APIs after restoring CRDs, so any instances of newly-restored CRDs could subsequently be restored.

With all of these fixes and improvements in place, we hope that the CRD backup and restore experience is now seamless across all supported versions of Kubernetes.


#### Multi-Arch Docker Images

Thanks to community members [@Prajyot-Parab](https://github.com/Prajyot-Parab) and [@shaneutt](https://github.com/shaneutt), Velero now provides multi-arch container images by using Docker manifest lists.  We are currently publishing images for `linux/amd64`, `linux/arm64`, `linux/arm`, and `linux/ppc64le` in [our Docker repository](https://hub.docker.com/r/velero/velero/tags?page=1&name=v1.3&ordering=last_updated).

Users don’t need to change anything other than updating their version tag - the v1.3 image is `velero/velero:v1.3.0`, and Docker will automatically pull the proper architecture for the host.

For more information on manifest lists, see [Docker’s documentation](https://docs.docker.com/registry/spec/manifest-v2-2/). 


#### Bug Fixes, Usability Enhancements, and More

We fixed a large number of bugs and made some smaller usability improvements in this release. Here are a few highlights:

- Support private registries with custom ports for the restic restore helper image ([PR #1999](https://github.com/vmware-tanzu/velero/pull/1999), [@cognoz](https://github.com/cognoz))
- Use AWS profile from BackupStorageLocation when invoking restic ([PR #2096](https://github.com/vmware-tanzu/velero/pull/2096), [@dinesh](https://github.com/dinesh))
- Allow restores from schedules in other clusters ([PR #2218](https://github.com/vmware-tanzu/velero/pull/2218), [@cpanato](https://github.com/cpanato))
- Fix memory leak & race condition in restore code ([PR #2201](https://github.com/vmware-tanzu/velero/pull/2201), [@skriss](https://github.com/skriss))


### All Changes
  * Corrected the selfLink for Backup CR in site/docs/main/output-file-format.md (#2292, @RushinthJohn)
  * Back up schema-less CustomResourceDefinitions as v1beta1, even if they are retrieved via the v1 endpoint. (#2264, @nrb)
  * Bug fix: restic backup volume snapshot to the second location failed (#2244, @jenting)
  * Added support of using PV name from volumesnapshotter('SetVolumeID') in case of PV renaming during the restore (#2216, @mynktl)
  * Replaced deprecated helm repo url at all it appearance at docs. (#2209, @markrity)
  * added support for arm and arm64 images (#2227, @shaneutt)
  * when restoring from a schedule, validate by checking for backup(s) labeled with the schedule name rather than existence of the schedule itself, to allow for restoring from deleted schedules and schedules in other clusters (#2218, @cpanato)
  * bug fix: back up server-preferred version of CRDs rather than always the `v1beta1` version (#2230, @skriss)
  * Wait for CustomResourceDefinitions to be ready before restoring CustomResources. Also refresh the resource list from the Kubernetes API server after restoring CRDs in order to properly restore CRs. (#1937, @nrb)
  * When restoring a v1 CRD with PreserveUnknownFields = True, make sure that the preservation behavior is maintained by copying the flag into the Open API V3 schema, but update the flag so as to allow the Kubernetes API server to accept the CRD without error. (#2197, @nrb)
  * Enable pruning unknown CRD fields (#2187, @jenting)
  * bump restic to 0.9.6 to fix some issues with non AWS standard regions (#2210, @Sh4d1)
  * bug fix: fix race condition resulting in restores sometimes succeeding despite restic restore failures (#2201, @skriss)
  * Bug fix: Check for nil LastMaintenanceTime in ResticRepository dueForMaintenance (#2200, @sseago)
  * repopulate backup_last_successful_timestamp metrics for each schedule after server restart (#2196, @skriss)
  * added support for ppc64le images and manifest lists (#1768, @prajyot)
  * bug fix: only prioritize restoring `replicasets.apps`, not `replicasets.extensions` (#2157, @skriss)
  * bug fix: restore both `replicasets.apps` *and* `replicasets.extensions` before `deployments` (#2120, @skriss)
  * bug fix: don't restore cluster-scoped resources when restoring specific namespaces and IncludeClusterResources is nil (#2118, @skriss)
  * Enabling Velero to switch credentials (`AWS_PROFILE`) if multiple s3-compatible backupLocations are present (#2096, @dinesh)
  * bug fix: deep-copy backup's labels when constructing snapshot tags, so the PV name isn't added as a label to the backup (#2075, @skriss)
  * remove the `fsfreeze-pause` image being published from this repo; replace it with `ubuntu:bionic` in the nginx example app (#2068, @skriss)
  * add support for a private registry with a custom port in a restic-helper image (#1999, @cognoz)
  * return better error message to user when cluster config can't be found via `--kubeconfig`, `$KUBECONFIG`, or in-cluster config (#2057, @skriss)
