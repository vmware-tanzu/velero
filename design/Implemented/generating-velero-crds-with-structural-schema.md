# Generating Velero CRDs with structural schema support

As the apiextensions.k8s.io API moves to GA, structural schema in Custom Resource Definitions (CRDs) will become required.

This document proposes updating the CRD generation logic as part of `velero install` to include structural schema for each Velero CRD.

## Goals

- Enable structural schema and validation for Velero Custom Resources.

## Non Goals

- Update Velero codebase to use Kubebuilder for controller/code generation.
- Solve for keeping CRDs in the Velero Helm chart up-to-date.

## Background

Currently, Velero CRDs created by the `velero install` command do not contain any structural schema.
The CRD is simply [generated at runtime](https://github.com/heptio/velero/blob/8b0cf3855c2b8aa631cf22e63da0955f7b1d06a8/pkg/install/crd.go#L39) using the name and plurals from the [`velerov1api.CustomResources()`](https://github.com/heptio/velero/blob/8b0cf3855c2b8aa631cf22e63da0955f7b1d06a8/pkg/apis/velero/v1/register.go#L60) info.

Updating the info returned by that method would be one way to add support for structural schema when generating the CRDs, but this would require manually describing the schema and would duplicate information from the API structs (e.g. comments describing a field).

Instead, the [controller-tools](https://github.com/kubernetes-sigs/controller-tools) project from Kubebuilder provides tooling for generating CRD manifests (YAML) from the Velero API types.
This document proposes adding _controller-tools_ to the project to automatically generate CRDs, and use these generated CRDs as part of `velero install`.

## High-Level Design

_controller-tools_ works by reading the Go files that contain the API type definitions.
It uses a combination of the struct fields, types, tags and comments to build the OpenAPIv3 schema for the CRDs. The tooling makes some assumptions based on conventions followed in upstream Kubernetes and the ecosystem, which involves some changes to the Velero API type definitions, especially around optional fields.

In order for _controller-tools_ to read the Go files containing Velero API type defintiions, the CRDs need to be generated at build time, as these files are not available at runtime (i.e. the Go files are not accessible by the compiled binary).
These generated CRD manifests (YAML) will then need to be available to the `pkg/install` package for it to include when installing Velero resources.

## Detailed Design

### Changes to Velero API type definitions

API type definitions need to be updated to correctly identify optional and required fields for each API type.
Upstream Kubernetes defines all optional fields using the `omitempty` tag as well as a `// +optional` annotation above the field (e.g. see [PodSpec definition](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2835-L2838)).
_controller-tools_ will mark a field as optional if it sees either the tag or the annotation, but to keep consistent with upstream, optional fields will be updated to use both indicators (as [suggested](https://github.com/kubernetes-sigs/kubebuilder/issues/479) by the Kubebuilder project).
Additionally, upstream Kubernetes defines the metav1.ObjectMeta, metav1.ListMeta, Spec and Status as [optional on all types](https://github.com/kubernetes/api/blob/master/core/v1/types.go#L3517-L3531).
Some Velero API types set the `omitempty` tag on Status, but not on other fields - these will all need to be updated to be made optional.

Below is a list of the Velero API type fields and what changes (if any) will be made.
Note that this only includes fields used in the spec, all status fields will become optional.

| Type                            | Field                   | Changes                                                     |
|---------------------------------|-------------------------|-------------------------------------------------------------|
| BackupSpec                      | IncludedNamespaces      | make optional                                               |
|                                 | ExcludedNamespaces      | make optional                                               |
|                                 | IncludedResources       | make optional                                               |
|                                 | ExcludedResources       | make optional                                               |
|                                 | LabelSelector           | make optional                                               |
|                                 | SnapshotVolumes         | make optional                                               |
|                                 | TTL                     | make optional                                               |
|                                 | IncludeClusterResources | make optional                                               |
|                                 | Hooks                   | make optional                                               |
|                                 | StorageLocation         | make optional                                               |
|                                 | VolumeSnapshotLocations | make optional                                               |
| BackupHooks                     | Resources               | make optional                                               |
| BackupResourceHookSpec          | Name                    | none (required)                                             |
|                                 | IncludedNamespaces      | make optional                                               |
|                                 | ExcludedNamespaces      | make optional                                               |
|                                 | IncludedResources       | make optional                                               |
|                                 | ExcludedResources       | make optional                                               |
|                                 | LabelSelector           | make optional                                               |
|                                 | PreHooks                | make optional                                               |
|                                 | PostHooks               | make optional                                               |
| BackupResourceHook              | Exec                    | none (required)                                             |
| ExecHook                        | Container               | make optional                                               |
|                                 | Command                 | required, validation: MinItems=1                            |
|                                 | OnError                 | make optional                                               |
|                                 | Timeout                 | make optional                                               |
| HookErrorMode                   |                         | validation: Enum                                            |
| BackupStorageLocationSpec       | Provider                | none (required)                                             |
|                                 | Config                  | make optional                                               |
|                                 | StorageType             | none (required)                                             |
|                                 | AccessMode              | make optional                                               |
| StorageType                     | ObjectStorage           | make required                                               |
| ObjectStorageLocation           | Bucket                  | none (required)                                             |
|                                 | Prefix                  | make optional                                               |
| BackupStorageLocationAccessMode |                         | validation: Enum                                            |
| DeleteBackupRequestSpec         | BackupName              | none (required)                                             |
| DownloadRequestSpec             | Target                  | none (required)                                             |
| DownloadTarget                  | Kind                    | none (required)                                             |
|                                 | Name                    | none (required)                                             |
| DownloadTargetKind              |                         | validation: Enum                                            |
| PodVolumeBackupSpec             | Node                    | none (required)                                             |
|                                 | Pod                     | none (required)                                             |
|                                 | Volume                  | none (required)                                             |
|                                 | BackupStorageLocation   | none (required)                                             |
|                                 | RepoIdentifier          | none (required)                                             |
|                                 | Tags                    | make optional                                               |
| PodVolumeRestoreSpec            | Pod                     | none (required)                                             |
|                                 | Volume                  | none (required)                                             |
|                                 | BackupStorageLocation   | none (required)                                             |
|                                 | RepoIdentifier          | none (required)                                             |
|                                 | SnapshotID              | none (required)                                             |
| ResticRepositorySpec            | VolumeNamespace         | none (required)                                             |
|                                 | BackupStorageLocation   | none (required)                                             |
|                                 | ResticIdentifier        | none (required)                                             |
|                                 | MaintenanceFrequency    | none (required)                                             |
| RestoreSpec                     | BackupName              | none (required) - should be set to "" if using ScheduleName |
|                                 | ScheduleName            | make optional                                               |
|                                 | IncludedNamespaces      | make optional                                               |
|                                 | ExcludedNamespaces      | make optional                                               |
|                                 | IncludedResources       | make optional                                               |
|                                 | ExcludedResources       | make optional                                               |
|                                 | NamespaceMapping        | make optional                                               |
|                                 | LabelSelector           | make optional                                               |
|                                 | RestorePVs              | make optional                                               |
|                                 | IncludeClusterResources | make optional                                               |
| ScheduleSpec                    | Template                | none (required)                                             |
|                                 | Schedule                | none (required)                                             |
| VolumeSnapshotLocationSpec      | Provider                | none (required)                                             |
|                                 | Config                  | make optional                                               |

### Build-time generation of CRD manifests

The build image will be updated as follows to include the _controller-tool_ tooling:


```diff
diff --git a/hack/build-image/Dockerfile b/hack/build-image/Dockerfile
index b69a8c8a..07eac9c6 100644
--- a/hack/build-image/Dockerfile
+++ b/hack/build-image/Dockerfile
@@ -21,6 +21,8 @@ RUN mkdir -p /go/src/k8s.io && \
     git clone -b kubernetes-1.15.3 https://github.com/kubernetes/apimachinery && \
     # vendor code-generator go modules to be compatible with pre-1.15
     cd /go/src/k8s.io/code-generator && GO111MODULE=on go mod vendor && \
+    go get -d sigs.k8s.io/controller-tools/cmd/controller-gen && \
+    cd /go/src/sigs.k8s.io/controller-tools && GO111MODULE=on go mod vendor && \
     go get golang.org/x/tools/cmd/goimports && \
     cd /go/src/golang.org/x/tools && \
     git checkout 40a48ad93fbe707101afb2099b738471f70594ec && \
```

To tie in the CRD manifest generation with existing scripts/workflows, the `hack/update-generated-crd-code.sh` script will be updated to use _controller-tools_ to generate CRDs manifests after it generates the client code.

The generated CRD manifests will be placed in the `pkg/generated/crds/manifests` folder.
Similarly to client code generation, these manifests will be checked-in to the git repo.
Checking in these manifests allows including documentation and schema changes to API types as part of code review.

### Updating `velero install` to include generated CRD manifests

As described above, CRD generation using _controller-tools_ will happen at build time due to need to inspect Go files.
To enable the `velero install` to access the generated CRD manifests at runtime, the `pkg/generated/crds/manifests` folder will be embedded as binary data in the Velero binary (e.g. using a tool like [vfsgen](https://github.com/shurcooL/vfsgen) - see [POC branch](https://github.com/prydonius/velero/commit/4aa7413f97ce9b23e071b6054f600dd0c283351e)).

`velero install` will then unmarshal the binary data as `unstructured.Unstructured` types and append them to the [resources list](https://github.com/heptio/velero/blob/8b0cf3855c2b8aa631cf22e63da0955f7b1d06a8/pkg/install/resources.go#L217) in place of the existing CRD generation.

## Alternatives Considered

Instead of generating and bundling CRD manifests, it could be possible to instead embed the `pkg/apis` package in the Velero binary.
With this, _controller-tools_ could be run at runtime during `velero install` to generate the CRD manifests.
However, this would require including _controller-tools_ as a dependency in the project, which might not be desirable as it is a developer tool.

Another option, to avoid embedding static files in the binary, would be to generate the CRD manifest as one YAML file in CI and upload it as a release artifact (e.g. using GitHub releases).
`velero install` could then download this file for the current version and use it on install.
The downside here is that `velero install` becomes dependent on the GitHub network, and we lose visibility on changes to the CRD manifests in the Git history.

## Security Considerations

n/a
