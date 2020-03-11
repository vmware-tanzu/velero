# Proposal for a more intuitive CLI to install and configure Velero

Currently, the Velero CLI tool has a `install` command that configures numerous major and minor aspects of Velero. As a result, the combined set of flags for this `install` command makes it hard to intuit and reason about the different Velero components. This document proposes changes to improve the UX for installation and configuration in a way that would make it easier for the user to discover what needs to be configured by looking at what is available in the CLI rather then having to rely heavily on our documentation for the usage. At the same time, it is expected that the documentation update to reflect these changes will also make the documentation flow easier to follow.

This proposal prioritizes discoverability and self-documentation over minimizing length or number of commands and flags.

## Goals

- Split flags currently under the `velero install` command into multiple commands, and group flags under commands in a way that allows a good level of discovery and self-documentation
- Maintain compatibility with gitops practices (i.e. ability to generate a full set of yaml for install that can be stored in source control)
- Have a clear path for deprecating commands

## Non Goals

- Introduce new CLI features 
- Propose changes to the CLI that go beyond the functionality of install and configure
- Optimize for shorter length or number of commands/flags

## Background

This document proposes users could benefit from a more intuitive and self-documenting CLI setup as compared to our existing CLI UX. Ultimately, it is proposed that a recipe-style CLI flow for installation, configuration and use would greatly contribute to this purpose.

Also, the `install` command currently can be reused to update Velero deployment configurations. For server and restic related install and configurations, settings will be moved to under `velero config`.

## High-Level Design

The naming and organization of the proposed new CLI commands below have been inspired on the `kubectl` commands, particularly `kubectl set` and `kubectl config`.

#### Commands

The organization of the commands follows this format:

```
velero [resource] [operation] [flags]
```

To conform with Velero's current practice: 
- commands will also work by swapping the operation/resource.
- the "object" of a command is an argument, and flags are strictly for modifiers (example: `backup get my-backup` and not `backup get --name my-backup`)

All commands will include the `--dry-run` flag, which can be used to output yaml files containing the commands' configuration for resource creation or patching.

`--dry-run                                    generate resources, but don't send them to the cluster. Use with -o. Optional.`

The `--help` and `--output` flags will also be included for all commands, omitted below for brevity.

Below is the proposed set of new commands to setup and configure Velero. 

1) `velero config`

```
      server                                         Configure up the namespace, RBAC, deployment, etc., but does not add any external plugins, BSL/VSL definitions. This would be the minimum set of commands to get the Velero server up and running and ready to accept other configurations.
        --label-columns stringArray                  a comma-separated list of labels to be displayed as columns
        --show-labels                                show labels in the last column
        --image string                               image to use for the Velero and restic server pods. Optional. (default "velero/velero:latest")
        --pod-annotations mapStringString            annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
        --restore-only                               run the server in restore-only mode. Optional.
        --pod-cpu-limit string                       CPU limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
        --pod-cpu-request string                     CPU request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "500m")
        --pod-mem-limit string                       memory limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "256Mi")
        --pod-mem-request string                     memory request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "128Mi")
        --client-burst int                           maximum number of requests by the server to the Kubernetes API in a short period of time (default 30)
        --client-qps float32                         maximum number of requests per second by the server to the Kubernetes API once the burst limit has been reached (default 20)
        --default-backup-ttl duration                how long to wait by default before backups can be garbage collected (default 720h0m0s)
        --disable-controllers strings                list of controllers to disable on startup. Valid values are backup,backup-sync,schedule,gc,backup-deletion,restore,download-request,restic-repo,server-status-request
        --log-format                                 the format for log output. Valid values are text, json. (default text)
        --log-level                                  the level at which to log. Valid values are debug, info, warning, error, fatal, panic. (default info)
        --metrics-address string                     the address to expose prometheus metrics (default ":8085")
        --plugin-dir string                          directory containing Velero plugins (default "/plugins")
        --profiler-address string                    the address to expose the pprof profiler (default "localhost:6060")
        --restore-only                               run in a mode where only restores are allowed; backups, schedules, and garbage-collection are all disabled. DEPRECATED: this flag will be removed in v2.0. Use read-only backup storage locations instead.
        --restore-resource-priorities strings        desired order of resource restores; any resource not in the list will be restored alphabetically after the prioritized resources (default [namespaces,storageclasses,persistentvolumes,persistentvolumeclaims,secrets,configmaps,serviceaccounts,limitranges,pods,replicaset,customresourcedefinitions])
        --terminating-resource-timeout duration      how long to wait on persistent volumes and namespaces to terminate during a restore before timing out (default 10m0s)

      restic                                        Configuration for restic operations.
        --default-prune-frequency duration          how often 'restic prune' is run for restic repositories by default. Optional.
        --pod-annotations mapStringString           annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
        --pod-cpu-limit string                      CPU limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
        --pod-cpu-request string                    CPU request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
        --pod-mem-limit string                      memory limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
        --pod-mem-request string                    memory request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
        --deployment                                create restic deployment. Default is false. Optional. Other flags will only work if set to true. (NEW, was `velero install use-restic`)
        --timeout duration                          how long backups/restores of pod volumes should be allowed to run before timing out (default 1h0m0s)
        repo  
          get                                         Get restic repositories
```

