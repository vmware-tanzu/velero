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

#### General CLI improvements

These are improvements that are part of this proposal:
- Go over all flags and document what is optional, what is required, and default values.
- Capitalize all help messages

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
        --timeout duration                          how long backups/restores of pod volumes should be allowed to run before timing out (default 1h0m0s)
        repo  
          get                                         Get restic repositories
```
The `velero config server` command will create the following resources:

```
Namespace
Deployment
backups.velero.io
backupstoragelocations.velero.io
deletebackuprequests.velero.io
downloadrequests.velero.io
podvolumebackups.velero.io
podvolumerestores.velero.io
resticrepositories.velero.io
restores.velero.io
schedules.velero.io
serverstatusrequests.velero.io
volumesnapshotlocations.velero.io
```

Note: Velero will maintain the `velero server` command run by the Velero pod, which starts the Velero server deployment.

2) `velero backup-location`
Commands/flags for backup locations.

```
      set
        --default string                                  sets the default backup storage location (default "default") (NEW, -- was `server --default-backup-storage-location; could be set as an annotation on the BSL)
        --credentials mapStringString                     sets the name of the corresponding credentials secret for a provider. Format is provider:credentials-secret-name. (NEW)
        --cacert-file mapStringString                     configuration to use for creating a secret containing a custom certificate for an S3 location of a plugin provider. Format is provider:path-to-file. (NEW)

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
        --credentials mapStringString                     sets the name of the corresponding credentials secret for a provider. Format is provider:credentials-secret-name. (NEW)
        --cacert-file mapStringString                     configuration to use for creating a secret containing a custom certificate for an S3 location of a plugin provider. Format is provider:path-to-file. (NEW)

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
        --credentials mapStringString                     sets the list of name of the corresponding credentials secret for providers. Format is (provider1:credentials-secret-name1,provider2:credentials-secret-name2,...) (NEW)

      create                                              NAME [flags]
        --default                                         Sets these new locations to be the new default snapshot locations. Default is false. (NEW)
        --config  mapStringString                         configuration to use for creating a volume snapshot location. Format is key1=value1,key2=value2 (was also in `velero install --`snapshot-location-config`). Required.
        --provider string                                 provider name for volume storage. Required.
        --label-columns stringArray                       a comma-separated list of labels to be displayed as columns
        --labels mapStringString                          labels to apply to the volume snapshot location
        --provider string                                 name of the volume snapshot provider (e.g. aws, azure, gcp)
        --show-labels                                     show labels in the last column
        --credentials mapStringString                     sets the list of name of the corresponding credentials secret for providers. Format is (provider1:credentials-secret-name1,provider2:credentials-secret-name2,...) (NEW)

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
        --credentials-file mapStringString      configuration to use for creating a secret containing the AIM credentials for a plugin provider. Format is provider:path-to-file. (was `secret-file`)
        --no-secret                             flag indicating if a secret should be created. Must be used as confirmation if create --secret-file is not provided. Optional. (MOVED FROM install -- not sure we need it?)
        --sa-annotations mapStringString        annotations to add to the Velero ServiceAccount for GKE. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
```

#### Example

Considering this proposal, let's consider what a high-level documentation for getting Velero ready to do backups could look like for Velero users:

After installing the Velero CLI:
```
velero config server [flags] (required)
velero config restic [flags]
velero plugin add IMAGES [flags] (add/config provider plugins)
velero backup-location/snapshot-location create NAME [flags] (run `velero plugin --get` to see what kind of plugins are available; create locations)
velero backup/restore/schedule create/get/delete NAME [flags]
```

The above recipe-style documentation should highlight 1) the main components of Velero, and, 2) the relationship/dependency between the main components

### Deprecation

#### Timeline

In order to maintain compatibility with the current Velero version for a sufficient amount of time, and give users a chance to upgrade any install scripts they might have, we will keep the current `velero install` command in parallel with the new commands until the next major Velero version, which will be Velero 2.0. In the mean time, ia deprecation warning will be added to the `velero install` command. 

#### Commands/flags deprecated or moved

##### Velero Install 
`velero install (DEPRECATED)`

Flags moved to...

...`velero config server`:
```
      --image string                               image to use for the Velero and restic server pods. Optional. (default "velero/velero:latest")
      --label-columns stringArray                  a comma-separated list of labels to be displayed as columns
      --pod-annotations mapStringString            annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
      --show-labels                                show labels in the last column
      --pod-cpu-limit string                CPU limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --pod-cpu-request string              CPU request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --pod-mem-limit string                memory limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "256Mi")
      --pod-mem-request string              memory request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "128Mi")
