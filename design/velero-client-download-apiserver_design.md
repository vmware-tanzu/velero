# Velero Endpoint Download Client

## Abstract

<!-- One to two sentences that describes the goal of this proposal and the problem being solved by the proposed change.
The reader should be able to tell by the title, and the opening paragraph, if this document is relevant to them. -->
Velero data is stored in an object store.
When CLI users want to download data from the object store, it connects to the specific object store directly to download data.
This assumes the target storing the data is always accessible from the client side. This is not always true, especially in the on-premise environments.
Velero may also save the data in non object stores in the future so an object store may not be available for the CLI to download from.

This design proposes to add a new endpoint on the Velero server that CLI users can connect to and download data from without having to connect to an object store directly.

## Background
Velero CLI commands such as following download data from the object store directly today.
```
velero backup download
velero describe
velero backup logs
...
```
We want them to work even when the object store is not accessible from the client side.
Even if the object store is accessible from the client side, today things like cacert and insecure-skip-tls-verify flags are specified manually from the client even though the Velero server already has the information. This work eliminates the client from having to know about the object store configuration and certificates.
## Goals
- A short list of things which will be accomplished by implementing this proposal.
- Two things is ok.
- Three is pushing it.
- More than three goals suggests that the proposal's scope is too large.

## Non Goals
- A short list of items which are:
- a. out of scope
- b. follow on items which are deliberately excluded from this proposal.


## High-Level Design
One to two paragraphs that describe the high level changes that will be made to implement this proposal.

We will [configure a Kubernetes API Aggregation Layer](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/) to communicate with a new Velero extension apiserver in order to authenticate and authorize requests to download data from the Velero server.

Velero extension apiserver will be a new component that will be deployed as part of the Velero server.
It will be a thin layer that will authenticate and authorize requests to download data from the Velero server.
It will also be responsible for downloading data from the object store and streaming it back to the client.

The Velero CLI will be updated to connect to the Velero extension apiserver to download data from the Velero server.

## Detailed Design
<!-- A detailed design describing how the changes to the product should be made.

The names of types, fields, interfaces, and methods should be agreed on here, not debated in code review.
The same applies to changes in CRDs, YAML examples, and so on.

Ideally the changes should be made in sequence so that the work required to implement this design can be done incrementally, possibly in parallel. -->
TBD

## Alternatives Considered
<!-- If there are alternative high level or detailed designs that were not pursued they should be called out here with a brief explanation of why they were not pursued. -->
TBD

## Security Considerations
<!-- If this proposal has an impact to the security of the product, its users, or data stored or transmitted via the product, they must be addressed here. -->
Since this design may enable self service non-admin users to download data from the Velero server in the future, we need to make sure that only authorized user for a specific backup/restore/storage location can download those data from the Velero server.

There will need to be tests to make sure that only authorized users can download data from the Velero server.

## Compatibility
A discussion of any compatibility issues that need to be considered

## Implementation
A description of the implementation, timelines, and any resources that have agreed to contribute.

## Open Issues
A discussion of issues relating to this proposal for which the author does not know the solution. This section may be omitted if there are none.
