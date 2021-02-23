
Velero kubeconfigs:

* Uses multiple kube clients:
    * kubeClient (probably mgmt cluster kubeconfig)
        * checks if namespace of CRs (backups/restores/schedules etc.) exists
        * checks if restic DaemonSet is found
        * passed to restic.RepositoryManager in order to retrieve PVs/PVCs
        * used for pre and post hooks
        * passed to `backupSyncController` but not used anywhere
    * veleroClient (probably mgmt cluster kubeconfig)
        * operates with Velero specific CRs (backups/restores/schedules etc.)

    * discoveryClient & discoveryHelper (probably customer cluster kubeconfig)
        * returned from `veleroClient.Discovery()`
        * discovers resources
    * dynamicClient (probably customer cluster kubeconfig)
        * generically collects resources


* veleroClient: management cluster kubeconfig: 99%
    * should be exclusively used for Velero CRDs

* discoveryClient:
    * one used for velero discovery as discoveryHelper
    * (TODO) (all other)
    * restorer & backup:
        * customer cluster
    * one used for csi snapshot discovery (is from veleroClient but could be from kubeClient)

* dynamicClient:
    * probably for backup to be able to handle all resources
    * restorer & backup:
        * customer cluster
    * => getClients(cluster string) (dynamicClient, discoveryClient)


* s.mgr.GetClient
    * probably only management cluster

* NewRepositoryManager
    * veleroClient: sharedInformerFactory
    * kubeClient
    * mgr.GetClient()

* NewBackupSyncController
    * veleroClient: sharedInformerFactory
    * csiSnapshotClient
    * kubeClient
    * s.mgr.GetClient

* NewBackupController
    * veleroClient: sharedInformerFactory
    * s.mgr.GetClient
    * NewKubernetesBackupper
        * veleroClient
            * sharedInformerFactory
        * dynamicClient
        * kubeClient / kubeClientConfig

* NewScheduleController
    * veleroClient: sharedInformerFactory

* NewGCController
    * veleroClient: sharedInformerFactory
    * s.mgr.GetClient

* NewBackupDeletionController
    * veleroClient: sharedInformerFactory
    * backupTracker
    * resticManager
    * csiVSLister / csiVSCLister
    * s.mgr.GetClient

* NewRestoreController
    * NewKubernetesRestorer
        * dynamicClient
        * kubeClient
    * veleroClient: sharedInformerFactory
    * s.mgr.GetClient

* NewResticRepositoryController
    * veleroClient: sharedInformerFactory
    * s.mgr.GetClient

* NewDownloadRequestController
    * veleroClient: sharedInformerFactory
    * s.mgr.GetClient(),

* BackupStorageLocationReconciler
    * s.mgr.GetClient(),
* ServerStatusRequestReconciler
    * s.mgr.GetClient(),


TODO:
* test backup/restore with our fork with 2 separate nightlies

Open multi-cluster issues: (1 management : n customer)
* we have to create restConfig / kubeClients dynamically
* we have to create the backupper/restorer dynamically

Open "upstream" issues:
* we didn't adjust the csi lister / watcher (probably just change them to customer cluster)