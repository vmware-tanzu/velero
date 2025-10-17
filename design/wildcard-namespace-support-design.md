
# Wildcard Namespace Support

## Abstract

Velero currently treats namespace patterns with glob characters as literal strings. This design adds wildcard expansion to support flexible namespace selection using patterns like `app-*` or `test-{dev,staging}`.

## Background

Requested in [#1874](https://github.com/vmware-tanzu/velero/issues/1874) for more flexible namespace selection.

## Goals

- Support glob pattern expansion in namespace includes/excludes
- Maintain backward compatibility with existing `*` behavior

## Non-Goals

- Complex regex patterns beyond basic globs

## High-Level Design

Wildcard expansion occurs early in both backup and restore flows, converting patterns to literal namespace lists before normal processing.

### Backup Flow

Expansion happens in `getResourceItems()` before namespace collection:
1. Check if wildcards exist using `ShouldExpandWildcards()`
2. Expand patterns against active cluster namespaces
3. Replace includes/excludes with expanded literal namespaces
4. Continue with normal backup processing

### Restore Flow

Expansion occurs in `execute()` after parsing backup contents:
1. Extract available namespaces from backup tar
2. Expand patterns against backup namespaces (not cluster namespaces)
3. Update restore context with expanded namespaces
4. Continue with normal restore processing

This ensures restore wildcards match actual backup contents, not current cluster state.

## Detailed Design

### Status Fields

Add wildcard expansion tracking to backup and restore CRDs:

```go
type WildcardNamespaceStatus struct {
    // IncludeWildcardMatches records namespaces that matched include patterns
    // +optional
    IncludeWildcardMatches []string `json:"includeWildcardMatches,omitempty"`
    
    // ExcludeWildcardMatches records namespaces that matched exclude patterns  
    // +optional
    ExcludeWildcardMatches []string `json:"excludeWildcardMatches,omitempty"`
    
    // WildcardResult records final namespaces after wildcard processing
    // +optional
    WildcardResult []string `json:"wildcardResult,omitempty"`
}

// Added to both BackupStatus and RestoreStatus
type BackupStatus struct {
    // WildcardNamespaces contains wildcard expansion results
    // +optional
    WildcardNamespaces *WildcardNamespaceStatus `json:"wildcardNamespaces,omitempty"`
}
```

### Wildcard Expansion Package

New `pkg/util/wildcard/expand.go` package provides:

- `ShouldExpandWildcards()` - Skip expansion for simple "*" case
- `ExpandWildcards()` - Main expansion function using `github.com/gobwas/glob`
- Pattern validation rejecting unsupported regex symbols

**Supported patterns**: `*`, `?`, `[abc]`, `{a,b,c}`  
**Unsupported**: `|()`, `**`

### Implementation Details

#### Backup Integration (`pkg/backup/item_collector.go`)

Expansion in `getResourceItems()`:
- Call `wildcard.ExpandWildcards()` with cluster namespaces
- Update `NamespaceIncludesExcludes` with expanded results
- Populate status fields with expansion results

#### Restore Integration (`pkg/restore/restore.go`)

Expansion in `execute()`:
```go
if wildcard.ShouldExpandWildcards(includes, excludes) {
    availableNamespaces := extractNamespacesFromBackup(backupResources)
    expandedIncludes, expandedExcludes, err := wildcard.ExpandWildcards(
        availableNamespaces, includes, excludes)
    // Update context and status
}
```

## Alternatives Considered

1. **Client-side expansion**: Rejected because it wouldn't work for scheduled backups
2. **Expansion in `collectNamespaces`**: Rejected because these functions expect literal namespaces

## Compatibility

Maintains full backward compatibility - existing "*" behavior unchanged.

## Implementation

Target: Velero 1.18
