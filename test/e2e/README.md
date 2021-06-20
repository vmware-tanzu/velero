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

## Configuration for E2E tests

Please look at the [e2e_suite_test.go](e2e_suite_test.go) for the set of flags needed to configure Velero. Those configurations or parameters are used to generate install options for Velero for each test suite.

Tests can be run with the Kubernetes cluster hosted in various cloud providers or in a _Kind_ cluster with storage in
a specified object store type.  Currently supported cloud provider types are _aws_, _azure_, _vsphere_ and _Kind_.
## Running tests locally using `make`

E2E tests can be run from the Velero repository root by running `make test-e2e`. While running E2E tests using `make` the E2E test configuration values are passed using `make` variables.

### Run tests with Kind

- Run Velero tests on a Kind cluster with AWS (or Minio) as the storage provider.

    ```bash
    BSL_PREFIX=<PREFIX_UNDER_BUCKET> BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> CREDS_FILE=/path/to/aws-creds CLOUD_PROVIDER=Kind OBJECT_STORE_PROVIDER=aws make test-e2e
    ```

### Run tests on a provider

Note: When running tests that take a snapshot on a provider, the optional paramenter `vsl-config` **must** be configued. This parameter is optional for tests where only objects (and not snapshots) are being backed up /restored. If you don't configure this parameter, be sure you are only running tests that don't need a snapshot. See the section [Filtering tests](#Filtering-tests) below for how to select specific tests.

The test will detect when it is configured with a provider other than Kind but without this configuration and ask for confirmation. To skip this check, add `FORCE=true` to pass it as a variable to the test command. To configure the snapshot settings, pass the `VSL_CONFIG` parameter with the proper values for your provider. Here's an example for AWS: `VSL_CONFIG=region=us-west-2`.


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
   BSL_CONFIG="region=minio,s3ForcePathStyle=\"true\",s3Url=<ip address>:9000" BSL_PREFIX=<prefix> BSL_BUCKET=<bucket> CREDS_FILE=<absolute path to minio credentials file> CLOUD_PROVIDER=Kind OBJECT_STORE_PROVIDER=aws GINKGO_FOCUS="API group versions" make test-e2e
   ```

1. Run Velero tests in a Kind cluster with AWS (or Minio) as the storage provider and use Microsoft Azure as the storage provider for an additional Backup Storage Location:

    ```bash
    make test-e2e \
      CLOUD_PROVIDER=Kind OBJECT_STORE_PROVIDER=aws BSL_BUCKET=<BUCKET_FOR_E2E_TEST_BACKUP> BSL_PREFIX=<PREFIX_UNDER_BUCKET> CREDS_FILE=/path/to/aws-creds \
      ADDITIONAL_OBJECT_STORE_PROVIDER=azure ADDITIONAL_BSL_BUCKET=<BUCKET_FOR_AZURE_BSL> ADDITIONAL_BSL_PREFIX=<PREFIX_UNDER_BUCKET> ADDITIONAL_BSL_CONFIG=<CONFIG_FOR_AZURE_BUCKET> ADDITIONAL_CREDS_FILE=/path/to/azure-creds
    ```

   Please refer to `velero-plugin-for-microsoft-azure` documentation for instruction to [set up permissions for Velero](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#set-permissions-for-velero) and to [set up azure storage account and blob container](https://github.com/vmware-tanzu/velero-plugin-for-microsoft-azure#setup-azure-storage-account-and-blob-container)

## Code under test

By default, the e2e tests will run against the `velero/velero:main` version of Velero. If you would like to test the code with any other image of Velero, pass the `<registry/image:version>` value to the `VELERO_IMAGE` paramenter to the `make test-e2e` command.

## Filtering tests

Velero E2E tests uses [Ginkgo](https://onsi.github.io/ginkgo/) testing framework which allows a subset of the tests to be run using the [`-focus` and `-skip`](https://onsi.github.io/ginkgo/#focused-specs) flags to ginkgo.

The `-focus` flag is passed to ginkgo using the `GINKGO_FOCUS` make variable. This can be used to focus on specific tests.

Example:  `GINKGO_FOCUS='APIGroup'`

## Adding tests

### API clients
When adding a test, aim to instantiate an API client only once at the beginning of the test. There is a constructor `newTestClient` that facilitates the configuration and instantiation of clients. Also, please use the `kubebuilder` runtime controller client for any new test, as we will phase out usage of `client-go` API clients.

### Tips
Look for the ⛵ emoji printed at the end of each install and uninstall log. There should not be two install/unintall in a row, and there should be tests between an install and an uninstall. 