### 1. Object Store environment
Velero supports many options for object store and because of my necessity to test this on onpremise environment I would like to test this with Ceph and Rados Gateway.
Check all supported object store in this link .

Create a S3 user

```
[root@ceph-mon1 ~]# sudo radosgw-admin user create --subuser=velero:s3 --display-name="Velero Kubernetes Backup" --key-type=s3 --access=full
    "user_id": "velero",
    "display_name": "Velero Kubernetes Backup",
    "email": "",
    "suspended": 0,
    "max_buckets": 1000,
    "subusers": [
        {
            "id": "velero:s3",
            "permissions": "full-control"
        }
    ],
    "keys": [
        {
            "user": "velero:s3",
            "access_key": "AOTBA6CUYR4P2WD7Q7ZK",
            "secret_key": "d4ZY0cmAQcsmviwcpshE0bjWfyT5RDUROUE0BmJ6"
        },
        {
            "user": "velero:s3",
            "access_key": "RKF0CW7T2XA16BMJI8FW",
            "secret_key": "Z8BF56cAsuj5KFSSjMOD0At1nGfVmTjPx3sOFpWZ"
        }
    ],
    "swift_keys": [],
    "caps": [],
    "op_mask": "read, write, delete",
    "default_placement": "",
    "default_storage_class": "",
    "placement_tags": [],
    "bucket_quota": {
        "enabled": false,
        "check_on_raw": false,
        "max_size": -1,
        "max_size_kb": 0,
        "max_objects": -1
    },
    "user_quota": {
        "enabled": false,
        "check_on_raw": false,
        "max_size": -1,
        "max_size_kb": 0,
        "max_objects": -1
    },
    "temp_url_keys": [],
    "type": "rgw",
    "mfa_ids": []
}
```

Install s3cmd to create a bucket by CLI

```
[root@ceph-mon1 ~]# yum install s3cmd
```

Configure s3cmd to use my Rados GW endpoint

```
[root@ceph-mon1 ~]# s3cmd --configure
Enter new values or accept defaults in brackets with Enter.
Refer to user manual for detailed description of all options.
Access key and Secret key are your identifiers for Amazon S3. Leave them empty for using the env variables.
Access Key [AOTBA6CUYR4P2WD7Q7ZK]: 
Secret Key [d4ZY0cmAQcsmviwcpshE0bjWfyT5RDUROUE0BmJ6]: 
Default Region [US]: 
Use "s3.amazonaws.com" for S3 Endpoint and not modify it to the target Amazon S3.
S3 Endpoint [radosgw.local.lab:80]: 
Use "%(bucket)s.s3.amazonaws.com" to the target Amazon S3. "%(bucket)s" and "%(location)s" vars can be used
if the target S3 system supports dns based buckets.
DNS-style bucket+hostname:port template for accessing a bucket [%(bucket)s.radosgw.local.lab]: radosgw.local.lab:80
Encryption password is used to protect your files from reading
by unauthorized persons while in transfer to S3
Encryption password: 
Path to GPG program [/usr/bin/gpg]: 
When using secure HTTPS protocol all communication with Amazon S3
servers is protected from 3rd party eavesdropping. This method is
slower than plain HTTP, and can only be proxied with Python 2.7 or newer
Use HTTPS protocol [No]: 
On some networks all internet access must go through a HTTP proxy.
Try setting it here if you can't connect to S3 directly
HTTP Proxy server name: 
New settings:
  Access Key: AOTBA6CUYR4P2WD7Q7ZK
  Secret Key: d4ZY0cmAQcsmviwcpshE0bjWfyT5RDUROUE0BmJ6
  Default Region: US
  S3 Endpoint: radosgw.local.lab:80
  DNS-style bucket+hostname:port template for accessing a bucket: radosgw.local.lab:80
  Encryption password: 
  Path to GPG program: /usr/bin/gpg
  Use HTTPS protocol: False
  HTTP Proxy server name: 
  HTTP Proxy server port: 0
Test access with supplied credentials? [Y/n] y
Please wait, attempting to list all buckets...
Success. Your access key and secret key worked fine :-)
Now verifying that encryption works...
Not configured. Never mind.
Save settings? [y/N] y
Configuration saved to '/root/.s3cfg'
```

Create a bucket for Velero
```
[root@ceph-mon1 ~]# s3cmd mb s3://velero
Bucket 's3://velero/' created
```

2. Velero Install

