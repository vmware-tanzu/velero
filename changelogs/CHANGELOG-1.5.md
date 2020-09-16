## v1.5.1
### 2020-09-16

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.5.1

### Container Image
`velero/velero:v1.5.1`

### Documentation
https://velero.io/docs/v1.5/

### Upgrading
https://velero.io/docs/v1.5/upgrade-to-1.5/

### Highlights

 * Auto Volume Backup Using Restic with `--default-volumes-to-restic` flag
 * DeleteItemAction plugins
 * Code modernization
 * Restore Hooks: InitContianer Restore Hooks and Exec Restore Hooks

### All Changes

  * 🏃‍♂️ add shortnames for CRDs (#2911, @ashish-amarnath)
  * Use format version instead of version on `velero backup describe` since version has been deprecated (#2901, @jenting)
  * fix EnableAPIGroupersions output log format (#2882, @jenting)
  * Convert ServerStatusRequest controller to kubebuilder (#2838, @carlisia)
  * rename the PV if VolumeSnapshotter has modified the PV name (#2835, @pawanpraka1)
  * Implement post-restore exec hooks in pod containers (#2804, @areed)
  * Check for errors on restic backup command (#2863, @dymurray)
  * 🐛 fix passing LDFLAGS across build stages (#2853, @ashish-amarnath)
  * Feature: Invoke DeleteItemAction plugins based on backup contents when a backup is deleted. (#2815, @nrb)
  * When JSON logging format is enabled, place error message at "error.message" instead of "error" for compatibility with Elasticsearch/ELK and the Elastic Common Schema (#2830, @bgagnon)
  * discovery Helper support get GroupVersionResource and an APIResource from GroupVersionKind (#2764, @runzexia)
  * Migrate site from Jekyll to Hugo (#2720, @tbatard)
  * Add the DeleteItemAction plugin type (#2808, @nrb)
  * 🐛 Manually patch the generated yaml for restore CRD as a hacky workaround (#2814, @ashish-amarnath)
  * Setup crd validation github action on k8s versions (#2805, @ashish-amarnath)
  * 🐛 Supply command to run restic-wait init container (#2802, @ashish-amarnath)
  * Make init and exec restore hooks as optional in restore hookSpec (#2793, @ashish-amarnath)
  * Implement restore hooks injecting init containers into pod spec (#2787, @ashish-amarnath)
  * Pass default-volumes-to-restic flag from create schedule to backup (#2776, @ashish-amarnath)
  * Enhance Backup to support backing up resources in specific orders and add --ordered-resources option to support this feature. (#2724, @phuong)
  * Fix inconsistent type for the "resource" structured logging field (#2796, @bgagnon)
  * Add the ability to set the allowPrivilegeEscalation flag in the securityContext for the Restic restore helper. (#2792, @doughepi)
  * Add cacert flag for velero backup-location create (#2778, @jenting)
  * Exclude volumes mounting secrets and configmaps from defaulting volume backups to restic (#2762, @ashish-amarnath)
  * Add types to implement restore hooks (#2761, @ashish-amarnath)
  * Add wait group and error channel for restore hooks to restore context. (#2755, @areed)
  * Refactor image builds to use buildx for multi arch image building (#2754, @robreus)
  * Add annotation key constants for restore hooks (#2750, @ashish-amarnath)
  * Adds Start and CompletionTimestamp to RestoreStatus
Displays the Timestamps when issued a print or describe (#2748, @thejasbabu)
  * Move pkg/backup/item_hook_handlers.go to internal/hook (#2734, @nrb)
  * add metrics for restic back up operation (#2719, @ashish-amarnath)
  * StorageGrid compatibility by removing explicit gzip accept header setting (#2712, @fvsqr)
  * restic: add support for setting SecurityContext (runAsUser, runAsGroup) for restore (#2621, @jaygridley)
  * Add backupValidationFailureTotal to metrics (#2714, @kathpeony)
  * bump Kubernetes module dependencies to v0.18.4 to fix https://github.com/vmware-tanzu/velero/issues/2540 by adding code compatibility with kubernetes v1.18 (#2651, @laverya)
  * Add a BSL controller to handle validation + update BSL status phase (validation removed from the server and no longer blocks when there's any invalid BSL) (#2674, @carlisia)
  * updated acceptable values on cron schedule from 0-7 to 0-6 (#2676, @dthrasher)
  * Improve velero download doc (#2660, @carlisia)
  * Update basic-install and release-instructions documentation for Windows Chocolatey package (#2638, @adamrushuk)
  * move CSI plugin out of prototype into beta (#2636, @ashish-amarnath)
  * Add a new supported provider for an object storage plugin for Storj (#2635, @jessicagreben)
  * Update basic-install.md documentation: Add windows cli installation option via chocolatey (#2629, @adamrushuk)
  * Documentation: Update Jekyll to 4.1.0. Switch from redcarpet to kramdown for Markdown renderer (#2625, @tbatard)
  * improve builder image handling so that we don't rebuild each `make shell` (#2620, @mauilion)
    * first check if there are pending changed on the build-image dockerfile if so build it.
    * then check if there is an image in the registry if so pull it.
    * then build an image cause we don't have a cached image. (this handles the backward compat case.)
    * fix make clean to clear go mod cache before removing dirs (for containerized builds)
  * Add linter checks to Makefile (#2615, @tbatard)
  * add a CI check for a changelog file (#2613, @ashish-amarnath)
  * implement option to back up all volumes by default with restic  (#2611, @ashish-amarnath)
  * When a timeout string can't be parsed, log the error as a warning instead of silently consuming the error. (#2610, @nrb)
  * Azure: support using `aad-pod-identity` auth when using restic (#2602, @skriss)
  * log a warning instead of erroring if an additional item returned from a plugin can't be found in the Kubernetes API (#2595, @skriss)
  * when creating new backup from schedule from cli, allow backup name to be automatically generated (#2569, @cblecker)
  * Convert manifests + BSL api client to kubebuilder (#2561, @carlisia)
  * backup/restore: reinstantiate backup store just before uploading artifacts to ensure credentials are up-to-date (#2550, @skriss)
