# Run Velero on AlibabaCloud

To set up Velero on AlibabaCloud, you:

* Download an official release of Velero
* Create your OSS bucket
* Create an RAM user for Velero
* Install the server

## Download Velero

1. Download the [latest official release's](https://github.com/heptio/velero/releases) tarball for your client platform.

    _We strongly recommend that you use an [official release](https://github.com/heptio/velero/releases) of
Velero. The tarballs for each release contain the `velero` command-line client. The code in the master branch
of the Velero repository is under active development and is not guaranteed to be stable!_

1. Extract the tarball:
    ```bash
    tar -xvf <RELEASE-TARBALL-NAME>.tar.gz -C /dir/to/extract/to 
    ```
    We'll refer to the directory you extracted to as the "Velero directory" in subsequent steps.

1. Move the `velero` binary from the Velero directory to somewhere in your PATH.

## Create OSS bucket

Velero requires an object storage bucket to store backups in, preferrably unique to a single Kubernetes cluster (see the [FAQ][20] for more details). Create an OSS bucket, replacing placeholders appropriately:

```bash
BUCKET=<YOUR_BUCKET>
REGION=<YOUR_REGION>
ossutil mb oss://$BUCKET \
        --storage-class Standard \
        --acl=private
```

For more information about ossutil, see [the AlibabaCloud documentation on ossutil][23].

## Create RAM user

For more information, see [the AlibabaCloud documentation on RAM users guides][14].

1. Create the RAM user:

    Follow [the AlibabaCloud documentation on RAM users][22].
    
    > If you'll be using Velero to backup multiple clusters with multiple OSS buckets, it may be desirable to create a unique username per cluster rather than the default `velero`.

2. Attach policies to give `velero` the necessary permissions:

    ```bash
    {
        "Version": "1",
        "Statement": [
            {
                "Action": [
                    "ecs:DescribeSnapshots",
                    "ecs:CreateSnapshot",
                    "ecs:DeleteSnapshot",
                    "ecs:DescribeDisks",
                    "ecs:CreateDisk",
                    "ecs:Addtags",
                    "oss:PutObject",
                    "oss:GetObject",
                    "oss:DeleteObject",
                    "oss:GetBucket"
                ],
                "Resource": [
                    "*"
                ],
                "Effect": "Allow"
            }
        ]
    }
    ```

3. Create an access key for the user:

    Follow [the AlibabaCloud documentation on create AK][24].

4. Create a Velero-specific credentials file (`credentials-velero`) in your local directory:

    ```
    ALIBABA_CLOUD_ACCESS_KEY_ID=<ALIBABA_CLOUD_ACCESS_KEY_ID>
    ALIBABA_CLOUD_ACCESS_KEY_SECRET=<ALIBABA_CLOUD_ACCESS_KEY_SECRET>
    ALIBABA_CLOUD_OSS_ENDPOINT=<ALIBABA_CLOUD_OSS_ENDPOINT>
    ```

    where the access key id and secret are the values get from the step 3 and the oss endpoint is the value oss-$REGION.aliyuncs.com.


## Install and start Velero

Install Velero, including all prerequisites, into the cluster and start the deployment. This will create a namespace called `velero`, and place a deployment named `velero` in it.

```bash
velero install \
    --provider alibabacloud \
    --bucket $BUCKET \
    --secret-file ./credentials-velero \
    --backup-location-config region=$REGION
```

Additionally, you can specify `--use-restic` to enable restic support, and `--wait` to wait for the deployment to be ready.

(Optional) Specify [additional configurable parameters][21] for the `--backup-location-config` flag.

(Optional) Specify [additional configurable parameters][6] for the `--snapshot-location-config` flag.

For more complex installation needs, use either the Helm chart, or add `--dry-run -o yaml` options for generating the YAML representation for the installation.


## Installing the nginx example (optional)

If you want to run a nginx example with pv on alibabacloud, you need replace `<PV_NAME>` with ecs disk's id in with-pv.yaml below:

```bash
# Copyright 2017 the Velero contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

---
apiVersion: v1
kind: Namespace
metadata:
  name: nginx-example
  labels:
    app: nginx

---
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    pv.kubernetes.io/bound-by-controller: "yes"
  labels:
    alicloud-pvname: <PV_NAME>
  name: <PV_NAME>
  namespace: nginx-example
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 20Gi
  flexVolume:
    driver: alicloud/disk
    fsType: ext4
    options:
      volumeId: <PV_NAME>
  storageClassName: disk

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
  name: nginx-logs
  namespace: nginx-example
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
  selector:
    matchLabels:
      alicloud-pvname: <PV_NAME>
  storageClassName: disk
  volumeName: <PV_NAME>

---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: nginx-example
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        pre.hook.backup.velero.io/container: fsfreeze
        pre.hook.backup.velero.io/command: '["/sbin/fsfreeze", "--freeze", "/var/log/nginx"]'
        post.hook.backup.velero.io/container: fsfreeze
        post.hook.backup.velero.io/command: '["/sbin/fsfreeze", "--unfreeze", "/var/log/nginx"]'
    spec:
      volumes:
        - name: nginx-logs
          persistentVolumeClaim:
           claimName: nginx-logs
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
        volumeMounts:
          - mountPath: "/var/log/nginx"
            name: nginx-logs
            readOnly: false
      - image: gcr.io/heptio-images/fsfreeze-pause:latest
        name: fsfreeze
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: "/var/log/nginx"
            name: nginx-logs
            readOnly: false

---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: nginx
  name: my-nginx
  namespace: nginx-example
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer

```


[6]: api-types/volumesnapshotlocation.md#alibabacloud
[14]: https://www.alibabacloud.com/help/doc-detail/28645.htm
[20]: faq.md
[21]: api-types/backupstoragelocation.md#alibabacloud
[22]: https://www.alibabacloud.com/help/doc-detail/93720.htm
[23]: https://www.alibabacloud.com/help/doc-detail/50452.htm
[24]: https://www.alibabacloud.com/help/doc-detail/53045.htm
