---
title: "Hooks"
layout: docs
---

Heptio Ark currently supports executing commands in containers in pods during a backup.

## Backup Hooks

When performing a backup, you can specify one or more commands to execute in a container in a pod
when that pod is being backed up. There are two ways to specify hooks: annotations on the pod
itself, and in the Backup spec.

### Specifying Hooks As Pod Annotations

You can use the following annotations on a pod to make Ark execute a hook when backing up the pod:

| Annotation Name | Description |
| --- | --- |
| `hook.backup.ark.heptio.com/container` | The container where the command should be executed.  Defaults to the first container in the pod. Optional. |
| `hook.backup.ark.heptio.com/command` | The command to execute. If you need multiple arguments, specify the command as a JSON array, such as `["/usr/bin/uname", "-a"]` |
| `hook.backup.ark.heptio.com/on-error` | What to do if the command returns a non-zero exit code.  Defaults to Fail. Valid values are Fail and Continue. Optional. |
| `hook.backup.ark.heptio.com/timeout` | How long to wait for the command to execute. The hook is considered in error if the command exceeds the timeout. Defaults to 30s. Optional. |

### Specifying Hooks in the Backup Spec

Please see the documentation on the [Backup API Type][1] for how to specify hooks in the Backup
spec.

[1]: api-types/backup.md
