# Design proposal template (replace with your proposal's title)

Currently, Velero only supports a single AIM access credential secret per location provider/plugin. Velero creates and stores the plugin credential secret under the hard-coded key `secret.cloud-credentials.data.cloud`.

This makes it so switching from one plugin to another necessitates overriding the existing credential secret with the appropriate one for the new plugin.

## Goals

- To allow Velero to create and store multiple secrets for provider credentials, even multiple credentials for the same provider
- To improve the UX for configuring the velero deployment with multiple plugins/providers, and corresponding IAM secrets.

## Non Goals

- To make any change except what's necessary to handle multiple credentials

## Design overview

- Instead of one credential per Velero deployment, multiple credentials can be added and will be associated with plugins.
  Credentials can be added when plugins are added or can be set later.
  
- When new credentials are added, the `cloud-credentials` secret will be updated with a new entry containing the new credentials value.
  We will add keys to the existing secret, rather than creating new ones, due to the fact that it is not possible to modify volume mounts in a running pod or container.
  Changes to secrets (such as adding new data keys) are automatically updated within the volume mount.
  This means that any update to the `cloud-credentials` secret would automatically be available to the pods where that secret is mounted.
  This allows us to work around the restriction of modifying volume mounts to instead still use a single secret, but add more keys to it for new sets of credentials.
 
- BSLs and VSLs will be updated to contain a reference to a particular credential name, which will be a key in the `cloud-credentials` secret.
  When a new BSL or VSL is created, a specific set of credentials can be selected for use.
  The set of credentials associated with a BSL or VSL will be used when running the plugin processes needed for that storage location.

## Detailed Design

- The name of the flag changes from `secret-file` to `--credentials-file`.

- The arguments to `velero plugin (add|set) --credentials-file` will be a map of the credentials name as a key, and the path to the file as a value.
  This way, we can have multiple credential secrets and each secret per provider/plugin will be unique.
  There are two ways in which we can choose to name the keys stored in the `cloud-credentials` secret:
    1. Combine the name of the provider (extracted from the plugin name) and the given credentials name.
       For example, if the user were to add a plugin using: `velero plugin add velero-plugin-for-aws --credentials-file project-1:/path/to/credentials`, an additional key `aws-project-1` would be added to the `cloud-credentials` secret with the encoded contents of `/path/to/credentials`.
       In the case where the plugin is not a Velero provided plugin, we would use the full name of the plugin but adapt the name to be valid for use as a key, for example, `myproject.io/cloud-provider` could become `myproject_io_cloud-provider`, and would be combined with the given name of the credential.
    1. Do not include the provider name in the resulting credentials key, and instead leave it up to the user to give a unique and identifying name for a credential.
       For example, a user adding a plugin with `velero plugin add velero-plugin-for-aws --credentials-file aws-project-1:/path/to/credentials` would result in the key `aws-project-1` being added to the `cloud-credentials` secret.

    Bridget: My preference is for the second of the two options above.
    Plugin names are provided by the plugins themselves, and there is special handling in the codebase for using plugins provided by the Velero team vs plugins provided by the community.
    Plugins provided by Velero such as `aws`, `azure`, or `gcp` could be included in the key names for secrets easily, however the [rules that we enforce for plugin naming](https://velero.io/docs/v1.5/custom-plugins/#plugin-naming) result in plugin names that are not valid as key names in secrets.
    This means that we will need to convert them into a valid form where only alphanumeric characters, `-`, `_` or `.` are used.
    By having the user name the secrets directly, it will be explicit and clearer for the user to understand how credentials are added and stored, and make it easier for them to understand and edit their own deployments.

- See discussion https://github.com/vmware-tanzu/velero/pull/2259#discussion_r384700723 for the two items below.
    - The `velero backup-location (create|set)` will have a new flag to set the credentials based on the option selected above. If the first option, the new flag will be `--credentials mapStringString` which sets the name of the corresponding credentials secret for a provider. Format is provider:credentials-secret-name.
      If the second option, the new flag will be `--credentials string` where the argument will be the name of the credential set when adding a plugin.
    - The `velero snapshot-location (create|set)` will have a new flag to set the credentials based on the option selected above. If the first option, the new flag will be `--credentials mapStringString` which sets the name of the corresponding credentials secret for a provider. Format is provider:credentials-secret-name.
      If the second option, the new flag will be `--credentials string` where the argument will be the name of the credential set when adding a plugin.

  Note that for this logic to work we must have a controller loop checking for when a corresponding secret is present before marking the BSL/VSL as ready.

- The spec for BSLs and VSLs will updated to contain a new field `Credentials`. The value in this field will be the name of the key in the `cloud-credentials` secret to use for authenticating with the storage provider.

- Plugins will need to be invoked differently so that the correct credential is used.
  Currently, there is a single secret, which is mounted into every pod deployed by Velero (the Velero Deployment and the Restic DaemonSet).
  This secret contains a single data key (`cloud`) which is accessible within the pods at the path `/credentials/cloud`.
  This path is made known to all plugins through provider specific environment variables.
  All possible provider environment variables are set to this path.
  Instead of setting the environment within all the pods, we can modify `restartableProcess` to set the environment variables before running a plugin process.
  Each plugin process would still have the same set of environment variables set, however the value used for each of these variables would instead be a different path within the `/credentials` directory, formed from the credential selected for a particular BSL/VSL.
  Taking this approach would not require any changes from plugins as the credentials information would be made available to them in the same way.
  We will also need to ensure that the restic controllers are updated in the same way so that correct credentials are used (when creating a `ResticRepository` or processing `PodVolumeBackup`/`PodVolumeRestore`).

## Alternatives Considered

Instead of associating credentials with plugins, credentials will exist as standalone entities which can be queried using a new command `velero credentials`.
This will be a wrapper around Kubernetes API calls to query the secret used by Velero.
Credentials can be listed using `velero credentials get`, which will list the names of the keys within the `cloud-credentials` secret.
Credentials can be added or removed using `velero credentials add|delete`.

The rest of the workflow of how to use these credentials would still apply, such as specifying the name of the credential to use for a particular BSL/VSL.

## Security Considerations

N/A
