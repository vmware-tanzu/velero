
# Wildcard namespace includes/excludes support for backups and restores

## Abstract
One to two sentences that describes the goal of this proposal and the problem being solved by the proposed change.
The reader should be able to tell by the title, and the opening paragraph, if this document is relevant to them.

Velero currently does not have any support for wildcard characters in the namespace spec. 
It fully expects the namespaces to be string literals.

The only and notable exception is the "*" character by it's lonesome, which acts as an include all and ignore excludes option.
Internally Velero treats not specifying anything as the "*" case.

This document details the approach to implementing wildcard namespaces, while keeping the "*" characters purpose intact for legacy purposes.

## Background
This was raised in Issue [#1874](https://github.com/vmware-tanzu/velero/issues/1874)


## Goals
- A short list of things which will be accomplished by implementing this proposal.
- Two things is ok.
- Three is pushing it.
- More than three goals suggests that the proposal's scope is too large.

- Add support for wildcard namespaces in --include-namespaces and --exclude-namespaces
- Ensure legacy "*" support is not affected

## Non Goals
- A short list of items which are:
- a. out of scope
- b. follow on items which are deliberately excluded from this proposal.

- Completely rethinking the way "*" is treated and allowing it to work with wildcard excludes.


## High-Level Design
One to two paragraphs that describe the high level changes that will be made to implement this proposal.

Points of interest are two funcs within the utility layer, in file `velero/pkg/backup/item_collector.go`
- [collectNamespaces](https://github.com/vmware-tanzu/velero/blob/1535afb45e33a3d3820088e4189800a21ba55293/pkg/backup/item_collector.go#L742)
- [getNamespacesToList](https://github.com/vmware-tanzu/velero/blob/1535afb45e33a3d3820088e4189800a21ba55293/pkg/backup/item_collector.go#L638)

collectNamespaces gets all the active namespaces and matches it against the user spec for included namespaces (r.backupRequest.Backup.Spec.IncludedNamespaces)
This is an ideal point where wildcard expansion can take place.
The implementation would mean that just like "*", namespaces with wildcard symbols would also be passed through without an existence check.
The resolved namespaces are stored in new status fields on the backup.

## Detailed Design
A detailed design describing how the changes to the product should be made.

The names of types, fields, interfaces, and methods should be agreed on here, not debated in code review.
The same applies to changes in CRDs, YAML examples, and so on.

Ideally the changes should be made in sequence so that the work required to implement this design can be done incrementally, possibly in parallel.

1. Add new status fields to the backup CRD to store expanded wildcard namespaces
```

```
2. Create a util package for wildcard expansion

3. If required, expand wildcards and replace the request's includes and excludes with expanded namespaces
4. Populate the expanded namespace status field with the namespaces.

## Alternatives Considered
If there are alternative high level or detailed designs that were not pursued they should be called out here with a brief explanation of why they were not pursued.

## Security Considerations
If this proposal has an impact to the security of the product, its users, or data stored or transmitted via the product, they must be addressed here.

## Compatibility
A discussion of any compatibility issues that need to be considered

## Implementation
A description of the implementation, timelines, and any resources that have agreed to contribute.

## Open Issues
A discussion of issues relating to this proposal for which the author does not know the solution. This section may be omitted if there are none.
