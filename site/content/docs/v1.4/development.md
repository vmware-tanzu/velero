---
title: "Development "
layout: docs
---

## Update generated files

Run `make update` to regenerate files if you make the following changes:

* Add/edit/remove command line flags and/or their help text
* Add/edit/remove commands or subcommands
* Add new API types
* Add/edit/remove plugin protobuf message or service definitions

The following files are automatically generated from the source code:

* The clientset
* Listers
* Shared informers
* Documentation
* Protobuf/gRPC types

You can run `make verify` to ensure that all generated files (clientset, listers, shared informers, docs) are up to date.

## Test

To run unit tests, use `make test`.

## Vendor dependencies

If you need to add or update the vendored dependencies, see [Vendoring dependencies][11].

[11]: vendoring-dependencies.md