2) `velero backup-location`
Commands/flags for backup locations.

```
      set 
        --default string                                  sets the default backup storage location (default "default") (NEW, -- was `server --default-backup-storage-location; could be set as an annotation on the BSL)
        --cacert string                                   sets the name of the corresponding CA cert secret for the object storage

      create                                              NAME [flags]
        --default                                         Sets this new location to be the new default backup location. Default is false. (NEW) 
        --access-mode                                     access mode for the backup storage location. Valid values are ReadWrite,ReadOnly (default ReadWrite)
        --backup-sync-period 0s                           how often to ensure all Velero backups in object storage exist as Backup API objects in the cluster. Optional. Set this to 0s to disable sync
        --bucket string                                   name of the object storage bucket where backups should be stored. Required.
        --config mapStringString                          configuration to use for creating a backup storage location. Format is key1=value1,key2=value2 (was also in `velero install --backup-location-config`). Required for Azure.
        --provider string                                 provider name for backup storage. Required.
        --label-columns stringArray                       a comma-separated list of labels to be displayed as columns
        --labels mapStringString                          labels to apply to the backup storage location
        --prefix string                                   prefix under which all Velero data should be stored within the bucket. Optional.
        --provider string                                 name of the backup storage provider (e.g. aws, azure, gcp)
        --show-labels                                     show labels in the last column
        --cacert string                                   sets the name of the corresponding CA cert secret for the object storage

      get                                                 Display backup storage locations
        --default                                         displays the current default backup storage location (NEW)
        --label-columns stringArray                       a comma-separated list of labels to be displayed as columns
        -l, --selector string                             only show items matching this label selector
        --show-labels                                     show labels in the last column

```

3) `velero snapshot-location`
Commands/flags for snapshot locations. 

```
     set 
        --default mapStringString                         sets the list of unique volume providers and default volume snapshot location (provider1:location-01,provider2:location-02,...) (NEW, -- was `server --default-volume-snapshot-locations; could be set as an annotation on the VSL)  

      create                                              NAME [flags]
        --default                                         Sets these new locations to be the new default snapshot locations. Default is false. (NEW) 
        --config  mapStringString                         configuration to use for creating a volume snapshot location. Format is key1=value1,key2=value2 (was also in `velero install --`snapshot-location-config`). Required.
        --provider string                                 provider name for volume storage. Required.
        --label-columns stringArray                       a comma-separated list of labels to be displayed as columns
        --labels mapStringString                          labels to apply to the volume snapshot location
        --provider string                                 name of the volume snapshot provider (e.g. aws, azure, gcp)
        --show-labels                                     show labels in the last column

      get                                                 Display snapshot locations
        --default                                         list of unique volume providers and default volume snapshot location (provider1:location-01,provider2:location-02,...) (NEW -- was `server --default-volume-snapshot-locations`))
```

4) `velero plugin`
Configuration for plugins.

```
      add stringArray                           IMAGES [flags] - add plugin container images to install into the Velero Deployment

      get                                       get information for all plugins on the velero server (was `get`)
        --timeout duration                      maximum time to wait for plugin information to be reported (default 5s)

      remove                                    Remove a plugin [NAME | IMAGE] 

      set 
        --secret-file string                    PATH file containing credentials for plugin provider. If not specified, set --no-secret must be used for confirmation. Optional (MOVED FROM install). [NOTE]: we currently only support a single secret per provider 
        --no-secret                             flag indicating if a secret should be created. Must be used as confirmation if create --secret-file is not provided. Optional. (MOVED FROM install)
        --sa-annotations mapStringString        annotations to add to the Velero ServiceAccount for GKE. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
        --cacert-file string                    PATH file containing the certificate for the S3 location
