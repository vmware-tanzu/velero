---
title: Velero v1.1 backing up and restoring Stateful apps on vSphere
slug: Velero-v1-1-Stateful-Backup-vSphere
image: /img/posts/cassandra.gif
excerpt: This post demonstrates how Velero can be used on Kubernetes running on vSphere to backup a Stateful application. For the purposes of this example, we will backup and restore a Cassandra NoSQL database management system.
author_name: Cormac Hogan
author_avatar: /img/contributors/cormac-pic.png
categories: ['kubernetes']
# Tag should match author to drive author pages
tags: ['Velero', 'Cormac Hogan', 'how-to']
---
Velero version 1.1 provides support to backup applications orchestrated on upstream Kubernetes running natively on vSphere. This post will provide detailed information on how to use Velero v1.1 to backup and restore a stateful application (`Cassandra`) that is running in a Kubernetes cluster deployed on vSphere. At this time there is no vSphere plugin for snapshotting stateful applications during a Velero backup. In this case, we rely on a third party program called `restic` to copy the data contents from Persistent Volumes. The data is stored in the same S3 object store where the Kubernetes object metadata is stored.

## Overview of steps

* Download and deploy Cassandra
* Create and populate a database and table in Cassandra
* Prepare Cassandra for a Velero backup by adding appropriate annotations
* Use Velero to take a backup
* Destroy the Cassandra deployment
* Use Velero to restore the Cassandra application
* Verify that the Cassandra database and table of contents have been restored

## What this post does not show

