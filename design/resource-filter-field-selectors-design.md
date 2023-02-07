# Extend Resource Policy To Support Field Selectors

## Abstract

This design proposal extends the [`ResourcePolicies` API][0] to support resource filtering using [field selectors][1].
The proposed field selection design complements the existing [filtering API][2], to enable users to filter backup resources by Kubernetes resource fields.

## Background

The current mechanism to include cluster-scoped resources in backups via the `IncludeResources`, `IncludeClusterResources` and `IncludeClusterScopedResources` API filters the resources by their [`ResourceGroup`][3].
This is insufficent for operator backup use cases, where specific Custom Resource Definition (CRD) must be included in order for the restored operator to work.
See issue [#4876][4].

The existing [label selection][5] mechanism doesn't scale on large Kubernetes clusters, due to constraints beyond the users' control like:

* Inconsistent resource labeling scheme across different providers
* Unique labeling to correctly group CRDs according to operators
* Strict cluster-scoped RBAC permissions (to label CRDs)
* Dynamically generated custom resource definitions

## Goals

- Users can define backup filter criteria based on Kubernetes field selectors.

## Non Goals

- N/A

## High-Level Design

Extends the `ResourcePolicies` API with a `FieldSelectorPolicies` configuration.

## Detailed Design

The `FieldSelectorPolicies` works in complement to the `IncludeClusterScopedResources` API.
For example, to include the Nginx ingress controller CRD, one can define a field selector policy that looks like this:

```sh
cat <<EOF > backup-policy-nginx-crd.yaml
version: v1
fieldSelectorPolicies:
- groupResource: 
    group: apiextensions.k8s.io
    resource: customresourcedefinitions
  metadata.name: virtualservers.k8s.nginx.org
  metadata.name: virtualserverroutes.k8s.nginx.org
  metadata.name: transportservers.k8s.nginx.org
  metadata.name: policies.k8s.nginx.org
EOF
```

This YAML policy is used to create the policy configmap used by Velero to backup the CRDs:

```sh
kubectl -n velero create cm backup-policy-nginx-crd --from-file backup-policy-nginx-crd.yaml

velero backup create --include-cluster-scoped-resources="customresourcedefinitions" --resource-policies-configmap=backup-policy-nginx-crd
```

Although the supported field selectors vary across Kubernetes resource types, all resource types support the `metadata.name` and `metadata.namespace` fields.
Using unsupported field selectors produces an error.
Only the `=` operator is supported by this proposed API.
For more information, see the Kubernetes documentation on [field selectors][1].

### API Design

The following code snippet illustrates the proposed API structure to encapsulate the field selector properties:

```go
type ResourcePolicies struct {
  Version        string                          `yaml:"version"`
  VolumePolicies []VolumePolicy                  `yaml:"volumePolicies"`

  // FieldSelectorPolicies defines a list of policies to enable Kubernetes
  // resource field selection to support filtering by resource fields.
  FieldSelectorPolicies []FieldSelectorPolicy    `yaml:"fieldSelectorPolicies"`

  // we may support other resource policies in the future, and they could be added separately
  // OtherResourcePolicies: []OtherResourcePolicy
}

import (
  k8s.io/apimachinery/pkg/fields
  metav1 k8s.io/apimachinery/pkg/apis/meta/v1
)

// FieldSelectorPolicy describes the field selection properties used to identify a
// Kubernetes resource.
type FieldSelectorPolicy struct {
  // GroupResource specifies a group and a resource.
  GroupResource metav1.GroupResource `json:"groupResource,omitempty"`

  // FieldSelectors specifies a field name and its value, typically, implemented
  // as a map of string-to-string.
  // If omitted, then all resources of the group will be selected.
  // +optional
  FieldSelectors fields.Set `json:"fieldSelectors,omitempty"`
}
```

### Code Design

The field selector mechanism eliminates the needs to perform client-side filtering which requires all resources of a particular type to be fetched beforehand.
The selection criteria can be added as a filter option to the `LIST` requests using the [`metav1.ListOptions`][6] option.
The `k8s.io/cli-runtime` also offers a [`resource.Builder`][7] type that can be used to convert client-side field selectors into REST requests, to selectively retrieve Kubernetes resources.
For example, `kubectl get` uses the [following code][8] to instantiate a `resource.Builder` to retrieve pods that satisfy the given fields selection criteria:

```go
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		Subresource(o.Subresource).
		RequestChunksOf(chunkSize).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.transformRequests).
		Do()
```

## Alternatives Considered

Velero can issue requests to get all resources of a particular type, and then perform client-side field comparisons to find the resource that satisfies a specific field such as resource name. 
Such retrieval operations on large clusters can be very inefficient.
The existing label selection filtering mechanism has also been considered.
But on clusters with many operators, labeling of CRDs requires user to devise a precise labeling scheme to properly group the CRDs, according to operators, which can be error prone.

## Security Considerations

N/A.

## Compatibility

The field selector mechanism only works with the new `ResourcePolicies` API.

## Implementation

TBD.

## Open Issues

- https://github.com/vmware-tanzu/velero/issues/4876
- https://github.com/vmware-tanzu/velero/issues/5118
- https://github.com/vmware-tanzu/velero/issues/5152

[0]: https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/handle-backup-of-volumes-by-resources-filters.md?plain=1#L69
[1]: https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
[2]: https://velero.io/docs/main/resource-filtering/
[3]: https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#GroupResource
[4]: https://github.com/vmware-tanzu/velero/issues/4876
[5]: https://velero.io/docs/v1.10/resource-filtering/#--selector
[6]: https://pkg.go.dev/k8s.io/apimachinery@v0.26.1/pkg/apis/meta/v1#ListOptions
[7]: https://pkg.go.dev/k8s.io/cli-runtime@v0.26.1/pkg/resource#Builder
[8]: https://github.com/kubernetes/kubectl/blob/e67364c45abb19958d8f7c9168937489cfc01e76/pkg/cmd/get/get.go#L460-L491
