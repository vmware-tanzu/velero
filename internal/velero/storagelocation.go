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

	DefaultStorageLocation          string
	DefaultStoreValidationFrequency time.Duration

	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	NewPluginManager func(logrus.FieldLogger) clientmgmt.Manager
	NewBackupStore   func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

// IsReadyToValidate calculates if a given backup storage location is ready to be validated.
//
// Rules:
// Users can choose a validation frequency per location. This will overrite the server's default value
// To skip/stop validation, set the frequency to zero
// This will always return "true" for the first attempt at validating a location, regardless of its validation frequency setting
// Otherwise, it returns "ready" only when NOW is equal to or after the next validation time
// (next validation time: last validation time + validation frequency)
func (p *StorageLocation) IsReadyToValidate(location *velerov1api.BackupStorageLocation, log logrus.FieldLogger) bool {
	validationFrequency := p.DefaultStoreValidationFrequency
	// If the bsl validation frequency is not specifically set, skip this block and continue, using the server's default
	if location.Spec.ValidationFrequency != nil {
		validationFrequency = location.Spec.ValidationFrequency.Duration
	}

	if validationFrequency == 0 {
		log.Debug("Validation period for this backup location is set to 0, skipping validation")
		return false
	}

	if validationFrequency < 0 {
		log.Debugf("Validation period must be non-negative, changing from %d to %d", validationFrequency, p.DefaultStoreValidationFrequency)
		validationFrequency = p.DefaultStoreValidationFrequency
	}

	lastValidation := location.Status.LastValidationTime
	if lastValidation != nil { // always ready to validate the first time around, so only even do this check if this has happened before
		nextValidation := lastValidation.Add(validationFrequency) // next validation time: last validation time + validation frequency
		if time.Now().UTC().Before(nextValidation) {              // ready only when NOW is equal to or after the next validation time
			return false
		}
	}

	return true
}

// IsValidFor verifies if a storage is valid for a given backup storage location.
func (p *StorageLocation) IsValid(location *velerov1api.BackupStorageLocation, log logrus.FieldLogger) error {
	pluginManager := p.NewPluginManager(log)
	defer pluginManager.CleanupClients()

	backupStore, err := p.NewBackupStore(location, pluginManager, log)
	if err != nil {
		return err
	}

	if err := backupStore.IsValid(); err != nil {
		return err
	}

	return nil
}

// PatchStatus patches the status.phase field as well as the status.lastValidationTime to the current time
func (p *StorageLocation) PatchStatus(location *velerov1api.BackupStorageLocation, phase velerov1api.BackupStorageLocationPhase) error {
	statusPatch := client.MergeFrom(location.DeepCopyObject())
	location.Status.Phase = phase
	location.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
	if err := p.Client.Status().Patch(p.Ctx, location, statusPatch); err != nil {
		return err
	}

	return nil
}

// ListBackupStorageLocations verifies if there are any backup storage locations.
// For all purposes, if either there is an error while attempting to fetch items or
// if there are no items an error would be returned since the functioning of the system
// would be haulted.
func ListBackupStorageLocations(kbClient client.Client, ctx context.Context, namespace string) (velerov1api.BackupStorageLocationList, error) {
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
