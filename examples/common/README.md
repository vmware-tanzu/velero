# File Structure

## 00-prereqs.yaml

This file contains the prerequisites necessary to run the Ark server:

- `heptio-ark` namespace
- `ark` service account
- RBAC rules to grant permissions to the `ark` service account
- CRDs for the Ark-specific resources (Backup, Schedule, Restore, Config)

## 10-deployment.yaml

This deploys Ark and be used for AWS, GCP, and Minio. *Note that it cannot be used for Azure.*
