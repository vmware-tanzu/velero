# Plan to extract the provider plugins out of (the Velero) tree

Currently, the Velero project contains in-tree plugins for three cloud providers: AWS, Azure, and GCP. The Velero team has decided to extract each of those plugins into their own separate repository. This document details the steps necessary to create the new repositories, as well as a general design for what each plugin project will look like.

## Goals

- Have 3 new repositories for each cloud provider plugin currently supported by the Velero team: AWS, Azure, and GCP
- Have the currently in-tree cloud provider plugins behave like any other plugin external to Velero

## Non Goals

- Extend the Velero plugin framework capability in any way
- Create GH repositories for any plugin other then the currently 3 in-tree plugins
- Extract out any plugin that is not a cloud provider plugin (ex: item action related plugins)

## Background

With more and more providers wanting to support Velero, it gets more difficult to justify excluding those from being in-tree just as with the three original ones. At the same time, if we were to include any more plugins in-tree, it would ultimately become the responsibility of the Velero team to maintain an increasing number of plugins. This move aims to equalize the field so all plugins are treated equally. We also hope that, with time, developers interested in getting involved in the upkeep of those plugins will become active enough to be promoted to maintainers. Lastly, having the plugins live in their own individual repositories allows for iteration on them separately from the core codebase.

## Action items

### Todo list

#### Repository creation

- [ ] Use GH UI to create each repository in the new VMW org. Who: new org owner; TBD
- [ ] Make owners of the Velero repo owners of each repo in the new org. Who: new org owner; TBD
- [ ] Add Travis CI. Who: Any of the new repo owners; TBD
- [ ] Add webhook: travis CI. Who: Any of the new repo owners; TBD
- [ ] Add DCO for signoff check (https://probot.github.io/apps/dco/). Who: Any of the new repo owners; TBD

#### Plugin changes

- [ ] Modify Velero so it can install any of the provider plugins. https://github.com/heptio/velero/issues/1740 - Who: @nrb
- [ ] Extract each provider plugin into their own repo. https://github.com/heptio/velero/issues/1537
- [ ] Create deployment and gcr-push scripts with the new location path. Who: @carlisia
- [ ] Add documentation for how to use the plugin. Who: @carlisia
- [ ] Update Helm chart to install Velero using any of the provider plugins. https://github.com/heptio/velero/issues/1819
- [ ] Upgrade script. https://github.com/heptio/velero/issues/1889.

### Notes/How-Tos

#### Creating the GH repository

[Pending] The organization owner will make all current owners in the Velero repo also owners in each of the new org plugin repos.

#### Setting up Travis CI

Someone with owner permission on the new repository needs to go to their Travis CI account and authorize Travis CI on the repo. Here are instructions: https://docs.travis-ci.com/user/tutorial/.

After this, any webhook notifications can be added following these instructions: https://docs.travis-ci.com/user/notifications/#configuring-webhook-notifications.

## High-Level Design

Each provider plugin will be an independent project, using the Velero library to implement their specific functionalities.

The way Velero is installed will be changed to accommodate installing these plugins at deploy time, namely the Velero `install` command, as well as the Helm chart.

Each plugin repository will need to have their respective images built and pushed to the same registry as the Velero images.

## Detailed Design

### Projects

Each provider plugin will be an independent GH repository, named: `velero-plugin-aws`, `velero-plugin-azure`, and `velero-plugin-gcp`.

Build of the project will be done the same way as with Velero, using Travis.

Images for all the plugins will be pushed to the same repository as the Velero image, also using Travis.

Releases of each of these plugins will happen in sync with releases of Velero. This will consist of having a tag in the repo and a tagged image build with the same release version as Velero so it makes it easy to identify what versions are compatible, starting at v1.2.

Documentation for how to install and use the plugins will be augmented in the existing Plugins section of the Velero documentation.

Documentation for how to use each plugin will reside in their respective repos. The navigation on the Velero documentation will be modified for easy discovery of the docs/images for these plugins.

#### Version compatibility

We will keep the major and minor release points in sync, but the plugins can have multiple minor dot something releases as long as it remains compatible with the corresponding major/minor release of Velero. Ex:

| Velero  | Plugin  | Compatible?   |
|---|---|---|
|  v1.2 |  v1.2 | ✅  |
|  v1.2 |  v1.2.3 | ✅  |
|  v1.2 |  v1.3 | 🚫  |
|  v1.3 |  v1.2 | 🚫  |
|  v1.3 |  v1.3.3 | ✅  |

### Installation

As per https://github.com/heptio/velero/issues/1740, we will add a `plugins` flag to the Velero install command which will accept an array of URLs pointing to +1 images of plugins to be installed. The `velero plugin add` command should continue working as is, in specific, it should also allow the installation of any of the new 3 provider plugins. @nrb will provide specifics about how this change will be tackled, as well as what will be documented. Part of the work of adding the `plugins` flag will be removing the logic that adds `velero.io` name spacing to plugins that are added without it.

The Helm chart that allows the installation of Velero will be modified to accept the array of plugin images with an added `plugins` configuration item.

### Design code changes and considerations

The naming convention to use for name spacing each plugin will be `velero.io`, since they are currently maintained by the Velero team.

Install dep

Question: are there any places outside the plugins where we depend on the cloud-provider SDKs? can we eliminate those dependencies too? x

- the `restic` package uses the `aws`. SDK to get the bucket region for the AWS object store (https://github.com/carlisia/velero/blob/32d46871ccbc6b03e415d1e3d4ad9ae2268b977b/pkg/restic/config.go#L41)
- could not find usage of the cloud provider SDKs anywhere else.

Plugins such as the pod -> pvc -> pv backupitemaction ones make sense to stay in the core repo as they provide some important logic that just happens to be implemented in a plugin.

### Upgrade

The documentation for how to fresh install the out-of-tree plugin with Velero v1.2 will be specified together with the documentation for the install changes on issue https://github.com/heptio/velero/issues/1740.

For upgrades, we will provide a script that will:

- change the tag on the Velero deployment yaml for both the main image and any of th three plugins installed.
- rename existing aws, azure or gcp plugin names to have the `velero.io/` namespace preceding the name (ex: `velero.io/aws).

Alternatively, we could add CLI `velero upgrade` command that would make these changes. Ex: `velero upgrade 1.3` would upgrade from `v1.2` to `v1.3`.

For upgrading:

- Edit the provider field in the backupstoragelocations and volumesnapshotlocations CRDs to include the new namespace.

## Alternatives Considered

We considered having the plugins all live in the same GH repository. The downside of that approach is ending up with a binary and image bigger than necessary, since they would contain the SDKs of all three providers.

## Security Considerations

- Ensure that only the Velero core team has maintainer/owner privileges.