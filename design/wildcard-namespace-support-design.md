
# Wildcard Namespace Includes/Excludes Support for Backups and Restores

## Abstract

Velero currently does not support wildcard characters in namespace specifications, requiring all namespaces to be specified as string literals. The only exception is the standalone `*` character, which includes all namespaces and ignores excludes.

This document details the approach to implementing wildcard namespace support for `--include-namespaces` and `--exclude-namespaces` flags, while preserving the existing `*` behavior for backward compatibility.

## Background

This feature was requested in Issue [#1874](https://github.com/vmware-tanzu/velero/issues/1874) to provide more flexible namespace selection using wildcard patterns.

## Goals

- Add support for wildcard patterns in `--include-namespaces` and `--exclude-namespaces` flags
- Ensure legacy `*` behavior remains unchanged for backward compatibility

## Non-Goals

- Completely rethinking the way `*` is treated and allowing it to work with wildcard excludes
- Supporting complex regex patterns beyond basic glob patterns


## High-Level Design

The wildcard expansion implementation focuses on two key functions in `pkg/backup/item_collector.go`:

- [`collectNamespaces`](https://github.com/vmware-tanzu/velero/blob/1535afb45e33a3d3820088e4189800a21ba55293/pkg/backup/item_collector.go#L742) - Retrieves all active namespaces and processes include/exclude filters
- [`getNamespacesToList`](https://github.com/vmware-tanzu/velero/blob/1535afb45e33a3d3820088e4189800a21ba55293/pkg/backup/item_collector.go#L638) - Resolves namespace includes/excludes to final list

The `collectNamespaces` function is the ideal integration point because it:
- Already retrieves all active namespaces from the cluster
- Processes the user-specified namespace filters
- Can expand wildcard patterns against the complete namespace list
- Stores the resolved namespaces in new backup status fields for visibility

This approach ensures wildcard namespaces are handled consistently with the existing "*" behavior, bypassing individual namespace existence checks.

## Detailed Design

The implementation involves four main components that can be developed incrementally:

### Add new status fields to the backup CRD to store expanded wildcard namespaces

```go
// BackupStatus captures the current status of a Velero backup.
type BackupStatus struct {
    // ... existing fields ...
    
    // ExpandedIncludedNamespaces records the expanded include wildcard namespaces
    // +optional
    // +nullable
    ExpandedIncludedNamespaces []string `json:"expandedIncludedNamespaces,omitempty"`

    // ExpandedExcludedNamespaces records the expanded exclude wildcard namespaces
    // +optional
    // +nullable
    ExpandedExcludedNamespaces []string `json:"expandedExcludedNamespaces,omitempty"`
    
    // ... other fields ...
}
```

**Implementation**: Added status fields `ExpandedIncludedNamespaces` and `ExpandedExcludedNamespaces` to `pkg/apis/velero/v1/backup_types.go` to track the resolved namespace lists after wildcard expansion.

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

**Implementation**: In `pkg/backup/item_collector.go`:

```go
// collectNamespaces function (line 748-803)
if wildcard.ShouldExpandWildcards(namespaceSelector.GetIncludes(), namespaceSelector.GetExcludes()) {
    if err := r.expandNamespaceWildcards(activeNamespacesList, namespaceSelector); err != nil {
        return nil, errors.WithMessage(err, "failed to expand namespace wildcard patterns")
    }
}
```

The expansion occurs when collecting namespaces, after retrieving all active namespaces from the cluster. The `expandNamespaceWildcards` method:
- Calls `wildcard.ExpandWildcards()` with active namespaces and original patterns
- Updates the namespace selector with expanded results using `SetIncludes()` and `SetExcludes()`
- Preserves backward compatibility by skipping expansion for simple "*" pattern

**Performance Improvement**: As part of this implementation, active namespaces are stored in a hashset rather than being iterated for each resolved/literal namespace check. This eliminates a [nested loop anti-pattern](https://github.com/vmware-tanzu/velero/blob/1535afb45e33a3d3820088e4189800a21ba55293/pkg/backup/item_collector.go#L767) and improves performance.

### Populate the expanded namespace status field with the namespaces

**Implementation**: In `expandNamespaceWildcards` function (line 889-891):

```go
// Record the expanded wildcard includes/excludes in the request status
r.backupRequest.Status.ExpandedIncludedNamespaces = expandedIncludes
r.backupRequest.Status.ExpandedExcludedNamespaces = expandedExcludes
```

The status fields are populated immediately after successful wildcard expansion, providing visibility into which namespaces were actually matched by the wildcard patterns.

## Alternatives Considered

Several implementation approaches were considered for the wildcard expansion logic:

### 1. Wildcard expansion in `getNamespacesToList`

This approach was ruled out because:
- The `collectNamespaces` function encounters wildcard patterns first in the processing flow
- `collectNamespaces` already has access to the complete list of active namespaces, eliminating the need for additional API calls
- Placing the logic in `getNamespacesToList` would require redundant namespace retrieval

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
- The standalone `*` character continues to work as before (include all namespaces, ignore excludes)
- Existing backup configurations remain unaffected

### Known Limitations

1. **Mixed `*` and wildcard usage**: When using the standalone `*` in includes, the legacy implementation takes precedence and wildcard expansion is skipped. This means you cannot combine "*" (all namespaces) in includes with wildcard patterns in excludes.

2. **Selective exclusion limitation**: The current design does not support selective pattern-based exclusion when including all namespaces via `*`.

## Implementation

The implementation follows the detailed design outlined above, with the following timeline:

1. **Phase 1**: Add backup CRD status fields for expanded namespaces
2. **Phase 2**: Implement wildcard utility package with validation
3. **Phase 3**: Integrate expansion logic into backup item collection
4. **Phase 4**: Add status field population and logging

## Open Issues

**Restore Integration**: Currently investigating how restore operations should handle wildcard-expanded backups. The proposed approach is to apply wildcard patterns against the expanded namespace list stored in the backup's status fields, ensuring consistency between backup and restore operations.