```

...`velero config restic`
```
      --default-prune-frequency duration    how often 'restic prune' is run for restic repositories by default. Optional.
      --pod-cpu-limit string                CPU limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --pod-cpu-request string              CPU request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --pod-mem-limit string                memory limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
      --pod-mem-request string              memory request for restic pod. A value of "0" is treated as unbounded. Optional. (default "0")
```

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

...`plugin`
```
      --plugins stringArray                        Plugin container images to install into the Velero Deployment
      --sa-annotations mapStringString             annotations to add to the Velero ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
      --no-secret                                  flag indicating if a secret should be created. Must be used as confirmation if --secret-file is not provided. Optional.
      --secret-file string  (renamed `credentials-file`)   file containing credentials for backup and volume provider. If not specified, --no-secret must be used for confirmation. Optional.
```

Flags to deprecate:
```
      --no-default-backup-location                 flag indicating if a default backup location should be created. Must be used as confirmation if --bucket or --provider are not provided. Optional.
      --use-volume-snapshots                       whether or not to create snapshot location automatically. Set to false if you do not plan to create volume snapshots via a storage provider. (default true)
      --wait                                       wait for Velero deployment to be ready. Optional.
      --use-restic                                 (obsolete since now we have `velero config restic`)
```

##### Velero Server

These flags will be moved to under `velero config server`:

`velero server --default-backup-storage-location (DEPRECATED)` changed to `velero backup-location set --default`

`velero server --default-volume-snapshot-locations (DEPRECATED)` changed to `velero snapshot-location set --default`

The value for these flags will be stored as annotations.

## Detailed Design

#### Handling CA certs

In anticipation of a new configuration implementation to handle custom CA certs (as per design doc https://github.com/vmware-tanzu/velero/blob/main/design/custom-ca-support.md), a new flag `velero storage-location create/set --cacert-file mapStringString` is proposed. It sets the configuration to use for creating a secret containing a custom certificate for an S3 location of a plugin provider. Format is provider:path-to-file.

See discussion https://github.com/vmware-tanzu/velero/pull/2259#discussion_r384700723 for more clarification.

#### Renaming "provider" to "location-plugin"

As part of this change, we should change to use the term `location-plugin` instead of `provider`. The reasoning: in practice, we usually have 1 plugin per provider, and if there is an implementation for both object store and volume snapshotter for that provider, it will all be contained in the same plugin. When we handle plugins, we follow this logic. In other words, there's a plugin name (ex: `velero.io/aws`) and it can contain implementations of kind `ObjectStore` and/or `VolumeSnapshotter`.

But when we handle BSL or VSL (and the CLI commands/flags that configure them), we use the term `provider`, which can cause ambiguity as if that is a kind of thing different from a plugin. If the plugin is the "thing" that contains the implementation for the desired provider, we should make it easier for the user to guess that and change BackupStorageLocation/VolumeSnapshotLocation `Spec.Provider` field to be called `Spec.Location-Plugin` and all related CLI command flags to `location-plugin`, and update the docs accordingly.

This change will require a CRD version bump and deprecation cycle.

#### GitOps Compatibility

To maintain compatibility with gitops practices, each of the new commands will generate `yaml` output that can be stored in source control.

For content examples, please refer to the files here:

https://github.com/carlisia/velero/tree/c-cli-design/design/CLI/PoC

Note: actual `yaml` file names are defined by the user.

`velero config server` - base/deployment.yaml

`velero config restic` - overlays/plugins/restic.yaml

`velero backup-location create` - base/backupstoragelocations.yaml

`velero snapshot-location create` - base/volumasnapshotlocations.yaml

`velero plugin add velero/velero-plugin-for-aws:v1.0.1` - overlays/plugins/aws-plugin.yaml

`velero plugin add velero/velero-plugin-for-microsoft-azure:v1.0.1` - overlay/plugins/azure-plugin.yaml

These resources can be deployed/deleted using the included kustomize setup and running:

```
kubectl apply -k design/CLI/PoC/overlays/plugins/

