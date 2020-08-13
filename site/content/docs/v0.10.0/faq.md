---
title: "FAQ"
layout: docs
---

## When is it appropriate to use Ark instead of etcd's built in backup/restore?

Etcd's backup/restore tooling is good for recovering from data loss in a single etcd cluster. For
example, it is a good idea to take a backup of etcd prior to upgrading etcd itself. For more
sophisticated management of your Kubernetes cluster backups and restores, we feel that Ark is
generally a better approach. It gives you the ability to throw away an unstable cluster and restore
your Kubernetes resources and data into a new cluster, which you can't do easily just by backing up
and restoring etcd.

Examples of cases where Ark is useful:

* you don't have access to etcd (e.g. you're running on GKE)
* backing up both Kubernetes resources and persistent volume state
* cluster migrations
* backing up a subset of your Kubernetes resources
* backing up Kubernetes resources that are stored across multiple etcd clusters (for example if you
  run a custom apiserver)

## Will Ark restore my Kubernetes resources exactly the way they were before?

Yes, with some exceptions. For example, when Ark restores pods it deletes the `nodeName` from the
pod so that it can be scheduled onto a new node. You can see some more examples of the differences
in [pod_action.go](https://github.com/heptio/ark/blob/main/pkg/restore/pod_action.go)

## I'm using Ark in multiple clusters. Should I use the same bucket to store all of my backups?

We **strongly** recommend that you use a separate bucket per cluster to store backups. Sharing a bucket
across multiple Ark instances can lead to numerous problems - failed backups, overwritten backups,
inadvertently deleted backups, etc., all of which can be avoided by using a separate bucket per Ark
instance.

Related to this, if you need to restore a backup from cluster A into cluster B, please use restore-only
mode in cluster B's Ark instance (via the `--restore-only` flag on the `ark server` command specified
in your Ark deployment) while it's configured to use cluster A's bucket. This will ensure no 
new backups are created, and no existing backups are deleted or overwritten.
