# Custom CA Bundle Support for S3 Object Storage

It is desired that Velero performs SSL verification on the Object Storage
endpoint (BackupStorageLocation), but it is not guaranteed that the Velero
container has the endpoints' CA bundle in it's system store. Velero needs to
support the ability for a user to specify custom CA bundles at installation
time and Velero needs to support a mechanism in the BackupStorageLocation
Custom Resource to allow a user to specify a custom CA bundle. This mechanism
needs to also allow Restic to access and use this custom CA bundle.

## Goals

- Enable Velero to be configured with a custom CA bundle at installation
- Enable Velero support for custom CA bundles with S3 API BackupStorageLocations
- Enable Restic to use the custom CA bundles whether it is configured at installation time or on the BackupStorageLocation
- Enable Velero client to take a CA bundle as an argument

## Non Goals

- Support non-S3 providers

## Background

Currently, in order for Velero to perform SSL verification of the object
storage endpoint the user must manually set the `AWS_CA_BUNDLE` environment
variable on the Velero deployment. If the user is using Restic, the user has to
either:
1. Add the certs to the Restic container's system store
1. Modify Velero to pass in the certs as a CLI parameter to Restic - requiring
   a custom Velero deployment

## High-Level Design

There are really 2 methods of using Velero with custom certificates:
1. Including a custom certificate at Velero installation
1. Specifying a custom certificate to be used with a `BackupStorageLocation`

### Specifying a custom cert at installation

On the Velero deployment at install time, we can set the AWS environment variable
`AWS_CA_BUNDLE` which will allow Velero to communicate over https with the
proper certs when communicating with the S3 bucket. This means we will add the
ability to specify a custom CA bundle at installation time. For more
information, see "Install Command Changes".

On the Restic daemonset, we will want to also mount this secret at a pre-defined
location. In the `restic` pkg, the command to invoke restic will need to be
updated to pass the path to the cert file that is mounted if it is specified in
the config.

This is good, but doesn't allow us to specify different certs when
`BackupStorageLocation` resources are created.

### Specifying a custom cert on BSL

In order to support custom certs for object storage, Velero will add an
additional field to the `BackupStorageLocation`'s provider `Config` resource to
provide a secretRef which will contain the coordinates to a secret containing
the relevant cert file for object storage. 

In order for Restic to be able to consume and use this cert, Velero will need
the ability to write the CA bundle somewhere in memory for the Restic pod to
consume it.

To accomplish this, we can look at the code for managing restic repository
credentials. The way this works today is that the key is stored in a secret in
the Velero namespace, and each time Velero executes a restic command, the
contents of the secret are read and written out to a temp file. The path to
this file is then passed to restic and removed afterwards. pass the path of the
temp file to restic, and then remove the temp file afterwards. See ref #1 and #2.

This same approach can be taken for CA bundles. The bundle can be stored in a
secret which is referenced on the BSL and written to a temp file prior to
invoking Restic.

[1](https://github.com/vmware-tanzu/velero/blob/main/pkg/restic/repository_manager.go#L238-L245)
[2](https://github.com/vmware-tanzu/velero/blob/main/pkg/restic/common.go#L168-L203)

## Detailed Design

The `AWS_CA_BUNDLE` environment variable works for the Velero deployment
because this environment variable is passed into the AWS SDK which is used in
the [plugin][1] to build up the config object. This means that a user can
simply define the CA bundle in the deployment as an env var. This can be
utilized for the installation of Velero with a custom cert by simply setting
this env var to the contents of the CA bundle, or the env var can be mapped to
a secret which is controlled at installation time. I recommend using a secret
as it makes the Restic integration easier as well.

At installation time, if a user has specified a custom cert then the Restic
daemonset should be updated to include the secret mounted at a predefined path.
We could optionally use the system store for all custom certs added at
installation time. Restic supports using the custom certs [in addition][3] to
the root certs.

In the case of the BSL being created with a secret reference, then at runtime
the secret will need to be consumed. This secret will be read and applied to
the AWS `session` object. The `getSession()` function will need to be updated
to take in the custom CA bundle so it can be passed [here][4].

The Restic controller will need to be updated to write the contents of the CA
bundle secret out to a temporary file inside of the restic pod.The restic
[command invocation][2] will need to be updated to include the path to the file
as an argument to the restic server using `--cacert`. For the path when a user
defines a custom cert on the BSL, Velero will be responsible for updating the
daemonset to include the secret mounted as a volume at a predefined path.

Where we mount the secret is a fine detail, but I recommend mounting the certs
to `/certs` to keep it in line with the other volume mount paths being used.

### Install command changes

The installation flags should be updated to include the ability to pass in a
cert file. Then the install command would do the heavy lifting of creating a
secret and updating the proper fields on the deployment and daemonset to mount
the secret at a well defined path.

### Velero client changes

Since the Velero client is responsible for gathering logs and information about
the Object Storage, this implementation should include a new flag `--cacert`
which can be used when communicating with the Object Storage. Additionally, the
user should be able to set this in their client configuration. The command
would look like:
```
$ velero client config set cacert PATH
```

[1]: https://github.com/vmware-tanzu/velero-plugin-for-aws/blob/main/velero-plugin-for-aws/object_store.go#L135
[2]: https://github.com/vmware-tanzu/velero/blob/main/pkg/restic/command.go#L47
[3]: https://github.com/restic/restic/blob/main/internal/backend/http_transport.go#L81
[4]: https://github.com/vmware-tanzu/velero-plugin-for-aws/blob/main/velero-plugin-for-aws/object_store.go#L154
