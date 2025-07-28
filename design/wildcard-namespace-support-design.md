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

## Namespace Discovery Timing and Behavior

### Snapshot Timing
**Wildcard patterns are resolved at backup start time** and remain fixed for the entire backup duration. This provides:
- **Consistent behavior**: All resources in a backup come from the same namespace set
- **Predictable results**: Backup contents don't change mid-execution
- **Performance**: No repeated namespace enumeration during backup processing

### Runtime Namespace Changes
When namespaces are created or deleted during backup execution:

**Newly Created Namespaces:**
- If `prod-new` is created after backup starts, it will **NOT** be included even if it matches `prod-*`
- The resolved namespace list is fixed at backup start time

**Deleted Namespaces:**
- If a namespace matching the pattern is deleted during backup, the backup continues
- Resources already processed from that namespace remain in the backup
- Subsequent resource enumeration for that namespace may result in "not found" errors (handled gracefully)
  - This should ideally fail so  that the user can re-run it without a namespace being deleted while a backup is started which is rare.

**User Expectations:**
This behavior should be explicitly documented with examples:
```
# At backup start: namespaces [prod-app, prod-db] exist
velero backup create --include-namespaces "prod-*"

# During backup: prod-cache namespace is created
# Result: prod-cache is NOT included in this backup
# Recommendation: Run another backup to capture newly created namespaces
```

## Pattern Complexity and Validation

### Supported Patterns
**Basic Wildcard Support (`*` only):**
- `prefix-*` - Matches namespaces starting with "prefix-"
- `*-suffix` - Matches namespaces ending with "-suffix"
- `*-middle-*` - Matches namespaces containing "-middle-"
- `*` - Special case: matches all namespaces (preserves current behavior)

### Unsupported Patterns
**Not supported in initial implementation:**
- `?` for single character matching (e.g., `prod-?-app`)
- Character classes (e.g., `prod-[abc]-app`)
- Regex patterns (e.g., `prod-\d+-app`)

### Pattern Validation
**Creation-time validation:**
- Invalid patterns containing unsupported characters will be rejected at backup creation
- Validation occurs in CLI and API server admission controller
- Clear error messages guide users to supported patterns

**Example validation errors:**
```bash
# Unsupported pattern
velero backup create --include-namespaces "prod-?-app"
# Error: Pattern 'prod-?-app' contains unsupported character '?'. Only '*' wildcards are supported.

# Valid patterns
velero backup create --include-namespaces "prod-*,*-staging"
# Success: Patterns validated successfully
```

## Error Handling

### Kubernetes API Failures
**Namespace enumeration failures:**
- If initial namespace list API call fails → backup fails with clear error message
- Transient failures are retried using standard Kubernetes client retry logic
- No fallback to cached/partial data to ensure consistent behavior

**Error response example:**
```
Error: Failed to enumerate namespaces for wildcard resolution: unable to connect to Kubernetes API
Backup creation aborted. Please verify cluster connectivity and try again.
```

### Zero Namespace Matches
**When wildcard patterns match no namespaces:**
- **Behavior**: Warning logged, backup proceeds with empty namespace set
- **User notification**: Warning in backup status and logs
- **Rationale**: Allows for valid scenarios (e.g., temporary namespace absence)

**Warning example:**
```
Warning: Wildcard pattern 'prod-*' matched 0 namespaces. Backup will include no namespaces from this pattern.
```

### Dry-Run Support
**Preview functionality:**
```bash
# New flag to preview wildcard resolution
velero backup create my-backup --include-namespaces "prod-*" --dry-run=wildcards

# Output:
Wildcard pattern 'prod-*' would include namespaces: [prod-app, prod-db, prod-cache]
Wildcard pattern '*-staging' would include namespaces: [app-staging, db-staging]
Total namespaces: 5
```

## Restore Operations

### Wildcard Behavior During Restore
**Restore uses namespaces captured at backup time:**
- Wildcard patterns in backup specs are **not** re-evaluated during restore
- Restore operates on the concrete namespace list that was resolved during backup
- This ensures restore consistency even if cluster namespace state has changed

**Implementation approach:**
1. **Backup metadata storage**: Store both original patterns and resolved namespace lists
2. **Restore processing**: Use resolved namespace lists, ignore original patterns
3. **Audit trail**: Both patterns and resolved lists visible in backup metadata

