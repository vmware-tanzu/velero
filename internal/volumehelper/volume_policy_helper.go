package volumehelper

import (
	"fmt"
	"strings"

	"github.com/vmware-tanzu/velero/internal/resourcepolicies"

	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"

	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	pdvolumeutil "github.com/vmware-tanzu/velero/pkg/util/podvolume"
)

type VolumeHelper interface {
	GetVolumesForFSBackup(pod *corev1api.Pod, defaultVolumesToFsBackup, backupExcludePVC bool, kbclient kbclient.Client) ([]string, []string, error)
	ShouldPerformSnapshot(obj runtime.Unstructured, groupResource schema.GroupResource, kbclient kbclient.Client) (bool, error)
}

type VolumeHelperImpl struct {
	VolumePolicy *resourcepolicies.Policies
	Logger       logrus.FieldLogger
}

func (v *VolumeHelperImpl) ShouldPerformSnapshot(obj runtime.Unstructured, groupResource schema.GroupResource, kbclient kbclient.Client) (bool, error) {
	// check if volume policy exists and also check if the object(pv/pvc) fits a volume policy criteria and see if the associated action is snapshot
	// if it is not snapshot then skip the code path for snapshotting the PV/PVC
	pvc := new(corev1api.PersistentVolumeClaim)
	pv := new(corev1api.PersistentVolume)
	var err error

	if groupResource == kuberesource.PersistentVolumeClaims {
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pvc); err != nil {
			return false, err
		}

		pv, err = kubeutil.GetPVForPVC(pvc, kbclient)
		if err != nil {
			return false, err
		}
	}

	if groupResource == kuberesource.PersistentVolumes {
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pv); err != nil {
			return false, err
		}
	}

	if v.VolumePolicy != nil {
		action, err := v.VolumePolicy.GetMatchAction(pv)
		if err != nil {
			return false, err
		}

		if action != nil && action.Type == resourcepolicies.Snapshot {
			v.Logger.Infof(fmt.Sprintf("performing snapshot action for pv %s as it satisfies the volume policy criteria", pv.Name))
			return true, nil
		}
		v.Logger.Infof(fmt.Sprintf("skipping snapshot action for pv %s due to not satisfying the volume policy criteria for snapshot action", pv.Name))
		return false, nil
	}

	return false, nil
}

func (v *VolumeHelperImpl) GetVolumesForFSBackup(pod *corev1api.Pod, defaultVolumesToFsBackup, backupExcludePVC bool, kbclient kbclient.Client) ([]string, []string, error) {
	// Check if there is a fs-backup/snapshot volume policy specified by the user, if yes then use the volume policy approach to
	// get the list volumes for fs-backup else go via the legacy annotation based approach
	var fsBackupVolumePolicyVols, nonFsBackupVolumePolicyVols, volsToProcessByLegacyApproach = make([]string, 0), make([]string, 0), make([]string, 0)
	var err error

	if v.VolumePolicy != nil {
		// Get the list of volumes to back up using pod volume backup for the given pod matching fs-backup volume policy action
		v.Logger.Infof("Volume Policy specified by the user, using volume policy approach to segregate pod volumes for fs-backup")
		// GetVolumesMatchingFSBackupAction return 3 list of Volumes:
		// fsBackupVolumePolicyVols: Volumes that have a matching fs-backup action from the volume policy specified by the user
		// nonFsBackupVolumePolicyVols: Volumes that have an action matching from the volume policy specified by the user, but it is not fs-backup action
		// volsToProcessByLegacyApproach: Volumes that did not have any matching action i.e. action was nil from the volume policy specified by the user, these volumes will be processed via the legacy annotations based approach (fallback option)
		fsBackupVolumePolicyVols, nonFsBackupVolumePolicyVols, volsToProcessByLegacyApproach, err = v.GetVolumesMatchingFSBackupAction(pod, v.VolumePolicy, backupExcludePVC, kbclient)
		if err != nil {
			return fsBackupVolumePolicyVols, nonFsBackupVolumePolicyVols, err
		}
		// if volsToProcessByLegacyApproach is empty then no need to sue legacy approach as fallback option return from here
		if len(volsToProcessByLegacyApproach) == 0 {
			return fsBackupVolumePolicyVols, nonFsBackupVolumePolicyVols, nil
		}
	}

	// process legacy annotation based approach, this will done when:
	// 1. volume policy os specified by the user
	// 2. And there are some volumes for which the volume policy approach did not get any supported matching actions
	if v.VolumePolicy != nil && len(volsToProcessByLegacyApproach) > 0 {
		v.Logger.Infof("volume policy specified by the user but there are volumes with no matching action, using legacy approach based on annotations as a fallback for those volumes")
		includedVolumesFromLegacyFallBack, optedOutVolumesFromLegacyFallBack := pdvolumeutil.GetVolumesByPod(pod, defaultVolumesToFsBackup, backupExcludePVC, volsToProcessByLegacyApproach)
		// merge the volumePolicy approach and legacy Fallback lists
		fsBackupVolumePolicyVols = append(fsBackupVolumePolicyVols, includedVolumesFromLegacyFallBack...)
		nonFsBackupVolumePolicyVols = append(nonFsBackupVolumePolicyVols, optedOutVolumesFromLegacyFallBack...)
		return fsBackupVolumePolicyVols, nonFsBackupVolumePolicyVols, nil
	}
	// Normal legacy workflow
	// Get the list of volumes to back up using pod volume backup from the pod's annotations.
	// We will also pass the list of volume that did not have any supported volume policy action matched in legacy approach so that
	// those volumes get processed via legacy annotation based approach, this is a fallback option on annotation based legacy approach
	v.Logger.Infof("fs-backup or snapshot Volume Policy not specified by the user, using legacy approach based on annotations")
	includedVolumes, optedOutVolumes := pdvolumeutil.GetVolumesByPod(pod, defaultVolumesToFsBackup, backupExcludePVC, volsToProcessByLegacyApproach)
	return includedVolumes, optedOutVolumes, nil
}

