---
title: "Plugins"
layout: docs
---

Velero has a plugin architecture that allows users to add their own custom functionality to Velero backups & restores 
without having to modify/recompile the core Velero binary. To add custom functionality, users simply create their own binary 
containing implementations of Velero's plugin kinds (described below), plus a small amount of boilerplate code to 
expose the plugin implementations to Velero. This binary is added to a container image that serves as an init container for 
the Velero server pod and copies the binary into a shared emptyDir volume for the Velero server to access. 

Multiple plugins, of any type,  can be implemented in this binary.

A fully-functional [sample plugin repository][1] is provided to serve as a convenient starting point for plugin authors.

## Plugin Kinds

Velero currently supports the following kinds of plugins:

- **Object Store** - persists and retrieves backups, backup logs and restore logs
- **Block Store** - creates volume snapshots (during backup) and restores volumes from snapshots (during restore)
- **Backup Item Action** - executes arbitrary logic for individual items prior to storing them in a backup file
- **Restore Item Action** - executes arbitrary logic for individual items prior to restoring them into a cluster

## Plugin Logging

Velero provides a [logger][2] that can be used by plugins to log structured information to the main Velero server log or 
per-backup/restore logs. See the [sample repository][1] for an example of how to instantiate and use the logger 
within your plugin.



[1]: https://github.com/heptio/velero-plugin-example
[2]: https://github.com/heptio/velero/blob/main/pkg/plugin/logger.go
