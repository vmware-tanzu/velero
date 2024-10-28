# Kopia Backup to Local Volume Provider (PVC/NFS)

_Note_: The preferred style for design documents is one sentence per line.
*Do not wrap lines*.
This aids in review of the document as changes to a line are not obscured by the reflowing those changes caused and has a side effect of avoiding debate about one or two space after a period.

_Note_: The name of the file should follow the name pattern `<short meaningful words joined by '-'>_design.md`, e.g:
`listener-design.md`.

## Abstract
Backups involving kopia are currently stored to blob storage only, this proposal aims to add support for storing backups to local volumes (PVC/NFS).

## Background
Users have asked for the ability to store backups to local volumes (PVC/NFS) for various reasons including:
- Compliance requirements
- Cost
  - Existing infrastructure
- Performance
- Flexibility

## Goals
<!-- - A short list of things which will be accomplished by implementing this proposal.
- Two things is ok.
- Three is pushing it.
- More than three goals suggests that the proposal's scope is too large. -->

## Non Goals
<!-- - A short list of items which are:
- a. out of scope
- b. follow on items which are deliberately excluded from this proposal. -->


## High-Level Design
if backup storage location is a local volume (PVC/NFS) then:

Add a new backend type `PVCBackend` to `pkg/repository/config/config.go`

```go
const (
	AWSBackend   BackendType = "velero.io/aws"
	AzureBackend BackendType = "velero.io/azure"
	GCPBackend   BackendType = "velero.io/gcp"
	FSBackend    BackendType = "velero.io/fs"
	PVCBackend BackendType = "replicated.com/pvc"
)
```

This plugin will be responsible for storing backups to a local volume PVC which can be NFS backed.

When `PVCBackend` is configured, velero will use kopia to store backups to the local volume instead of blob storage.

## Detailed Design
A detailed design describing how the changes to the product should be made.

The names of types, fields, interfaces, and methods should be agreed on here, not debated in code review.
The same applies to changes in CRDs, YAML examples, and so on.

Ideally the changes should be made in sequence so that the work required to implement this design can be done incrementally, possibly in parallel.

## Alternatives Considered
If there are alternative high level or detailed designs that were not pursued they should be called out here with a brief explanation of why they were not pursued.

## Security Considerations
If this proposal has an impact to the security of the product, its users, or data stored or transmitted via the product, they must be addressed here.

## Compatibility
A discussion of any compatibility issues that need to be considered

## Implementation
A description of the implementation, timelines, and any resources that have agreed to contribute.

## Open Issues
<!-- A discussion of issues relating to this proposal for which the author does not know the solution. This section may be omitted if there are none. -->

Velero Logs, Velero Downloads will still not work and will be solved in [Download server for Velero client #6167
](https://github.com/vmware-tanzu/velero/issues/6167)