// GetVolumesMatchingFSBackupAction returns a list of volume names to backup for the provided pod having fs-backup volume policy action
func (v *VolumeHelperImpl) GetVolumesMatchingFSBackupAction(pod *corev1api.Pod, volumePolicies *resourcepolicies.Policies, backupExcludePVC bool, kbclient kbclient.Client) ([]string, []string, []string, error) {
	FSBackupActionMatchingVols := []string{}
	FSBackupNonActionMatchingVols := []string{}
	NoActionMatchingVols := []string{}

	for i, vol := range pod.Spec.Volumes {
		if !v.ShouldIncludeVolumeInBackup(vol, backupExcludePVC) {
			continue
		}

		if vol.PersistentVolumeClaim != nil {
			// fetch the associated PVC first
			pvc, err := kubeutil.GetPVCForPodVolume(&pod.Spec.Volumes[i], pod, kbclient)
			if err != nil {
				return FSBackupActionMatchingVols, FSBackupNonActionMatchingVols, NoActionMatchingVols, err
			}
			// now fetch the PV and call GetMatchAction on it
			pv, err := kubeutil.GetPVForPVC(pvc, kbclient)
			if err != nil {
				return FSBackupActionMatchingVols, FSBackupNonActionMatchingVols, NoActionMatchingVols, err
			}
			// now get the action for pv
			action, err := volumePolicies.GetMatchAction(pv)
			if err != nil {
				return FSBackupActionMatchingVols, FSBackupNonActionMatchingVols, NoActionMatchingVols, err
			}

			// Record volume list having no matched action so that they are processed in legacy fallback option
			if action == nil {
				NoActionMatchingVols = append(NoActionMatchingVols, vol.Name)
			}

			// Now if the matched action is not nil and is `fs-backup` then add that Volume to the FSBackupActionMatchingVols
			// else add that volume to FSBackupNonActionMatchingVols
			// we already tracked the volume not matching any kind actions supported by volume policy in  NoActionMatchingVols
			// The NoActionMatchingVols list will be processed via legacy annotation based approach as a fallback option
			if action != nil && action.Type == resourcepolicies.FSBackup {
				FSBackupActionMatchingVols = append(FSBackupActionMatchingVols, vol.Name)
			} else if action != nil {
				FSBackupNonActionMatchingVols = append(FSBackupNonActionMatchingVols, vol.Name)
			}
		}
	}
	return FSBackupActionMatchingVols, FSBackupNonActionMatchingVols, NoActionMatchingVols, nil
}

func (v *VolumeHelperImpl) ShouldIncludeVolumeInBackup(vol corev1api.Volume, backupExcludePVC bool) bool {
	includeVolumeInBackup := true
	// cannot backup hostpath volumes as they are not mounted into /var/lib/kubelet/pods
	// and therefore not accessible to the node agent daemon set.
	if vol.HostPath != nil {
		includeVolumeInBackup = false
	}
	// don't backup volumes mounting secrets. Secrets will be backed up separately.
	if vol.Secret != nil {
		includeVolumeInBackup = false
	}
	// don't backup volumes mounting ConfigMaps. ConfigMaps will be backed up separately.
	if vol.ConfigMap != nil {
		includeVolumeInBackup = false
	}
	// don't backup volumes mounted as projected volumes, all data in those come from kube state.
	if vol.Projected != nil {
		includeVolumeInBackup = false
	}
	// don't backup DownwardAPI volumes, all data in those come from kube state.
	if vol.DownwardAPI != nil {
		includeVolumeInBackup = false
	}
	if vol.PersistentVolumeClaim != nil && backupExcludePVC {
		includeVolumeInBackup = false
	}
	// don't include volumes that mount the default service account token.
	if strings.HasPrefix(vol.Name, "default-token") {
		includeVolumeInBackup = false
	}
	return includeVolumeInBackup
}
