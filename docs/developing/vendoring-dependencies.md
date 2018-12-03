# Vendoring dependencies

## Overview

We use [dep][0] to manage dependencies. To install dep, see [TODO CHECK TITLE][1].

## Add a new dependency

Run `dep ensure`.

For verbose output, append `-v`: `dep ensure -v`.

## Update an existing dependency

Run `dep ensure -update <pkg> [<pkg> ...]` to update dependencies.

[0]: https://github.com/golang/dep
[1]: https://golang.github.io/dep/docs/installation.html
