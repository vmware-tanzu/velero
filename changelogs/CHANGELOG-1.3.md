## v1.3.0-beta.2
#### 2020-02-24

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.3.0-beta.2

### Container Image
`velero/velero:v1.3.0-beta.2`

### Documentation
https://velero.io/docs/v1.3.0-beta.2/

### Upgrading
```bash
kubectl set image \
  --namespace velero \
  deployment/velero velero=velero/velero:v1.3.0-beta.2

# if using restic:
kubectl set image \
  --namespace velero \
  daemonset/restic restic=velero/velero:v1.3.0-beta.2
```

### All Changes
  * Back up schema-less CustomResourceDefinitions as v1beta1, even if they are retrieved via the v1 endpoint. (#2264, @nrb)
  * Bug fix: restic backup volume snapshot to the second location failed (#2244, @jenting)
  * Added support of using PV name from volumesnapshotter('SetVolumeID') in case of PV renaming during the restore (#2216, @mynktl)
  * Replaced deprecated helm repo url at all it appearance at docs. (#2209, @markrity)

## v1.3.0-beta.1
#### 2020-02-04

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.3.0-beta.1

### Container Image
`velero/velero:v1.3.0-beta.1`

### Documentation
https://velero.io/docs/v1.3.0-beta.1/

### Upgrading
```bash
kubectl set image \
  --namespace velero \
  deployment/velero velero=velero/velero:v1.3.0-beta.1

# if using restic:
kubectl set image \
  --namespace velero \
  daemonset/restic restic=velero/velero:v1.3.0-beta.1
```

### All Changes
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
  * Enableing Velero to switch credentials (`AWS_PROFILE`) if multiple s3-compatible backupLocations are present (#2096, @dinesh)
  * bug fix: deep-copy backup's labels when constructing snapshot tags, so the PV name isn't added as a label to the backup (#2075, @skriss)
  * remove the `fsfreeze-pause` image being published from this repo; replace it with `ubuntu:bionic` in the nginx example app (#2068, @skriss)
  * add support for a private registry with a custom port in a restic-helper image (#1999, @cognoz)
  * return better error message to user when cluster config can't be found via `--kubeconfig`, `$KUBECONFIG`, or in-cluster config (#2057, @skriss)
