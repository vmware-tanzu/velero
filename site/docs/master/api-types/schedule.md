# Schedule API Type

## Use

The `Schedule` API type is used as a repeatable request for the Velero server to perform a backup for a given cron notation. Once created, the
Velero Server will start the backup process. It will then wait for the next valid point of the given cron expression and execute the backup 
process on a repeating basis.

## API GroupVersion

Schedule belongs to the API group version `velero.io/v1`.

## Definition

Here is a sample `Schedule` object with each of the fields documented:

```yaml
# Standard Kubernetes API Version declaration. Required.
apiVersion: velero.io/v1
# Standard Kubernetes Kind declaration. Required.
kind: Schedule
# Standard Kubernetes metadata. Required.
metadata:
  # Schedule name. May be any valid Kubernetes object name. Required.
  name: a
  # Schedule namespace. Must be the namespace of the Velero server. Required.
  namespace: velero
# Parameters about the scheduled backup. Required.
spec:
status:
```
