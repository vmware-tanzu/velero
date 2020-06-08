package backupstoragelocation

import (
	"time"

	"github.com/vmware-tanzu/velero/pkg/persistence"

	"github.com/sirupsen/logrus"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

type Processor struct {
	DefaultStorageLocation          string
	DefaultStoreValidationFrequency time.Duration
	NewPluginManager                func(logrus.FieldLogger) clientmgmt.Manager
}

func (p *Processor) IsReadyToValidate(location *velerov1api.BackupStorageLocation, log logrus.FieldLogger) bool {
	storeValidationFrequency := p.DefaultStoreValidationFrequency
	// If the bsl validation frequency is not specifically set, skip this block and use the server's default
	if location.Spec.ValidationFrequency != nil {
		storeValidationFrequency = location.Spec.ValidationFrequency.Duration
		if storeValidationFrequency == 0 {
			log.Debug("Validation period for this backup location is set to 0, skipping validation")
			return false
		}

		if storeValidationFrequency < 0 {
			log.Debug("Validation period must be non-negative")
			storeValidationFrequency = p.DefaultStoreValidationFrequency
		}
	}

	lastValidation := location.Status.LastValidationTime
	if lastValidation != nil {
		nextValidation := lastValidation.Add(storeValidationFrequency)
		if time.Now().UTC().Before(nextValidation) {
			//location.Status.Phase = velerov1api.BackupStorageLocationPhaseUnverified
			//c.updateCurrentTallyOfAvailability(location)
			return false
		}
	}

	return true
}

func (p *Processor) IsValidFor(location *velerov1api.BackupStorageLocation, log logrus.FieldLogger) error {
	pluginManager := p.NewPluginManager(log)
	defer pluginManager.CleanupClients()

	var newBackupStore func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
	newBackupStore = persistence.NewObjectBackupStore
	backupStore, err := newBackupStore(location, pluginManager, log)
	if err != nil {
		return err
	}

	if err := backupStore.IsValid(); err != nil {
		return err
	}

	return nil
}
