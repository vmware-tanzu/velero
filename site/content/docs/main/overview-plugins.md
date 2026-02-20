---
title: "Velero plugin system"
layout: docs
---

Velero uses storage provider plugins to integrate with a variety of storage systems to support backup and snapshot operations.

For server installation, Velero requires that at least one plugin is added (with the `--plugins` flag). The plugin will be either of the type object store or volume snapshotter, or a plugin that contains both. An exception to this is that when the user is not configuring a backup storage location or a snapshot storage location at the time of install, this flag is optional.

Any plugin can be added after Velero has been installed by using the command `velero plugin add <registry/image:version>`.

Example with a dockerhub image: `velero plugin add velero/velero-plugin-for-aws:v1.0.0`.

In the same way, any plugin can be removed by using the command `velero plugin remove <registry/image:version>` or `velero plugin remove <plugin-name>`.

You can also list installed plugins using `velero plugin get` to see which plugins are built-in (mandatory) and cannot be removed.

## Built-in vs External Plugins

Velero distinguishes between two types of plugins:

- **Built-in plugins**: These are plugins that are part of the Velero server binary itself (e.g., `velero.io/pod`, `velero.io/pv`). These plugins are mandatory and cannot be removed as they are essential for core Velero functionality.

- **External plugins**: These are plugins installed via init containers (e.g., `velero.io/aws`, `velero.io/gcp`). These can be added and removed as needed.

Use `velero plugin get` to see which plugins are built-in (marked as `true` in the BuiltIn column) and which are external (marked as `false`).

## Creating a new plugin

Anyone can add integrations for any platform to provide additional backup and volume storage without modifying the Velero codebase. To write a plugin for a new backup or volume storage platform, take a look at our [example repo][1] and at our documentation for [Custom plugins][2].

## Adding a new plugin

After you publish your plugin on your own repository, open a PR that adds a link to it under the appropriate list of [supported providers][3] page in our documentation.

You can also add the [`velero-plugin` GitHub Topic][4] to your repo, and it will be shown under the aggregated list of repositories automatically.

[1]: https://github.com/vmware-tanzu/velero-plugin-example/
[2]: custom-plugins.md
[3]: supported-providers.md
[4]: https://github.com/topics/velero-plugin
