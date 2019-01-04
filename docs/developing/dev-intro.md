# Developing with Ark

Ark is an [open source project][1], so you can extend or develop it for yourself. We also welcome contributions to the code base! See [our contributing guidelines][2] for the formal requirements and our communication channels.

This guide provides more details about how to get started extending Ark or contributing directly to the original project.

- [Before you start developing][3]
- [Build][4]
- [Run][5]
- [Test][6]

## Extending Ark

Ark also includes mechanisms for extending the core functionality to meet your individual backup/restore needs:

* [Hooks][27] allow you to specify commands to be executed within running pods during a backup. This is useful if you need to run a workload-specific command prior to taking a backup (for example, to flush disk buffers or to freeze a database).
* [Plugins][28] allow you to develop custom object/block storage back-ends or per-item backup/restore actions that can execute arbitrary logic, including modifying the items being backed up/restored. Plugins can be used by Ark without needing to be compiled into the core Ark binary.

TODO: put here or in prerequisites assumptions we make about what you should already know before you start contributing?

[1]: https://github.com/heptio/ark/LICENSE
[2]: https://github.com/heptio/ark/CONTRIBUTING.md
[3]: dev-prereq.md
[4]: build.md
[5]: dev-run.md
[6]: dev-test.md
[27]: hooks.md
[28]: plugins.md