**Example scenario:**
```yaml
# Original backup spec
includedNamespaces: ["prod-*"]

# Stored in backup metadata
resolvedNamespaces: ["prod-app", "prod-db"] 
originalPatterns: ["prod-*"]

# During restore (even if prod-cache now exists)
# Only prod-app and prod-db are restored
```

### Disaster Recovery Scenarios
**Cross-cluster restore behavior:**
- Restore attempts to create resources in target namespaces
- If target namespaces don't exist, Velero creates them (existing behavior)
- Wildcard patterns are not re-evaluated against target cluster

## Scheduled Backups

### Namespace State Changes Between Runs
**Each scheduled backup run performs fresh wildcard resolution:**
- Pattern `prod-*` may include different namespaces in each backup run
- This allows scheduled backups to automatically capture newly created namespaces
- **Trade-off**: Backup contents may vary between runs vs. automatic inclusion of new resources

**Storage implications:**
- Varying namespace sets between runs may affect deduplication efficiency
- Each backup stores its own resolved namespace list independently

**Example behavior:**
```
# Monday backup: prod-* matches [prod-app, prod-db]
# Tuesday: prod-cache namespace created
# Tuesday backup: prod-* matches [prod-app, prod-db, prod-cache]
```

**User expectations:**
- Document that scheduled backups automatically include newly matching namespaces
- Provide guidance on namespace naming conventions for predictable backup behavior

## Testing Strategy

### Unit Tests
**Pattern matching tests:**
```go
func TestWildcardPatterns(t *testing.T) {
    tests := []struct {
        pattern   string
        namespace string
        expected  bool
    }{
        {"prod-*", "prod-app", true},
        {"prod-*", "staging-app", false},
        {"*-staging", "app-staging", true},
        {"*-test-*", "app-test-db", true},
    }
    // ... test implementation
}
```

**Edge cases:**
- Empty pattern list
- Pattern with no matches
- Pattern matching single namespace
- Multiple overlapping patterns
- Special case lone `*` behavior

### Integration Tests
**Kubernetes cluster scenarios:**
- Create namespaces, verify wildcard resolution
- Test namespace creation/deletion during backup
- Verify thread safety with concurrent backup operations
- Error scenarios (API failures, network issues)

**Concurrency testing:**
- Multiple concurrent `ShouldInclude()` calls
- Thread safety verification
- Cache hit ratio measurement

## Example Usage

### CLI Usage
```bash
# Single wildcard pattern
velero backup create prod-backup --include-namespaces "prod-*"

# Multiple patterns
velero backup create env-backup --include-namespaces "prod-*,staging-*,dev-*"

# Mixed literal and wildcard
velero backup create mixed-backup --include-namespaces "prod-*,kube-system,monitoring"

# Exclude patterns
velero backup create no-test --include-namespaces "*" --exclude-namespaces "*-test,*-temp"

# Preview before creating
velero backup create my-backup --include-namespaces "prod-*" --dry-run=wildcards
```

### Backup Specification YAML
```yaml
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: production-backup
  namespace: velero
spec:
  # Wildcard patterns in includedNamespaces
  includedNamespaces:
  - "prod-*"           # All namespaces starting with "prod-"
  - "production-*"     # All namespaces starting with "production-"
  - "critical-app"     # Literal namespace (mixed with wildcards)
  
  # Wildcard patterns in excludedNamespaces  
  excludedNamespaces:
  - "*-test"           # Exclude any test namespaces
  - "*-temp"           # Exclude any temporary namespaces
  
  # Other backup configuration
  storageLocation: default
  volumeSnapshotLocations:
  - default
  includeClusterResources: false
```

### Stored Backup Metadata
```yaml
# What gets stored in backup metadata
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: production-backup
status:
  # Original user patterns preserved for audit
  originalIncludePatterns: ["prod-*", "production-*", "critical-app"]
  originalExcludePatterns: ["*-test", "*-temp"]
  
  # Resolved concrete namespace lists (used for restore)
  resolvedIncludedNamespaces: ["prod-app", "prod-db", "production-web", "critical-app"]
  resolvedExcludedNamespaces: ["prod-app-test", "staging-temp"]
  
  # Resolution timestamp
  namespaceResolutionTime: "2024-01-15T10:30:00Z"
```

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