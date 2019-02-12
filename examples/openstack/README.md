# Openstack Example

Be sure to create a bucket in swift for your backup.  Reference this bucket in the backupstoragelocation spec.

Ensure that a secret is created for ark to authenticate to openstack cloud.  You can source your openstack rc file and then create the secret directly.
```
kubectl -n heptio-ark create secret generic cloud-credentials \
 --from-literal OS_PROJECT_ID=$OS_PROJECT_ID \
 --from-literal OS_REGION_NAME=$OS_REGION_NAME \
 --from-literal OS_USER_DOMAIN_NAME=$OS_USER_DOMAIN_NAME \
 --from-literal OS_PROJECT_NAME=$OS_PROJECT_NAME \
 --from-literal OS_IDENTITY_API_VERSION=$OS_IDENTITY_API_VERSION \
 --from-literal OS_PASSWORD=$OS_PASSWORD \
 --from-literal OS_AUTH_URL=$OS_AUTH_URL \
 --from-literal OS_USERNAME=$OS_USERNAME \
 --from-literal OS_INTERFACE=$OS_INTERFACE \
 --from-literal OS_PROJECT_DOMAIN_NAME=$OS_PROJECT_DOMAIN_NAME \
 --from-literal OS_DOMAIN_NAME=$OS_DOMAIN_NAME
```

Create all of the objects in the common directory
```
kubectl -n heptio-ark create -f ./common
```

Create all of the resources in this directory
- Backup storage location object
- Volume Snapshot Location
- The ark deployment resource
- The restic daemonset resource
```
kubectl -n heptio-ark create -f ./openstack
```

