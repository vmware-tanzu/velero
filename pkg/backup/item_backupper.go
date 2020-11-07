/*
Copyright 2020 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backup

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	"github.com/vmware-tanzu/velero/internal/hook"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

// itemBackupper can back up individual items to a tar writer.
type itemBackupper struct {
	backupRequest           *Request
	tarWriter               tarWriter
	dynamicFactory          client.DynamicFactory
	discoveryHelper         discovery.Helper
	resticBackupper         restic.Backupper
	resticSnapshotTracker   *pvcSnapshotTracker
	volumeSnapshotterGetter VolumeSnapshotterGetter

	itemHookHandler                    hook.ItemHookHandler
	snapshotLocationVolumeSnapshotters map[string]velero.VolumeSnapshotter
}

// backupItem backs up an individual item to tarWriter. The item may be excluded based on the
// namespaces IncludesExcludes list.
// In addition to the error return, backupItem also returns a bool indicating whether the item
// was actually backed up.
func (ib *itemBackupper) backupItem(logger logrus.FieldLogger, obj runtime.Unstructured, groupResource schema.GroupResource, preferredGVR schema.GroupVersionResource) (bool, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	namespace := metadata.GetNamespace()
	name := metadata.GetName()

	log := logger.WithField("name", name)
	log = log.WithField("resource", groupResource.String())
	log = log.WithField("namespace", namespace)

	if metadata.GetLabels()["velero.io/exclude-from-backup"] == "true" {
		log.Info("Excluding item because it has label velero.io/exclude-from-backup=true")
		return false, nil
	}

	// NOTE: we have to re-check namespace & resource includes/excludes because it's possible that
	// backupItem can be invoked by a custom action.
	if namespace != "" && !ib.backupRequest.NamespaceIncludesExcludes.ShouldInclude(namespace) {
		log.Info("Excluding item because namespace is excluded")
		return false, nil
	}

	// NOTE: we specifically allow namespaces to be backed up even if IncludeClusterResources is
	// false.
	if namespace == "" && groupResource != kuberesource.Namespaces && ib.backupRequest.Spec.IncludeClusterResources != nil && !*ib.backupRequest.Spec.IncludeClusterResources {
		log.Info("Excluding item because resource is cluster-scoped and backup.spec.includeClusterResources is false")
		return false, nil
	}

	if !ib.backupRequest.ResourceIncludesExcludes.ShouldInclude(groupResource.String()) {
		log.Info("Excluding item because resource is excluded")
		return false, nil
	}

	if metadata.GetDeletionTimestamp() != nil {
		log.Info("Skipping item because it's being deleted.")
		return false, nil
	}

	key := itemKey{
		resource:  resourceKey(obj),
		namespace: namespace,
		name:      name,
	}

	if _, exists := ib.backupRequest.BackedUpItems[key]; exists {
		log.Info("Skipping item because it's already been backed up.")
		// returning true since this item *is* in the backup, even though we're not backing it up here
		return true, nil
	}
	ib.backupRequest.BackedUpItems[key] = struct{}{}

	log.Info("Backing up item")

	log.Debug("Executing pre hooks")
	if err := ib.itemHookHandler.HandleHooks(log, groupResource, obj, ib.backupRequest.ResourceHooks, hook.PhasePre); err != nil {
		return false, err
	}

	var (
		backupErrs            []error
		pod                   *corev1api.Pod
		resticVolumesToBackup []string
	)

	if groupResource == kuberesource.Pods {
		// pod needs to be initialized for the unstructured converter
		pod = new(corev1api.Pod)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pod); err != nil {
			backupErrs = append(backupErrs, errors.WithStack(err))
			// nil it on error since it's not valid
			pod = nil
		} else {
			// Get the list of volumes to back up using restic from the pod's annotations. Remove from this list
			// any volumes that use a PVC that we've already backed up (this would be in a read-write-many scenario,
			// where it's been backed up from another pod), since we don't need >1 backup per PVC.
			for _, volume := range restic.GetPodVolumesUsingRestic(pod, boolptr.IsSetToTrue(ib.backupRequest.Spec.DefaultVolumesToRestic)) {
				if found, pvcName := ib.resticSnapshotTracker.HasPVCForPodVolume(pod, volume); found {
					log.WithFields(map[string]interface{}{
						"podVolume": volume,
						"pvcName":   pvcName,
					}).Info("Pod volume uses a persistent volume claim which has already been backed up with restic from another pod, skipping.")
					continue
				}

				resticVolumesToBackup = append(resticVolumesToBackup, volume)
			}

			// track the volumes that are PVCs using the PVC snapshot tracker, so that when we backup PVCs/PVs
			// via an item action in the next step, we don't snapshot PVs that will have their data backed up
			// with restic.
			ib.resticSnapshotTracker.Track(pod, resticVolumesToBackup)
		}
	}

	// capture the version of the object before invoking plugin actions as the plugin may update
	// the group version of the object.
	// group version of this object
	// Used on filepath to backup up all groups and versions
	version := resourceVersion(obj)

	updatedObj, err := ib.executeActions(log, obj, groupResource, name, namespace, metadata)
	if err != nil {
		backupErrs = append(backupErrs, err)

		// if there was an error running actions, execute post hooks and return
		log.Debug("Executing post hooks")
		if err := ib.itemHookHandler.HandleHooks(log, groupResource, obj, ib.backupRequest.ResourceHooks, hook.PhasePost); err != nil {
			backupErrs = append(backupErrs, err)
		}

		return false, kubeerrs.NewAggregate(backupErrs)
	}
	obj = updatedObj
	if metadata, err = meta.Accessor(obj); err != nil {
		return false, errors.WithStack(err)
	}
	// update name and namespace in case they were modified in an action
	name = metadata.GetName()
	namespace = metadata.GetNamespace()

	if groupResource == kuberesource.PersistentVolumes {
		if err := ib.takePVSnapshot(obj, log); err != nil {
			backupErrs = append(backupErrs, err)
		}
	}

	if groupResource == kuberesource.Pods && pod != nil {
		// this function will return partial results, so process podVolumeBackups
		// even if there are errors.
		podVolumeBackups, errs := ib.backupPodVolumes(log, pod, resticVolumesToBackup)

		ib.backupRequest.PodVolumeBackups = append(ib.backupRequest.PodVolumeBackups, podVolumeBackups...)
		backupErrs = append(backupErrs, errs...)
	}

	log.Debug("Executing post hooks")
	if err := ib.itemHookHandler.HandleHooks(log, groupResource, obj, ib.backupRequest.ResourceHooks, hook.PhasePost); err != nil {
		backupErrs = append(backupErrs, err)
	}

	if len(backupErrs) != 0 {
		return false, kubeerrs.NewAggregate(backupErrs)
	}

	// Getting the preferred group version of this resource
	preferredVersion := preferredGVR.Version

	var filePath string

	// API Group version is now part of path of backup as a subdirectory
	// it will add a prefix to subdirectory name for the preferred version
	versionPath := version

	if version == preferredVersion {
		versionPath = version + velerov1api.PreferredVersionDir
	}

	if namespace != "" {
		filePath = filepath.Join(velerov1api.ResourcesDir, groupResource.String(), versionPath, velerov1api.NamespaceScopedDir, namespace, name+".json")
	} else {
		filePath = filepath.Join(velerov1api.ResourcesDir, groupResource.String(), versionPath, velerov1api.ClusterScopedDir, name+".json")
	}

	itemBytes, err := json.Marshal(obj.UnstructuredContent())
	if err != nil {
		return false, errors.WithStack(err)
	}

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(itemBytes)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}

	if err := ib.tarWriter.WriteHeader(hdr); err != nil {
		return false, errors.WithStack(err)
	}

	if _, err := ib.tarWriter.Write(itemBytes); err != nil {
		return false, errors.WithStack(err)
	}

	// backing up the preferred version backup without API Group version on path -  this is for backward compatibility

	log.Debugf("Resource %s/%s, version= %s, preferredVersion=%s", groupResource.String(), name, version, preferredVersion)
	if version == preferredVersion {
		if namespace != "" {
			filePath = filepath.Join(velerov1api.ResourcesDir, groupResource.String(), velerov1api.NamespaceScopedDir, namespace, name+".json")
		} else {
			filePath = filepath.Join(velerov1api.ResourcesDir, groupResource.String(), velerov1api.ClusterScopedDir, name+".json")
		}

		hdr = &tar.Header{
			Name:     filePath,
			Size:     int64(len(itemBytes)),
			Typeflag: tar.TypeReg,
			Mode:     0755,
			ModTime:  time.Now(),
		}

		if err := ib.tarWriter.WriteHeader(hdr); err != nil {
			return false, errors.WithStack(err)
		}

		if _, err := ib.tarWriter.Write(itemBytes); err != nil {
			return false, errors.WithStack(err)
		}
	}

	return true, nil
}

// backupPodVolumes triggers restic backups of the specified pod volumes, and returns a list of PodVolumeBackups
// for volumes that were successfully backed up, and a slice of any errors that were encountered.
func (ib *itemBackupper) backupPodVolumes(log logrus.FieldLogger, pod *corev1api.Pod, volumes []string) ([]*velerov1api.PodVolumeBackup, []error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	if ib.resticBackupper == nil {
		log.Warn("No restic backupper, not backing up pod's volumes")
		return nil, nil
	}

	return ib.resticBackupper.BackupPodVolumes(ib.backupRequest.Backup, pod, volumes, log)
}

func (ib *itemBackupper) executeActions(
	log logrus.FieldLogger,
	obj runtime.Unstructured,
	groupResource schema.GroupResource,
	name, namespace string,
	metadata metav1.Object,
) (runtime.Unstructured, error) {
	for _, action := range ib.backupRequest.ResolvedActions {
		if !action.resourceIncludesExcludes.ShouldInclude(groupResource.String()) {
			log.Debug("Skipping action because it does not apply to this resource")
			continue
		}

		if namespace != "" && !action.namespaceIncludesExcludes.ShouldInclude(namespace) {
			log.Debug("Skipping action because it does not apply to this namespace")
			continue
		}

		if namespace == "" && !action.namespaceIncludesExcludes.IncludeEverything() {
			log.Debug("Skipping action because resource is cluster-scoped and action only applies to specific namespaces")
			continue
		}

		if !action.selector.Matches(labels.Set(metadata.GetLabels())) {
			log.Debug("Skipping action because label selector does not match")
			continue
		}

		log.Info("Executing custom action")

		updatedItem, additionalItemIdentifiers, err := action.Execute(obj, ib.backupRequest.Backup)
		if err != nil {
			return nil, errors.Wrapf(err, "error executing custom action (groupResource=%s, namespace=%s, name=%s)", groupResource.String(), namespace, name)
		}
		obj = updatedItem

		for _, additionalItem := range additionalItemIdentifiers {
			gvr, resource, err := ib.discoveryHelper.ResourceFor(additionalItem.GroupResource.WithVersion(""))
			if err != nil {
				return nil, err
			}

			client, err := ib.dynamicFactory.ClientForGroupVersionResource(gvr.GroupVersion(), resource, additionalItem.Namespace)
			if err != nil {
				return nil, err
			}

			item, err := client.Get(additionalItem.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				log.WithFields(logrus.Fields{
					"groupResource": additionalItem.GroupResource,
					"namespace":     additionalItem.Namespace,
					"name":          additionalItem.Name,
				}).Warnf("Additional item was not found in Kubernetes API, can't back it up")
				continue
			}
			if err != nil {
				return nil, errors.WithStack(err)
			}

			if _, err = ib.backupItem(log, item, gvr.GroupResource(), gvr); err != nil {
				return nil, err
			}
		}
	}

	return obj, nil
}

// volumeSnapshotter instantiates and initializes a VolumeSnapshotter given a VolumeSnapshotLocation,
// or returns an existing one if one's already been initialized for the location.
func (ib *itemBackupper) volumeSnapshotter(snapshotLocation *velerov1api.VolumeSnapshotLocation) (velero.VolumeSnapshotter, error) {
	if bs, ok := ib.snapshotLocationVolumeSnapshotters[snapshotLocation.Name]; ok {
		return bs, nil
	}

	bs, err := ib.volumeSnapshotterGetter.GetVolumeSnapshotter(snapshotLocation.Spec.Provider)
	if err != nil {
		return nil, err
	}

	if err := bs.Init(snapshotLocation.Spec.Config); err != nil {
		return nil, err
	}

	if ib.snapshotLocationVolumeSnapshotters == nil {
		ib.snapshotLocationVolumeSnapshotters = make(map[string]velero.VolumeSnapshotter)
	}
	ib.snapshotLocationVolumeSnapshotters[snapshotLocation.Name] = bs

	return bs, nil
}

// zoneLabelDeprecated is the label that stores availability-zone info
// on PVs this is deprecated on Kubernetes >= 1.17.0
// zoneLabel is the label that stores availability-zone info
// on PVs
const (
	zoneLabelDeprecated = "failure-domain.beta.kubernetes.io/zone"
	zoneLabel           = "topology.kubernetes.io/zone"
)

// takePVSnapshot triggers a snapshot for the volume/disk underlying a PersistentVolume if the provided
// backup has volume snapshots enabled and the PV is of a compatible type. Also records cloud
// disk type and IOPS (if applicable) to be able to restore to current state later.
func (ib *itemBackupper) takePVSnapshot(obj runtime.Unstructured, log logrus.FieldLogger) error {
	log.Info("Executing takePVSnapshot")

	if boolptr.IsSetToFalse(ib.backupRequest.Spec.SnapshotVolumes) {
		log.Info("Backup has volume snapshots disabled; skipping volume snapshot action.")
		return nil
	}

	pv := new(corev1api.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pv); err != nil {
		return errors.WithStack(err)
	}

	log = log.WithField("persistentVolume", pv.Name)

	// If this PV is claimed, see if we've already taken a (restic) snapshot of the contents
	// of this PV. If so, don't take a snapshot.
	if pv.Spec.ClaimRef != nil {
		if ib.resticSnapshotTracker.Has(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name) {
			log.Info("Skipping snapshot of persistent volume because volume is being backed up with restic.")
			return nil
		}
	}

	// TODO: -- once failure-domain.beta.kubernetes.io/zone is no longer
	// supported in any velero-supported version of Kubernetes, remove fallback checking of it
	pvFailureDomainZone, labelFound := pv.Labels[zoneLabel]
	if !labelFound {
		log.Infof("label %q is not present on PersistentVolume, checking deprecated label...", zoneLabel)
		pvFailureDomainZone, labelFound = pv.Labels[zoneLabelDeprecated]
		if !labelFound {
			log.Infof("label %q is not present on PersistentVolume", zoneLabelDeprecated)
		}
	}

	var (
		volumeID, location string
		volumeSnapshotter  velero.VolumeSnapshotter
	)

	for _, snapshotLocation := range ib.backupRequest.SnapshotLocations {
		log := log.WithField("volumeSnapshotLocation", snapshotLocation.Name)

		bs, err := ib.volumeSnapshotter(snapshotLocation)
		if err != nil {
			log.WithError(err).Error("Error getting volume snapshotter for volume snapshot location")
			continue
		}

		if volumeID, err = bs.GetVolumeID(obj); err != nil {
			log.WithError(err).Errorf("Error attempting to get volume ID for persistent volume")
			continue
		}
		if volumeID == "" {
			log.Infof("No volume ID returned by volume snapshotter for persistent volume")
			continue
		}

		log.Infof("Got volume ID for persistent volume")
		volumeSnapshotter = bs
		location = snapshotLocation.Name
		break
	}

	if volumeSnapshotter == nil {
		log.Info("Persistent volume is not a supported volume type for snapshots, skipping.")
		return nil
	}

	log = log.WithField("volumeID", volumeID)

	// create tags from the backup's labels
	tags := map[string]string{}
	for k, v := range ib.backupRequest.GetLabels() {
		tags[k] = v
	}
	tags["velero.io/backup"] = ib.backupRequest.Name
	tags["velero.io/pv"] = pv.Name

	log.Info("Getting volume information")
	volumeType, iops, err := volumeSnapshotter.GetVolumeInfo(volumeID, pvFailureDomainZone)
	if err != nil {
		return errors.WithMessage(err, "error getting volume info")
	}

	log.Info("Snapshotting persistent volume")
	snapshot := volumeSnapshot(ib.backupRequest.Backup, pv.Name, volumeID, volumeType, pvFailureDomainZone, location, iops)

	var errs []error
	snapshotID, err := volumeSnapshotter.CreateSnapshot(snapshot.Spec.ProviderVolumeID, snapshot.Spec.VolumeAZ, tags)
	if err != nil {
		errs = append(errs, errors.Wrap(err, "error taking snapshot of volume"))
		snapshot.Status.Phase = volume.SnapshotPhaseFailed
	} else {
		snapshot.Status.Phase = volume.SnapshotPhaseCompleted
		snapshot.Status.ProviderSnapshotID = snapshotID
	}
	ib.backupRequest.VolumeSnapshots = append(ib.backupRequest.VolumeSnapshots, snapshot)

	// nil errors are automatically removed
	return kubeerrs.NewAggregate(errs)
}

func volumeSnapshot(backup *velerov1api.Backup, volumeName, volumeID, volumeType, az, location string, iops *int64) *volume.Snapshot {
	return &volume.Snapshot{
		Spec: volume.SnapshotSpec{
			BackupName:           backup.Name,
			BackupUID:            string(backup.UID),
			Location:             location,
			PersistentVolumeName: volumeName,
			ProviderVolumeID:     volumeID,
			VolumeType:           volumeType,
			VolumeAZ:             az,
			VolumeIOPS:           iops,
		},
		Status: volume.SnapshotStatus{
			Phase: volume.SnapshotPhaseNew,
		},
	}
}

// resourceKey returns a string representing the object's GroupVersionKind (e.g.
// apps/v1/Deployment).
func resourceKey(obj runtime.Unstructured) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s/%s", gvk.GroupVersion().String(), gvk.Kind)
}

// resourceVersion returns a string representing the object's API Version (e.g.
// v1 if item belongs to apps/v1
func resourceVersion(obj runtime.Unstructured) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return gvk.Version
}
