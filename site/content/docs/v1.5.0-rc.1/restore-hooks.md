---
title: "Restore Hooks"
layout: docs
---

Velero supports Restore Hooks, custom actions that can be executed during or after the restore process. There are two kinds of Restore Hooks:

1. InitContainer Restore Hooks: These will add init containers into restored pods to perform any necessary setup before the application containers of the restored pod can start.
1. Exec Restore Hooks: These can be used to execute custom commands or scripts in containers of a restored Kubernetes pod.

Only InitContainer Restore Hooks is supported and support for Exec Restore Hooks is coming soon.

## InitContainer Restore Hooks

Use an `InitContainer` hook to add init containers into a pod before it's restored. You can use these init containers to run any setup needed for the pod to resume running from its backed-up state.

There are two ways to specify `InitContainer` restore hooks:
1. Specifying restore hooks in annotations
1. Specifying restore hooks in the restore spec

### Specifying Restore Hooks As Pod Annotations

Below are the annotations that can be added to a pod to specify restore hooks:
* `init.hook.restore.velero.io/container-image`
    * The container image for the init container to be added.
* `init.hook.restore.velero.io/container-name`
    * The name for the init container that is being added.
* `init.hook.restore.velero.io/command`
    * This is the `ENTRYPOINT` for the init container being added. This command is not executed within a shell and the container image's `ENTRYPOINT` is used if this is not provided.

#### Example

Use the below commands to add annotations to the pods before taking a backup.

```bash
$ kubectl annotate pod -n <POD_NAMESPACE> <POD_NAME> \
    init.hook.restore.velero.io/container-name=restore-hook \
    init.hook.restore.velero.io/container-image=alpine:latest \
    init.hook.restore.velero.io/command='["/bin/ash", "-c", "date"]'
```

With the annotation above, Velero will add the following init container to the pod when it's restored.

```json
{
  "command": [
    "/bin/ash",
    "-c",
    "date"
  ],
  "image": "alpine:latest",
  "imagePullPolicy": "Always",
  "name": "restore-hook"
  ...
}
```

### Specifying Restore Hooks In Restore Spec

Init container restore hooks can also be specified using the `RestoreSpec`.
Please refer to the documentation on the [Restore API Type][1] for how to specify hooks in the Restore spec.

#### Example

Below is an example of specifying restore hooks in `RestoreSpec`

```yaml
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: r2
  namespace: velero
spec:
  backupName: b2
  excludedResources:
  ...
  includedNamespaces:
  - '*'
  hooks:
    resources:
    - name: restore-hook-1
      includedNamespaces:
      - app
      postHooks:
      - init:
          initContainers:
          - name: restore-hook-init1
            image: alpine:latest
            volumeMounts:
            - mountPath: /restores/pvc1-vm
              name: pvc1-vm
            command:
            - /bin/ash
            - -c
            - echo -n "FOOBARBAZ" >> /restores/pvc1-vm/foobarbaz
          - name: restore-hook-init2
            image: alpine:latest
            volumeMounts:
            - mountPath: /restores/pvc2-vm
              name: pvc2-vm
            command:
            - /bin/ash
            - -c
            - echo -n "DEADFEED" >> /restores/pvc2-vm/deadfeed
```

The `hooks` in the above `RestoreSpec`, when restored, will add two init containers to every pod in the `app` namespace

```json
{
  "command": [
    "/bin/ash",
    "-c",
    "echo -n \"FOOBARBAZ\" >> /restores/pvc1-vm/foobarbaz"
  ],
  "image": "alpine:latest",
  "imagePullPolicy": "Always",
  "name": "restore-hook-init1",
  "resources": {},
  "terminationMessagePath": "/dev/termination-log",
  "terminationMessagePolicy": "File",
  "volumeMounts": [
    {
      "mountPath": "/restores/pvc1-vm",
      "name": "pvc1-vm"
    }
  ]
  ...
}
```

and

```json
{
  "command": [
    "/bin/ash",
    "-c",
    "echo -n \"DEADFEED\" >> /restores/pvc2-vm/deadfeed"
  ],
  "image": "alpine:latest",
  "imagePullPolicy": "Always",
  "name": "restore-hook-init2",
  "resources": {},
  "terminationMessagePath": "/dev/termination-log",
  "terminationMessagePolicy": "File",
  "volumeMounts": [
    {
      "mountPath": "/restores/pvc2-vm",
      "name": "pvc2-vm"
    }
  ]
  ...
}
```

[1]: api-types/restore.md
