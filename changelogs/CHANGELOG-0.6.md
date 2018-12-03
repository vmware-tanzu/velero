- [v0.6.0](#v060)

## v0.6.0
#### 2017-11-30
### Download
  - https://github.com/heptio/ark/tree/v0.6.0

### Highlights
  * **Plugins** - We now support user-defined plugins that can extend Ark functionality to meet your custom backup/restore needs without needing to be compiled into the core binary. We support pluggable block and object stores as well as per-item backup and restore actions that can execute arbitrary logic, including modifying the items being backed up or restored. For more information see the [documentation](docs/plugins.md), which includes a reference to a fully-functional sample plugin repository. (#174 #188 #206 #213 #215 #217 #223 #226)
  * **Describers** - The Ark CLI now includes `describe` commands for `backups`, `restores`, and `schedules` that provide human-friendly representations of the relevant API objects.

### Breaking Changes
  * The config object format has changed. In order to upgrade to v0.6.0, the config object will have to be updated to match the new format. See the [examples](examples) and [documentation](docs/config-definition.md) for more information.
  * The restore object format has changed. The `warnings` and `errors` fields are now ints containing the counts, while full warnings and errors are now stored in the object store instead of etcd. Restore objects created prior to v.0.6.0 should be deleted, or a new bucket used, and the old restore objects deleted from Kubernetes (`kubectl -n heptio-ark delete restore --all`).

### All New Features
  * Add `ark plugin add` and `ark plugin remove` commands #217, @skriss
  * Add plugin support for block/object stores, backup/restore item actions #174 #188 #206 #213 #215 #223 #226, @skriss @ncdc
  * Improve Azure deployment instructions #216, @ncdc
  * Change default TTL for backups to 30 days #204, @nrb
  * Improve logging for backups and restores #199, @ncdc
  * Add `ark backup describe`, `ark schedule describe` #196, @ncdc
  * Add `ark restore describe` and move restore warnings/errors to object storage #173 #201 #202, @ncdc
  * Upgrade to client-go v5.0.1, kubernetes v1.8.2 #157, @ncdc
  * Add Travis CI support #165 #166, @ncdc

### Bug Fixes
  * Fix log location hook prefix stripping #222, @ncdc
  * When running `ark backup download`, remove file if there's an error #154, @ncdc
  * Update documentation for AWS KMS Key alias support #163, @lli-hiya
  * Remove clock from `volume_snapshot_action` #137, @athampy
