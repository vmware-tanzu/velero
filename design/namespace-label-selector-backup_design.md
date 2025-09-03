# Design proposal: Namespace Selection by Label Selector for Velero Backups

## Abstract

This proposal modifies Velero's backup behavior to treat namespaces selected by `LabelSelector` or `OrLabelSelectors` as if they were explicitly listed in `includedNamespaces`, causing all resources within those namespaces to be included in the backup.
Currently, when a namespace matches a label selector, only the namespace object itself and individually labeled resources are backed up, which is not the expected behavior for most users who want complete namespace backups.

## Background

Users frequently need to backup entire namespaces dynamically based on labels rather than maintaining static lists in `includedNamespaces`.
The current `LabelSelector` behavior only includes resources that individually match the selector, making it difficult to achieve complete namespace backups through labeling.
This creates operational overhead where users must either manually maintain namespace lists or label every resource within target namespaces.

Issue #7492 requests this functionality with strong community support (+5 reactions) and multiple real-world use cases from users managing multi-cluster environments where namespace lists vary dynamically.

## Goals

- Enable complete namespace backup through namespace label selection
- Provide clear precedence rules when combining inclusion/exclusion methods
- Support full Kubernetes label selector syntax including complex operators

## Non Goals

- Changing behavior of resource-level label selectors within namespaces (resources still respect individual selectors within selected namespaces)
- Adding new API fields (we will modify existing `LabelSelector` behavior)
- Supporting per-resource granular control within label-selected namespaces

## High-Level Design

When a namespace matches `spec.labelSelector` or any `spec.orLabelSelectors`, Velero will treat that namespace as if it were explicitly listed in `spec.includedNamespaces`.
All resources within matching namespaces will be included in the backup, regardless of their individual labels.
The final namespace selection follows the precedence: `(includedNamespaces ∪ labelSelector_matches ∪ orLabelSelector_matches) - excludedNamespaces`.

## Detailed Design

### Namespace Selection Logic

The namespace selection algorithm follows this formula:

```text
Final Namespaces = (ExplicitIncludes ∪ LabelSelectorMatches ∪ OrLabelSelectorMatches) - ExplicitExcludes
```

Where:

- `ExplicitIncludes`: Namespaces in `spec.includedNamespaces`
- `LabelSelectorMatches`: Namespaces matching `spec.labelSelector`
- `OrLabelSelectorMatches`: Namespaces matching any selector in `spec.orLabelSelectors[]`
- `ExplicitExcludes`: Namespaces in `spec.excludedNamespaces` (highest precedence)

### Implementation Changes

#### Core Logic Modification

The primary change occurs in `pkg/backup/item_collector.go` in the `nsTracker.init()` method (lines 102-166).
Currently, this method tracks namespaces for filtering but doesn't change the fundamental inclusion behavior.

**Current behavior:**
```go
// Namespace matches selector -> track for resource filtering only
if nt.singleLabelSelector != nil && nt.singleLabelSelector.Matches(labels.Set(namespace.GetLabels())) {
    nt.track(namespace.GetName())
}
```

**New behavior:**
```go
// Namespace matches selector -> track AND include all resources in namespace
if nt.singleLabelSelector != nil && nt.singleLabelSelector.Matches(labels.Set(namespace.GetLabels())) {
    nt.track(namespace.GetName())
    // This namespace now behaves like it's in includedNamespaces
}
```

#### Label Selector Support

Full Kubernetes label selector support including:

**Equality-based requirements:**
- `key=value` (equality)
- `key!=value` (inequality)

**Set-based requirements:**
- `key in (value1,value2)` (set inclusion)
- `key notin (value1,value2)` (set exclusion)
- `key` (key exists)
- `!key` (key does not exist)

**MatchExpressions support:**
```yaml
labelSelector:
  matchLabels:
    environment: production
  matchExpressions:
  - key: tier
    operator: In
    values: ["web", "api"]
  - key: deprecated
    operator: DoesNotExist
```

### API Schema (No Changes Required)

The existing `BackupSpec` already supports the required fields:
```go
type BackupSpec struct {
    // ... existing fields ...
    LabelSelector    *metav1.LabelSelector   `json:"labelSelector,omitempty"`
    OrLabelSelectors []*metav1.LabelSelector `json:"orLabelSelectors,omitempty"`
}
```

### Example Usage Scenarios

#### Scenario 1: Basic Environment-Based Backup
```yaml
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: production-backup
spec:
  labelSelector:
    matchLabels:
      environment: production
```
Result: All namespaces with `environment=production` are completely backed up.

#### Scenario 2: Complex Selection with Exclusions
```yaml
spec:
  includedNamespaces: ["critical-ns"]
  labelSelector:
    matchExpressions:
    - key: backup-tier
      operator: In
      values: ["daily", "weekly"]
    - key: deprecated
      operator: DoesNotExist
  excludedNamespaces: ["temp-ns"]
```
Result: `critical-ns` + (namespaces with `backup-tier` in daily/weekly AND no `deprecated` label) - `temp-ns`.

#### Scenario 3: OrLabelSelectors Usage
```yaml
spec:
  orLabelSelectors:
  - matchLabels:
      backup: "true"
  - matchLabels:
      critical: "true"
      environment: "production"
```
Result: Namespaces with `backup=true` OR (`critical=true` AND `environment=production`).

### Logging and Observability

Enhanced logging will provide clear visibility into namespace selection:

```
INFO Backup namespace selection:
  Explicitly included: [ns1, ns2]
  Selected by labelSelector 'environment=production': [ns3, ns4]
  Selected by orLabelSelectors: [ns5, ns6]
  Explicitly excluded: [ns2, temp-ns]
  Final included namespaces: [ns1, ns3, ns4, ns5, ns6]
```

