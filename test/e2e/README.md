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
1. `-plugin-provider`: Provider of object store and volume snapshotter plugins. Required.
1. `-velerocli`: Path to the velero application to use. Optional, by default uses `velero` in the `$PATH`
1. `-velero-image`: Image for the velero server to be tested. Optional, by default uses `velero/velero:main`
1. `-bsl-config`: Configuration to use for the backup storage location. Format is key1=value1,key2=value2. Optional.
1. `-prefix`: Prefix in the `bucket`, under which all Velero data should be stored within the bucket. Optional.
1. `-vsl-config`: Configuration to use for the volume snapshot location. Format is key1=value1,key2=value2. Optional.

These configurations or parameters are used to generate install options for Velero for each test suite.

## Running tests locally

### Running using `make`

E2E tests can be run from the Velero repository root by running `make test-e2e`. While running E2E tests using `make` the E2E test configuration values are passed using `make` variables.

Below is a mapping between `make` variables to E2E configuration flags.
1. `CREDS_FILE`: `-credentials-file`. Required.
1. `BSL_BUCKET`: `-bucket`. Required.
1. `PLUGIN_PROVIDER`: `-plugin-provider`. Required.
1. `VELERO_CLI`: the `-velerocli`. Optional.
1. `VELERO_IMAGE`: the `-velero-image`. Optional.
1. `BSL_PREFIX`: `-prefix`. Optional.
1. `BSL_CONFIG`: `-bsl-config`. Optional.
1. `VSL_CONFIG`: `-vsl-config`. Optional.

For example, E2E tests can be run from Velero repository roots using the below command:

1. Run Velero tests using AWS as the storage provider:
    ```bash
    BSL_PREFIX=<PREFIX_UNDER_BUCKET> BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> CREDS_FILE=/path/to/aws-creds PLUGIN_PROVIDER=aws make test-e2e
    ```
1. Run Velero tests using Microsoft Azure as the storage provider:
    ```bash
    BSL_CONFIG="resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,storageAccount=$AZURE_STORAGE_ACCOUNT_ID,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID" BSL_BUCKET=velero CREDS_FILE=~/bin/velero-dev/aks-creds PLUGIN_PROVIDER=azure make test-e2e
    ```
    Please refer to `velero-plugin-for-microsoft-azure` documentation for instruction to [set up permissions for Velero](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#set-permissions-for-velero) and to [set up azure storage account and blob container](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#setup-azure-storage-account-and-blob-container)

## Filtering tests

Velero E2E tests uses [Ginkgo](https://onsi.github.io/ginkgo/) testing framework which allows a subset of the tests to be run using the [`-focus` and `-skip`](https://onsi.github.io/ginkgo/#focused-specs) flags to ginkgo.

The `-focus` flag is passed to ginkgo using the `GINKGO_FOCUS` make variable. This can be used to focus on specific tests.
