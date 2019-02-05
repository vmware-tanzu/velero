# File Structure

## 00-prereqs.yaml

This file contains the prerequisites necessary to run the Velero server:

- `velero` namespace
- `velero` service account
- RBAC rules to grant permissions to the `velero` service account
- CRDs for the Velero-specific resources (Backup, Schedule, Restore, etc.)