Backup status will include selected namespace information:
```yaml
status:
  phase: Completed
  namespaces:
    included: ["ns1", "ns3", "ns4", "ns5", "ns6"]
    explicitlyIncluded: ["ns1"]
    selectedByLabels: ["ns3", "ns4", "ns5", "ns6"]
    excluded: ["ns2", "temp-ns"]
```

### Validation and Warnings

Warning messages for potentially confusing configurations:
- "Namespace 'ns2' is both explicitly included and excluded - exclusion takes precedence"
- "Using both labelSelector and orLabelSelectors - both will be evaluated"
- "Complex label selectors may select unexpected namespaces - verify selection before running"

### Breaking Change Considerations

This is a **breaking change** from current behavior:
- Existing backups using `labelSelector` will include more resources
- Backup size and duration may increase
- Current behavior has limited practical value, reducing migration impact

Migration considerations:
- Document change prominently in release notes
- Provide before/after examples
- Consider deprecation warning in prior version

## Alternatives Considered

### Alternative 1: New NamespaceSelector Field
```go
type BackupSpec struct {
    NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}
```
**Pros:** No breaking changes, clear separation of concerns
**Cons:** Adds API complexity, community feedback against new fields

### Alternative 2: Boolean Flag to Change Behavior
```go
type BackupSpec struct {
    IncludeResourcesInLabeledNamespaces *bool `json:"includeResourcesInLabeledNamespaces,omitempty"`
}
```
**Pros:** Backward compatible, explicit opt-in
**Cons:** Complex configuration, confusing API surface

### Alternative 3: Separate Namespace and Resource Selectors
Keep existing `labelSelector` for resources, add `namespaceSelector` for namespaces.
**Cons:** Most complex option, unclear interaction patterns

**Decision:** Modify existing behavior (chosen approach) based on community feedback and limited utility of current behavior.

## Security Considerations

### RBAC Implications
Users with permission to modify backup specs can now potentially backup additional namespaces through label manipulation.
Existing RBAC controls on backup resources remain the primary security boundary.

### Label-based Access Control
Organizations using label-based policies should ensure backup service accounts have appropriate namespace and resource access.
No additional privileges are required beyond current backup operations.

### Sensitive Data Exposure
The expanded scope of backups may include additional sensitive data.
Users should audit namespace labels and exclusion rules to prevent unintended data exposure.

## Compatibility

### Backward Compatibility
**Breaking Change:** Existing backups using `labelSelector` will behave differently.
Impact assessment:
- Current `labelSelector` usage is limited due to reduced utility
- Most users already avoid this pattern in favor of explicit namespace listing
- Change aligns behavior with user expectations

### API Compatibility
No API schema changes required.
Existing backup configurations remain syntactically valid but may produce different results.

### Velero Version Compatibility
- Backups created with new behavior remain compatible with older Velero versions for restore
- Backup metadata format unchanged
- No changes to backup storage format

### Kubernetes Version Compatibility
No changes to Kubernetes version requirements.
Label selector functionality leverages existing Kubernetes APIs.

## Implementation

### Development Phases

**Phase 1: Core Logic Implementation**
- Modify `nsTracker` behavior in `item_collector.go`
- Update namespace selection algorithm
- Add comprehensive unit tests
- Timeline: 2 weeks

**Phase 2: Full Kubernetes Label Selector Support**
- Implement support for all operator types (`In`, `NotIn`, `Exists`, `DoesNotExist`)
- Add validation for complex selectors
- Timeline: 1 week

**Phase 3: Enhanced Logging and Observability**
- Add detailed namespace selection logging
- Update backup status reporting
- Add validation warnings
- Timeline: 1 week

**Phase 4: Documentation and Testing**
- Update user documentation with examples
- Add integration and E2E tests
- Create migration guide
- Timeline: 2 weeks

### Testing Strategy

**Unit Tests:**
- Namespace selection algorithm with all operator combinations
- Precedence rules with various include/exclude patterns
- Edge cases (empty selectors, wildcard usage, non-existent namespaces)

**Integration Tests:**
- Backup creation with namespace selectors
- Verification of complete namespace resource inclusion
- Backup status and logging validation

**E2E Tests:**
- Multi-namespace backup/restore scenarios
- Cross-cluster restore operations
- Performance testing with large namespace counts

### Resources

- **Primary Developer:** Tiger Kaovilai (@kaovilai)
- **Reviewers:** Daniel Jiang, blackpiglet, shubham-pampattiwar
- **Target Release:** Velero v1.18
- **Estimated Effort:** 6 weeks total development + testing

## Open Issues

### Issue 1: Performance Impact with Large Cluster
Large clusters with many namespaces may experience performance degradation during label selector evaluation.
**Potential Solution:** Implement caching for namespace label queries, optimize selector evaluation.

### Issue 2: Label Change Detection
Currently, Velero doesn't re-evaluate namespace selection if labels change between backup creation and execution.
**Potential Solution:** Document current behavior, consider future enhancement for dynamic re-evaluation.

### Issue 3: Complex Selector User Education
Users may create overly complex selectors that are difficult to understand or maintain.
**Potential Solution:** Provide comprehensive documentation, examples, and possibly a selector validation tool.

### Issue 4: Interaction with Other Velero Features
Need to verify behavior interaction with:
- Resource policies
- Item actions and plugins
- Hooks and pre/post backup operations
**Status:** Requires additional analysis and testing during implementation.