---
title: "Troubleshooting"
layout: docs
---

These tips can help you troubleshoot known issues. If they don't help, you can [file an issue][4], or talk to us on the [#ark-dr channel][25] on the Kubernetes Slack server. 

In `ark` version >= `0.1.0`, you can use the `ark bug` command to open a [Github issue][4] by launching a browser window with some prepopulated values. Values included are OS, CPU architecture, `kubectl` client and server versions (if available) and the `ark` client version. This information isn't submitted to Github until you click the `Submit new issue` button in the Github UI, so feel free to add, remove or update whatever information you like.

Some general commands for troubleshooting that may be helpful:

* `ark backup describe <backupName>` - describe the details of a backup
* `ark backup logs <backupName>` - fetch the logs for this specific backup. Useful for viewing failures and warnings, including resources that could not be backed up.
* `ark restore describe <restoreName>` - describe the details of a restore
* `ark restore logs <restoreName>` - fetch the logs for this specific restore. Useful for viewing failures and warnings, including resources that could not be restored.
* `kubectl logs deployment/ark -n heptio-ark` - fetch the logs of the Ark server pod. This provides the output of the Ark server processes.

## Getting ark debug logs

You can increase the verbosity of the Ark server by editing your Ark deployment to look like this:


```
kubectl edit deployment/ark -n heptio-ark
...
   containers:
     - name: ark
       image: gcr.io/heptio-images/ark:latest
       command:
         - /ark
       args:
         - server
         - --log-level # Add this line
         - debug       # Add this line
...
```


## [Debug installation/setup issues][2]

## [Debug restores][1]

## Miscellaneous issues

### Ark reports `custom resource not found` errors when starting up.

Ark's server will not start if the required Custom Resource Definitions are not found in Kubernetes. Apply
the `examples/common/00-prereqs.yaml` file to create these definitions, then restart Ark.

### `ark backup logs` returns a `SignatureDoesNotMatch` error

Downloading artifacts from object storage utilizes temporary, signed URLs. In the case of S3-compatible
providers, such as Ceph, there may be differences between their implementation and the official S3
API that cause errors.

Here are some things to verify if you receive `SignatureDoesNotMatch` errors:

  * Make sure your S3-compatible layer is using [signature version 4][5] (such as Ceph RADOS v12.2.7)
  * For Ceph, try using a native Ceph account for credentials instead of external providers such as OpenStack Keystone


[1]: debugging-restores.md
[2]: debugging-install.md
[4]: https://github.com/heptio/ark/issues
[5]: https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-authenticating-requests.html
[25]: https://kubernetes.slack.com/messages/ark-dr
