# Run Ark more securely with restrictive RBAC settings

By default Ark runs with an RBAC policy of ClusterRole `cluster-admin`. This is to make sure that Ark can back up or restore anything in your cluster. But `cluster-admin` access is wide open -- it gives Ark components access to everything in your cluster. Depending on your environment and your security needs, you should consider whether to configure additional RBAC policies with more restrictive access. 

**Note:** Roles and RoleBindings are associated with a single namespaces, not with an entire cluster. PersistentVolume backups are associated only with an entire cluster. This means that any backups or restores that use a restrictive Role and RoleBinding pair can manage only the resources that belong to the namespace. You do not need a wide open RBAC policy to manage PersistentVolumes, however. You can configure a ClusterRole and ClusterRoleBinding that allow backups and restores only of PersistentVolumes, not of all objects in the cluster.

For more information about RBAC and access control generally in Kubernetes, see the Kubernetes documentation about [access control][1], [managing service accounts][2], and [RBAC authorization][3].

## Set up Roles and RoleBindings

Here's a sample Role and RoleBinding pair.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: YOUR_NAMESPACE_HERE
  name: ROLE_NAME_HERE
  labels:
    component: ark
rules:
  - apiGroups:
      - ark.heptio.com
    verbs:
      - "*"
    resources:
      - "*"
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ROLEBINDING_NAME_HERE
subjects:
  - kind: ServiceAccount
    name: YOUR_SERVICEACCOUNT_HERE
roleRef:
  kind: Role
  name: ROLE_NAME_HERE
  apiGroup: rbac.authorization.k8s.io
```

[1]: https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/
[2]: https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
[3]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[4]: namespace.md
