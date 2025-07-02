# Wildcard Namespace Support Design

## Abstract
This proposal introduces wildcard pattern support for namespace inclusion and exclusion in Velero backups (e.g., `prod-*`, `*-staging`).
The implementation uses lazy evaluation within the existing `ShouldInclude()` method to resolve wildcards on-demand with request-scoped caching.
Based on [Issue #1874](https://github.com/vmware-tanzu/velero/issues/1874).

## Background
- Currently, Velero users must explicitly list each namespace for backup operations
- In environments with many namespaces following naming conventions (e.g., `prod-app`, `prod-db`, `prod-cache`), this becomes:
  - Cumbersome to maintain
  - Error-prone to manage
- Users have requested wildcard support to enable patterns like `--include-namespaces "prod-*"`

## Goals
- Enable wildcard pattern support for namespace includes and excludes in Velero backup specifications
- Maintain optimal performance with lazy evaluation and request-scoped caching
- Preserve original wildcard patterns in backup specifications for audit and readability purposes

## Non Goals
- Support for complex regex patterns beyond basic glob-style wildcards (`*`)
- Persistent caching of namespace resolution across backup requests
- Real-time namespace discovery that changes during backup execution

## High-Level Design

**Core Approach:** We're making the existing concrete type (`*IncludesExcludes`) polymorphic so we can substitute our new lazy evaluation type (`*LazyNamespaceIncludesExcludes`) without changing any calling code.

- Implementation at **backup request level** within the `ShouldInclude()` method
- Uses lazy evaluation with `LazyNamespaceIncludesExcludes` wrapper
- On-demand namespace resolution with thread-safe caching
- First call triggers Kubernetes API namespace enumeration and wildcard resolution
- Results cached for subsequent calls within the same backup request

## Detailed Design

### Polymorphic Interface Approach

The key insight is that all existing backup code already calls the same 4 methods on namespace filtering:
- `ShouldInclude(namespace string) bool` - Core filtering logic
- `IncludesString() string` - Logging display
- `ExcludesString() string` - Logging display  
- `IncludeEverything() bool` - Optimization checks

By creating a `NamespaceIncludesExcludesInterface` with these methods, we can:
1. **Standard case**: Use existing `*IncludesExcludes` (no wildcards)
2. **Wildcard case**: Use new `*LazyNamespaceIncludesExcludes` (with K8s API enumeration)

**No calling code changes needed** - the interface abstraction handles everything.

**Cache Scope:** Single backup request only - automatic cleanup when request completes.

### Implementation Strategy

**Location:** `pkg/util/collections/includes_excludes.go`
- New interface defining the 4 required methods
- `LazyNamespaceIncludesExcludes` struct embedding `*IncludesExcludes` for fallback
- Lazy resolution with thread-safe caching using mutex
- Special case handling for lone `*` to preserve existing efficient behavior

**Integration:** `pkg/backup/backup.go`  
- Wildcard detection logic determines which implementation to return
- Lone `*` pattern → standard `IncludesExcludes` (preserve current behavior)
- Any other wildcards → lazy `LazyNamespaceIncludesExcludes`

**Type Updates:** Change struct fields from concrete `*IncludesExcludes` to interface type
- `pkg/backup/request.go` - Request struct field type
- `pkg/backup/item_collector.go` - Function parameter types

### Performance Characteristics
- **First `ShouldInclude()` call:** ~500ms (K8s API namespace enumeration + wildcard resolution)
- **Subsequent calls:** ~1ms (cached lookup with read lock)
- **Memory overhead:** Minimal (resolved namespace list stored once per backup request)
- **Concurrency:** Full concurrent read access to cached results

## Alternatives Considered

### CLI-Level Resolution
**Problem:** Resolving wildcards during `velero backup create` command

**Why rejected:**
- **Lost User Intent:** Backup specs store resolved lists instead of original patterns
- **Audit Trail Issues:** Original wildcard intent not visible when examining backup specifications
- **CLI Complexity:** CLI requires cluster access and namespace enumeration capabilities

### Server-Level (Controller) Resolution  
**Problem:** Resolving wildcards in backup controller with persistent caching

**Why rejected:**
- **Architectural Complexity:** Requires additional API schema changes for storing resolved namespace lists
- **Cache Management:** Need cache invalidation, storage, and lifecycle management
- **Limited Benefit:** Performance gain only applies to narrow controller reconciliation retry scenarios
- **State Management:** Introduces persistent state maintained across backup lifecycle

### Request-Level (ShouldInclude) Resolution
**Chosen Approach:** Lazy evaluation within backup request processing

**Benefits:**
- **Preserved Intent:** Original wildcard patterns remain in backup specifications
- **Optimal Performance:** First resolution (~500ms), subsequent calls (~1ms) with request-scoped caching
- **Clean Architecture:** No persistent state, no API schema changes, minimal code changes
- **Thread Safety:** Proper mutex usage for concurrent worker access
- **Scoped Lifetime:** Cache automatically cleaned up when backup request completes

## Security Considerations
- Implementation requires Velero service account to have `list` permissions on namespace resources
- Aligns with existing Velero RBAC requirements
- No additional privileges or security surface area introduced

## Addressing Implementation Concerns

### Multiple Pattern Support
Multiple wildcards work naturally: `--include-namespaces "prod-*,staging-*,dev-*"` - each pattern evaluated independently during lazy resolution.

### Mixed Literal and Wildcard Detection
Simple approach: strings containing `*` are wildcards, others use existing literal namespace logic. Zero breaking changes for existing validation.

### Include/Exclude Conflict Detection
Runtime resolution simplifies conflicts - wildcards resolve to actual namespace lists first, then standard include/exclude precedence applies.

### Backward Compatibility
Lazy evaluation triggers only when wildcards detected. Non-wildcard backups have zero overhead and identical behavior to current implementation.

## Special Consideration: Existing `*` Behavior

**Current Velero Behavior:** `--include-namespaces "*"` (the CLI default) means "include all namespaces" and uses special logic that doesn't enumerate namespaces - it simply bypasses namespace filtering entirely.

**Potential Breaking Change:** Our wildcard implementation would treat `*` as a glob pattern, resolving it to a specific list of namespaces at backup start time, which changes the behavior from "include everything" to "include these specific namespaces."

**Required Solution:** Special-case handling for the lone `*` pattern to preserve existing behavior by using original `IncludesExcludes` logic instead of wildcard resolution.

This ensures that `--include-namespaces "*"` continues to work exactly as before, while enabling new wildcard patterns like `prod-*`, `*-staging`, etc.

## Compatibility
- Full backward compatibility with existing backup specifications using literal namespace lists
- No changes required to CLI commands, existing backups, or restore operations

## Implementation
The implementation consists of approximately 200 lines of new code across four files:
- `pkg/util/collections/includes_excludes.go`: Core lazy evaluation logic (~150 lines)
- `pkg/backup/backup.go`: Wildcard detection logic (~20 lines)  
- `pkg/backup/request.go`: Interface type usage (~5 lines)
- `pkg/backup/item_collector.go`: Compatibility method calls (~25 lines)

## Open Issues
None. 