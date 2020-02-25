package backup

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

// Processor handles end-to-end processing of a Backup request, from validation
// and defaulting to execution & persistence to updating of the request object.
type Processor struct {
	client                 velerov1client.BackupsGetter
	backupper              Backupper
	backupTracker          Tracker
	backupLocationLister   velerov1listers.BackupStorageLocationLister
	snapshotLocationLister velerov1listers.VolumeSnapshotLocationLister
	log                    logrus.FieldLogger
	metrics                *metrics.ServerMetrics

	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore   func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)

	defaultBackupLocation    string
	defaultSnapshotLocations map[string]string
	defaultBackupTTL         time.Duration
	backupLogLevel           logrus.Level
	formatFlag               logging.Format

	clock clock.Clock
}

func NewProcessor(
	client velerov1client.BackupsGetter,
	backupper Backupper,
	backupTracker Tracker,
	backupLocationLister velerov1listers.BackupStorageLocationLister,
	snapshotLocationLister velerov1listers.VolumeSnapshotLocationLister,
	log logrus.FieldLogger,
	metrics *metrics.ServerMetrics,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	defaultBackupLocation string,
	defaultSnapshotLocations map[string]string,
	defaultBackupTTL time.Duration,
	backupLogLevel logrus.Level,
	formatFlag logging.Format,
) *Processor {
	return &Processor{
		client:                 client,
		backupper:              backupper,
		backupTracker:          backupTracker,
		backupLocationLister:   backupLocationLister,
		snapshotLocationLister: snapshotLocationLister,
		log:                    log,
		metrics:                metrics,

		newPluginManager: newPluginManager,
		newBackupStore:   persistence.NewObjectBackupStore,

		defaultBackupLocation:    defaultBackupLocation,
		defaultSnapshotLocations: defaultSnapshotLocations,
		defaultBackupTTL:         defaultBackupTTL,
		backupLogLevel:           backupLogLevel,
		formatFlag:               formatFlag,

		clock: &clock.RealClock{},
	}
}

func (p *Processor) Process(backup *velerov1api.Backup) error {
	// Double-check we have the correct phase. In the unlikely event that multiple controller
	// instances are running, it's possible for controller A to succeed in changing the phase to
	// InProgress, while controller B's attempt to patch the phase fails. When controller B
	// reprocesses the same backup, it will either show up as New (informer hasn't seen the update
	// yet) or as InProgress. In the former case, the patch attempt will fail again, until the
	// informer sees the update. In the latter case, after the informer has seen the update to
	// InProgress, we still need this check so we can return nil to indicate we've finished processing
	// this key (even though it was a no-op).
	switch backup.Status.Phase {
	case "", velerov1api.BackupPhaseNew:
		// only process new backups
	default:
		return nil
	}

	log := p.log.WithField("backup", kubeutil.NamespaceAndName(backup))

	log.Debug("Preparing backup request")
	request := p.prepareBackupRequest(backup)

	if len(request.Status.ValidationErrors) > 0 {
		request.Status.Phase = velerov1api.BackupPhaseFailedValidation
	} else {
		request.Status.Phase = velerov1api.BackupPhaseInProgress
		request.Status.StartTimestamp = &metav1.Time{Time: p.clock.Now()}
	}

	// update status
	updatedBackup, err := patchBackup(backup, request.Backup, p.client)
	if err != nil {
		return errors.Wrapf(err, "error updating Backup status to %s", request.Status.Phase)
	}
	// store ref to just-updated item for creating patch
	backup = updatedBackup
	request.Backup = updatedBackup.DeepCopy()

	if request.Status.Phase == velerov1api.BackupPhaseFailedValidation {
		return nil
	}

	p.backupTracker.Add(request.Namespace, request.Name)
	defer p.backupTracker.Delete(request.Namespace, request.Name)

	log.Debug("Running backup")

	backupScheduleName := request.GetLabels()[velerov1api.ScheduleNameLabel]
	p.metrics.RegisterBackupAttempt(backupScheduleName)

	// execution & upload of backup
	if err := p.runBackup(request); err != nil {
		// even though runBackup sets the backup's phase prior
		// to uploading artifacts to object storage, we have to
		// check for an error again here and update the phase if
		// one is found, because there could've been an error
		// while uploading artifacts to object storage, which would
		// result in the backup being Failed.
		log.WithError(err).Error("backup failed")
		request.Status.Phase = velerov1api.BackupPhaseFailed
	}

	switch request.Status.Phase {
	case velerov1api.BackupPhaseCompleted:
		p.metrics.RegisterBackupSuccess(backupScheduleName)
	case velerov1api.BackupPhasePartiallyFailed:
		p.metrics.RegisterBackupPartialFailure(backupScheduleName)
	case velerov1api.BackupPhaseFailed:
		p.metrics.RegisterBackupFailed(backupScheduleName)
	}

	log.Debug("Updating backup's final status")
	if _, err := patchBackup(backup, request.Backup, p.client); err != nil {
		log.WithError(err).Error("error updating backup's final status")
	}

	return nil
}

