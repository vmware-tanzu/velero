# Velero Upgrade Design  

## Background

Besides a fresh installation, users also require to upgrade from old version with preserving the existing tasks (i.e., backup schedules) and configurations (i.e., Velero server & daemonset parameters). At present, the recommended Velero upgrade approach is finished manually with below steps:  
- Update all CRDs by running ```velero install --crds-only --dry-run -o yaml | kubectl apply -f -```
- Update Velero server and daemonset image by running ```kubectl set image```  

For a complete example of the existing upgrade approach, refer to the Velero document at https://velero.io/docs/v1.9/upgrade-to-1.9/.

The manual approach works for the existing scenarios, however, if we want to do some complex tasks during upgrade (i.e., installing a new module or changing/adding complex parameters), it will be very unfriendly in the user experiences.  
For an concrete example, in v1.10, we have done some refactors to Velero's pod volume backup (a.k.a Restic backup formerly), two of the refactors are:  
- Rename some Velero server and daemonset parameters
- Rename Velero daemonset from "restic" to "node-agent"

As a result, the existing upgrade approach is not sufficient:
- If we still use manual steps, we have to tell users the detailed parameters that have been renamed, and users then needs to check their existence and modify them one by one.
- If we still use manual steps, users have to delete the old restic daemonset and run ```velero install``` with the full parameters they specified during the fresh installation, because for one thing, Kubernetes doesn't support to rename a resource; for another, Velero doesn't provide a way to install a single resource only. 

## Solution

We want to add a new command besides ```velero install``` called ```velero upgrade```. In a upgrade case, users could call ```velero upgrade``` without bothering to know the details. This bring below benefits:  
- Solve the upgrade problems in v1.10
- Make Velero more user friendly, because the upgrade process is significantly simplified
- Support future evolvements. For example, in future, Velero will have various data movement features, for which we will add new moudles inevitably. Then we can add enhancement incrementally to ```velero upgrade```, because inside the command, we can do whatever complex things 

The ```velero upgrade``` command will fully replace the existing manual steps in the aforementioned Velero document.

## Design

At present, ```velero upgrade``` will do below things:
- Get the new version from the client binary itself, that is, the upgrade target version
- Get the old version from the existing velero server, that is, the current version
- Retrieve Velero server deployment object
- Retrieve Velero daemonset object
- Iterate from current version through target version, apply the changes (i.e., the image, name and parameter changes) to the deployment object once per version. For example, when upgrading from 1.9 to 1.11, we need to apply the changes between 1.9 and 1.10 first, then apply the changes between 1.10 and 1.11. See the details below for how to drive the changes apply
- Do the same changes apply to Velero daemonset object as the deployment object
- Delete the existing Velero deployment
- Delete the existing Velero daemonset
- Extract the new CRDs from the client binary and apply them
- Create the new Velero daemonset from the modified daemonset object
- Create the new Velero server deployment from the modified deployment object

### Action Table
We create below action functions to apply changes for each target version, the number in the function name indicates the target version:  
```
func UpgradeV10(deploy *v1.Deployment, ds *v1.DaemonSet) error
func UpgradeV11(deploy *v1.Deployment, ds *v1.DaemonSet) error
func UpgradeV12(deploy *v1.Deployment, ds *v1.DaemonSet) error
```

Then we put these functions to a vector, so that the action functions could be retrieved by indexes:  
```
type UpgradeAction struct {
	Target string
	Action func(*v1.Deployment, *v1.DaemonSet) error
}

var UpgradeActions = []UpgradeAction {
	{
		"v1.10",
		UpgradeV10,
	},
	{
		"v1.11",
		UpgradeV11,
	},
	{
		"v1.12",
		UpgradeV12,
	},	
}
```
So ```UpgradeActions[0]``` means the action to upgrade from older versions to v1.10; ```UpgradeActions[1]``` means the action to upgrade from v1.10 to v1.11.  
Then we match the current version and target version in ```UpgradeActions``` to get a begin-end index pair.  
Finally, we call the action functions between the begin index and end index.  

### Image Selection
The target image is mapped by the target version, which is retrieved from the Velero client binary. If the client is from a under-release version, we will set the image as ```velero/velero:latest```.

As we can see above, all the required information is self-retrieved, and the upgrade actions are self-driven, so we don't need to add any extra parameters to ```velero upgrade```.  

## Non-Goals
As the best practise, there should be some kind of recovery measure so that when the upgrade fails, the original environment could be recovered. This design doesn't include it, we could handle this in the future enhancements.  
Therefore, at present, under this situation, users need to manual fix the half-way environment or uninstall it and then reinstall it freshly.  