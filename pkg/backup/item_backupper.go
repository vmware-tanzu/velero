/*
Copyright 2017 the Heptio Ark contributors.

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
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/kuberesource"
	"github.com/heptio/ark/pkg/podexec"
	"github.com/heptio/ark/pkg/restic"
	"github.com/heptio/ark/pkg/volume"
)

type itemBackupperFactory interface {
	newItemBackupper(
		backup *Request,
		backedUpItems map[itemKey]struct{},
		podCommandExecutor podexec.PodCommandExecutor,
		tarWriter tarWriter,
		dynamicFactory client.DynamicFactory,
		discoveryHelper discovery.Helper,
		resticBackupper restic.Backupper,
		resticSnapshotTracker *pvcSnapshotTracker,
		blockStoreGetter BlockStoreGetter,
	) ItemBackupper
}

type defaultItemBackupperFactory struct{}

func (f *defaultItemBackupperFactory) newItemBackupper(
	backupRequest *Request,
	backedUpItems map[itemKey]struct{},
	podCommandExecutor podexec.PodCommandExecutor,
	tarWriter tarWriter,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	resticBackupper restic.Backupper,
	resticSnapshotTracker *pvcSnapshotTracker,
	blockStoreGetter BlockStoreGetter,
) ItemBackupper {
	ib := &defaultItemBackupper{
		backupRequest:         backupRequest,
		backedUpItems:         backedUpItems,
		tarWriter:             tarWriter,
		dynamicFactory:        dynamicFactory,
		discoveryHelper:       discoveryHelper,
		resticBackupper:       resticBackupper,
		resticSnapshotTracker: resticSnapshotTracker,
		blockStoreGetter:      blockStoreGetter,

		itemHookHandler: &defaultItemHookHandler{
			podCommandExecutor: podCommandExecutor,
		},
	}

	// this is for testing purposes
	ib.additionalItemBackupper = ib

	return ib
}

type ItemBackupper interface {
	backupItem(logger logrus.FieldLogger, obj runtime.Unstructured, groupResource schema.GroupResource) error
}

type defaultItemBackupper struct {
	backupRequest         *Request
	backedUpItems         map[itemKey]struct{}
	tarWriter             tarWriter
	dynamicFactory        client.DynamicFactory
	discoveryHelper       discovery.Helper
	resticBackupper       restic.Backupper
	resticSnapshotTracker *pvcSnapshotTracker
	blockStoreGetter      BlockStoreGetter

	itemHookHandler             itemHookHandler
	additionalItemBackupper     ItemBackupper
	snapshotLocationBlockStores map[string]cloudprovider.BlockStore
}

// backupItem backs up an individual item to tarWriter. The item may be excluded based on the
// namespaces IncludesExcludes list.
func (ib *defaultItemBackupper) backupItem(logger logrus.FieldLogger, obj runtime.Unstructured, groupResource schema.GroupResource) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	namespace := metadata.GetNamespace()
	name := metadata.GetName()

	log := logger.WithField("name", name)
	if namespace != "" {
		log = log.WithField("namespace", namespace)
	}

	// NOTE: we have to re-check namespace & resource includes/excludes because it's possible that
	// backupItem can be invoked by a custom action.
	if namespace != "" && !ib.backupRequest.NamespaceIncludesExcludes.ShouldInclude(namespace) {
		log.Info("Excluding item because namespace is excluded")
		return nil
	}

	// NOTE: we specifically allow namespaces to be backed up even if IncludeClusterResources is
	// false.
	if namespace == "" && groupResource != kuberesource.Namespaces && ib.backupRequest.Spec.IncludeClusterResources != nil && !*ib.backupRequest.Spec.IncludeClusterResources {
		log.Info("Excluding item because resource is cluster-scoped and backup.spec.includeClusterResources is false")
		return nil
	}

	if !ib.backupRequest.ResourceIncludesExcludes.ShouldInclude(groupResource.String()) {
		log.Info("Excluding item because resource is excluded")
		return nil
	}

	if metadata.GetDeletionTimestamp() != nil {
		log.Info("Skipping item because it's being deleted.")
		return nil
	}
	key := itemKey{
		resource:  groupResource.String(),
		namespace: namespace,
		name:      name,
	}

	if _, exists := ib.backedUpItems[key]; exists {
		log.Info("Skipping item because it's already been backed up.")
		return nil
	}
	ib.backedUpItems[key] = struct{}{}

	log.Info("Backing up resource")

	log.Debug("Executing pre hooks")
	if err := ib.itemHookHandler.handleHooks(log, groupResource, obj, ib.backupRequest.ResourceHooks, hookPhasePre); err != nil {
		return err
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
			// get the volumes to backup using restic, and add any of them that are PVCs to the pvc snapshot
			// tracker, so that when we backup PVCs/PVs via an item action in the next step, we don't snapshot
			// PVs that will have their data backed up with restic.
			resticVolumesToBackup = restic.GetVolumesToBackup(pod)

			ib.resticSnapshotTracker.Track(pod, resticVolumesToBackup)
		}
	}

	updatedObj, err := ib.executeActions(log, obj, groupResource, name, namespace, metadata)
	if err != nil {
		log.WithError(err).Error("Error executing item actions")
		backupErrs = append(backupErrs, err)

		// if there was an error running actions, execute post hooks and return
		log.Debug("Executing post hooks")
		if err := ib.itemHookHandler.handleHooks(log, groupResource, obj, ib.backupRequest.ResourceHooks, hookPhasePost); err != nil {
			backupErrs = append(backupErrs, err)
		}

		return kubeerrs.NewAggregate(backupErrs)
	}
	obj = updatedObj
	if metadata, err = meta.Accessor(obj); err != nil {
		return errors.WithStack(err)
	}

	if groupResource == kuberesource.PersistentVolumes {
		if err := ib.takePVSnapshot(obj, log); err != nil {
			backupErrs = append(backupErrs, err)
		}
	}

	if groupResource == kuberesource.Pods && pod != nil {
		// this function will return partial results, so process volumeSnapshots
		// even if there are errors.
		volumeSnapshots, errs := ib.backupPodVolumes(log, pod, resticVolumesToBackup)

		// annotate the pod with the successful volume snapshots
		for volume, snapshot := range volumeSnapshots {
			restic.SetPodSnapshotAnnotation(metadata, volume, snapshot)
		}

		backupErrs = append(backupErrs, errs...)
	}

	log.Debug("Executing post hooks")
	if err := ib.itemHookHandler.handleHooks(log, groupResource, obj, ib.backupRequest.ResourceHooks, hookPhasePost); err != nil {
		backupErrs = append(backupErrs, err)
	}

	if len(backupErrs) != 0 {
		return kubeerrs.NewAggregate(backupErrs)
	}

	var filePath string
	if namespace != "" {
		filePath = filepath.Join(api.ResourcesDir, groupResource.String(), api.NamespaceScopedDir, namespace, name+".json")
	} else {
		filePath = filepath.Join(api.ResourcesDir, groupResource.String(), api.ClusterScopedDir, name+".json")
	}

	itemBytes, err := json.Marshal(obj.UnstructuredContent())
	if err != nil {
		return errors.WithStack(err)
	}

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(itemBytes)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}

	if err := ib.tarWriter.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}

	if _, err := ib.tarWriter.Write(itemBytes); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// backupPodVolumes triggers restic backups of the specified pod volumes, and returns a map of volume name -> snapshot ID
// for volumes that were successfully backed up, and a slice of any errors that were encountered.
func (ib *defaultItemBackupper) backupPodVolumes(log logrus.FieldLogger, pod *corev1api.Pod, volumes []string) (map[string]string, []error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	if ib.resticBackupper == nil {
		log.Warn("No restic backupper, not backing up pod's volumes")
		return nil, nil
	}

	return ib.resticBackupper.BackupPodVolumes(ib.backupRequest.Backup, pod, log)
}

func (ib *defaultItemBackupper) executeActions(
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

		if !action.selector.Matches(labels.Set(metadata.GetLabels())) {
			log.Debug("Skipping action because label selector does not match")
			continue
		}

		log.Info("Executing custom action")

		updatedItem, additionalItemIdentifiers, err := action.Execute(obj, ib.backupRequest.Backup)
		if err != nil {
			// We want this to show up in the log file at the place where the error occurs. When we return
			// the error, it get aggregated with all the other ones at the end of the backup, making it
			// harder to tell when it happened.
			log.WithError(err).Error("error executing custom action")

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

			additionalItem, err := client.Get(additionalItem.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			if err = ib.additionalItemBackupper.backupItem(log, additionalItem, gvr.GroupResource()); err != nil {
				return nil, err
			}
		}
	}

	return obj, nil
}

// blockStore instantiates and initializes a BlockStore given a VolumeSnapshotLocation,
// or returns an existing one if one's already been initialized for the location.
func (ib *defaultItemBackupper) blockStore(snapshotLocation *api.VolumeSnapshotLocation) (cloudprovider.BlockStore, error) {
	if bs, ok := ib.snapshotLocationBlockStores[snapshotLocation.Name]; ok {
		return bs, nil
	}

	bs, err := ib.blockStoreGetter.GetBlockStore(snapshotLocation.Spec.Provider)
	if err != nil {
		return nil, err
	}

	if err := bs.Init(snapshotLocation.Spec.Config); err != nil {
		return nil, err
	}

	if ib.snapshotLocationBlockStores == nil {
		ib.snapshotLocationBlockStores = make(map[string]cloudprovider.BlockStore)
	}
	ib.snapshotLocationBlockStores[snapshotLocation.Name] = bs

	return bs, nil
}

// zoneLabel is the label that stores availability-zone info
// on PVs
const zoneLabel = "failure-domain.beta.kubernetes.io/zone"

// takePVSnapshot triggers a snapshot for the volume/disk underlying a PersistentVolume if the provided
// backup has volume snapshots enabled and the PV is of a compatible type. Also records cloud
// disk type and IOPS (if applicable) to be able to restore to current state later.
func (ib *defaultItemBackupper) takePVSnapshot(obj runtime.Unstructured, log logrus.FieldLogger) error {
	log.Info("Executing takePVSnapshot")

	if ib.backupRequest.Spec.SnapshotVolumes != nil && !*ib.backupRequest.Spec.SnapshotVolumes {
		log.Info("Backup has volume snapshots disabled; skipping volume snapshot action.")
		return nil
	}

	pv := new(corev1api.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pv); err != nil {
		return errors.WithStack(err)
	}

	// If this PV is claimed, see if we've already taken a (restic) snapshot of the contents
	// of this PV. If so, don't take a snapshot.
	if pv.Spec.ClaimRef != nil {
		if ib.resticSnapshotTracker.Has(pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name) {
			log.Info("Skipping Persistent Volume snapshot because volume has already been backed up.")
			return nil
		}
	}

	metadata, err := meta.Accessor(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	pvFailureDomainZone := metadata.GetLabels()[zoneLabel]
	if pvFailureDomainZone == "" {
		log.Infof("label %q is not present on PersistentVolume", zoneLabel)
	}

	var (
		volumeID, location string
		blockStore         cloudprovider.BlockStore
	)

	for _, snapshotLocation := range ib.backupRequest.SnapshotLocations {
		bs, err := ib.blockStore(snapshotLocation)
		if err != nil {
			log.WithError(err).WithField("volumeSnapshotLocation", snapshotLocation).Error("Error getting block store for volume snapshot location")
			continue
		}

		log := log.WithFields(map[string]interface{}{
			"volumeSnapshotLocation": snapshotLocation.Name,
			"persistentVolume":       metadata.GetName(),
		})

		if volumeID, err = bs.GetVolumeID(obj); err != nil {
			log.WithError(err).Errorf("Error attempting to get volume ID for persistent volume")
			continue
		}
		if volumeID == "" {
			log.Infof("No volume ID returned by block store for persistent volume")
			continue
		}

		log.Infof("Got volume ID for persistent volume")
		blockStore = bs
		location = snapshotLocation.Name
		break
	}

	if blockStore == nil {
		log.Info("PersistentVolume is not a supported volume type for snapshots, skipping.")
		return nil
	}

	log = log.WithField("volumeID", volumeID)

	tags := map[string]string{
		"ark.heptio.com/backup": ib.backupRequest.Name,
		"ark.heptio.com/pv":     metadata.GetName(),
	}

	log.Info("Getting volume information")
	volumeType, iops, err := blockStore.GetVolumeInfo(volumeID, pvFailureDomainZone)
	if err != nil {
		log.WithError(err).Error("error getting volume info")
		return errors.WithMessage(err, "error getting volume info")
	}

	log.Info("Snapshotting PersistentVolume")
	snapshot := volumeSnapshot(ib.backupRequest.Backup, metadata.GetName(), volumeID, volumeType, pvFailureDomainZone, location, iops)

	var errs []error
	snapshotID, err := blockStore.CreateSnapshot(snapshot.Spec.ProviderVolumeID, snapshot.Spec.VolumeAZ, tags)
	if err != nil {
		log.WithError(err).Error("error creating snapshot")
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

func volumeSnapshot(backup *api.Backup, volumeName, volumeID, volumeType, az, location string, iops *int64) *volume.Snapshot {
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