CLI Download
```
$ wget https://github.com/vmware-tanzu/velero/releases/download/v1.2.0/velero-v1.2.0-linux-amd64.tar.gz
$ tar -xzf velero-v1.2.0-linux-amd64.tar.gz
$ sudo cp velero-v1.2.0-linux-amd64/velero /usr/local/sbin
```

Create a file with s3 credentials
```
$ vi credentials-store
[default]
aws_access_key_id = AOTBA6CUYR4P2WD7Q7ZK
aws_secret_access_key = d4ZY0cmAQcsmviwcpshE0bjWfyT5RDUROUE0BmJ6
```

Velero deployment with Rados Gateway
We are using the S3 credentials for access the bucket velero with aws S3 sdk plugin , so , change the s3Url for Rados Gateway like our example http://radosgw.local.lab
```
$ velero install  --provider aws --bucket velero \
--plugins velero/velero-plugin-for-aws:v1.0.0 \
--secret-file ./credentials-velero \
--use-volume-snapshots=false \
--backup-location-config \
region=minio,s3ForcePathStyle="true",s3Url=http://radosgw.local.lab
```

3. Velero Backup and Restore
Create a single Nginx deployment for the test
```
$ kubectl velero-v1.2.0-linux-amd64/examples/nginx-app/base.yaml
```
Create a backup from this Nginx app
```
$ velero backup create nginx-backup --selector app=nginx --snapshot-volumes=true
$ velero backup describe nginx-backup --details
```

List the job backup

```
$ velero get backup

NAME           STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
nginx-backup   Completed   2020-06-05 13:56:40 +0000 UTC   29d       default            app=nginx
Check the namespace and resources created for this example
# Get Namespaces
$ kubectl get namespaces
NAME              STATUS   AGE
default           Active   139m
kube-node-lease   Active   139m
kube-public       Active   139m
kube-system       Active   139m
metallb-system    Active   74m
nginx-example     Active   27m
velero            Active   132m

# Get Pods 
$ kubectl get pods -n nginx-example
NAME                                READY   STATUS    RESTARTS   AGE
nginx-deployment-7cd5ddccc7-ctvg8   1/1     Running   0          27m
nginx-deployment-7cd5ddccc7-kpfz5   1/1     Running   0          27m
```

Delete the namespace
```
$ kubectl delete namespace nginx-example
```


Restore backup for nginx-example

```
$ velero restore create --from-backup nginx-backup
```

Restore request "nginx-backup-20200605135949" submitted successfully.
Run `velero restore describe nginx-backup-20200605135949` or `velero restore logs nginx-backup-20200605135949` for more details.

Check the restore
```
$ kubectl get namespaces | grep nginx 
nginx-example     Active   3m44s

$ kubectl get pods -n nginx-example 
NAME                                READY   STATUS    RESTARTS   AGE
nginx-deployment-7cd5ddccc7-ctvg8   1/1     Running   0          3m52s
nginx-deployment-7cd5ddccc7-kpfz5   1/1     Running   0          3m51s
```

The content can be viewed when you check the bucket with s3cmd

```
$ s3cmd ls s3://velero
                          DIR  s3://velero/backups/
                          DIR  s3://velero/restores/
```

**Environment:**

- Velero version: 
[root@master1 ~]# velero version
Client:
	Version: v1.2.0
	Git commit: 5d008491bbf681658d3e372da1a9d3a21ca4c03c
Server:
	Version: v1.2.0
[root@master1 ~]# 

- Kubernetes version: (use `kubectl version`):

[root@master1 ~]# kubectl version
Client Version: version.Info{Major:"1", Minor:"17", GitVersion:"v1.17.4", GitCommit:"8d8aa39598534325ad77120c120a22b3a990b5ea", GitTreeState:"clean", BuildDate:"2020-03-12T21:03:42Z", GoVersion:"go1.13.8", Compiler:"gc", Platform:"linux/amd64"}
Server Version: version.Info{Major:"1", Minor:"17", GitVersion:"v1.17.4", GitCommit:"8d8aa39598534325ad77120c120a22b3a990b5ea", GitTreeState:"clean", BuildDate:"2020-03-12T20:55:23Z", GoVersion:"go1.13.8", Compiler:"gc", Platform:"linux/amd64"}
[root@master1 ~]# 


- Kubernetes installer & version: My deployment is based on Ansible kubespray deployment. 
- Cloud provider or hardware configuration: KVM libvirt 4.5 
- OS (e.g. from `/etc/os-release`): 
cat /etc/redhat-release 
CentOS Linux release 7.7.1908 (Core)