func (p *Processor) prepareBackupRequest(backup *velerov1api.Backup) *Request {
	request := &Request{
		Backup: backup.DeepCopy(), // don't modify items in the cache
	}

	// set backup version
	request.Status.Version = BackupVersion

	if request.Spec.TTL.Duration == 0 {
		// set default backup TTL
		request.Spec.TTL.Duration = p.defaultBackupTTL
	}

	// calculate expiration
	request.Status.Expiration = &metav1.Time{Time: p.clock.Now().Add(request.Spec.TTL.Duration)}

	// default storage location if not specified
	if request.Spec.StorageLocation == "" {
		request.Spec.StorageLocation = p.defaultBackupLocation
	}

	// add the storage location as a label for easy filtering later.
	if request.Labels == nil {
		request.Labels = make(map[string]string)
	}
	request.Labels[velerov1api.StorageLocationLabel] = label.GetValidName(request.Spec.StorageLocation)

	// validate the included/excluded resources
	for _, err := range collections.ValidateIncludesExcludes(request.Spec.IncludedResources, request.Spec.ExcludedResources) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	// validate the included/excluded namespaces
	for _, err := range collections.ValidateIncludesExcludes(request.Spec.IncludedNamespaces, request.Spec.ExcludedNamespaces) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	// validate the storage location, and store the BackupStorageLocation API obj on the request
	if storageLocation, err := p.backupLocationLister.BackupStorageLocations(request.Namespace).Get(request.Spec.StorageLocation); err != nil {
		if apierrors.IsNotFound(err) {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("a BackupStorageLocation CRD with the name specified in the backup spec needs to be created before this backup can be executed. Error: %v", err))
		} else {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("error getting backup storage location: %v", err))
		}
	} else {
		request.StorageLocation = storageLocation

		if request.StorageLocation.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors,
				fmt.Sprintf("backup can't be created because backup storage location %s is currently in read-only mode", request.StorageLocation.Name))
		}
	}

	// validate and get the backup's VolumeSnapshotLocations, and store the
	// VolumeSnapshotLocation API objs on the request
	if locs, errs := p.validateAndGetSnapshotLocations(request.Backup); len(errs) > 0 {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, errs...)
	} else {
		request.Spec.VolumeSnapshotLocations = nil
		for _, loc := range locs {
			request.Spec.VolumeSnapshotLocations = append(request.Spec.VolumeSnapshotLocations, loc.Name)
			request.SnapshotLocations = append(request.SnapshotLocations, loc)
		}
	}

	return request
}

