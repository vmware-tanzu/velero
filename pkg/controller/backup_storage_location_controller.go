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

package controller

import (
	"encoding/json"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

type backupStorageLocationController struct {
	*genericController

	namespace                       string
	defaultBackupLocation           string
	defaultStoreValidationFrequency time.Duration
	backupLocationClient            velerov1client.BackupStorageLocationsGetter
	backupStorageLocationLister     velerov1listers.BackupStorageLocationLister
	newPluginManager                func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore                  func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

func NewBackupStorageLocationController(
	namespace string,
	defaultBackupLocation string,
	defaultStoreValidationFrequency time.Duration,
	backupLocationClient velerov1client.BackupStorageLocationsGetter,
	backupStorageLocationLister velerov1listers.BackupStorageLocationLister,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	logger logrus.FieldLogger,
) Interface {
	if defaultStoreValidationFrequency <= 0 {
		defaultStoreValidationFrequency = time.Minute
	}

	logger.Infof("Backup location validation period is %v", defaultStoreValidationFrequency)

	c := backupStorageLocationController{
		genericController:               newGenericController("backup-storage-location", logger),
		namespace:                       namespace,
		defaultBackupLocation:           defaultBackupLocation,
		defaultStoreValidationFrequency: defaultStoreValidationFrequency,
		backupLocationClient:            backupLocationClient,
		backupStorageLocationLister:     backupStorageLocationLister,

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager: newPluginManager,
		newBackupStore:   persistence.NewObjectBackupStore,
	}

	c.resyncFunc = c.run
	c.resyncPeriod = 30 * time.Second

	return c
}

func (c *backupStorageLocationController) run() {
	c.logger.Info("Checking for existing backup locations ready to be validated; there needs to be at least 1 backup location that is available")

	locations, err := c.backupStorageLocationLister.BackupStorageLocations(c.namespace).List(labels.Everything())
	if err != nil {
		c.logger.WithError(err).Error("Error listing locations, at least one available backup location is required")
		return
	}

	if len(locations) == 0 {
		c.logger.Error("No locations found, at least one available backup location is required")
		return
	}

	pluginManager := c.newPluginManager(c.logger)
	defer pluginManager.CleanupClients()

	var unavailableErrors []string
	log := c.logger
	var defaultFound bool
	for _, location := range locations {
		locationName := location.Name
		log = c.logger.WithField("backupLocation", locationName)

		storeValidationFrequency := c.defaultStoreValidationFrequency
		if location.Spec.ValidationFrequency != nil {
			storeValidationFrequency = location.Spec.ValidationFrequency.Duration
			if storeValidationFrequency == 0 {
				log.Debug("Validation period for this backup location is set to 0, skipping validation")
				continue
			}

			if storeValidationFrequency < 0 {
				log.Debug("Validation period must be non-negative")
				storeValidationFrequency = c.defaultStoreValidationFrequency
			}
		}

		lastValidation := location.Status.LastValidationTime
		if lastValidation != nil {
			log.Debug("Checking if backup location needs to be validated at this time")
			nextValidation := lastValidation.Add(storeValidationFrequency)
			if time.Now().UTC().Before(nextValidation) {
				continue
			}
		}

		log.Debug("Validation backup location")

		backupStore, err := c.newBackupStore(location, pluginManager, log)
		if err != nil {
			unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "error getting backup store for backup location %q", locationName).Error())
			continue
		}

		if err := backupStore.IsValid(); err != nil {
			if location.Name == c.defaultBackupLocation {
				defaultFound = true
				log.Warnf("The specified default backup location named %q is unavailable; for convenience, be sure to configure it properly or make another backup location that is available the default", c.defaultBackupLocation)
			}

			// update status
			err2 := c.patchBackupStorageLocation(location, func(r *velerov1api.BackupStorageLocation) {
				r.Status.Phase = velerov1api.BackupStorageLocationPhaseUnavailable
				r.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
			})
			if err2 != nil {
				unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "error updating backup location to %s status", velerov1api.BackupStorageLocationPhaseUnavailable).Error())
			} else {
				unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "backup location %q is unavailable", locationName).Error())
			}
		} else {
			if location.Name == c.defaultBackupLocation {
				defaultFound = true
			}

			// update status
			err := c.patchBackupStorageLocation(location, func(r *velerov1api.BackupStorageLocation) {
				r.Status.Phase = velerov1api.BackupStorageLocationPhaseAvailable
				r.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
			})
			if err != nil {
				unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "error updating backup location %q to %s status", locationName, velerov1api.BackupStorageLocationPhaseAvailable).Error())
			}
		}
	}

	if !defaultFound {
		log.Warnf("The specified default backup location named %q was not found; for convenience, be sure to create one or make another backup location that is available the default", c.defaultBackupLocation)
	}

	if len(unavailableErrors) == len(locations) { // no BSL available
		log.Errorf("There are no locations available, at least one available backup location is required: %s", strings.Join(unavailableErrors, "; "))
	} else if len(unavailableErrors) > 0 { // some but not all BSL unavailable
		log.Warnf("Unavailable backup locations detected: %s", strings.Join(unavailableErrors, "; "))
	}
}

func (c backupStorageLocationController) patchBackupStorageLocation(req *velerov1api.BackupStorageLocation, mutate func(*velerov1api.BackupStorageLocation)) error {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return errors.Wrap(err, "error marshalling original BackupStorageLocations")
	}

	// Mutate
	mutate(req)

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return errors.Wrap(err, "error marshalling updated BackupStorageLocations")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return errors.Wrap(err, "error creating json merge patch for BackupStorageLocations")
	}

	_, err = c.backupLocationClient.BackupStorageLocations(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return errors.Wrap(err, "error patching BackupStorageLocations")
	}

	return nil
}
