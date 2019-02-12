# Openstack Example

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