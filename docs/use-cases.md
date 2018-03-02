# Why Ark?

Kubernetes workloads are ephemeral. You describe your cluster's state, and Kubernetes makes it so. So why do you still need to worry about backups and restores? Ark addresses these use cases:

* Disaster recovery for your persistent volumes

* If you don't start with your Kubernetes manifests in source control, you can run Ark to download the entire cluster and then put the manifests in source control.

* Copy your environment by cloning a cluster, or a portion of a cluster, to another cluster or to the same cluster. You can use the copy for scale testing, reproducing bugs, and more.

  * If you specify CPU and memory requests and limits for a pod, you can test these settings with a clone.

* Support for backing up any directory in any container's file system is planned for a future release. This feature will let you migrate your data across cloud providers more easily.

See also [the example scenarios][0].

[0]: scenarios.md