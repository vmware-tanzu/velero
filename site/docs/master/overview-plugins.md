
# Velero plugin system

Velero has a plugin system which allows integration with a variety of providers for backup storage and volume snapshot operations. Please see the links on the left under `Plugins`.

Anyone can add integrations for any platform to provide additional backup and volume storage without modifying the Velero codebase.

## Creating a new plugin

To write a plugin for a new backup or volume storage platform, take a look at our [example repo][1] and at our documentation for [how to develop custom plugins][2].

## Adding a new plugin

After you publish your plugin on your own repository, open a PR that adds a link to it under the appropriate list on the [community plugins][3] page in our documentation.

[1]: https://github.com/heptio/velero-plugin-example/
[2]: custom-plugins
[3]: community-plugins