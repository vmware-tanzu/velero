# Uninstalling Velero

If you would like to completely uninstall Velero from your cluster, the following commands will remove all resources created by `velero install`:

```bash
kubectl delete namespace/velero clusterrolebinding/velero
kubectl delete crds -l component=velero
```