# Debugging Restores

* [Example][0]
* [Structure][1]
* [Restoring a load balancer Service][2]

## Example

When Heptio Ark finishes a Restore, its status changes to "Completed" regardless of whether there were issues during the process. The number of warnings and errors are indicated in the output columns from `ark restore get`:

```
NAME                          BACKUP          STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
backup-test-20170726180512    backup-test     Completed   155        76        2017-07-26 11:41:14 -0400 EDT   <none>
backup-test-20170726180513    backup-test     Completed   121        14        2017-07-26 11:48:24 -0400 EDT   <none>
backup-test-2-20170726180514  backup-test-2   Completed   0          0         2017-07-26 13:31:21 -0400 EDT   <none>
backup-test-2-20170726180515  backup-test-2   Completed   0          1         2017-07-26 13:32:59 -0400 EDT   <none>
```

To delve into the warnings and errors into more detail, you can use `ark restore describe`:
```
ark restore describe backup-test-20170726180512
```
The output looks like this:
```
Name:         backup-test-20170726180512
Namespace:    heptio-ark
Labels:       <none>
Annotations:  <none>

Backup:  backup-test

Namespaces:
  Included:  *
  Excluded:  <none>

Resources:
  Included:        serviceaccounts
  Excluded:        nodes
  Cluster-scoped:  auto

Namespace mappings:  <none>

Label selector:  <none>

Restore PVs:  auto

Phase:  Completed

Validation errors:  <none>

Warnings:
  Ark:        <none>
  Cluster:    <none>
  Namespaces:
    heptio-ark:   serviceaccounts "ark" already exists
                  serviceaccounts "default" already exists
    kube-public:  serviceaccounts "default" already exists
    kube-system:  serviceaccounts "attachdetach-controller" already exists
                  serviceaccounts "certificate-controller" already exists
                  serviceaccounts "cronjob-controller" already exists
                  serviceaccounts "daemon-set-controller" already exists
                  serviceaccounts "default" already exists
                  serviceaccounts "deployment-controller" already exists
                  serviceaccounts "disruption-controller" already exists
                  serviceaccounts "endpoint-controller" already exists
                  serviceaccounts "generic-garbage-collector" already exists
                  serviceaccounts "horizontal-pod-autoscaler" already exists
                  serviceaccounts "job-controller" already exists
                  serviceaccounts "kube-dns" already exists
                  serviceaccounts "namespace-controller" already exists
                  serviceaccounts "node-controller" already exists
                  serviceaccounts "persistent-volume-binder" already exists
                  serviceaccounts "pod-garbage-collector" already exists
                  serviceaccounts "replicaset-controller" already exists
                  serviceaccounts "replication-controller" already exists
                  serviceaccounts "resourcequota-controller" already exists
                  serviceaccounts "service-account-controller" already exists
                  serviceaccounts "service-controller" already exists
                  serviceaccounts "statefulset-controller" already exists
                  serviceaccounts "ttl-controller" already exists
    default:      serviceaccounts "default" already exists

Errors:
  Ark:        <none>
  Cluster:    <none>
  Namespaces: <none>
```

## Structure

Errors appear for incomplete or partial restores. Warnings appear for non-blocking issues (e.g. the
restore looks "normal" and all resources referenced in the backup exist in some form, although some
of them may have been pre-existing).

Both errors and warnings are structured in the same way:

* `Ark`: A list of system-related issues encountered by the Ark server (e.g. couldn't read directory).

* `Cluster`: A list of issues related to the restore of cluster-scoped resources.

* `Namespaces`: A map of namespaces to the list of issues related to the restore of their respective resources.

## Restoring a load balancer Service

> **Warning** When Ark restores a load balancer Service, the UID of the Service changes. The UID is set by the Kubernetes apiserver and cannot be manually changed. The identifier for your cloud provider load balancer also changes. If you specify a DNS CNAME for your application that points at the DNS name of your cloud provider load balancer, you should check the CNAME pointer after the Ark restore, and update it if necessary.

For more information, see the dicussion in these Kubernetes pull requests: 

* [#49903](https://github.com/kubernetes/kubernetes/pull/49903) 
* [#56832](https://github.com/kubernetes/kubernetes/pull/56832)

[0]: #example
[1]: #structure
[2]: #restoring-a-loadbalancer-service
