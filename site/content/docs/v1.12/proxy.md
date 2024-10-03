---
title: "Behind Proxy"
layout: docs
toc: "true"
---

This document explains how to make Velero work behind proxy.
The procedures described in this document are concluded from the scenario that Velero is deployed behind proxy, and Velero needs to connect to a public MinIO server as storage location. Maybe other scenarios' configurations are not exactly the same, but basically they should share most parts.

## Set the proxy server address
Specify the proxy server address by environment variables in Velero deployment and node-agent DaemonSet.
Take the following as an example:
``` yaml
    ...
    spec:
      containers:
      - args:
        - server
        - --features=EnableCSI
        command:
        - /velero
        env:
        ...
        - name: HTTP_PROXY
          value: <proxy_address>
        - name: HTTPS_PROXY
          value: <proxy_address>
        # In case not all destinations that Velero connects to need go through proxy, users can specify the NO_PROXY to bypass proxy. 
        - name: NO_PROXY
          value: <address_list_not_use_proxy>
```

## Set the proxy required certificates
In some cases, the proxy requires certificate to connect. Set the certificate in the BSL's `Spec.ObjectStorage.CACert`.
It's possible that the object storage also requires certificate, and it's also set in `Spec.ObjectStorage.CACert`, then set both certificates in `Spec.ObjectStorage.CACert` field.

The following is an example file contains two certificates, then encode its content with base64, and set the encode result in the BSL.

``` bash
cat certs
-----BEGIN CERTIFICATE-----
certificates first content
-----END CERTIFICATE-----

-----BEGIN CERTIFICATE-----
certificates second content
-----END CERTIFICATE-----

cat certs | base64
LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmNlcnRpZmljYXRlcyBmaXJzdCBjb250ZW50Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KCi0tLS0tQkVHSU4gQ0VSVElGSUNBVEUtLS0tLQpjZXJ0aWZpY2F0ZXMgc2Vjb25kIGNvbnRlbnQKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
```

``` yaml
  apiVersion: velero.io/v1
  kind: BackupStorageLocation
  ...
  spec:
    ...
    default: true
    objectStorage:
      bucket: velero
      caCert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCmNlcnRpZmljYXRlcyBmaXJzdCBjb250ZW50Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KCi0tLS0tQkVHSU4gQ0VSVElGSUNBVEUtLS0tLQpjZXJ0aWZpY2F0ZXMgc2Vjb25kIGNvbnRlbnQKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  ...
```
