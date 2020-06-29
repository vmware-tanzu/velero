package velero

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/persistence"

	"github.com/sirupsen/logrus"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

// StorageLocation holds information for connecting with storage
// for a backup storage location.
type StorageLocation struct {
	Client client.Client
	Ctx    context.Context
	Log    logrus.FieldLogger

	DefaultStorageLocation          string
	DefaultStoreValidationFrequency time.Duration

	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	NewPluginManager func(logrus.FieldLogger) clientmgmt.Manager
	NewBackupStore   func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

// IsReadyToValidate calculates if a given backup storage location is ready to be validated
// based on the chosen validation frequency. Users can choose any validation frequency for a given location.
// A backup storage location's validation frequency overrites the server's default value.
// To skip validation all together, the frequency must be set to zero.
func (p *StorageLocation) IsReadyToValidate(location *velerov1api.BackupStorageLocation) bool {
	storeValidationFrequency := p.DefaultStoreValidationFrequency
	// If the bsl validation frequency is not specifically set, skip this block and use the server's default
	if location.Spec.ValidationFrequency != nil {
		storeValidationFrequency = location.Spec.ValidationFrequency.Duration
		if storeValidationFrequency == 0 {
			p.Log.Debug("Validation period for this backup location is set to 0, skipping validation")
			return false
		}

		if storeValidationFrequency < 0 {
			p.Log.Debug("Validation period must be non-negative, changing from %d to %d", storeValidationFrequency, p.DefaultStoreValidationFrequency)
			storeValidationFrequency = p.DefaultStoreValidationFrequency
		}
	}

	lastValidation := location.Status.LastValidationTime
	if lastValidation != nil {
		nextValidation := lastValidation.Add(storeValidationFrequency)
		if time.Now().UTC().Before(nextValidation) {
			return false
		}
	}

	return true
}

// IsValidFor verifies if a storage is valid for a given backup storage location.
func (p *StorageLocation) IsValidFor(location *velerov1api.BackupStorageLocation) error {
	pluginManager := p.NewPluginManager(p.Log)
	defer pluginManager.CleanupClients()

	backupStore, err := p.NewBackupStore(location, pluginManager, p.Log)
	if err != nil {
		return err
	}

	if err := backupStore.IsValid(); err != nil {
		return err
	}

	return nil
}

// UpdatePhase patches the status.phase field as well as the status.lastValidationTime to the current time
func (p *StorageLocation) UpdatePhase(location *velerov1api.BackupStorageLocation, phase velerov1api.BackupStorageLocationPhase) error {
	statusPatch := client.MergeFrom(location.DeepCopyObject())
	location.Status.Phase = phase
	location.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
	if err := p.Client.Status().Patch(p.Ctx, location, statusPatch); err != nil {
		return err
	}

	return nil
}

// BackupStorageLocationsExist verifies if there are any backup storage locations.
// For all purposes, if either there is an error while attempting to fetch items or
// if there are no items an error would be returned since the functioning of the system
// would be haulted.
func BackupStorageLocationsExist(kbClient client.Client, ctx context.Context, namespace string) (velerov1api.BackupStorageLocationList, error) {
	var locations velerov1api.BackupStorageLocationList
	if err := kbClient.List(ctx, &locations, &client.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return velerov1api.BackupStorageLocationList{}, err
	}

	if len(locations.Items) == 0 {
		return velerov1api.BackupStorageLocationList{}, errors.New("no backup storage locations found")
	}

	return locations, nil
}