* This tutorial does not show how to deploy Velero v1.1 on vSphere. This is available in other tutorials.
* For this backup to be successful, Velero needs to be installed with the `use-restic` flag. [More details on using Restic for stateful backups can be found in the docs here](https://velero.io/docs/v1.1.0/restic/#Setup)
* The assumption is that the Kubernetes nodes in your cluster have internet access in order to pull the necessary Velero images. This guide does not show how to pull images using a local repository.

## Download and Deploy Cassandra

For instructions on how to download and deploy a simple Cassandra StatefulSet, please refer to [this blog post](https://cormachogan.com/2019/06/12/kubernetes-storage-on-vsphere-101-statefulset/). This will show you how to deploy a Cassandra StatefulSet which we can use to do our Stateful application backup and restore. The manifests [available here](https://github.com/cormachogan/vsphere-storage-101/tree/master/StatefulSets) use an earlier version of Cassandra (v11) that includes the `cqlsh` tool which we will use now to create a database and populate a table with some sample data.

If you follow the instructions above on how to deploy Cassandra on Kubernetes, you should see a similar response if you run the following command against your deployment:

```bash
 kubectl exec -it cassandra-0 -n cassandra -- nodetool status
Datacenter: DC1-K8Demo
======================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens       Owns (effective)  Host ID                               Rack
UN  10.244.1.18  162.95 KiB  32           66.9%             2fc03eff-27ee-4934-b483-046e096ba116  Rack1-K8Demo
UN  10.244.1.19  174.32 KiB  32           61.4%             83867fd7-bb6f-45dd-b5ea-cdf5dcec9bad  Rack1-K8Demo
UN  10.244.2.14  161.04 KiB  32           71.7%             8d88d0ec-2981-4c8b-a295-b36eee62693c  Rack1-K8Demo
```

Now we will populate Cassandra with some data. Here we are connecting to the first Pod, cassandra-0 and running the `cqlsh` command which will allow us to create a Keyspace and a table.

```bash
$ kubectl exec -it cassandra-0 -n cassandra -- cqlsh
Connected to K8Demo at 127.0.0.1:9042.
[cqlsh 5.0.1 | Cassandra 3.9 | CQL spec 3.4.2 | Native protocol v4]
Use HELP for help.

cqlsh> CREATE KEYSPACE demodb WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor' : 3 };

cqlsh> use demodb;

cqlsh:demodb> CREATE TABLE emp(emp_id int PRIMARY KEY, emp_name text, emp_city text, emp_sal varint,emp_phone varint);

cqlsh:demodb> INSERT INTO emp (emp_id, emp_name, emp_city, emp_phone, emp_sal) VALUES (100, 'Cormac', 'Cork', 999, 1000000);

cqlsh:demodb> select * from emp;

 emp_id | emp_city | emp_name | emp_phone | emp_sal
--------+----------+----------+-----------+---------
    100 |     Cork |   Cormac |       999 | 1000000

(1 rows)
cqlsh:demodb> exit
```

Now that we have populated the application with some data, let's annotate each of the Pods, back it up, destroy the application and then try to restore it using Velero v1.1.

## Prepare Cassandra for a Velero stateful backup by adding Annotations

The first step is to add annotations to each of the Pods in the StatefulSet to indicate that the contents of the persistent volumes, mounted on cassandra-data, needs to be backed up as well. As mentioned previously, Velero uses the `restic` program at this time for capturing state/data from Kubernetes running on vSphere.

```bash
$ kubectl -n cassandra describe pod/cassandra-0 | grep Annotations
Annotations:        <none>
```

```bash
$ kubectl -n cassandra annotate pod/cassandra-0 backup.velero.io/backup-volumes=cassandra-data
pod/cassandra-0 annotated
```

```bash
$ kubectl -n cassandra describe pod/cassandra-0 | grep Annotations
Annotations:        backup.velero.io/backup-volumes: cassandra-data
```

Repeat this action for the other Pods, in this example, Pods cassandra-1 and cassandra-2. This is an indication that we need to backup the persistent volume contents associated with each Pod.

## Take a backup

```bash
$ velero backup create cassandra --include-namespaces cassandra
Backup request "cassandra" submitted successfully.
Run `velero backup describe cassandra` or `velero backup logs cassandra` for more details.
```

```bash
$ velero backup describe cassandra
Name:         cassandra
Namespace:    velero
Labels:       velero.io/storage-location=default
Annotations:  <none>

Phase:  InProgress

Namespaces:
  Included:  cassandra
  Excluded:  <none>

Resources:
  Included:        *
  Excluded:        <none>
  Cluster-scoped:  auto

Label selector:  <none>

Storage Location:  default

Snapshot PVs:  auto

TTL:  720h0m0s

Hooks:  <none>

Backup Format Version:  1

Started:    2019-09-02 15:37:19 +0100 IST
Completed:  <n/a>

Expiration:  2019-10-02 15:37:19 +0100 IST

Persistent Volumes: <none included>

Restic Backups (specify --details for more information):
  In Progress:  1
```

```bash
$ velero backup describe cassandra
Name:         cassandra
Namespace:    velero
Labels:       velero.io/storage-location=default
Annotations:  <none>

Phase:  Completed

Namespaces:
  Included:  cassandra
  Excluded:  <none>

Resources:
  Included:        *
  Excluded:        <none>
  Cluster-scoped:  auto

Label selector:  <none>

Storage Location:  default

Snapshot PVs:  auto

TTL:  720h0m0s

Hooks:  <none>

Backup Format Version:  1

Started:    2019-09-02 15:37:19 +0100 IST
Completed:  2019-09-02 15:37:34 +0100 IST

Expiration:  2019-10-02 15:37:19 +0100 IST

Persistent Volumes: <none included>

Restic Backups (specify --details for more information):
  Completed:  3
```

If we include the option `--details` to the previous command, we can see the various objects that were backed up.

```bash
$ velero backup describe cassandra --details
Name:         cassandra
Namespace:    velero
Labels:       velero.io/storage-location=default
Annotations:  <none>

Phase:  Completed

Namespaces:
  Included:  cassandra
  Excluded:  <none>

Resources:
  Included:        *
  Excluded:        <none>
  Cluster-scoped:  auto

Label selector:  <none>

Storage Location:  default

Snapshot PVs:  auto

TTL:  720h0m0s

Hooks:  <none>

Backup Format Version:  1

Started:    2019-09-02 15:37:19 +0100 IST
Completed:  2019-09-02 15:37:34 +0100 IST

Expiration:  2019-10-02 15:37:19 +0100 IST

Resource List:
  apps/v1/ControllerRevision:
    - cassandra/cassandra-55b978b564
  apps/v1/StatefulSet:
    - cassandra/cassandra
  v1/Endpoints:
    - cassandra/cassandra
  v1/Namespace:
    - cassandra
  v1/PersistentVolume:
    - pvc-2b574305-ca52-11e9-80e4-005056a239d9
    - pvc-51a681ad-ca52-11e9-80e4-005056a239d9
    - pvc-843241b7-ca52-11e9-80e4-005056a239d9
  v1/PersistentVolumeClaim:
    - cassandra/cassandra-data-cassandra-0
    - cassandra/cassandra-data-cassandra-1
    - cassandra/cassandra-data-cassandra-2
  v1/Pod:
    - cassandra/cassandra-0
    - cassandra/cassandra-1
    - cassandra/cassandra-2
  v1/Secret:
    - cassandra/default-token-bzh56
  v1/Service:
    - cassandra/cassandra
  v1/ServiceAccount:
    - cassandra/default

Persistent Volumes: <none included>

Restic Backups:
  Completed:
    cassandra/cassandra-0: cassandra-data
    cassandra/cassandra-1: cassandra-data
    cassandra/cassandra-2: cassandra-data
```

The command `velero backup logs` can be used to get additional information about the backup progress.

## Destroy the Cassandra deployment

Now that we have successfully taken a backup, which includes the `Restic` backups of the data, we will now go ahead and destroy the Cassandra namespace, and restore it once again.

```bash
$ kubectl delete ns cassandra
namespace "cassandra" deleted

$ kubectl get pv
No resources found.

$ kubectl get pods -n cassandra
No resources found.

$ kubestl get pvc -n cassandra
No resources found.
```

## Restore the Cassandra application via Velero

Now use Velero to restore the application and contents. The name of the backup must be specified at the command line using the `--from-backup` option. You can get the backup name from the following command:

```bash
$ velero backup get
NAME                 STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
cassandra1            Completed   2019-10-02 15:37:34 +0100 IST   31d       default            <none>
```

Next, initiate the restore:

```bash
$ velero restore create cassandra1 --from-backup cassandra1
Restore request "cassandra1" submitted successfully.
Run `velero restore describe cassandra1` or `velero restore logs cassandra1` for more details.
```

```bash
$ velero restore describe cassandra1
Name:         cassandra1
Namespace:    velero
Labels:       <none>
Annotations:  <none>

Phase:  InProgress

Backup:  cassandra1

Namespaces:
  Included:  *
  Excluded:  <none>

Resources:
  Included:        *
  Excluded:        nodes, events, events.events.k8s.io, backups.velero.io, restores.velero.io, resticrepositories.velero.io
  Cluster-scoped:  auto

Namespace mappings:  <none>

Label selector:  <none>

Restore PVs:  auto

Restic Restores (specify --details for more information):
  New:  3
```

Let's get some further information by adding the `--details` option.

```bash
$ velero restore describe cassandra1 --details
Name:         cassandra1
Namespace:    velero
Labels:       <none>
Annotations:  <none>

Phase:  InProgress

Backup:  cassandra1

Namespaces:
  Included:  *
  Excluded:  <none>

Resources:
  Included:        *
  Excluded:        nodes, events, events.events.k8s.io, backups.velero.io, restores.velero.io, resticrepositories.velero.io
  Cluster-scoped:  auto

Namespace mappings:  <none>

Label selector:  <none>

Restore PVs:  auto

Restic Restores:
  New:
    cassandra/cassandra-0: cassandra-data
    cassandra/cassandra-1: cassandra-data
    cassandra/cassandra-2: cassandra-data
```

When the restore completes, the `Phase` and `Restic Restores` should change to `Completed` as shown below.

```bash
$ velero restore describe cassandra1 --details
Name:         cassandra1
Namespace:    velero
Labels:       <none>
Annotations:  <none>

Phase:  Completed

Backup:  cassandra1

Namespaces:
  Included:  *
  Excluded:  <none>

Resources:
  Included:        *
  Excluded:        nodes, events, events.events.k8s.io, backups.velero.io, restores.velero.io, resticrepositories.velero.io
  Cluster-scoped:  auto

Namespace mappings:  <none>

Label selector:  <none>

Restore PVs:  auto

Restic Restores:
  Completed:
    cassandra/cassandra-0: cassandra-data
    cassandra/cassandra-1: cassandra-data
    cassandra/cassandra-2: cassandra-data
```

The `velero restore logs` command can also be used to track restore progress.

## Validate the restored application

Use some commands seen earlier to validate that not only is the application restored, but also the data.

```bash
$ kubectl get ns
NAME                  STATUS   AGE
cassandra             Active   2m35s
default               Active   13d
kube-node-lease       Active   13d
kube-public           Active   13d
kube-system           Active   13d
velero                Active   35m
wavefront-collector   Active   7d5h
```

```bash
$ kubectl get pv
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                  STORAGECLASS   REASON   AGE
pvc-51ae99a9-cd91-11e9-80e4-005056a239d9   1Gi        RWO            Delete           Bound    cassandra/cassandra-data-cassandra-0   cass-sc-csi             2m28s
pvc-51b15558-cd91-11e9-80e4-005056a239d9   1Gi        RWO            Delete           Bound    cassandra/cassandra-data-cassandra-1   cass-sc-csi             2m22s
pvc-51b4079c-cd91-11e9-80e4-005056a239d9   1Gi        RWO            Delete           Bound    cassandra/cassandra-data-cassandra-2   cass-sc-csi             2m27s
```

```bash
$ kubectl get pvc -n cassandra
NAME                         STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
cassandra-data-cassandra-0   Bound    pvc-51ae99a9-cd91-11e9-80e4-005056a239d9   1Gi        RWO            cass-sc-csi    2m49s
cassandra-data-cassandra-1   Bound    pvc-51b15558-cd91-11e9-80e4-005056a239d9   1Gi        RWO            cass-sc-csi    2m49s
cassandra-data-cassandra-2   Bound    pvc-51b4079c-cd91-11e9-80e4-005056a239d9   1Gi        RWO            cass-sc-csi    2m49s
```

```bash
$ kubectl exec -it cassandra-0 -n cassandra -- nodetool status
Datacenter: DC1-K8Demo
======================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens       Owns (effective)  Host ID                               Rack
UN  10.244.1.21  138.53 KiB  32           66.9%             2fc03eff-27ee-4934-b483-046e096ba116  Rack1-K8Demo
UN  10.244.1.22  166.45 KiB  32           71.7%             8d88d0ec-2981-4c8b-a295-b36eee62693c  Rack1-K8Demo
UN  10.244.2.23  160.43 KiB  32           61.4%             83867fd7-bb6f-45dd-b5ea-cdf5dcec9bad  Rack1-K8Demo
```

```bash
$ kubectl exec -it cassandra-0 -n cassandra -- cqlsh
Connected to K8Demo at 127.0.0.1:9042.
[cqlsh 5.0.1 | Cassandra 3.9 | CQL spec 3.4.2 | Native protocol v4]
Use HELP for help.
cqlsh> use demodb;

cqlsh:demodb> select * from emp;

 emp_id | emp_city | emp_name | emp_phone | emp_sal
--------+----------+----------+-----------+---------
    100 |     Cork |   Cormac |       999 | 1000000

(1 rows)
cqlsh:demodb>
```

It looks like the restore has been successful. Velero v1.1 has successfully restored the Kubenetes objects for the Cassandra application, as well as restored the database and table contents.

## Feedback and Participation

As always, we welcome feedback and participation in the development of Velero. [All information on how to contact us or become active can be found here](https://velero.io/community/)

You can find us on [Kubernetes Slack in the #velero channel](https://kubernetes.slack.com/messages/C6VCGP4MT), and follow us on Twitter at [@projectvelero](https://twitter.com/projectvelero).
