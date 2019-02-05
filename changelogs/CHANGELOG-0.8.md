- [v0.8.3](#v083)
- [v0.8.2](#v082)
- [v0.8.1](#v081)
- [v0.8.0](#v080)

## v0.8.3
#### 2018-06-29
### Download
  - https://github.com/heptio/ark/releases/tag/v0.8.3

### Bug Fixes:
  * Don't restore backup and restore resources to avoid possible data corruption (#622, @ncdc)


## v0.8.2
#### 2018-06-01
### Download
  - https://github.com/heptio/ark/releases/tag/v0.8.2

### Bug Fixes:
  * Don't crash when a persistent volume claim is missing spec.volumeName (#520, @ncdc)


## v0.8.1
#### 2018-04-23
### Download
  - https://github.com/heptio/ark/releases/tag/v0.8.1

### Bug Fixes:
  * Azure: allow pre-v0.8.0 backups with disk snapshots to be restored and deleted (#446 #449, @skriss)


## v0.8.0
#### 2018-04-19
### Download
  - https://github.com/heptio/ark/releases/tag/v0.8.0

### Highlights:
  * Backup deletion has been completely revamped to make it simpler and less error-prone. As a user, you still use the `ark backup delete` command to request deletion of a backup and its associated cloud
  resources; behind the scenes, we've switched to using a new `DeleteBackupRequest` Custom Resource and associated controller for processing deletion requests.
  * We've reduced the number of required fields in the Ark config. For Azure, `location` is no longer required, and for GCP, `project` is not needed.
  * Ark now copies tags from volumes to snapshots during backup, and from snapshots to new volumes during restore. 

### Breaking Changes:
  * Ark has moved back to a single namespace (`heptio-ark` by default) as part of #383.

### All New Features:
  * Add global `--kubecontext` flag to Ark CLI (#296, @blakebarnett)
  * Azure: support cross-resource group restores of volumes (#356 #378, @skriss)
  * AWS/Azure/GCP: copy tags from volumes to snapshots, and from snapshots to volumes (#341, @skriss)
  * Replace finalizer for backup deletion with `DeleteBackupRequest` custom resource & controller (#383 #431, @ncdc @nrb)
  * Don't log warnings during restore if an identical object already exists in the cluster (#405, @nrb)
  * Add bash & zsh completion support (#384, @containscafeine)
  
### Bug Fixes / Other Changes:
  * Error from the Ark CLI if attempting to restore a non-existent backup (#302, @ncdc)
  * Enable running the Ark server locally for development purposes (#334, @ncdc)
  * Add examples to `ark schedule create` documentation (#331, @lypht)
  * GCP: Remove `project` requirement from Ark config (#345, @skriss)
  * Add `--from-backup` flag to `ark restore create` and allow custom restore names (#342 #409, @skriss)
  * Azure: remove `location` requirement from Ark config (#344, @skriss)
  * Add documentation/examples for storing backups in IBM Cloud Object Storage (#321, @roytman)
  * Reduce verbosity of hooks logging (#362, @skriss)
  * AWS: Add minimal IAM policy to documentation (#363 #419, @hopkinsth)
  * Don't restore events (#374, @sanketjpatel)
  * Azure: reduce API polling interval from 60s to 5s (#359, @skriss)
  * Switch from hostPath to emptyDir volume type for minio example (#386, @containscafeine)
  * Add limit ranges as a prioritized resource for restores (#392, @containscafeine)
  * AWS: Add documentation on using Ark with kube2iam (#402, @domderen)
  * Azure: add node selector so Ark pod is scheduled on a linux node (#415, @ffd2subroutine)
  * Error from the Ark CLI if attempting to get logs for a non-existent restore (#391, @containscafeine)
  * GCP: Add minimal IAM policy to documentation (#429, @skriss @jody-frankowski)

### Upgrading from v0.7.1:
  Ark v0.7.1 moved the Ark server deployment into a separate namespace, `heptio-ark-server`. As of v0.8.0 we've
  returned to a single namespace, `heptio-ark`, for all Ark-related resources. If you're currently running v0.7.1,
  here are the steps you can take to upgrade:

1. Execute the steps from the **Credentials and configuration** section for your cloud:
    * [AWS](https://heptio.github.io/velero/v0.8.0/aws-config#credentials-and-configuration)
    * [Azure](https://heptio.github.io/velero/v0.8.0/azure-config#credentials-and-configuration)
    * [GCP](https://heptio.github.io/velero/v0.8.0/gcp-config#credentials-and-configuration)

    When you get to the secret creation step, if you don't have your `credentials-ark` file handy, 
    you can copy the existing secret from your `heptio-ark-server` namespace into the `heptio-ark` namespace:
    ```bash
    kubectl get secret/cloud-credentials -n heptio-ark-server --export -o json | \
      jq '.metadata.namespace="heptio-ark"' | \
      kubectl apply -f -
    ```

2. You can now safely delete the `heptio-ark-server` namespace:
    ```bash
    kubectl delete namespace heptio-ark-server
    ```

3. Execute the commands from the **Start the server** section for your cloud:
    * [AWS](https://heptio.github.io/velero/v0.8.0/aws-config#start-the-server)
    * [Azure](https://heptio.github.io/velero/v0.8.0/azure-config#start-the-server)
    * [GCP](https://heptio.github.io/velero/v0.8.0/gcp-config#start-the-server)
