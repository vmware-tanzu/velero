# Support for multiple provider credentials

Currently, Velero only supports a single credential secret per location provider/plugin. Velero creates and stores the plugin credential secret under the hard-coded key `secret.cloud-credentials.data.cloud`.

This makes it so switching from one plugin to another necessitates overriding the existing credential secret with the appropriate one for the new plugin.

## Goals

- To allow Velero to create and store multiple secrets for provider credentials, even multiple credentials for the same provider
- To improve the UX for configuring the velero deployment with multiple plugins/providers.

## Non Goals

- To make any change except what's necessary to handle multiple credentials
- To allow multiple credentials for or change the UX for node-based authentication (e.g. AWS IAM, GCP Workload Identity, Azure AAD Pod Identity).

## Design overview

Instead of one credential per Velero deployment, multiple credentials can be added and used with different BSLs VSLs.
  
There are two aspects to handling multiple credentials:

- Modifying how credentials are configured and specified by the user
- Modifying how credentials are provided to the plugin processes

Each of these aspects will be discussed in turn.

### Credential configuration

Currently, Velero creates a secret (`cloud-credentials`) during install with a single entry that contains the contents of the credentials file passed by the user.

Instead of adding new CLI options to Velero to create and manage credentials, users will create their own Kubernetes secrets within the Velero namespace and reference these.
This approach is being chosen as it allows users to directly manage Kubernetes secrets objects as they wish and it removes the need for wrapper functions to be created within Velero to manage the creation of secrets.
An initial approach to this problem included modifying the existing `cloud-credentials` secret to add a new entry with each new set of credentials.
It is likely that this approach would encounter problems as users added more credentials as the maximum size of Secret in Kubernetes is 1MB.
By allowing users to create Secrets as they need to, we remove these potential limitations.

