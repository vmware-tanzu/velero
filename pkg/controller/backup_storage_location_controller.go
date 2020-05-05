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
	"k8s.io/client-go/tools/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

type backupStorageLocationController struct {
	*genericController

	namespace                       string
	defaultBackupLocation           string
	defaultStoreValidationFrequency time.Duration
	backupLocationsPerPhase         map[string][]*velerov1api.BackupStorageLocation
	backupLocationClient            velerov1client.BackupStorageLocationsGetter
	backupStorageLocationLister     velerov1listers.BackupStorageLocationLister
	newPluginManager                func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore                  func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
	validatedBackupLocations        []*velerov1api.BackupStorageLocation
}

func NewBackupStorageLocationController(
	namespace string,
	defaultBackupLocation string,
	defaultStoreValidationFrequency time.Duration,
	backupLocationInformer velerov1informers.BackupStorageLocationInformer,
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

	backupLocationInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			//wip
		},
	)
	c.resyncFunc = c.run
	c.resyncPeriod = 30 * time.Second

	return c
}

func (c *backupStorageLocationController) run() {
	c.logger.Info("Checking for existing backup locations ready to be verified; there needs to be at least 1 backup location available")

	locations, err := c.backupStorageLocationLister.BackupStorageLocations(c.namespace).List(labels.Everything())
	if err != nil {
		c.logger.WithError(err).Error("Error listing backup locations, at least one available backup location is required")
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
	c.backupLocationsPerPhase = make(map[string][]*velerov1api.BackupStorageLocation)
	for _, location := range locations {
		locationName := location.Name
		log = c.logger.WithField("backupLocation", locationName)

		if location.Name == c.defaultBackupLocation {
			defaultFound = true
		}

		storeValidationFrequency := c.defaultStoreValidationFrequency
		// If the bsl validation frequency is not specifically set, skip this block and use the server's default
		if location.Spec.ValidationFrequency != nil {
			storeValidationFrequency = location.Spec.ValidationFrequency.Duration
			if storeValidationFrequency == 0 {
				log.Debug("Validation period for this backup location is set to 0, skipping validation")
				// Note: we don't want to patch the bsl's phase here to "unverified" because we don't want to move
				// an existing valid bsl to any other state when validation frequency changes to 0s; but we
				// do want to update the current tally for a more transparent final reporting
				location.Status.Phase = velerov1api.BackupStorageLocationPhaseUnverified
				c.updateCurrentTallyOfAvailability(location)
				continue
			}

			if storeValidationFrequency < 0 {
				log.Debug("Validation period must be non-negative")
				storeValidationFrequency = c.defaultStoreValidationFrequency
			}
		}

		lastValidation := location.Status.LastValidationTime
		if lastValidation != nil {
			log.Debug("Checking if backup location needs to be verified at this time")
			nextValidation := lastValidation.Add(storeValidationFrequency)
			if time.Now().UTC().Before(nextValidation) {
				location.Status.Phase = velerov1api.BackupStorageLocationPhaseUnverified
				c.updateCurrentTallyOfAvailability(location)
				continue
			}
		}

		log.Debug("Backup location ready to be verified")

		var patchedLocation *velerov1api.BackupStorageLocation
		var err2 error
		backupStore, err := c.newBackupStore(location, pluginManager, log)
		if err != nil {
			patchedLocation, err2 = c.patchBackupStorageLocation(location, velerov1api.BackupStorageLocationPhaseUnverified)
			if err2 != nil {
				log.Errorf("error updating backup location phase to %s", velerov1api.BackupStorageLocationPhaseUnverified)
				return
			}
			unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "error getting backup store for backup location %q", locationName).Error())
			continue
		}

		if err := backupStore.IsValid(); err != nil {
			log.Debug("Backup location verified, not valid")

			if location.Name == c.defaultBackupLocation {
				log.Warnf("The specified default backup location named %q is unavailable; for convenience, be sure to configure it properly or make another backup location that is available the default", c.defaultBackupLocation)
			}

			patchedLocation, err2 = c.patchBackupStorageLocation(location, velerov1api.BackupStorageLocationPhaseUnavailable)
			if err2 != nil {
				log.Errorf("error updating backup location phase to %s", velerov1api.BackupStorageLocationPhaseUnavailable)
				return
			}
			unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "backup location %q is unavailable", locationName).Error())
		} else {
			log.Debug("Backup location verified and it is valid")
			patchedLocation, err = c.patchBackupStorageLocation(location, velerov1api.BackupStorageLocationPhaseAvailable)
			if err != nil {
				log.Errorf("error updating backup location phase to %s", velerov1api.BackupStorageLocationPhaseAvailable)
				return
			}

		}
		if patchedLocation != nil {
			c.updateCurrentTallyOfAvailability(patchedLocation)
			if location.Status.Phase != patchedLocation.Status.Phase {
				log.Debugf("Backup location updated from %q to %q", location.Status.Phase, patchedLocation.Status.Phase)
			}
		}
	}
	c.currentPhaseStatus(len(locations), defaultFound, unavailableErrors)
}

func (c *backupStorageLocationController) currentPhaseStatus(numLocations int, defaultFound bool, errs []string) {
	unavailable := len(c.backupLocationsPerPhase["unavailable"])
	unverified := len(c.backupLocationsPerPhase["unverified"])

	if len(c.backupLocationsPerPhase["unavailable"])+len(c.backupLocationsPerPhase["unverified"]) == numLocations { // no BSL available
		if len(errs) > 0 {
			c.logger.Errorf("Amongst the backup locations that were ready to be verified, none were valid (unavailable: %v, unverified: %v); at least one valid location is required: %v", unavailable, unverified, strings.Join(errs, "; "))
		} else {
			c.logger.Errorf("Amongst the backup locations that were ready to be verified, none were valid (unavailable: %v, unverified: %v); at least one valid location is required", unavailable, unverified)
		}
	} else if len(c.backupLocationsPerPhase["unavailable"]) > 0 { // some but not all BSL unavailable
		c.logger.Warnf("Unavailable backup locations detected: %s", strings.Join(errs, "; "))
	}

	if !defaultFound {
		c.logger.Warnf("The specified default backup location named %q was not found; for convenience, be sure to create one or make another backup location that is available the default", c.defaultBackupLocation)
	}
}

func (c *backupStorageLocationController) updateCurrentTallyOfAvailability(location *velerov1api.BackupStorageLocation) {
	if location.Status.Phase == velerov1api.BackupStorageLocationPhaseAvailable {
		c.backupLocationsPerPhase["available"] = append(c.backupLocationsPerPhase["available"], location)
	} else if location.Status.Phase == velerov1api.BackupStorageLocationPhaseUnavailable {
		c.backupLocationsPerPhase["unavailable"] = append(c.backupLocationsPerPhase["unavailable"], location)
	} else {
		c.backupLocationsPerPhase["unverified"] = append(c.backupLocationsPerPhase["unverified"], location)
	}
}

func (c backupStorageLocationController) patchBackupStorageLocation(req *velerov1api.BackupStorageLocation, phase velerov1api.BackupStorageLocationPhase) (*velerov1api.BackupStorageLocation, error) {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original BackupStorageLocations")
	}

	req.Status.Phase = phase
	req.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated BackupStorageLocations")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for BackupStorageLocations")
	}

	location, err := c.backupLocationClient.BackupStorageLocations(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching BackupStorageLocations")
	}

	return location, nil
}
