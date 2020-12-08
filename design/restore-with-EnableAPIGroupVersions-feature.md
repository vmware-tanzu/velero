# Restore API Group Version by Priority Level When EnableAPIGroupVersions Feature is Set

Status: Draft

## Abstract

This document proposes a solution to select an API group version to restore from the versions backed up using the feature flag EnableAPIGroupVersions.

## Background

It is possible that between the time a backup has been made and a restore occurs that the target Kubernetes version has incremented more than one version. In such a case where at least a versions of Kubernetes was skipped, the preferred source cluster's API group versions for resources may no longer be supported by the target cluster. With [PR#2373](https://github.com/vmware-tanzu/velero/pull/2373), all supported API group versions were backed up if the EnableAPIGroupVersions feature flag was set for Velero. The next step (outlined by this design proposal) will be to see if any of the backed up versions are supported in the target cluster and if so, choose one to restore for each backed up resource.

## Goals

- Choose an API group to restore from backups given a priority system or a user-provided prioritization of versions.
- Restore resources using the chosen API group version.

## Non Goals

- Allow users to restore onto a cluster that is running a Kubernetes version older than the source cluster. The changes proposed here only allow for skipping ahead to a newer Kubernetes version, but not going backward.
- Allow restoring from backups created using Velero version 1.3 or older. This proposal will only work on backups created using Velero 1.4+.
- Modifying the compressed backup tarball files. We don't want to risk corrupting the backups.
- Using plugins to restore a resource when the target supports none of the source cluster's API group versions. The ability to use plugins will hopefully be something added in the future, but not at this time.

## High-Level Design

During restore, the proposal is that Velero will determine if the `APIGroupVersionsFeatureFlag` was enabled in the target cluster and `Status.FormatVersion 1.1.0` was used during backup. Only if these two conditions are met will the changes proposed here take effect.

