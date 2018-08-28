# Ark Config definition and Ark server deployment

* [Config][8]
  * [Example][9]
  * [Parameter Reference][6]
    * [Main config][7]
    * [AWS][0]
    * [GCP][1]
    * [Azure][2]
* [Deployment][11]
  * [Sample Deployment][13]
  * [Parameter Options][14]

## Config

Heptio Ark defines its own Config object (a custom resource) for specifying Ark backup and cloud provider settings. When the Ark server is first deployed, it waits until you create a Config --specifically one named `default`-- in the `heptio-ark` namespace.

> *NOTE*: There is an underlying assumption that you're running the Ark server as a Kubernetes deployment. If the `default` Config is modified, the server shuts down gracefully. Once the kubelet restarts the Ark server pod, the server then uses the updated Config values.

### Example

A sample YAML `Config` looks like the following:

```yaml
apiVersion: ark.heptio.com/v1
kind: Config
metadata:
  namespace: heptio-ark
  name: default
persistentVolumeProvider:
  name: aws
  config:
    region: us-west-2
```

### Parameter Reference

The configurable parameters are as follows:

#### Main config parameters

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `persistentVolumeProvider` | CloudProviderConfig | None (Optional) | The specification for whichever cloud provider the cluster is using for persistent volumes (to be snapshotted), if any.<br><br>If not specified, Backups and Restores requesting PV snapshots & restores, respectively, are considered invalid. <br><br> *NOTE*: For Azure, your Kubernetes cluster needs to be version 1.7.2+ in order to support PV snapshotting of its managed disks. |
| `persistentVolumeProvider/name` | String<br><br>(Ark natively supports `aws`, `gcp`, and `azure`. Other providers may be available via external plugins.) | None (Optional) | The name of the cloud provider the cluster is using for persistent volumes, if any. |
| `persistentVolumeProvider/config` | map[string]string<br><br>(See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs or your provider's documentation.) | None (Optional) | Configuration keys/values to be passed to the cloud provider for persistent volumes.  |

#### AWS

##### persistentVolumeProvider/config (AWS Only)

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `region` | string | Required Field | *Example*: "us-east-1"<br><br>See [AWS documentation][3] for the full list. |

#### Azure

##### persistentVolumeProvider/config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `apiTimeout` | metav1.Duration | 2m0s | How long to wait for an Azure API request to complete before timeout. |

#### GCP

##### persistentVolumeProvider/config

No parameters required.


## Deployment

Heptio Ark also defines its own Deployment object for starting the Ark server on Kubernetes. When the Ark server is deployed, there are specific configurations that might be changed.

### Sample Deployment

A sample YAML `Deployment` looks like the following:

```yaml
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  namespace: heptio-ark
  name: ark
spec:
  replicas: 1
  template:
    metadata:
      labels:
        component: ark
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8085"
        prometheus.io/path: "/metrics"
    spec:
      restartPolicy: Always
      serviceAccountName: ark
      containers:
        - name: ark
          image: gcr.io/heptio-images/ark:latest
          command:
            - /ark
          args:
            - server
            - --backup-sync-period
            - 30m
          volumeMounts:
            - name: cloud-credentials
              mountPath: /credentials
            - name: plugins
              mountPath: /plugins
            - name: scratch
              mountPath: /scratch
          env:
            - name: AWS_SHARED_CREDENTIALS_FILE
              value: /credentials/cloud
            - name: ARK_SCRATCH_DIR
              value: /scratch
      volumes:
        - name: cloud-credentials
          secret:
            secretName: cloud-credentials
        - name: plugins
          emptyDir: {}
        - name: scratch
          emptyDir: {}
```

### Parameter Options

The list of configurable options for the `ark server` deployment can be found on the [CLI reference][12] document.



[0]: #aws
[1]: #gcp
[2]: #azure
[3]: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
[6]: #parameter-reference
[7]: #main-config-parameters
[8]: #config
[9]: #example
[10]: http://docs.aws.amazon.com/kms/latest/developerguide/overview.html
[11]: #deployment
[12]: cli-reference/ark_server.md
[13]: #sample-deployment
[14]: #parameter-options
