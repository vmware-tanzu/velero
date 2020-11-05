# Restore API Group Version by Priority Level When EnableAPIGroupVersions Feature is Set

Status: Draft

## Abstract

This document proposes a solution to select an API group version to restore from the versions backed up using the feature flag EnableAPIGroupVersions.

## Background

It is possible that between the time a backup has been made and a restore occurs that the target Kubernetes version has incremented more than one version. In such a case where multiple versions of Kubernetes were skipped, the preferred source cluster's API group versions for resources may no longer be supported by the target cluster. With PR #2373, all supported API group versions were backed up if the EnableAPIGroupVersions feature flag was set during Velero installation. The next step (outlined by this design proposal) will be to see if any of the backed up versions are supported in the target cluster and if so, choose one to restore for each backed up resource.

## Goals

- Restore resources using the chosen API group version.

## Non Goals

- Allowing users to customize the predetermined API group versions priorities.
- Modifying the compressed backup tarball files.
- Using plugins to restore a resource when the target supports non of the source cluster's API group versions.

## High-Level Design

During restore, the proposal is that Velero will read the directory names in the backup tarball file and see which API group versions were backed up from the source cluster. Velero will also look at the target cluster and determine which API group versions are supported. Given the two lists of source and target cluster supported API versions, a priority system will be used to determine which version will be backed up. The priority of versions in order are target cluster preferred version, source cluster preferred version, and the latest common supported version. Once the version to restore is chosen, the file path to the backed up resource in the tarball will be modified such that it points to the resources' chosen API group version. If no version is found in common between the source and target clusters, the chosen version will default to the source cluster's preferred version (the version being restored currently without the changes proposal here). Restore will be allowed to continue as before.

## Detailed Design

There are four main objectives to achieve the above stated goal:

1. Determine if the APIGroupVersionsFeatureFlag feature flag has been enabled.
1. List the backed up API group versions.
1. List the API group versions supported by the target cluster.
1. Use a priority system to determine which version to restore. The source preferred version will be the default if the priorities fail.
1. Modify the paths to the backup files in the tarball in the resource restore process.

### Objective 1: Determine if the APIGroupVersionsFeatureFlag feature flag has been enabled

For restore to be able to choose from multiple supported backed up versions, the feature flag must have been enabled during both the backup and restore processes. And the check to see if the feature flag is/was enabled must be done for both processes.

The reason for checking for the feature flag during restore is to ensure the user would like to restore a version that is among potentially multiple versions backed up with the feature flag enabled.

The reason for checking to see if the feature flag was enabled during backup is to ensure the changes made by this proposed design is backward compatible. Only with Velero version 1.4 and forward was Format Version 1.0.0 used to structure the backup directories. Format Version 1.0.0 is required for the restore process proposed in this design doc to work. Before v1.4, the backed up files were in a directory structure that will not be recognized by the proposed code changes. Therefore, restore should not attempt to restore from multiple versions as they will not exist.

Checking if the feature flag is enabled during restore is straightforward using `features.IsEnabled(velerov1api.APIGroupVersionsFeatureFlag)`.

Checking if the feature flag was enabled during backup can be done indirectly by checking the directory structure and version directory names. For backups generated using Format Version 1.0.0 (required for the changes proposed here), there will always be a version directory with `-preferredversion` in the directory name. Only one resource will need to be checked to determine if the feature flag was enabled during backup.

### Objective 2: List the backed up API group versions

