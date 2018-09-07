# Overview

This document is a proposal for a revised layout for backup storage locations.

# Current Layout

Backup storage locations are structured as follows:
```
<bucket>/
  <prefix>/ (optional)
    backup-1/
      ark-backup.json
      backup-1.tar.gz
      backup-1-logs.tar.gz
      restore-of-backup-1-logs.gz
      restore-of-backup-1-results.gz
    backup-2/
      ark-backup.json
      backup-2.tar.gz
      backup-2-logs.tar.gz
      restore-of-backup-2-logs.gz
      restore-of-backup-2-results.gz
    ...
  <prefix>/
    ...
```

Ark's restic integration requires a separate "root" location, i.e. a different bucket + prefix.  Restic locations are structured as follows:
```
<bucket>/
  <prefix>/ (optional)
    namespace-1/
      config
      data/
      index/
      keys/
      locks/
      snapshots/
    namespace-2/
      config
      data/
      index/
      keys/
      locks/
      snapshots/
    ...
```

# Revised Layout

The revised layout adds top-level "directories" within the root of the backup storage location to organize the contents.
The initial set of top-level directories is:
- **metadata** - holds metadata files describing the backup storage location, including `owner`, `revision`, etc.
- **backups** - holds all backup-related data, similar to what's stored directly in the root of the backup storage location today.
- **restores** - holds restore logs and results, with a subdirectory per restore. 
- **restic** - holds the data currently stored in the restic location, i.e. a subdirectory per namespace, each containing a restic repository.

Here's what it looks like:
```
<bucket>/
  <prefix>/ (optional)
    metadata/
      revision
      owner
      ...
    backups/
      backup-1/
        ark-backup.json
        backup-1.tar.gz
        backup-1-logs.tar.gz
      backup-2/
        ark-backup.json
        backup-2.tar.gz
        backup-2-logs.tar.gz
      ...
    restores/
      restore-of-backup-1/
        restore-of-backup-1-logs.gz
        restore-of-backup-1-results.gz
      restore-of-backup-2/
        restore-of-backup-2-logs.gz
        restore-of-backup-2-results.gz
      ...
    restic/
      namespace-1/
        config
        data/
        index/
        keys/
        locks/
        snapshots/
      namespace-2/
        config
        data/
        index/
        keys/
        locks/
        snapshots/
      ...
```

# Migrating to the New Layout

There are two main approaches I see for migrating to the new layout:
- **Manual Migration** - the Ark codebase gets modified to support only the new layout. As part of the release containing this change,
we provide a set of script templates, one per provider, to rearrange existing backup storage locations to conform to the new layout.
These scripts would execute a series of commands (e.g. `aws s3 cp` for S3) to move existing directories/files around.
- **Backwards Compatibility** - the Ark codebase is modified such that new backups/restores/etc. are stored within the new layout structure,
but existing backup data stored directly in the root of the backup storage location is still recognized and supported as the basis for restores,
delete operations, etc. Over time, all backups directly in the root should eventually be garbage-collected and only the new layout will remain.
Additionally, we would probably remove all traces of the backwards-compatibility code as of Ark v1.0.

My preferred approach is **manual migration**. We're still pre-1.0 so we don't make any guarantees about backwards compatibility. Any code that
we write to support the old layout alongside the new will be throwaway as of v1.0, and will increase the complexity of the codebase (meaning 
higher likelihood of bugs, harder to maintain, etc). And the migration scripts are still good-faith efforts to make upgrading as easy as possible
for our current users.