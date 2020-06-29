# Vendoring dependencies

## Overview

We are using [dep][0] to manage dependencies. You can install it by running

```
go get -u github.com/golang/dep/cmd/dep
```

Dep currently pulls in a bit more than we'd like, so
we have created a script to remove these extra files: `hack/dep-save.sh`.

## Adding a new dependency

Run `hack/dep-save.sh`. If you want to see verbose output, you can append `-v` as in
`hack/dep-save.sh -v`.

## Updating an existing dependency

Run `hack/dep-save.sh -update <pkg> [<pkg> ...]` to update one or more dependencies.

[0]: https://github.com/golang/dep
