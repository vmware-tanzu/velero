# End-to-end tests

Document for running Velero end-to-end test suite.

The E2E tests are validating end-to-end behavior of Velero including install, backup and restore operations. These tests take longer to complete and is not expected to be part of day-to-day developer workflow. It is for this reason that they are disabled when running unit tests. This is accomplished by running unit tests in [`short`](https://golang.org/pkg/testing/#Short) mode using the `-short` flag to `go test`.

If you previously ran unit tests using the `go test ./...` command or any of its variations, then you will now run the same command with the  `-short` flag to `go test` to accomplish the same behavior. Alternatively, you can use the `make test` command to run unit tests.

## Prerequisites

Running the E2E tests expects:
1. A running kubernetes cluster:
    1. With DNS and CNI installed.
    1. Compatible with Velero- running Kubernetes v1.10 or later.
    1. With necessary storage drivers/provisioners installed.
1. `kubectl` installed locally.

## Limitations

These are the current set of limitations with the E2E tests.

1. E2E tests only accepts credentials only for a single provider and for that reason, only tests for a single provider can be run at a time.
1. Each E2E test suite installs an instance of Velero to run tests and uninstalls it after test completion. It is possible that a test suite may be installing Velero while another may be uninstalling Velero. This race condition can lead to tests being flaky and cause false negatives. The options for resolving this are:
    1. Make each test suite setup wait for Velero to be uninstalled before attempting to install as part of its setup.
    1. Make each test suite install Velero in a different namespace.

## Configuration for E2E tests

Below is a list of the configuration used by E2E tests.
These configuration parameters are expected as values to the following command line flags:

1. `-credentials-file`: File containing credentials for backup and volume provider. Required.
1. `-bucket`: Name of the object storage bucket where backups from e2e tests should be stored. Required.
1. `-cloud-provider`: The cloud the tests will be run in.  Appropriate plug-ins will be installed except for kind which requires
the object-store-provider to be specified.
1. `-object-store-provider`: Object store provider to use. Required when kind is the cloud provider.
1. `-velerocli`: Path to the velero application to use. Optional, by default uses `velero` in the `$PATH`
1. `-velero-image`: Image for the velero server to be tested. Optional, by default uses `velero/velero:main`
1. `-bsl-config`: Configuration to use for the backup storage location. Format is key1=value1,key2=value2. Optional.
1. `-prefix`: Prefix in the `bucket`, under which all Velero data should be stored within the bucket. Optional.
1. `-vsl-config`: Configuration to use for the volume snapshot location. Format is key1=value1,key2=value2. Optional.
1. `-velero-namespace`: Namespace to install velero in.  Optional, defaults to "velero".
1. `-install-velero`: Specifies whether to install/uninstall velero for the tests.  Optional, defaults to "true".
1. `-additional-bsl-object-store-provider`: Provider of object store plugin for additional backup storage location. Required if testing multiple credentials support.
1. `-additional-bsl-bucket`: Name of the object storage bucket for additional backup storage location. Required if testing multiple credentials support.
1. `-additional-bsl-prefix`: Prefix in the `additional-bsl-bucket`, under which all Velero data should be stored. Optional.
1. `-additional-bsl-config`: Configuration to use for the additional backup storage location. Format is key1=value1,key2=value2. Optional.
1. `-additional-bsl-credentials-file`: File containing credentials for the additional backup storage location. Required if testing multiple credentials support.

These configurations or parameters are used to generate install options for Velero for each test suite.

Tests can be run with the Kubernetes cluster hosted in various cloud providers or in a _kind_ cluster with storage in
a specified object store type.  Currently supported cloud provider types are _aws_, _azure_, _vsphere_ and _kind_.
## Running tests locally

### Running using `make`

E2E tests can be run from the Velero repository root by running `make test-e2e`. While running E2E tests using `make` the E2E test configuration values are passed using `make` variables.

Below is a mapping between `make` variables to E2E configuration flags.
1. `CREDS_FILE`: `-credentials-file`. Required.
1. `BSL_BUCKET`: `-bucket`. Required.
1. `CLOUD_PROVIDER`: `-cloud-provider`. Required
1. `OBJECT_STORE_PROVIDER`: `-object-store-provider`. Required when kind is the cloud provider.
1. `VELERO_CLI`: the `-velerocli`. Optional.
1. `VELERO_IMAGE`: the `-velero-image`. Optional.
1. `BSL_PREFIX`: `-prefix`. Optional.
1. `BSL_CONFIG`: `-bsl-config`. Optional.
1. `VSL_CONFIG`: `-vsl-config`. Optional.
1. `ADDITIONAL_OBJECT_STORE_PROVIDER`: `-additional-bsl-object-store-provider`. Optional.
1. `ADDITIONAL_CREDS_FILE`: `-additional-bsl-bucket`. Optional.
1. `ADDITIONAL_BSL_BUCKET`: `-additional-bsl-prefix`. Optional.
1. `ADDITIONAL_BSL_PREFIX`: `-additional-bsl-config`. Optional.
1. `ADDITIONAL_BSL_CONFIG`: `-additional-bsl-credentials-file`. Optional.

For example, E2E tests can be run from Velero repository roots using the commands below:

1. Run Velero tests in a kind cluster with AWS (or Minio) as the storage provider:
    ```bash
    BSL_PREFIX=<PREFIX_UNDER_BUCKET> BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> CREDS_FILE=/path/to/aws-creds CLOUD_PROVIDER=kind OBJECT_STORE_PROVIDER=aws make test-e2e
    ```
1. Run Velero tests in an AWS cluster:
    ```bash
    BSL_PREFIX=<PREFIX_UNDER_BUCKET> BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> CREDS_FILE=/path/to/aws-creds CLOUD_PROVIDER=aws make test-e2e
    ```
1. Run Velero tests in a Microsoft Azure cluster:
    ```bash
    BSL_CONFIG="resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,storageAccount=$AZURE_STORAGE_ACCOUNT_ID,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID" BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> CREDS_FILE=/path/to/azure-creds CLOUD_PROVIDER=azure make test-e2e
    ```
    Please refer to `velero-plugin-for-microsoft-azure` documentation for instruction to [set up permissions for Velero](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#set-permissions-for-velero) and to [set up azure storage account and blob container](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#setup-azure-storage-account-and-blob-container)
1. Run Ginko-focused Restore Multi-API Groups tests using Minio as the backup storage location: 
   ```bash
   BSL_CONFIG="region=minio,s3ForcePathStyle=\"true\",s3Url=<ip address>:9000" BSL_PREFIX=<prefix> BSL_BUCKET=<bucket> CREDS_FILE=<absolute path to minio credentials file> CLOUD_PROVIDER=kind OBJECT_STORE_PROVIDER=aws VELERO_NAMESPACE="velero" GINKGO_FOCUS="API group versions" make test-e2e
   ```
1. Run Velero tests in a kind cluster with AWS (or Minio) as the storage provider and use Microsoft Azure as the storage provider for an additional Backup Storage Location:
    ```bash
    make test-e2e \
      CLOUD_PROVIDER=kind OBJECT_STORE_PROVIDER=aws BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> BSL_PREFIX=<PREFIX_UNDER_BUCKET> CREDS_FILE=/path/to/aws-creds \
      ADDITIONAL_OBJECT_STORE_PROVIDER=azure ADDITIONAL_BSL_BUCKET=<BUCKET_FOR_AZURE_BSL> ADDITIONAL_BSL_PREFIX=<PREFIX_UNDER_BUCKET> ADDITIONAL_BSL_CONFIG=<CONFIG_FOR_AZURE_BUCKET> ADDITIONAL_CREDS_FILE=/path/to/azure-creds
    ```
   Please refer to `velero-plugin-for-microsoft-azure` documentation for instruction to [set up permissions for Velero](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#set-permissions-for-velero) and to [set up azure storage account and blob container](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#setup-azure-storage-account-and-blob-container)

## Filtering tests

Velero E2E tests uses [Ginkgo](https://onsi.github.io/ginkgo/) testing framework which allows a subset of the tests to be run using the [`-focus` and `-skip`](https://onsi.github.io/ginkgo/#focused-specs) flags to ginkgo.

The `-focus` flag is passed to ginkgo using the `GINKGO_FOCUS` make variable. This can be used to focus on specific tests.

## Adding tests

### API clients
When adding a test, aim to instantiate an API client only once at the beginning of the test. There is a constructor `newTestClient` that facilitates the configuration and instantiation of clients. Also, please use the `kubebuilder` runtime controller client for any new test, as we will phase out usage of `client-go` API clients.

### Tips
Look for the ⛵ emoji printed at the end of each install and uninstall log. There should not be two install/unintall in a row, and there should be tests between an install and an uninstall. 