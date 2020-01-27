# Proposal for a more intuitive CLI to install and configure Velero

Currently, the Velero CLI tool has a `install` command that configures numerous major and minor aspects of Velero. As a result, the combined set of flags for this `install` command makes it hard to intuit and reason about the different Velero components. This document proposes changes to improve the UX for installation and configuration in a way that would make it easier for the user to discover what needs to be configured by looking at what is available in the CLI rather then having to rely heavily on our documentation for the usage. At the same time, it is expected that the documentation update to reflect these changes will also make the documentation flow easier to follow.

This proposal prioritizes discoverability and self-documentation over minimalizing length or number of commands and flags.

## Goals

- Split flags currently under the `velero install` command into multiple commands
- Groups flags under commands in a way that allows a good level of discovery and self-documentation
- Rename commands and flags as needed

## Non Goals

- Introduce new CLI features (new commands for existing functionality ok)
- Propose changes to the CLI that go beyond the functionality of install and configure
- Optimize for shorter length or number of commands/flags

## Background

This document proposes users could benefit from a more intuitive and self-documenting CLI setup as compared to our existing CLI UX. Ultimately, it is proposed that a recipe-style CLI flow for installation, configuration and use would greatly contribute to this purpose.

Also, the `install` command currently can be reused to update Velero configurations, a behavior more appropriate for a command named `config`.

## High-Level Design

<!-- One to two paragraphs that describe the high level changes that will be made to implement this proposal. -->
The naming and organization of the proposed new CLI commands below have been inspired on the `kubectl` commands, particularly `kubectl set` and `kubectl config`.

#### Grouping commands

Below is the proposed set of new commands to install and configure Velero.

1) `velero init`
Configures up the namespace, RBAC, deployment, etc., but does not add any external plugins, BSL/VSL definitions. This would be the minimum set of commands to get the Velero server up and running and ready to accept other configurations. Mostly things to be run once at setup time. Could be named something else, like `install`.

These ones might make sense to include under all the other commands
```
      --dry-run                                    generate resources, but don't send them to the cluster. Use with -o. Optional.
      -h, --help                                   help for install
      --label-columns stringArray                  a comma-separated list of labels to be displayed as columns
      -o, --output string                          Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --wait                                       wait for Velero deployment to be ready. Optional.
      --show-labels                                show labels in the last column
```

Minimum set for initialization
```
      --image string                               image to use for the Velero and restic server pods. Optional. (default "velero/velero:latest")
      --sa-annotations mapStringString             annotations to add to the Velero ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
```
   
2) `velero set`
All the other configuration that is not component specific nor necessary for initialization. Mostly things to be updated. Might be grouped under `init`.

```
      --pod-annotations mapStringString            annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
      --restore-only                               run the server in restore-only mode. Optional.
      --pod-cpu-limit string                       CPU limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --pod-cpu-request string                     CPU request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --pod-mem-limit string                       memory limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "256Mi")
      --pod-mem-request string                     memory request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "128Mi")
```

3) `velero config`
Component specific configuration for both backup and snapshot locations. 

Much like `kubectl config` is about clusters and context, so does this `velero config` should be only about the backup/snapshot locations.

```
      --no-secret                                       flag indicating if a secret should be created. Must be used as confirmation if --secret-file is not provided. Optional.
      --secret-file string                              file containing credentials for backup and volume provider. If not specified, --no-secret must be used for confirmation. Optional.

      --backup-location mapStringString                 configuration to use for creating a backup storage location. Format is key1=value1,key2=value2 (was `backup-location-config`)
      --get-backup-locations                            Display backup storage locations
      --current-backup-location                         displays the current default backup storage location (NEW)
      --use-backup-location string                      sets the default backup storage location (default "default") (was `default-backup-storage-location`)

      --snapshot-location  mapStringString              configuration to use for creating a volume snapshot location. Format is key1=value1,key2=value2 (was `snapshot-location-config`)
      --get-snapshot-locations                          Display snapshot locations
      --current-snapshot-locations                      displays the current default volume snapshot locations (NEW)
      --use-snapshot-locations mapStringString          sets the list of unique volume providers and default volume snapshot location (provider1:location-01,provider2:location-02,...) (was `default-volume-snapshot-locations`)

      --set-default-location                            configuration to create a default locations. Format is bucket=value,prefix=value,plugin-name=value,snapshot-location=true/false. Optional.

      --provider string                                 provider name for backup and volume storage - (DEPRECATED)
      --no-default-backup-location                      flag indicating if a default backup location should be created. Must be used as confirmation if --bucket or --provider are not provided. Optional. (DEPRECATED)
      --use-volume-snapshots                            whether or not to create snapshot location automatically. Set to false if you do not plan to create volume snapshots via a storage provider. (default true) (DEPRECATED)
```

4) `velero plugin`
Component specific configuration for plugins.

```
      --add stringArray                           Add plugin container images to install into the Velero Deployment
      --list                                      Get information for all plugins on the velero server (was `get`)
      --remove                                    Remove a plugin
      --plugin-dir string                         directory containing Velero plugins (default "/plugins")
```

5) `velero restic`
Component specific configuration for restic operations.

```
      --default-prune-frequency duration          how often 'restic prune' is run for restic repositories by default. Optional.
      --pod-annotations mapStringString           annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
      --pod-cpu-limit string                      CPU limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --pod-cpu-request string                    CPU request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --pod-mem-limit string                      memory limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --pod-mem-request string                    memory request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --create                                    create restic deployment. Optional. (was `use-restic`)
      --repo                                      Work with restic repositories
      --restic-timeout duration                   how long backups/restores of pod volumes should be allowed to run before timing out (default 1h0m0s)
```

#### Example

Considering this proposal, let's exemplify what a high-level documentation for getting Velero ready to do backups could look like for two types of Velero users, in a very recipe-like manner:

##### Administrator

After installing the Velero CLI:
```
velero init ... (required setup)
velero set ... (optional setup)
velero plugin ... (add/manage provider connectors)
velero config ... (run `velero plugin --list` to see what you can use to configure locations; configure locations)
```


##### Operator

```
velero plugin ... Optional. (manage list of provider connectors as needed)
velero config ... Optional. (run `velero config --get-backup-locations` to see available backup locations or `velero config --current-backup-location`; then run `velero plugin --list` to see what providers you can use if you need to configure new locations; configure locations)
velero backup/restore/schedule create/get/delete <NAME> ...
```

The above recipe-style documentation should highlight 1) the main components of Velero, and, 2) the relationship/dependency between the main components

## Detailed Design

A detailed design describing how the changes to the product should be made.

The names of types, fields, interfaces, and methods should be agreed on here, not debated in code review.
The same applies to changes in CRDs, YAML examples, and so on.

Ideally the changes should be made in sequence so that the work required to implement this design can be done incrementally, possibly in parallel.

## Alternatives Considered

If there are alternative high level or detailed designs that were not pursued they should be called out here with a brief explanation of why they were not pursued.

## Security Considerations

If this proposal has an impact to the security of the product, its users, or data stored or transmitted via the product, they must be addressed here.
