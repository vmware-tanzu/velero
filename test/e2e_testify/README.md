# End-to-end tests using stretcher/testify

# Example command to start test

```bash
BSL_CONFIG="region=minio,s3ForcePathStyle=\"true\",s3Url=http://192.168.1.124:9000" BSL_PREFIX=veldat BSL_BUCKET=velero VELERO_IMAGE=projects.registry.vmware.com/tanzu_migrator/velero-pr3050:0.0.4 CREDS_FILE=~/go/src/github.com/vmware-tanzu/velero/frankie-secrets/credentials-minio PLUGIN_PROVIDER=aws make test-e2e-testify
```

Note that the path to credentials is the full, absolute path.