The proposed code starts with creating three lists for each backed up resource. The three lists will be created by
  (1) reading the directory names in the backup tarball file and seeing which API group versions were backed up from the source cluster,
  (2) looking at the target cluster and determining which API group versions are supported, and
  (3) getting config maps from the target cluster in order to get user-defined prioritization of versions.

  The three lists will be used to create a map of chosen versions for each resource to restore. If there is a user-defined list of priority versions, the versions will be checked against the supported versions lists. The highest user-defined priority version that is/was supported by both target and source clusters will be the chosen version for that resource. If no user specified versions are supported by neither target nor source, the versions will be logged and the restore will continue with other prioritizations.

  Without a user-defined prioritization of versions, the following version prioritization will be followed, starting from the highest priority: target cluster preferred version, source cluster preferred version, and a common supported version. Should there be multiple common supported versions, the one that will be chosen will be based on the [Kubernetes version priorities](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#version-priority).
  
  Once the version to restore is chosen, the file path to the backed up resource in the tarball will be modified such that it points to the resources' chosen API group version. If no version is found in common between the source and target clusters, the chosen version will default to the source cluster's preferred version (the version being restored currently without the changes proposed here). Restore will be allowed to continue as before.

## Detailed Design

There are six objectives to achieve the above stated goals:

1. Determine if the APIGroupVersionsFeatureFlag is enabled and Backup Objects use Status.FormatVersion 1.1.0.
1. List the backed up API group versions.
1. List the API group versions supported by the target cluster.
1. Get the user-defined version priorities.
1. Use a priority system to determine which version to restore. The source preferred version will be the default if the priorities fail.
1. Modify the paths to the backup files in the tarball in the resource restore process.

### Objective 1: Determine if the APIGroupVersionsFeatureFlag is enabled and Backup Objects use Status.FormatVersion 1.1.0

For restore to be able to choose from multiple supported backed up versions, the feature flag must have been enabled during the restore processes. Backup objects must also have [Status.FormatVersion == "1.1.0"](https://github.com/vmware-tanzu/velero/blob/a1e182e723a8c5f6d4175d8db2361233a94d2502/pkg/backup/backup.go#L58).

The reason for checking for the feature flag during restore is to ensure the user would like to restore a version that might not be the source cluster preferred version. This check is done via `features.IsEnabled(velerov1api.APIGroupVersionsFeatureFlag)`.

The reason for checking `Status.FormatVersion` is to ensure the changes made by this proposed design is backward compatible. Only with Velero version 1.4 and forward was Format Version 1.1.0 used to structure the backup directories. Format Version 1.1.0 is required for the restore process proposed in this design doc to work. Before v1.4, the backed up files were in a directory structure that will not be recognized by the proposed code changes. In this case, restore should not attempt to restore from multiple versions as they will not exist.

The [`Status.FormatVersion`](https://github.com/vmware-tanzu/velero/blob/6808acd92e30848056a21faf373af03ddb8a3b71/pkg/apis/velero/v1/backup.go#L235) is stored in a `restoreContext` struct field called [`backup`](https://github.com/vmware-tanzu/velero/blob/6808acd92e30848056a21faf373af03ddb8a3b71/pkg/restore/restore.go#L229). The full chain is `ctx.backup.Status.FormatVersion`.  

The above two checks can be done inside a new method on the `*restoreContext` object with the method signature `meetsAPIGVRestoreReqs() bool`. This method can remain in the `restore` package, but for organizational purposes, it can be moved to a file called `prioritize_group_version.go`.

### Objective 2: List the backed up API group versions

Currently, in `pkg/restore/restore.go`, in the `execute(...)` method, around [line 363](https://github.com/vmware-tanzu/velero/blob/7a103b9eda878769018386ecae78da4e4f8dde83/pkg/restore/restore.go#L363), the resources and their backed up items are saved in a map called `backupResources`.

At this point, the feature flag and format versions can be checked (described in Objective #1). If the requirements are met, the `backedupResources` map can be sent to a method (to be created) with the signature `ctx.chooseAPIVersionsToRestore(backupResources)`. The `ctx` object has the type `*restore.Context`.

The `chooseAPIVersionsToRestore` method can remain in the `restore` package, but for organizational purposes, it can be moved to a file called `prioritize_group_version.go`.

Inside the `chooseAPIVersionsToRestore` method, we can take advantage of the `archive` package's `Parser` type.  `ParseGroupVersions(backupDir string) (map[string]metav1.APIGroup, error)`. The `ParseGroupVersions(...)` method will loop through the `resources`, `resource.group`, and group version directories to populate a map called `sourceRGVersions`.

The `sourceRGVersions` map's keys will be strings in the format `<resource>.<group>`, e.g. "horizontalpodautoscalers.autoscaling". The values will be APIGroup structs. The API Group struct can be imported from k8s.io/apimachinery/pkg/apis/meta/v1. Order the APIGroup.Versions slices using a sort function copied from `k8s.io/apimachinery/pkg/version`.

```go
sort.SliceStable(gvs, func(i, j int) bool {
    return version.CompareKubeAwareVersionStrings(gvs[i].Version, gvs[j].Version) > 0
})
```

### Objective 3: List the API group versions supported by the target cluster

Still within the `chooseAPIVersionsToRestore` method, the target cluster's resource group versions can now be obtained.

```go
targetRGVersions := ctx.discoveryHelper.APIGroups()
```

Order the APIGroup.Versions slices using a sort function copied from `k8s.io/apimachinery/pkg/version`.

```go
sort.SliceStable(gvs, func(i, j int) bool {
    return version.CompareKubeAwareVersionStrings(gvs[i].Version, gvs[j].Version) > 0
})
```

### Objective 4: Get the user-defined version priorities

Still within the `chooseAPIVersionsToRestore` method, the user-defined version priorities can be retrieved. These priorities are expected to be in a config map named `enableapigroupversions` in the `velero` namespace. An example config map is

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: enableapigroupversions
  namespace: velero
data:
  restoreResourcesVersionPriority: | -
    rockbands.music.example.io=v2beta1,v2beta2
    orchestras.music.example.io=v2,v3alpha1
    subscriptions.operators.coreos.com=v2,v1
```

In the config map, the resources and groups and the user-defined version priorities will be listed in the `data.restoreResourcesVersionPriority` field following the following general format: `<group>.<resource>=<version 1>[, <version n> ...]`.

A map will be created to store the user-defined priority versions. The map's keys will be strings in the format `<resource>.<group>`. The values will be APIGroup structs that will be imported from `k8s.io/apimachinery/pkg/apis/meta/v1`. Within the APIGroup structs will be versions in the order that the user provides in the config map. The PreferredVersion field in APIGroup struct will be left empty.

### Objective 5: Use a priority system to determine which version to restore. The source preferred version will be the default if the priorities fail

Determining the priority will also be done in the `chooseAPIVersionsToRestore` method. Once a version is chosen, it will be stored in a new map of the form `map[string]ChosenGRVersion` where the key is the `<resource>.<group>` and the values are of the `ChosenGroupVersion` struct type (shown below). The map will be saved to the `restore.Context` object in a field called `chosenGrpVersToRestore`.

```go
type ChosenGroupVersion struct {
    Group   string
    Version string
    Dir     string
}
```

The first method called will be `ctx.gatherSTUVersions()` and it will gather the source cluster group resource and versions (`sgvs`), target cluster group versions (`tgvs`), and custom user resource and group versions (`ugvs`).

Loop through the source cluster resource and group versions (`sgvs`). Find the versions for the group in the target cluster.

An attempt will first be made to `findSupportedUserVersion`. Loop through the resource.groups in the custom user resource and group versions (`ugvs`) map. If a version is supported by both `tgvs` and `sgvs`, that will be set as the chosen version for the corresponding resource in `ctx.chosenGrpVersToRestore`

If no three-way match can be made between the versions in `ugvs`, `tgvs`, and `sgvs`, move on to attempting to use the target cluster preferred version. Loop through the `sgvs` versions for the resource and see if any of them match the first item in the `tgvs` version list. Because the versions in `tgvs` have been ordered, the first version in the version slide will be the preferred version.

If target preferred version cannot be used, attempt to choose the source cluster preferred version. Loop through the target versions and see if any of them match the first item in the source version slice, which will be the preferred version due to Kubernetes version ordering.

If neither clusters' preferred version can be used, look through remaining versions in the target version list and see if there is a match with the remaining versions in the source versions list.

If none of the previous checks produce a chosen version, the source preferred version will be the default and the restore process will continue.

Here is another way to list the priority versions described above:

- **Priority 0** ((User override). Users determine restore version priority using a config map
- **Priority 1**. Target preferred version can be used.
- **Priority 2**. Source preferred version can be used.
- **Priority 3**. A common supported version can be used. This means
  - target supported version == source supported version
  - if multiple support versions intersect, choose the version using the [Kubernetes’ version prioritization system](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#version-priority)

If there is no common supported version between target and source clusters, then the default `ChosenGRVersion` will be the source preferred version. This is the version that would have been assumed for restore before the changes proposed here.

Note that adding a field to `restore.Context` will mean having to make a map for the field during instantiation.

To see example cases with version priorities, see a blog post written by Rafael Brito: https://github.com/brito-rafa/k8s-webhooks/tree/master/examples-for-projectvelero.

### Objective 6: Modify the paths to the backup files in the tarball

The method doing the bulk of the restoration work is `ctx.restoreResource(...)`. Inside this method, around [line 714](https://github.com/vmware-tanzu/velero/blob/7a103b9eda878769018386ecae78da4e4f8dde83/pkg/restore/restore.go#L714) in `pkg/restore/restore.go`, the path to backup json file for the item being restored is set.

After the groupResource is instantiated at pkg/restore/restore.go:733, and before the `for` loop that ranges through the `items`, the `ctx.chosenGRVsToRestore` map can be checked. If the groupResource exists in the map, the path saved to `resource` variable can be updated.

Currently, the item paths look something like

```bash
/var/folders/zj/vc4ln5h14djg9svz7x_t1d0r0000gq/T/620385697/resources/horizontalpodautoscalers.autoscaling/namespaces/myexample/php-apache-autoscaler.json
```

This proposal will have the path changed to something like

```bash
/var/folders/zj/vc4ln5h14djg9svz7x_t1d0r0000gq/T/620385697/resources/horizontalpodautoscalers.autoscaling/v2beta2/namespaces/myexample/php-apache-autoscaler.json
```

The `horizontalpodautoscalers.autoscaling` part of the path will be updated to `horizontalpodautoscalers.autoscaling/v2beta2` using

```go
version, ok := ctx.chosenGVsToRestore[groupResource.String()]
  if ok {
    resource = filepath.Join(groupResource.String(), version.VerDir)
  }
```

The restore can now proceed as normal.

## Alternatives Considered

- Look for plugins if no common supported API group version could be found between the target and source clusters. We had considered searching for plugins that could handle converting an outdated resource to a new one that is supported in the target cluster, but it is difficult, will take a lot of time, and currently will not be useful because we are not aware of such plugins. It would be better to keep the initial changes simple to see how it works out and progress to more complex solutions as demand necessitates.
- It was considered to modify the backed up json files such that the resources API versions are supported by the target but modifying backups is discouraged for several reasons, including introducing data corruption.

## Security Considerations

I can't think of any additional risks in terms of Velero security here.

## Compatibility

I have made it such that the changes in code will only affect Velero installations that have `APIGroupVersionsFeatureFlag` enabled during restore and Format Version 1.1.0 was used during backup. If both these requirements are not met, the changes will have no affect on the restore process, making the changes here entirely backward compatible.

## Implementation

This first draft of the proposal will be submitted Oct. 30, 2020. Once this proposal is approved, I can have the code and unit tests written within a week and submit a PR that fixes Issue #2551.

## Open Issues

At the time of writing this design proposal, I had not seen any of @jenting's work for solving Issue #2551. He had independently covered the first two priorities I mentioned above before I was even aware of the issue. I hope to not let his efforts go to waste and welcome incorporating his ideas here to make this design proposal better.
