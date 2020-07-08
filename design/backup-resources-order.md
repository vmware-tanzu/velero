## Backup Resources Order
This document proposes a solution that allows user to specify a backup order for resources of specific resource type.

## Background
During backup process, user may need to back up resources of specific type in some specific order to ensure the resources were backup properly because these resources are related and ordering might be required to preserve the consistency for the apps to recover itself  from the backup image 
(Ex: primary-secondary database pods in a cluster).

## Goals
- Enable user to specify an order of back up resources belong to specific resource type

## Alternatives Considered
- Use a plugin to backup an resources and all the sub resources.  For example use a plugin for StatefulSet and backup pods belong to the StatefulSet in specific order.  This plugin solution is not generic and requires plugin for each resource type.

## High-Level Design
User will specify a map of resource type to list resource names (separate by semicolons).  Each name will be in the format "namespaceName/resourceName" to enable ordering accross namespaces.  Based on this map, the resources of each resource type will be sorted by the order specified in the list of resources.  If a resource instance belong to that specific type but its name is not in the order list, then it will be put behind other resources that are in the list.

### Changes to BackupSpec
Add new field to BackupSpec

    type BackupSpec struct {
        ...
        // OrderedResources contains a list of key-value pairs that represent the order
        // of backup of resources that belong to specific resource type
        // +optional
        // +nullable
        OrderedResources map[string]string
    }

### Changes to itemCollector
Function getResourceItems collects all items belong to a specific resource type.  This function will be enhanced to check with the map to see whether the OrderedResources has specified the order for this resource type.  If such order exists, then sort the items by such order being process before return.

### Changes to velero CLI
Add new flag "--ordered-resources" to Velero backup create command which takes a string of key-values pairs which represents the map between resource type and the order of the items of such resource type. Key-value pairs are separated by semicolon, items in the value are separated by commas.

Example:
>velero backup create mybackup --ordered-resources "pod=ns1/pod1,ns1/pod2;persistentvolumeclaim=n2/slavepod,ns2/primarypod"

## Open Issues
- In the CLI, the design proposes to use commas to separate items of a resource type and semicolon to separate key-value pairs.  This follows the convention of using commas to separate items in a list (For example: --include-namespaces ns1,ns2).  However, the syntax for map in labels and annotations use commas to seperate key-value pairs.  So it introduces some inconsistency.
- For pods that managed by Deployment or DaemonSet, this design may not work because the pods' name is randomly generated and if pods are restarted, they would have different names so the Backup operation may not consider the restarted pods in the sorting algorithm.  This problem will be addressed when we enhance the design to use regular expression to specify the OrderResources instead of exact match. 
