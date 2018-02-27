# Heptio Ark Roadmap

## Released Versions

### v0.5.0 - 2017-10-26
- Add User-Agent to Ark clients
- Fix config change detection
- Glog -> Logrus
- Exclude nodes from restorations
- Support multi-zone snapshots & restores
- Back up PVs claimed by PVCs
- Add --include-cluster-resources flag
- Backup hooks, only to execute a command in a container
- Can specify via annotation on pod
- Can specify via backup spec
- Verb-noun command aliases
- Make Ark ServiceAccount cluster-admin
- Proper prioritization of both cluster and namespace scoped resources (breaking tarball format change)
- Multi platform binaries

### v0.6.0 - 2017-11-30
- Deeper Azure integration
- Plugin mechanism (look at https://github.com/hashicorp/go-plugin) so third-party integrators can add their own custom:
    - Object store implementation (storing backup tarballs)
    - Block store implementation (volume snapshotting)
    - Per-item backup actions
    - Per-item restore actions
- Ark describe backup, restore, schedule

### v0.7.0 - 2018-02-15
- Allow Ark to run in any namespace - [#106](https://github.com/heptio/ark/issues/106)
- Ark backup delete command - [#252](https://github.com/heptio/ark/issues/252)
- Always try to upload backup log file, even if backup fails - [#250](https://github.com/heptio/ark/issues/250)
- Support pre and post pod exec hooks - [#244](https://github.com/heptio/ark/issues/244)
- Add serviceaccounts to list of default prioritized resources during restore - [#239](https://github.com/heptio/ark/issues/239)

## Upcoming Versions

The following versions, dates, and features are approximate and are subject to change.

### v0.8.0 - ~ 2018-04-19
- Backup targets
- Snapshot & restore non-cloud volumes - [#19](https://github.com/heptio/ark/issues/19)
- Custom conflict resolution handlers - [#205](https://github.com/heptio/ark/issues/205)

### v0.9.0 - ~ 2018-06-14
- Backup & restore across multiple regions and zones - [#103](https://github.com/heptio/ark/issues/103)
- Ability to clone PVs - [#192](https://github.com/heptio/ark/issues/192)
- Ark install command - [#52](https://github.com/heptio/ark/issues/52)
- Backup & restore progress reporting - [#20](https://github.com/heptio/ark/issues/20) [#21](https://github.com/heptio/ark/issues/21)
