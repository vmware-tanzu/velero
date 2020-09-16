---
title: "Vendoring dependencies"
layout: docs
---

## Overview

We are using [dep][0] to manage dependencies. You can install it by following [these
instructions][1].

## Adding a new dependency

Run `dep ensure`. If you want to see verbose output, you can append `-v` as in
`dep ensure -v`.

## Updating an existing dependency

Run `dep ensure -update <pkg> [<pkg> ...]` to update one or more dependencies.

[0]: https://github.com/golang/dep
[1]: https://golang.github.io/dep/docs/installation.html
