
# Wildcard (*) support for Include/Exclude Namespaces

## Abstract

- Velero currently does **not** support wildcard characters in namespace include/exclude specs, they must be specified as string literals 
- This design details an approach to implementing wildcard namespace support

## Background

This feature was requested in Issue [#1874](https://github.com/vmware-tanzu/velero/issues/1874) to provide more flexible namespace selection using wildcard patterns.

## Goals

- Add support for wildcard patterns in `--include-namespaces` and `--exclude-namespaces` flags for both backup and restore
- Ensure existing `*` behavior remains unchanged for backward compatibility

## Non-Goals

- Supporting complex regex patterns beyond basic glob patterns: this could be explored later


## High-Level Design

### NamespaceIncludesExcludes struct

- `NamespaceIncludesExcludes` is a wrapper around `IncludesExcludes`
- It has a field `activeNamespaces`, which will be used to match wildcard patterns against active namespaces

### Backup

The wildcard expansion implementation expands the patterns before the normal execution of the backup. 
The goal here is ensuring wildcard namespaces are expanded before they're required.
It focuses on two key functions in `pkg/backup/item_collector.go`:

- [`collectNamespaces`](https://github.com/vmware-tanzu/velero/blob/main/pkg/backup/item_collector.go#L742) 
- [`getNamespacesToList`](https://github.com/vmware-tanzu/velero/blob/main/pkg/backup/item_collector.go#L638)

Call to expand wildcards is made at:
- [`getResourceItems`](https://github.com/vmware-tanzu/velero/blob/main/pkg/backup/item_collector.go#L356) - Integration point


Wildcard expansion is conditionally run when the [`NamespaceIncludesExcludes.ResolveNamespaceList()`](https://github.com/vmware-tanzu/velero/blob/main/pkg/util/collections/includes_excludes.go#L146) function is called:

unc (nie *NamespaceIncludesExcludes) ResolveNamespaceList() ([]string, bool, error) {

- It calls [`wildcard.ShouldExpandWildcards()`](https://github.com/vmware-tanzu/velero/blob/main/pkg/util/wildcard/wildcard.go) to determine if expansion is needed
- If wildcards are detected, it uses [`wildcard.ExpandWildcards()`](https://github.com/vmware-tanzu/velero/blob/main/pkg/util/wildcard/wildcard.go) to resolve them against active namespaces
- It overwrites the `NamespaceIncludesExcludes` struct's `Includes` and `Excludes` with the expanded literal namespaces using [`SetIncludes()`](https://github.com/vmware-tanzu/velero/blob/main/pkg/util/collections/includes_excludes.go#L100) and [`SetExcludes()`](https://github.com/vmware-tanzu/velero/blob/main/pkg/util/collections/includes_excludes.go#L107)
- It returns the effective list of namespaces to be backed up and a bool indicating whether wildcard expansion occured





### Restore

The wildcard expansion implementation for restore operations focuses on the main execution flow in `pkg/restore/restore.go`:

- [`execute`](https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L430) - Main restore execution that parses backup contents and processes namespace filters
- [`extractNamespacesFromBackup`](https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L2407) - Extracts available namespaces from backup tar contents

The `execute` function is the ideal integration point because it:
- Already parses the backup tar file to understand available resources
- Processes the user-specified namespace filters for the restore operation
- Can expand wildcard patterns against namespaces that actually exist in the backup
- Stores the resolved namespaces in new restore status fields for visibility

This approach ensures wildcard namespaces in restore operations are based on actual backup contents rather than original backup specifications, providing safety and consistency regardless of how the backup was created.

## Detailed Design

The implementation involves four main components that can be developed incrementally:

### Add new status fields to the backup and restore CRDs to store expanded wildcard namespaces

#### Backup

```go
// WildcardNamespaceStatus contains information about wildcard namespace matching results
type WildcardNamespaceStatus struct {
    // IncludeWildcardMatches records the namespaces that matched include wildcard patterns
    // +optional
    // +nullable
    IncludeWildcardMatches []string `json:"includeWildcardMatches,omitempty"`

    // ExcludeWildcardMatches records the namespaces that matched exclude wildcard patterns
    // +optional
    // +nullable
    ExcludeWildcardMatches []string `json:"excludeWildcardMatches,omitempty"`

    // WildcardResult records the final namespaces after applying wildcard include/exclude logic
    // +optional
    // +nullable
    WildcardResult []string `json:"wildcardResult,omitempty"`
}

// BackupStatus captures the current status of a Velero backup.
type BackupStatus struct {
    // ... existing fields ...
    
    // WildcardNamespaces contains information about wildcard namespace processing
    // +optional
    // +nullable
    WildcardNamespaces *WildcardNamespaceStatus `json:"wildcardNamespaces,omitempty"`
    
    // ... other fields ...
}
```

#### Restore

```go
// WildcardNamespaceStatus contains information about wildcard namespace matching results
type WildcardNamespaceStatus struct {
    // IncludeWildcardMatches records the namespaces that matched include wildcard patterns
    // +optional
    // +nullable
    IncludeWildcardMatches []string `json:"includeWildcardMatches,omitempty"`

    // ExcludeWildcardMatches records the namespaces that matched exclude wildcard patterns
    // +optional
    // +nullable
    ExcludeWildcardMatches []string `json:"excludeWildcardMatches,omitempty"`

    // WildcardResult records the final namespaces after applying wildcard include/exclude logic
    // +optional
    // +nullable
    WildcardResult []string `json:"wildcardResult,omitempty"`
}

// RestoreStatus captures the current status of a Velero restore.
type RestoreStatus struct {
    // ... existing fields ...
    
    // WildcardNamespaces contains information about wildcard namespace processing
    // +optional
    // +nullable
    WildcardNamespaces *WildcardNamespaceStatus `json:"wildcardNamespaces,omitempty"`
    
    // ... other fields ...
}
```

**Implementation**: Added a structured `WildcardNamespaceStatus` type and `WildcardNamespaces` field to `pkg/apis/velero/v1/backup_types.go` and `pkg/apis/velero/v1/restore_types.go` to track the resolved namespace lists after wildcard expansion in a well-organized manner.

### Create a util package for wildcard expansion

**Implementation**: Created `pkg/util/wildcard/expand.go` package containing:

- `ShouldExpandWildcards(includes, excludes []string) bool` - Determines if wildcard expansion is needed (excludes simple "*" case)
- `ExpandWildcards(activeNamespaces, includes, excludes []string) ([]string, []string, error)` - Main expansion function
- `containsWildcardPattern(pattern string) bool` - Detects wildcard patterns (`*`, `?`, `[abc]`, `{a,b,c}`)
- `validateWildcardPatterns(patterns []string) error` - Validates patterns and rejects unsupported regex symbols
- Uses `github.com/gobwas/glob` library for glob pattern matching

**Supported patterns**: 
- `*` (any characters)
- `?` (single character)
- `[abc]` (character classes)
- `{a,b,c}` (alternatives)

**Unsupported**: Regex symbols `|()`, consecutive asterisks `**`

### If required, expand wildcards and replace the request's includes and excludes with expanded namespaces

### Backup:

**Implementation**: In `pkg/backup/item_collector.go`:


The expansion occurs when collecting namespaces, after retrieving all active namespaces from the cluster. The `expandNamespaceWildcards` method:
- Calls `wildcard.ExpandWildcards()` with active namespaces and original patterns
- Updates the namespace selector with expanded results using `SetIncludes()` and `SetExcludes()`
- Preserves backward compatibility by skipping expansion for simple "*" pattern

**Performance Improvement**: As part of this implementation, active namespaces are stored in a hashset rather than being iterated for each resolved/literal namespace check. This eliminates a [nested loop anti-pattern](https://github.com/vmware-tanzu/velero/blob/1535afb45e33a3d3820088e4189800a21ba55293/pkg/backup/item_collector.go#L767) and improves performance.

### Restore

**Implementation**: In `pkg/restore/restore.go`:

```go
// Lines 478-509: Wildcard expansion in restore context
if wildcard.ShouldExpandWildcards(ctx.restore.Spec.IncludedNamespaces, ctx.restore.Spec.ExcludedNamespaces) {
    availableNamespaces := extractNamespacesFromBackup(backupResources)
    expandedIncludes, expandedExcludes, err := wildcard.ExpandWildcards(
        availableNamespaces,
        ctx.restore.Spec.IncludedNamespaces, 
        ctx.restore.Spec.ExcludedNamespaces,
    )
    // Update restore context with expanded patterns
    ctx.namespaceIncludesExcludes = collections.NewIncludesExcludes().
        Includes(expandedIncludes...).
        Excludes(expandedExcludes...)
}
```

The restore expansion occurs after parsing the backup tar contents, using `extractNamespacesFromBackup` to determine which namespaces are actually available for restoration. This ensures wildcard patterns are applied against materialized backup contents rather than original backup specifications.



### Populate the expanded namespace status field with the namespaces

#### Backup Status Fields

**Implementation**: In `expandNamespaceWildcards` function (line 889-891):

```go
// Record the expanded wildcard includes/excludes in the request status
r.backupRequest.Status.WildcardNamespaces = &velerov1api.WildcardNamespaceStatus{
	IncludeWildcardMatches: expandedIncludes,
	ExcludeWildcardMatches: expandedExcludes,
	WildcardResult:         wildcardResult,
}
```

#### Restore Status Fields

**Implementation**: In `pkg/restore/restore.go` (lines 499-502):

```go
// Record the expanded wildcard includes/excludes in the restore status
ctx.restore.Status.WildcardNamespaces = &velerov1api.WildcardNamespaceStatus{
	IncludeWildcardMatches: expandedIncludes,
	ExcludeWildcardMatches: expandedExcludes,
	WildcardResult:         wildcardResult,
}
```

The status fields are populated immediately after successful wildcard expansion, providing visibility into which namespaces were actually matched by the wildcard patterns and the final list of namespaces that will be processed.

## Alternatives Considered

Several implementation approaches were considered for the wildcard expansion logic:

### 1. Wildcard expansion in `getNamespacesToList` or `collectNamespaces`

This approach was ruled out because:
- Both funcs expect non-wildcard namespaces in the NamespaceIncludesExcludes struct
- Calling it at a higher level within getResourceItems is ideal

### 2. Client-side wildcard expansion

Expanding wildcards on the CLI side was considered but rejected because:
- It would only work for command-line initiated backups
- Scheduled backups would not benefit from this approach
- The server-side approach provides consistent behavior across all backup initiation methods

## Security Considerations

This feature does not introduce any security vulnerabilities as it only affects namespace selection logic within the existing backup authorization framework.

## Compatibility

### Backward Compatibility

The implementation maintains full backward compatibility with existing behavior:
- The standalone "*" character continues to work as before

### Known Limitations

N/A

## Implementation

Aiming for 1.18

## Open Issues

N/A
