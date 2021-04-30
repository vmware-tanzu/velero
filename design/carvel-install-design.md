# Design proposal for Velero Installation with Carvel

Currently, the Velero CLI tool has a `install` command that configures numerous major and minor aspects of Velero. This document outlines how we can also have Velero be installed and upgraded using the Carvel tooling.


## Goals
- Have one source of truth for all Velero configuration
- Have compatibility with gitop practices (i.e. ability to generate a full set of yaml for install that can be stored in source control)
- Have a clear path for deprecating of the `install` command

## Requirements
- Ability for the Carvel deployment to consume/inject configuration data as needed for a normal Velero deployment with default resources. For example: it should configure a default BSL alongside the deployment.
- The Carvel toolset must be able to, in addition to install, upgrade Velero as well as update its configuration settings.
- "Ask once" approach to configuring the Velero deployment defaults. In other words: reuse given parameters for putting together the needed overlays and packaging configuration.
- Ideally, setup the Carvel tooling so that it discovers and uses configuration data from known distros.
- The toolset and artifacts must be setup in a way that it will access and consume the CRDs that match the intended Velero version. Users should have uncomplicated access to the proper versions of the CRDs to be able to upgrade and downgrade Velero.
- The toolset and artifacts must be laid out in a directory structure in a way that it can be discovered and used by other projects.
- Migrate deployment configuration from using Go code to using Carvel manifests, and wrap the `install` command around the toolset.
- Deprecate the `install` command once the Carvel tooling is fully setup and well tested.
- Envirornments that need to be supported: AWS, VSphere, Azure.

## Non Goals
- Introduce new configuration
- Propose changes to the configuration that go beyond the ability to install and update as currently exists.