// validateAndGetSnapshotLocations gets a collection of VolumeSnapshotLocation objects that
// this backup will use (returned as a map of provider name -> VSL), and ensures:
// - each location name in .spec.volumeSnapshotLocations exists as a location
// - exactly 1 location per provider
// - a given provider's default location name is added to .spec.volumeSnapshotLocations if one
//   is not explicitly specified for the provider (if there's only one location for the provider,
//   it will automatically be used)
func (p *Processor) validateAndGetSnapshotLocations(backup *velerov1api.Backup) (map[string]*velerov1api.VolumeSnapshotLocation, []string) {
	errors := []string{}
	providerLocations := make(map[string]*velerov1api.VolumeSnapshotLocation)

	for _, locationName := range backup.Spec.VolumeSnapshotLocations {
		// validate each locationName exists as a VolumeSnapshotLocation
		location, err := p.snapshotLocationLister.VolumeSnapshotLocations(backup.Namespace).Get(locationName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				errors = append(errors, fmt.Sprintf("a VolumeSnapshotLocation CRD for the location %s with the name specified in the backup spec needs to be created before this snapshot can be executed. Error: %v", locationName, err))
			} else {
				errors = append(errors, fmt.Sprintf("error getting volume snapshot location named %s: %v", locationName, err))
			}
			continue
		}

		// ensure we end up with exactly 1 location *per provider*
		if providerLocation, ok := providerLocations[location.Spec.Provider]; ok {
			// if > 1 location name per provider as in ["aws-us-east-1" | "aws-us-west-1"] (same provider, multiple names)
			if providerLocation.Name != locationName {
				errors = append(errors, fmt.Sprintf("more than one VolumeSnapshotLocation name specified for provider %s: %s; unexpected name was %s", location.Spec.Provider, locationName, providerLocation.Name))
				continue
			}
		} else {
			// keep track of all valid existing locations, per provider
			providerLocations[location.Spec.Provider] = location
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	allLocations, err := p.snapshotLocationLister.VolumeSnapshotLocations(backup.Namespace).List(labels.Everything())
	if err != nil {
		errors = append(errors, fmt.Sprintf("error listing volume snapshot locations: %v", err))
		return nil, errors
	}

	// build a map of provider->list of all locations for the provider
	allProviderLocations := make(map[string][]*velerov1api.VolumeSnapshotLocation)
	for i := range allLocations {
		loc := allLocations[i]
		allProviderLocations[loc.Spec.Provider] = append(allProviderLocations[loc.Spec.Provider], loc)
	}

	// go through each provider and make sure we have/can get a VSL
	// for it
	for provider, locations := range allProviderLocations {
		if _, ok := providerLocations[provider]; ok {
			// backup's spec had a location named for this provider
			continue
		}

		if len(locations) > 1 {
			// more than one possible location for the provider: check
			// the defaults
			defaultLocation := p.defaultSnapshotLocations[provider]
			if defaultLocation == "" {
				errors = append(errors, fmt.Sprintf("provider %s has more than one possible volume snapshot location, and none were specified explicitly or as a default", provider))
				continue
			}
			location, err := p.snapshotLocationLister.VolumeSnapshotLocations(backup.Namespace).Get(defaultLocation)
			if err != nil {
				errors = append(errors, fmt.Sprintf("error getting volume snapshot location named %s: %v", defaultLocation, err))
				continue
			}

			providerLocations[provider] = location
			continue
		}

		// exactly one location for the provider: use it
		providerLocations[provider] = locations[0]
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return providerLocations, nil
}

// runBackup runs and uploads a validated backup. Any error returned from this function
// causes the backup to be Failed; if no error is returned, the backup's status's Errors
// field is checked to see if the backup was a partial failure.
func (p *Processor) runBackup(backup *Request) error {
	p.log.WithField("backup", kubeutil.NamespaceAndName(backup)).Info("Setting up backup log")

	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup log")
	}
	gzippedLogFile := gzip.NewWriter(logFile)
	// Assuming we successfully uploaded the log file, this will have already been closed below. It is safe to call
	// close multiple times. If we get an error closing this, there's not really anything we can do about it.
	defer gzippedLogFile.Close()
	defer closeAndRemoveFile(logFile, p.log)

	// Log the backup to both a backup log file and to stdout. This will help see what happened if the upload of the
	// backup log failed for whatever reason.
	logger := logging.DefaultLogger(p.backupLogLevel, p.formatFlag)
	logger.Out = io.MultiWriter(os.Stdout, gzippedLogFile)

	logCounter := logging.NewLogCounterHook()
	logger.Hooks.Add(logCounter)

	backupLog := logger.WithField("backup", kubeutil.NamespaceAndName(backup))

	backupLog.Info("Setting up backup temp file")
	backupFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup")
	}
	defer closeAndRemoveFile(backupFile, backupLog)

	backupLog.Info("Setting up plugin manager")
	pluginManager := p.newPluginManager(backupLog)
	defer pluginManager.CleanupClients()

	backupLog.Info("Getting backup item actions")
	actions, err := pluginManager.GetBackupItemActions()
	if err != nil {
		return err
	}

	backupLog.Info("Setting up backup store")
	backupStore, err := p.newBackupStore(backup.StorageLocation, pluginManager, backupLog)
	if err != nil {
		return err
	}

	exists, err := backupStore.BackupExists(backup.StorageLocation.Spec.StorageType.ObjectStorage.Bucket, backup.Name)
	if exists || err != nil {
		backup.Status.Phase = velerov1api.BackupPhaseFailed
		backup.Status.CompletionTimestamp = &metav1.Time{Time: p.clock.Now()}
		if err != nil {
			return errors.Wrapf(err, "error checking if backup already exists in object storage")
		}
		return errors.Errorf("backup already exists in object storage")
	}

	var fatalErrs []error
	if err := p.backupper.Backup(backupLog, backup, backupFile, actions, pluginManager); err != nil {
		fatalErrs = append(fatalErrs, err)
	}

	// Mark completion timestamp before serializing and uploading.
	// Otherwise, the JSON file in object storage has a CompletionTimestamp of 'null'.
	backup.Status.CompletionTimestamp = &metav1.Time{Time: p.clock.Now()}

	backup.Status.VolumeSnapshotsAttempted = len(backup.VolumeSnapshots)
	for _, snap := range backup.VolumeSnapshots {
		if snap.Status.Phase == volume.SnapshotPhaseCompleted {
			backup.Status.VolumeSnapshotsCompleted++
		}
	}

	recordBackupMetrics(backupLog, backup.Backup, backupFile, p.metrics)

	if err := gzippedLogFile.Close(); err != nil {
		p.log.WithError(err).Error("error closing gzippedLogFile")
	}

	backup.Status.Warnings = logCounter.GetCount(logrus.WarnLevel)
	backup.Status.Errors = logCounter.GetCount(logrus.ErrorLevel)

	// Assign finalize phase as close to end as possible so that any errors
	// logged to backupLog are captured. This is done before uploading the
	// artifacts to object storage so that the JSON representation of the
	// backup in object storage has the terminal phase set.
	switch {
	case len(fatalErrs) > 0:
		backup.Status.Phase = velerov1api.BackupPhaseFailed
	case logCounter.GetCount(logrus.ErrorLevel) > 0:
		backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed
	default:
		backup.Status.Phase = velerov1api.BackupPhaseCompleted
	}

	if errs := persistBackup(backup, backupFile, logFile, backupStore, p.log); len(errs) > 0 {
		fatalErrs = append(fatalErrs, errs...)
	}

	p.log.Info("Backup completed")

	// if we return a non-nil error, the calling function will update
	// the backup's phase to Failed.
	return kerrors.NewAggregate(fatalErrs)
}