Currently, in `pkg/restore/restore.go`, in the `execute(...)` method, around [line 363](https://github.com/vmware-tanzu/velero/blob/7a103b9eda878769018386ecae78da4e4f8dde83/pkg/restore/restore.go#L363), the resources and their backed up items are saved in a map called `backupResources`.

At this point, it can be checked if the feature flag is/was enabled during restore/backup, respectively (described previously in Objective #1). If it is and was enabled, the `backedupResources` map can be sent to a method (to be created) with the signature `ctx.chooseAPIVersionsToRestore(backupResources)`. Notice that the method is called on `ctx` which has the type `*restore.Context`.

The `chooseAPIVersionsToRestore` method can remain in the `restore` package, but for organizational purposes, it can be moved to a file called `prioritize_group_version.go`.

Inside the `chooseAPIVersionsToRestore` method, we can take advantage of the `archive` package's `Parser` type. A new parser object can be created using the construction function `NewParser(...)`. A method for the parser object will be created that has the signature `ParseGroupVersions(backupDir string) (map[string]metav1.APIGroup, error)`. The `ParseGroupVerisons(...)` method will loop through the `resources`, `resource.group`, and group version directories to populate a map called `sourceRGVersions`. 

The `sourceRGVersions` map's keys will be strings in the format `<resource>.<group>`, e.g. "horizontalpodautoscalers.autoscaling". The values will be APIGroup structs. The API Group struct can be imported from k8s.io/apimachinery/pkg/apis/meta/v1.

### Objective 3: List the API group versions supported by the target cluster

Still within the `chooseAPIVersionsToRestore` method, the target cluster's resource group versions can now be obtained.

```go
targetGroupVersions := ctx.discoveryHelper.APIGroups()
```

### Objective 4: Use a priority system to determine which version to back up, if any

Determining the priority will also be done in the `chooseAPIVersionsToRestore` method.

Loop through both the source and target supported group versions. Have a series of if statements that will follow the priority levels to determine which version to restore. The priority hierarchy will be

- **Priority 1 (Highest priority)**. Target preferred version can be used. This means one of two things:
  - (A) target preferred version == source preferred version OR
  - (B) target preferred version == source supported version

- **Priority 2**. Source preferred version can be used. This means
  - source preferred version == target supported version

- **Priority 3**. A common supported version can be used. This means
  - target supported version == source supported version
  - if multiple support versions intersect, choose the latest version (v2 > v1, beta > alpha, etc).

If there is no common supported version between target and source clusters, then the default `ChosenGrpVersion` will be the source preferred version. This is the version that would have been restored before the changes proposed here.

A new map type will be created of the form `map[string]ChosenGrpVersion` where the key is the `<resource>.<group>` and the values are `ChosenGrpVersion` types. The map will be saved to the `restore.Context` object in a field called `chosenGrpVersToRestore`.

`ChosenGrpVersion`, which is the value type of the `ctx.chosenGrpVersToRestore` map will be a new type of the form

```go
type ChosenGrpVersion struct {
    Group      string
    Version    string
    VerDirName string
}
```

Note that adding a field to `restore.Context` will mean having to make a map for the field during instantiation.

### Objective 5: Modify the paths to the backup files in the tarball

The method doing the bulk of the restoration work is `ctx.restoreResource(...)`. Inside this method, around [line 714](https://github.com/vmware-tanzu/velero/blob/7a103b9eda878769018386ecae78da4e4f8dde83/pkg/restore/restore.go#L714) in `pkg/restore/restore.go`, the path to backup json file for the item being restored is set.

Inside the `for` loop that ranges through the `items`, the `features.IsEnabled(velerov1api.APIGroupVersionsFeatureFlag)` can be checked. If it is true, the path saved to `resource` variable can be updated.

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
resource = filepath.Join(groupResource.String(), ctx.chosenGrpVersToRestore[groupResource.String()].VerDirName)
```

The restore can now proceed as normal.

## Alternatives Considered

- Look for plugins if no common supported API group version could be found between the target and source clusters. We had considered searching for plugins that could handle converting an outdated resource to a new one that is supported in the target cluster, but it is difficult, will take a lot of time, and currently will not be useful because we are not aware of such plugins. It would be better to keep the initial changes simple to see how it works out and progress to more complex solutions as demand necessitates.
- Have custom priorities based on user inputs. We considered being able to look at user input. The only way we could think users could specify their own version priorities was by modifying the backup json files somehow, for example, inject an annotation into one of them. However, this will require further design discussion as it is a difficult feature to add. We also don't know how many people would like to change the priorities. We can add the feature later as demand necessitates it.
- It was considered to modify the backed up json files such that the resources API versions are supported by the target but modifying backups is discouraged for several reasons, including introducing data corruption.

## Security Considerations

I can't think of any additional risks in terms of Velero security here.

## Compatibility

I have made it such that the changes in code will only affect Velero installations that have `APIGroupVersionsFeatureFlag` enabled. If it is not enabled, the changes will have no affect on the restore process, making the changes here entirely backward compatible.

## Implementation

This first draft of the proposal will be submitted Oct. 30, 2020. Once this proposal is approved, I can have the code and unit tests written within a week and submit a PR that fixes Issue #2551.

## Open Issues

At the time of writing this design proposal, I had not seen any of @jenting's work for solving Issue #2551. He had independently covered the first two priorities I mentioned above before I was even aware of the issue. I hope to not let his efforts go to waste and welcome incorporating his ideas here to make this design proposal better.
