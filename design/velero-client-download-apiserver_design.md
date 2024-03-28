# Velero Endpoint Download Client

## Abstract

<!-- One to two sentences that describes the goal of this proposal and the problem being solved by the proposed change.
The reader should be able to tell by the title, and the opening paragraph, if this document is relevant to them. -->
Velero data is stored in an object store.
When CLI users want to download data from the object store, it connects to the specific object store directly to download data.
This assumes the target storing the data is always accessible from the client side. This is not always true, especially in the on premise environments.
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
- Enable Velero CLI users to download data from the Velero server without having to connect to the object store directly for scenarios where the object store is not accessible from the client side.

## Non Goals
- Making the download server the default way to download data from object store.

## High-Level Design
Create an endpoint on the Velero server that CLI users can connect to and download data from without having to connect to the object store directly.

### Potential approaches:
One will be chosen before design is merged. The other will be moved to Alternatives Considered.

#### Approach 1: [configure a Kubernetes API Aggregation Layer](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/)
We will [configure a Kubernetes API Aggregation Layer](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/) to communicate with a new Velero extension apiserver in order to authenticate and authorize requests to download data from the Velero server.

Velero extension apiserver will be a new component that will be deployed as part of the Velero server.
It will be a thin layer that will authenticate and authorize requests to download data from the Velero server.
It will also be responsible for downloading data from the object store and streaming it back to the client.

The Velero CLI will be updated to connect to the Velero extension apiserver to download data from the Velero server.

Downside: if an aggregate API server 404 or unresponsive, [namespace could be stuck in deletion](https://github.com/kubernetes/kubernetes/issues/119662#issuecomment-1863523115)

#### Approach 2: Adding Ingress to Velero

Ingresses will be added per BSL to a Velero server that will accept requests to download data from those storage locations.

Creating Ingress per BSL creation uncouple velero helm chart from defining ingress which should allow for multiple velero instances support.

The current DownloadRequest CR status `downloadURL` will be updated to use the Ingress URL instead of the object store URL where available.

Velero will be updated to download data from the object store and stream it back to the client.

The Velero CLI will be updated to connect to the Ingress URL to download data serving from the Velero server.

#### Approach 3: Using Service LoadBalancer to Expose Download Server

Upon enabling download server api, velero will create a service of type loadBalancer, which will expose the download server to the outside world. The download server will be able to download data from the object store and stream it back to the client using the loadBalancer IP dynamically assigned by the cloud provider.

This approach will also [work on KinD clusters](https://kind.sigs.k8s.io/docs/user/loadbalancer/)

#### Comparison
| Wanted Features | 1. API Aggregation Layer | 2. Ingress | 3. Service LoadBalancer |
| --- | --- | --- | --- |
| Support Multiple Velero in one cluster | ✅ | ✅ | ✅ |
| Support multiple storage locations | ✅ | ✅ | ✅ |
| K8S Style API | ✅ | ❌ | ❌ |
| Authentication Built-in| ✅ | ❌ has to parse [TokenReview](https://dev-k8sref-io.web.app/docs/authentication/tokenreview-v1/) and [SubjectAccessReview](https://dev-k8sref-io.web.app/docs/authorization/subjectaccessreview-v1/)| ❌ has to parse [TokenReview](https://dev-k8sref-io.web.app/docs/authentication/tokenreview-v1/) and [SubjectAccessReview](https://dev-k8sref-io.web.app/docs/authorization/subjectaccessreview-v1/)|
| Client works without resolvable DNS/IP if kubectl works | ✅ | ❌ could avoid with `kubectl port-forward`-like approach?| ❌ |
| URL is unique per velero instance | ✅ | ? | ✅ each velero namespace will have its own external ip from cloud provider |
| Does not require ingress controller | ✅ | ❌ | ✅ |
| Does not require NodePort, ClusterIP, LoadBalancer | ✅ | ❌ | ❌ |
| Does not require apiserver [specific flags](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/#enable-kubernetes-apiserver-flags) | ❌ | ✅ | ✅ |
| Does not [block namespace deletion](https://github.com/kubernetes/kubernetes/issues/119662#issuecomment-1863523115) | ❌ | ✅ | ✅ |

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
