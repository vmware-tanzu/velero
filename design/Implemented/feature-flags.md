# Feature Flags

Status: Accepted

Some features may take a while to get fully implemented, and we don't necessarily want to have long-lived feature branches
A simple feature flag implementation allows code to be merged into main, but not used unless a flag is set.

## Goals

- Allow unfinished features to be present in Velero releases, but only enabled when the associated flag is set.

## Non Goals

- A robust feature flag library.

## Background

When considering the [CSI integration work](https://github.com/heptio/velero/pull/1661), the timelines involved presented a problem in balancing a release and longer-running feature work.
A simple implementation of feature flags can help protect unfinished code while allowing the rest of the changes to ship.

## High-Level Design

A new command line flag, `--features` will be added to the root `velero` command.

`--features` will accept a comma-separated list of features, such as `--features EnableCSI,Replication`.
Each feature listed will correspond to a key in a map in `pkg/features/features.go` defining whether a feature should be enabled.

Any code implementing the feature would then import the map and look up the key's value.

For the Velero client, a `features` key can be added to the `config.json` file for more convenient client invocations.

## Detailed Design

A new `features` package will be introduced with these basic structs:

```go
type FeatureFlagSet struct {
    flags map[string]bool
}

type Flags interface {
    // Enabled reports whether or not the specified flag is found.
    Enabled(name string) bool

    // Enable adds the specified flags to the list of enabled flags.
    Enable(names ...string)

    // All returns all enabled features
    All() []string
}

// NewFeatureFlagSet initializes and populates a new FeatureFlagSet
func NewFeatureFlagSet(flags ...string) FeatureFlagSet
```

When parsing the `--features` flag, the entire `[]string` will be passed to `NewFeatureFlagSet`.
Additional features can be added with the `Enable` function.
Parsed features will be printed as an `Info` level message on server start up.

No verification of features will be done in order to keep the implementation minimal.

On the client side, `--features` and the `features` key in `config.json` file will be additive, resulting in the union of both.

To disable a feature, the server must be stopped and restarted with a modified `--features` list.
Similarly, the client process must be stopped and restarted without features.

## Alternatives Considered
Omitted

## Security Considerations
Omitted
