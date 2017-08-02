# Files

This directory contains manifests for two versions of a sample Nginx app under the `nginx-example` namespace.

## `base.yaml`

This is the most basic version of the Nginx app, which can be used to test Ark's backup and restore functionality.

*This can be deployed as is.*

## `with-pv.yaml`

This sets up an Nginx app that logs to a persistent volume, so that Ark's PV snapshotting functionality can also be tested.

*This requires you to first replace the placeholder value `<YOUR_STORAGE_CLASS_NAME>`.*
