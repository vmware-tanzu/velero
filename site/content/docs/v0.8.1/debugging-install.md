---
title: "Debugging Installation Issues"
layout: docs
---

## General

### `invalid configuration: no configuration has been provided`
This typically means that no `kubeconfig` file can be found for the Ark client to use. Ark looks for a kubeconfig in the 
following locations:
* the path specified by the `--kubeconfig` flag, if any
* the path specified by the `$KUBECONFIG` environment variable, if any
* `~/.kube/config`

### Backups or restores stuck in `New` phase
This means that the Ark controllers are not processing the backups/restores, which usually happens because the Ark server is not running. Check the pod description and logs for errors:
```
kubectl -n heptio-ark describe pods
kubectl -n heptio-ark logs deployment/ark
```


## AWS

### `NoCredentialProviders: no valid providers in chain`
This means that the secret containing the AWS IAM user credentials for Ark has not been created/mounted properly 
into the Ark server pod. Ensure the following:
* The `cloud-credentials` secret exists in the Ark server's namespace
* The `cloud-credentials` secret has a single key, `cloud`, whose value is the contents of the `credentials-ark` file
* The `credentials-ark` file is formatted properly and has the correct values:
    
    ```
    [default]
    aws_access_key_id=<your AWS access key ID>
    aws_secret_access_key=<your AWS secret access key>
    ```
* The `cloud-credentials` secret is defined as a volume for the Ark deployment
* The `cloud-credentials` secret is being mounted into the Ark server pod at `/credentials`


## Azure

### `Failed to refresh the Token` or `adal: Refresh request failed`
This means that the secrets containing the Azure service principal credentials for Ark has not been created/mounted 
properly into the Ark server pod. Ensure the following:
* The `cloud-credentials` secret exists in the Ark server's namespace
* The `cloud-credentials` secret has seven keys and each one has the correct value (see [setup instructions][0])
* The `cloud-credentials` secret is defined as a volume for the Ark deployment
* The `cloud-credentials` secret is being mounted into the Ark server pod at `/credentials`


## GCE/GKE

### `open credentials/cloud: no such file or directory`
This means that the secret containing the GCE service account credentials for Ark has not been created/mounted properly 
into the Ark server pod. Ensure the following:
* The `cloud-credentials` secret exists in the Ark server's namespace
* The `cloud-credentials` secret has a single key, `cloud`, whose value is the contents of the `credentials-ark` file
* The `cloud-credentials` secret is defined as a volume for the Ark deployment
* The `cloud-credentials` secret is being mounted into the Ark server pod at `/credentials`

[0]: azure-config#credentials-and-configuration
