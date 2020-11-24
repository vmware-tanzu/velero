# End-to-end tests

Document for running Velero end-to-end test suite.

## Command line flags for E2E tests

Command line flags can be set after
```
velerocli - the velero CLI to use
kibishiins - the namespace to install kibishii in
cloudplatform - the cloud platform the tests will be run against (aws, vsphere, azure)
```

## Running tests locally

1. From Velero repository root

    ```
    make test-e2e
    ```

1. From `test/e2e/` directory

    ```
    make run
    ```

## Running tests based on cloud platforms

1. Running Velero E2E tests on KinD

    ```
    CLOUD_PLATFORM=kind make test-e2e
    ```

1. Running Velero E2E tests on AWS

    ```
    CLOUD_PLATFORM=aws make test-e2e
    ```

1. Running Velero E2E tests on Azure

    ```
    CLOUD_PLATFORM=azure make test-e2e
    ```