kubectl delete -k design/CLI/PoC/overlays/plugins/
```

Note: All CRDs, including the `ResticRepository`, may continue to be deployed at startup as it is now, or together with their respective instantiation.


#### Changes to startup behavior

To recap, this proposal redesigns the Velero CLI to make `velero install` obsolete, and instead breaks down the installation and configuration into separate commands. These are the major highlights:

- Plugins will only be installed separately via `velero plugin add`
- BSL/VSL will be continue to be configured separately, and now each will have an associated secret

Since each BSL/VSL will have its own association with a secret, the user will no longer need to upload a new secret whenever changing to, or adding, a BSL/VSL for a provider that is different from the one in use. This will be done at setup time. This will make it easier to support any number of BSL/VSL combinations, with different providers each.

The user will start up the Velero server on a cluster by using the command `velero config server`. This will create the Velero deployment resource with default values or values overwritten with flags, create the Velero CRDs, and anything else that is not specific to plugins or BSL/VSL.

The Velero server will start up, verify that the deployment is running, that all CRDs were found, and log a message that it is waiting for a BSL to be configured. at this point, other operations, such as configuring restic, will be allowed. Velero should keep track of its status, ie, if it is ready to create backups or not. This could be a field `ServerStatus` added to `ServerStatusRequest`. Possible values could be [ready|waiting]. "ready" would mean there is at least 1 valid BSL, and "waiting" would be anything but that.

When adding/configuring a BSL or VSL, we will allow creating locations, and continuously verify if there is a corresponding, valid plugin. When a valid match is found, mark the BSL/VSL as "ready". This would require adding a field to the BSL/VSL, or using the existing `Phase` field, and keep track of its status, possibly: [ready|waiting].

With the first approach: the server would transition into "ready" (to create backups) as soon as there is one BSL. It would require a set sequence of actions, ie, first install the plugin, only then the user can successfully configure a BSL.

With the second approach, the Velero server would continue looping and checking all existing BSLs for at least 1 with a "ready" status. Once it found that, it would set itself to "ready" also.

Another new behavior that must be added: the server needs to identify when there no longer exists a valid BSL. At this point, it should change its status from "ready" to one that indicates it is not ready, maybe "waiting". With the first approach above, this would mean checking if there is still at least one BSL. With the second approach, it would require checking the status of all BSLs to find at least one with the status of "ready".

As it is today, a valid VSL would not be required to create backups, unless the backup included a PV.

To make it easier for the user to identify if their Velero server is ready to create backups or not, a `velero status` command should be added. This issue has been created some time ago for this purpose: https://github.com/vmware-tanzu/velero/issues/1094.

## Alternatives Considered

It seems that the vast majority of tools document their usage with `kubectl` and `yaml` files to install and configure their Kubernetes resources. Many of them also make use of Helm, and to a lesser extent some of them have their own CLI tools. 

Amongst the tools that have their own CLI, not enough examples were found to establish a clear pattern of usage. It seems the most relevant priority should be to have output in `yaml` format.

Any set of `yaml` files can also be arranged to use with Kustomize by creating/updating resources, and patching them using Kustomize functionalities.

The way the Velero commands were arranged in this proposal with the ability to output corresponding `yaml` files, and the included Kustomize examples, makes it in line with the widely used practices for installation and configuration.

Some CLI tools do not document their usage with Kustomize, one could assume it is because anyone with knowledge of Kustomize and `yaml` files would know how to use it.

Here are some examples:

https://github.com/jetstack/kustomize-cert-manager-demo

https://github.com/istio/installer/tree/master/kustomize

https://github.com/weaveworks/flagger/tree/master/kustomize

https://github.com/jpeach/contour/tree/1c575c772e9fd747fba72ae41ab99bdae7a01864/kustomize (RFC)

## Security Considerations

N/A
