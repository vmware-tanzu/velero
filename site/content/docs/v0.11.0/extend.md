---
title: "Extend Velero"
layout: docs
---

Velero includes mechanisms for extending the core functionality to meet your individual backup/restore needs:

* [Hooks][27] allow you to specify commands to be executed within running pods during a backup. This is useful if you need to run a workload-specific command prior to taking a backup (for example, to flush disk buffers or to freeze a database).
* [Plugins][28] allow you to develop custom object/block storage back-ends or per-item backup/restore actions that can execute arbitrary logic, including modifying the items being backed up/restored. Plugins can be used by Velero without needing to be compiled into the core Velero binary.

[27]: hooks.md
[28]: plugins.md
