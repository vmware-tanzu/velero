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
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

type backupStorageLocationController struct {
	*genericController

	namespace                   string
	defaultBackupLocation       string
	backupLocationClient        velerov1client.BackupStorageLocationsGetter
	backupStorageLocationLister velerov1listers.BackupStorageLocationLister
	newPluginManager            func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore              func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

func NewBackupStorageLocationController(
	namespace string,
	defaultBackupLocation string,
	backupLocationClient velerov1client.BackupStorageLocationsGetter,
	backupStorageLocationLister velerov1listers.BackupStorageLocationLister,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	logger logrus.FieldLogger,
) Interface {
	c := backupStorageLocationController{
		genericController:           newGenericController("backup-storage-location", logger),
		namespace:                   namespace,
		defaultBackupLocation:       defaultBackupLocation,
		backupLocationClient:        backupLocationClient,
		backupStorageLocationLister: backupStorageLocationLister,

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
	c.logger.Info("Checking that there is at least 1 backup storage location that is ready")

	locations, err := c.backupStorageLocationLister.BackupStorageLocations(c.namespace).List(labels.Everything())
	if err != nil {
		c.logger.WithError(err).Error("Error listing backup storage locations, at least one available backup storage location is required")
		return
	}

	if len(locations) == 0 {
		c.logger.Error("No backup storage locations found, at least one available backup storage location is required")
		return
	}

	pluginManager := c.newPluginManager(c.logger)
	defer pluginManager.CleanupClients()

	var unavailable []string
	for _, location := range locations {
		backupStore, err := c.newBackupStore(location, pluginManager, c.logger)
		if err != nil {
			unavailable = append(unavailable, errors.Wrapf(err, "error getting backup store for location %q", location.Name).Error())
			continue
		}

		locationName := location.Name
		if err := backupStore.IsValid(); err != nil {
			// update status
			_, err2 := c.patchBackupStorageLocation(location, func(r *v1.BackupStorageLocation) {
				r.Status.Phase = velerov1api.BackupStorageLocationPhaseUnavailable
			})
			if err2 != nil {
				unavailable = append(unavailable, errors.Wrapf(err2, "error updating backup storage for location %q to %s status", locationName, velerov1api.BackupStorageLocationPhaseUnavailable).Error())
			} else {
				unavailable = append(unavailable, errors.Wrapf(err, "location %q is unavailable", locationName).Error())
			}
		} else {
			// update status
			_, err := c.patchBackupStorageLocation(location, func(r *v1.BackupStorageLocation) {
				r.Status.Phase = velerov1api.BackupStorageLocationPhaseAvailable
			})
			if err != nil {
				unavailable = append(unavailable, errors.Wrapf(err, "error updating backup storage for location %q to %s status", locationName, velerov1api.BackupStorageLocationPhaseAvailable).Error())
			}
		}
	}

	var defaultFound bool
	if len(unavailable) == len(locations) { // no BSL available
		c.logger.Errorf("There are no backup storage locations available, at least one available backup storage location is required: %s", strings.Join(unavailable, "; "))
	} else if len(unavailable) > 0 { // some but not all BSL unavailable
		for _, location := range locations {
			if location.Name == c.defaultBackupLocation && location.Status.Phase == velerov1api.BackupStorageLocationPhaseUnavailable {
				defaultFound = true
				c.logger.Warnf("The specified default backup storage location named %q is unavailable; for convenience, be sure to configure it properly or make another location that is available the default", c.defaultBackupLocation)
				break
			}
		}
		if !defaultFound {
			c.logger.Warnf("The specified default backup storage location named %q was not found; for convenience, be sure to create one or make another location that is available the default", c.defaultBackupLocation)
		}
		c.logger.Warnf("Unavailable backup storage locations detected: %s", strings.Join(unavailable, "; "))
	}
}

func (c backupStorageLocationController) patchBackupStorageLocation(req *velerov1api.BackupStorageLocation, mutate func(*v1.BackupStorageLocation)) (*velerov1api.BackupStorageLocation, error) {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original BackupStorageLocations")
	}

	// Mutate
	mutate(req)

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated BackupStorageLocations")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for BackupStorageLocations")
	}

	bsl, err := c.backupLocationClient.BackupStorageLocations(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching BackupStorageLocations")
	}

	return bsl, nil
}