func patchBackup(original, updated *velerov1api.Backup, client velerov1client.BackupsGetter) (*velerov1api.Backup, error) {
	origBytes, err := json.Marshal(original)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original backup")
	}

	updatedBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated backup")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(origBytes, updatedBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for backup")
	}

	res, err := client.Backups(original.Namespace).Patch(original.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching backup")
	}

	return res, nil
}

func persistBackup(backup *Request, backupContents, backupLog *os.File, backupStore persistence.BackupStore, log logrus.FieldLogger) []error {
	errs := []error{}
	backupJSON := new(bytes.Buffer)

	if err := encode.EncodeTo(backup.Backup, "json", backupJSON); err != nil {
		errs = append(errs, errors.Wrap(err, "error encoding backup"))
	}

	volumeSnapshots := new(bytes.Buffer)
	gzw := gzip.NewWriter(volumeSnapshots)

	if err := json.NewEncoder(gzw).Encode(backup.VolumeSnapshots); err != nil {
		errs = append(errs, errors.Wrap(err, "error encoding list of volume snapshots"))
	}
	if err := gzw.Close(); err != nil {
		errs = append(errs, errors.Wrap(err, "error closing gzip writer"))
	}

	podVolumeBackups := new(bytes.Buffer)
	gzw = gzip.NewWriter(podVolumeBackups)

	if err := json.NewEncoder(gzw).Encode(backup.PodVolumeBackups); err != nil {
		errs = append(errs, errors.Wrap(err, "error encoding pod volume backups"))
	}
	if err := gzw.Close(); err != nil {
		errs = append(errs, errors.Wrap(err, "error closing gzip writer"))
	}

	backupResourceList := new(bytes.Buffer)
	gzw = gzip.NewWriter(backupResourceList)

	if err := json.NewEncoder(gzw).Encode(backup.BackupResourceList()); err != nil {
		errs = append(errs, errors.Wrap(err, "error encoding backup resource list"))
	}
	if err := gzw.Close(); err != nil {
		errs = append(errs, errors.Wrap(err, "error closing gzip writer"))
	}

	if len(errs) > 0 {
		// Don't upload the JSON files or backup tarball if encoding to json fails.
		backupJSON = nil
		backupContents = nil
		volumeSnapshots = nil
		backupResourceList = nil
	}

	backupInfo := persistence.BackupInfo{
		Name:               backup.Name,
		Metadata:           backupJSON,
		Contents:           backupContents,
		Log:                backupLog,
		PodVolumeBackups:   podVolumeBackups,
		VolumeSnapshots:    volumeSnapshots,
		BackupResourceList: backupResourceList,
	}
	if err := backupStore.PutBackup(backupInfo); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func recordBackupMetrics(log logrus.FieldLogger, backup *velerov1api.Backup, backupFile *os.File, serverMetrics *metrics.ServerMetrics) {
	backupScheduleName := backup.GetLabels()[velerov1api.ScheduleNameLabel]

	var backupSizeBytes int64
	if backupFileStat, err := backupFile.Stat(); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error getting backup file info")
	} else {
		backupSizeBytes = backupFileStat.Size()
	}
	serverMetrics.SetBackupTarballSizeBytesGauge(backupScheduleName, backupSizeBytes)

	backupDuration := backup.Status.CompletionTimestamp.Time.Sub(backup.Status.StartTimestamp.Time)
	backupDurationSeconds := float64(backupDuration / time.Second)
	serverMetrics.RegisterBackupDuration(backupScheduleName, backupDurationSeconds)
	serverMetrics.RegisterVolumeSnapshotAttempts(backupScheduleName, backup.Status.VolumeSnapshotsAttempted)
	serverMetrics.RegisterVolumeSnapshotSuccesses(backupScheduleName, backup.Status.VolumeSnapshotsCompleted)
	serverMetrics.RegisterVolumeSnapshotFailures(backupScheduleName, backup.Status.VolumeSnapshotsAttempted-backup.Status.VolumeSnapshotsCompleted)
}

func closeAndRemoveFile(file *os.File, log logrus.FieldLogger) {
	if err := file.Close(); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error closing file")
	}
	if err := os.Remove(file.Name()); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error removing file")
	}
}