```

#### Example

Considering this proposal, let's consider what a high-level documentation for getting Velero ready to do backups could look like for Velero users:

After installing the Velero CLI:
```
velero config server [flags] (required)
velero plugin add IMAGES [flags] (add/config provider plugins)
velero backup-location/snapshot-location create NAME [flags] (run `velero plugin --get` to see what kind of plugins are available; create locations)
velero backup/restore/schedule create/get/delete NAME [flags]
velero config restic [flags]
```

The above recipe-style documentation should highlight 1) the main components of Velero, and, 2) the relationship/dependency between the main components

### Deprecation

#### Timeline

In order to maintain compatibility with the current Velero version for a sufficient amount of time, and give users a chance to upgrade any install scripts they might have, we will keep the current `velero install` command in parallel with the new commands until the next major Velero version, which will be Velero 2.0. In the mean time, ia deprecation warning will be added to the `velero install` command. 

#### Commands/flags deprecated or moved

##### Velero Install 
`velero install (DEPRECATED)`

Flags moved to `velero config server`:
```
      --image string                               image to use for the Velero and restic server pods. Optional. (default "velero/velero:latest")
      --label-columns stringArray                  a comma-separated list of labels to be displayed as columns
      --pod-annotations mapStringString            annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
      --restore-only                               run the server in restore-only mode. Optional.
      --show-labels                                show labels in the last column
      --velero-pod-cpu-limit string                CPU limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --velero-pod-cpu-request string              CPU request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --velero-pod-mem-limit string                memory limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "256Mi")
      --velero-pod-mem-request string              memory request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "128Mi")
```

Flags to delete:
```
      --no-default-backup-location                 flag indicating if a default backup location should be created. Must be used as confirmation if --bucket or --provider are not provided. Optional.
      --use-volume-snapshots                       whether or not to create snapshot location automatically. Set to false if you do not plan to create volume snapshots via a storage provider. (default true)
      --wait                                       wait for Velero deployment to be ready. Optional.
```

Flags moved to...

...`backup-location create`
```
      --backup-location-config mapStringString     configuration to use for the backup storage location. Format is key1=value1,key2=value2
      --bucket string                              name of the object storage bucket where backups should be stored
      --prefix string                              prefix under which all Velero data should be stored within the bucket. Optional.
```

...`snapshot-location create`
```
      --snapshot-location-config mapStringString   configuration to use for the volume snapshot location. Format is key1=value1,key2=value2
```

...both `backup-location create` and `snapshot-location create`
```
      --provider string                            provider name for backup and volume storage 
```

...`velero config restic`
```
      --default-restic-prune-frequency duration    how often 'restic prune' is run for restic repositories by default. Optional.
      --restic-pod-cpu-limit string                CPU limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --restic-pod-cpu-request string              CPU request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --restic-pod-mem-limit string                memory limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --restic-pod-mem-request string              memory request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
```

...`plugin`
```
      --plugins stringArray                        Plugin container images to install into the Velero Deployment
      --sa-annotations mapStringString             annotations to add to the Velero ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
      --no-secret                                  flag indicating if a secret should be created. Must be used as confirmation if --secret-file is not provided. Optional.
      --secret-file string                         file containing credentials for backup and volume provider. If not specified, --no-secret must be used for confirmation. Optional.
```

##### Velero Server
`velero server (RENAMED velero config server)` 
 
`velero server --default-backup-storage-location (DEPRECATED)` moved to `velero backup-location set --default` 

`velero server --default-volume-snapshot-locations (DEPRECATED)` moved to `velero snapshot-location set --default`

`velero server --default-restic-prune-frequency (DEPRECATED)` moved to `velero config restic set --default-prune-frequency` 

`velero server --restic-timeout  (DEPRECATED)` moved to `velero config restic set timeout`

`velero server --use-restic  (DEPRECATED)` see `velero config restic`

All other `velero server` flags moved to under `velero config server`.

## General CLI improvements

These are improvements that are part of this proposal:
- Go over all flags and document what is optional, what is required, and default values.å
- Capitalize all help messages

## Detailed Design

A detailed design describing how the changes to the product should be made.

The names of types, fields, interfaces, and methods should be agreed on here, not debated in code review.
The same applies to changes in CRDs, YAML examples, and so on.

Ideally the changes should be made in sequence so that the work required to implement this design can be done incrementally, possibly in parallel.

## Alternatives Considered

If there are alternative high level or detailed designs that were not pursued they should be called out here with a brief explanation of why they were not pursued.

## Security Considerations

If this proposal has an impact to the security of the product, its users, or data stored or transmitted via the product, they must be addressed here.