To enable the use of existing Kubernetes secrets, BSLs and VSLs will be modified to have a new field `Credential`.
This field will be a [`SecretKeySelector`](https://godoc.org/k8s.io/api/core/v1#SecretKeySelector) which will enable the user to specify which key within a particular secret the BSL/VSL should use.

The CLI for managing BSLs and VSLs will be modified to allow the user to set these credentials.
Both `velero backup-location (create|set)` and `velero snapshot-location (create|set)` will have a new flag (`--credential`) to specify the secret and key within the secret to use.
This flag will take a key-value pair in the format `<secret-name>=<key-in-secret>`.
The arguments will be validated to ensure that the secret exists in the Velero namespace.

### Making credentials available to plugins

There are three different approaches that can be taken to provide credentials to plugin processes:

1. Providing the path to the credentials file as an environment variable per plugin. This is how credentials are currently passed.
1. Include the path to the credentials file in the `config` map passed to a plugin.
1. Include the details of the secret in the `config` map passed to a plugin.

The last two options require changes to the plugin as the plugin will need to instantiate a client using the provided credentials.
The client libraries used by the plugins will not be able to rely on the credentials details being available in the environment as they currently do.

We have selected option 2 as the approach to take which will be described below.

### Including the credentials file path in the `config` map

Prior to using any secret for a BSL or VSL, it will need to be serialized to disk.
Using the details in the `Credential` field in the BSL/VSL, the contents of the Secret will be read and serialized.
To achieve this, we will create a new package, `credentials`, which will introduce new types and functions to manage the fetching of credentials based on a `SecretKeySelector`.
This will also be responsible for serializing the fetched credentials to a temporary directory on the Velero pod filesystem.

The path where a set of credentials will be written to will be a fixed path based on the namespace, name, and key from the secret rather than a randomly named file as is usual with temporary files.
The reason for this is that `BackupStore`s are frequently created within the controllers and the credentials must be serialized before any plugin APIs are called, which would result in a quick accumulation of temporary credentials files.
For example, the default validation frequency for BackupStorageLocations is one minute.
This means that any time a `BackupStore`, or other type which requires credentials, is created, the credentials will be fetched from the API server and may overwrite any existing use of that credential.

If we instead wanted to use an unique file each time, we could work around the of multiple files being written by cleaning up the temporary files upon completion of the plugin operations, if this information is known.

Once the credentials have been serialized, this path will be made available to the plugins.
Instead of setting the necessary environment variable for the plugin process, the `config` map for the BSL/VSL will be modified to include an addiitional entry with the path to the credentials file: `credentialsFile`.
This will be passed through when [initializing the BSL/VSL](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/velero/object_store.go#L27-L30) and it will be the responsibility of the plugin to use the passed credentials when starting a session.
For an example of how this would affect the AWS plugin, see [this PR](https://github.com/vmware-tanzu/velero-plugin-for-aws/pull/69).

The restic controllers will also need to be updated to use the correct credentials.
The BackupStorageLocation for a given PVB/PVR will be fetched and the `Credential` field from that BSL will be serialized.
The existing setup for the restic commands use the credentials from the environment variables with [some repo provider specific overrides](https://github.com/vmware-tanzu/velero/blob/main/pkg/controller/pod_volume_backup_controller.go#L260-L273).
Instead of relying on the existing environment variables, if there are credentials for a particular BSL, the environment will be specifically created for each `RepoIdentifier`.
This will use a lot of the existing logic with the exception that it will be modified to work with a serialized secret rather than find the secret file from an environment variable.
Currently, GCP is the only provider that relies on the existing environment variables with no specific overrides.
For GCP, the environment variable will be overwritten with the path of the serialized secret.


## Backwards compatibility

For now, regardless of the approaches used above, we will still support the existing workflow.

Users will be able to set credentials during install and a secret will be created for them.
This secret will still be mounted into the Velero pods and the appropriate environment variables set.
This will allow users to use versions of plugins which haven't yet been updated to use credentials directly, such as with many community created plugins.

Multiple credential handling will only be used in the case where a particular BSL/VSL has been modified to use an existing secret.

## Security Considerations

Although the handling of secrets will be similar to how credentials are currently managed within Velero, care must be taken to ensure that any new code does not leak the contents of secrets, for example, including them within logs.

## Alternatives Considered

As mentioned above, there were three potential approaches for providing this support.
The approaches that were not selected are detailed below for reference.

#### Providing the credentials via environment variables

To continue to provide the credentials via the environment, plugins will need to be invoked differently so that the correct credential is used.
Currently, there is a single secret, which is mounted into every pod deployed by Velero (the Velero Deployment and the Restic DaemonSet) at the path `/credentials/cloud`.
This path is made known to all plugins through provider specific environment variables and all possible provider environment variables are set to this path.

Instead of setting the environment variables for all the pods, we can modify plugin processes are created so that the environment variables are set on a per plugin process basis.
Prior to using any secret for a BSL or VSL, it will need to be serialized to disk.
Using the details in the `Credential` field in the BSL/VSL, the contents of the Secret will be read and serialized to a file.
Each plugin process would still have the same set of environment variables set, however the value used for each of these variables would instead be the path to the serialized secret.

To set the environment variables for a plugin process, the plugin manager must be modified so that when creating an ObjectStore or VolumeSnapshotter, we pass in the entire BSL/VSL object, rather than [just the provider](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/manager.go#L132-L158).
The plugin manager currently stores a map of [plugin executables to an associated `RestartableProcess`](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/manager.go#L59-L70).
New restartable processes are created only [with the executable that the process would run](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/manager.go#L122).
This could be modified to also take the necessary environment variables so that when [underlying go-plugin process is created](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/client_builder.go#L78), these environment variables could be provided and would be set on the plugin process.

Taking this approach would not require any changes from plugins as the credentials information would be made available to them in the same way.
However, it is quite a significant change in how we initialize and invoke plugins.

We would also need to ensure that the restic controllers are updated in the same way so that correct credentials are used (when creating a `ResticRepository` or processing `PodVolumeBackup`/`PodVolumeRestore`).
This could be achieved by modifying the existing function to [run a restic command](https://github.com/vmware-tanzu/velero/blob/main/pkg/restic/repository_manager.go#L237-L290).
This function already sets environment variables for the restic process depending on which storage provider is being used.

#### Include the details of the secret in `config` map passed to a plugin

This approach is like the selected approach of passing the credentials file via the `config` map, however instead of the Velero process being responsible for serializing the file to disk prior to invoking the plugin, the `Credential SecretKeySelector` details will be passed through to the plugin.
It will be the responsibility of the plugin to fetch the secret from the Kubernetes API and perform the necessary steps to make it available for use when creating a session, for example, serializing the contents to disk, or evaluating the contents and adding to the process environment.

This approach has an additional burden on the plugin author over the previous approach as it requires the author to create a client to communicate with the Kubernetes API to retrieve the secret.
Although it would be the responsibility of the plugin to serialize the credential and use it directly, Velero would still be responsible for serializing the secret so that it could be used with the restic controllers as in the selected approach.
