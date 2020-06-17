# FAQ

## When is it appropriate to use Ark instead of etcd's built in backup/restore?

Etcd's backup/restore tooling is good for recovering from data loss in a single etcd cluster. For
example, it is a good idea to take a backup of etcd prior to upgrading etcd istelf. For more
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
in [pod_action.go](https://github.com/heptio/ark/blob/master/pkg/restore/pod_action.go)
