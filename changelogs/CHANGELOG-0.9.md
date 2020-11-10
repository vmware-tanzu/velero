  - [v0.9.11](#v0911)
  - [v0.9.10](#v0910)
  - [v0.9.9](#v099)
  - [v0.9.8](#v098)
  - [v0.9.7](#v097)
  - [v0.9.6](#v096)
  - [v0.9.5](#v095)
  - [v0.9.4](#v094)
  - [v0.9.3](#v093)
  - [v0.9.2](#v092)
  - [v0.9.1](#v091)
  - [v0.9.0](#v090)

## v0.9.11
#### 2018-11-08
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.11

### Bug Fixes
  * Fix bug preventing PV snapshots from being restored (#1040, @ncdc)


## v0.9.10
#### 2018-11-01
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.10

### Bug Fixes
  * restore storageclasses before pvs and pvcs (#594, @shubheksha)
  * AWS: Ensure that the order returned by ListObjects is consistent (#999, @bashofmann)
  * Add CRDs to list of prioritized resources (#424, @domenicrosati)
  * Verify PV doesn't exist before creating new volume (#609, @nrb)
  * Update README.md - Grammar mistake corrected (#1018, @midhunbiju)


## v0.9.9
#### 2018-10-24
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.9

### Bug Fixes
  * Check if initContainers key exists before attempting to remove volume mounts. (#927, @skriss)


## v0.9.8
#### 2018-10-18
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.8

### Bug Fixes
  * Discard service account token volume mounts from init containers on restore (#910, @james-powis)
  * Support --include-cluster-resources flag when creating schedule (#942, @captjt)
  * Remove logic to get a GCP project (#926, @shubheksha)
  * Only try to back up PVCs linked PV if the PVC's phase is Bound (#920, @skriss)
  * Claim ownership of new AWS volumes on Kubernetes cluster being restored into (#801, @ljakimczuk)
  * Remove timeout check when taking snapshots (#928, @carlisia)


## v0.9.7
#### 2018-10-04
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.7

### Bug Fixes
  * Preserve explicitly-specified node ports during restore (#712, @timoreimann)
  * Enable restoring resources with ownerReference set (#837, @mwieczorek)
  * Fix error when restoring ExternalName services (#869, @shubheksha)
  * remove restore log helper for accurate line numbers (#891, @skriss)
  * Display backup StartTimestamp in `ark backup get` output (#894, @marctc)
  * Fix restic restores when using namespace mappings (#900, @skriss)


## v0.9.6
#### 2018-09-21
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.6

### Bug Fixes
  * Discard service account tokens from non-default service accounts on restore (#843, @james-powis)
  * Update Docker images to use `alpine:3.8` (#852, @nrb)


## v0.9.5
#### 2018-09-17
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.5

### Bug Fixes
  * Fix issue causing restic restores not to work (#834, @skriss)


## v0.9.4
#### 20180-09-05
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.4

### Bug Fixes
  * Terminate plugin clients to resolve memory leaks (#797, @skriss)
  * Fix nil map errors when merging annotations (#812, @nrb)


## v0.9.3
#### 2018-08-10
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.3
### Bug Fixes
  * Initialize Prometheus metrics when creating a new schedule (#689, @lemaral)


## v0.9.2
#### 2018-07-26
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.2) - 2018-07-26

### Bug Fixes:
  * Fix issue where modifications made by backup item actions were not being saved to backup tarball (#704, @skriss)


## v0.9.1
#### 2018-07-23
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.1

### Bug Fixes:
  * Require namespace for Ark's CRDs to already exist at server startup (#676, @skriss)
  * Require all Ark CRDs to exist at server startup (#683, @skriss)
  * Fix `latest` tagging in Makefile (#690, @skriss)
  * Make Ark compatible with clusters that don't have the `rbac.authorization.k8s.io/v1` API group (#682, @nrb)
  * Don't consider missing snapshots an error during backup deletion, limit backup deletion requests per backup to 1 (#687, @skriss)


## v0.9.0
#### 2018-07-06
### Download
  - https://github.com/heptio/ark/releases/tag/v0.9.0

### Highlights:
  * Ark now has support for backing up and restoring Kubernetes volumes using a free open-source backup tool called [restic](https://github.com/restic/restic).
    This provides users an out-of-the-box solution for backing up and restoring almost any type of Kubernetes volume, whether or not it has snapshot support
    integrated with Ark. For more information, see the [documentation](https://github.com/vmware-tanzu/velero/blob/main/docs/restic.md).
  * Support for Prometheus metrics has been added! View total number of backup attempts (including success or failure), total backup size in bytes, and backup
    durations. More metrics coming in future releases!

### All New Features:
  * Add restic support (#508 #532 #533 #534 #535 #537 #540 #541 #545 #546 #547 #548 #555 #557 #561 #563 #569 #570 #571 #606 #608 #610 #621 #631 #636, @skriss)
  * Add prometheus metrics (#531 #551 #564, @ashish-amarnath @nrb)
  * When backing up a service account, include cluster roles/cluster role bindings that reference it (#470, @skriss)
  * When restoring service accounts, copy secrets/image pull secrets into the target cluster even if the service account already exists (#403, @nrb)

### Bug Fixes / Other Changes:
  * Upgrade to Kubernetes 1.10 dependencies (#417, @skriss)
  * Upgrade to go 1.10 and alpine 3.7 (#456, @skriss)
  * Display no excluded resources/namespaces as `<none>` rather than `*` (#453, @nrb)
  * Skip completed jobs and pods when restoring (#463, @nrb)
  * Set namespace correctly when syncing backups from object storage (#472, @skriss)
  * When building on macOS, bind-mount volumes with delegated config (#478, @skriss)
  * Add replica sets and daemonsets to cohabitating resources so they're not backed up twice (#482 #485, @skriss)
  * Shut down the Ark server gracefully on SIGINT/SIGTERM (#483, @skriss)
  * Only back up resources that support GET and DELETE in addition to LIST and CREATE (#486, @nrb)
  * Show a better error message when trying to get an incomplete restore's logs (#496, @nrb)
  * Stop processing when setting a backup deletion request's phase to `Deleting` fails (#500, @nrb)
  * Add library code to install Ark's server components (#437 #506, @marpaia)
  * Properly handle errors when backing up additional items (#512, @carlpett)
  * Run post hooks even if backup actions fail (#514, @carlpett)
  * GCP: fail backup if upload to object storage fails (#510, @nrb)
  * AWS: don't require `region` as part of backup storage provider config (#455, @skriss)
  * Ignore terminating resources while doing a backup (#526, @yastij)
  * Log to stdout instead of stderr (#553, @ncdc)
  * Move sample minio deployment's config to an emptyDir (#566, @runyontr)
  * Add `omitempty` tag to optional API fields (@580, @nikhita)
  * Don't restore PVs with a reclaim policy of `Delete` and no snapshot (#613, @ncdc)
  * Don't restore mirror pods (#619, @ncdc)

### Docs Contributors:
  * @gianrubio
  * @castrojo
  * @dhananjaysathe
  * @c-knowles
  * @mattkelly
  * @ae-v
  * @hamidzr
