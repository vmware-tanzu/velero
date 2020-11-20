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
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

// BackupStorageLocationReconciler reconciles a BackupStorageLocation object
type BackupStorageLocationReconciler struct {
	Ctx                       context.Context
	Client                    client.Client
	Scheme                    *runtime.Scheme
	DefaultBackupLocationInfo storage.DefaultBackupLocationInfo
	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	NewPluginManager func(logrus.FieldLogger) clientmgmt.Manager
	NewBackupStore   func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations/status,verbs=get;update;patch
func (r *BackupStorageLocationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithField("controller", BackupStorageLocation)

	log.Debug("Validating availability of backup storage locations.")

	locationList, err := storage.ListBackupStorageLocations(r.Ctx, r.Client, req.Namespace)
	if err != nil {
		log.WithError(err).Error("No backup storage locations found, at least one is required")
		return ctrl.Result{Requeue: true}, err
	}

	pluginManager := r.NewPluginManager(log)
	defer pluginManager.CleanupClients()

	var defaultFound bool
	var unavailableErrors []string
	var anyVerified bool
	for i := range locationList.Items {
		location := &locationList.Items[i]
		log := r.Log.WithField("controller", BackupStorageLocation).WithField(BackupStorageLocation, location.Name)

		if location.Name == r.DefaultBackupLocationInfo.StorageLocation {
			defaultFound = true
		}

		if !storage.IsReadyToValidate(location.Spec.ValidationFrequency, location.Status.LastValidationTime, r.DefaultBackupLocationInfo, log) {
			log.Info("Validation not required, skipping...")
			continue
		}

		backupStore, err := r.NewBackupStore(location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting a backup store")
			continue
		}

		// Initialize the patch helper.
		patchHelper, err := patch.NewHelper(location, r.Client)
		if err != nil {
			log.WithError(err).Error("Error getting a patch helper to update this resource")
			continue
		}

		log.Info("Validating backup storage location")
		anyVerified = true
		if err := backupStore.IsValid(); err != nil {
			log.Info("Backup location is invalid, marking as unavailable")
			unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "Backup location %q is unavailable", location.Name).Error())

			if location.Name == r.DefaultBackupLocationInfo.StorageLocation {
				log.Warnf("The specified default backup location named %q is unavailable; for convenience, be sure to configure it properly or make another backup location that is available the default", r.DefaultBackupLocationInfo.StorageLocation)
			}

			location.Status.Phase = velerov1api.BackupStorageLocationPhaseUnavailable
		} else {
			log.Info("Backup location valid, marking as available")
			location.Status.Phase = velerov1api.BackupStorageLocationPhaseAvailable
		}
		location.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
		if err := patchHelper.Patch(r.Ctx, location); err != nil {
			log.WithError(err).Error("Error updating backup location phase")
		}
	}

	if !anyVerified {
		log.Info("No backup locations needed to be validated")
	}

	r.logReconciledPhase(defaultFound, locationList, unavailableErrors)

	return ctrl.Result{Requeue: true}, nil
}

func (r *BackupStorageLocationReconciler) logReconciledPhase(defaultFound bool, locationList velerov1api.BackupStorageLocationList, errs []string) {
	var availableBSLs []*velerov1api.BackupStorageLocation
	var unAvailableBSLs []*velerov1api.BackupStorageLocation
	var unknownBSLs []*velerov1api.BackupStorageLocation
	log := r.Log.WithField("controller", BackupStorageLocation)

	for i, location := range locationList.Items {
		phase := location.Status.Phase
		switch phase {
		case velerov1api.BackupStorageLocationPhaseAvailable:
			availableBSLs = append(availableBSLs, &locationList.Items[i])
		case velerov1api.BackupStorageLocationPhaseUnavailable:
			unAvailableBSLs = append(unAvailableBSLs, &locationList.Items[i])
		default:
			unknownBSLs = append(unknownBSLs, &locationList.Items[i])
		}
	}

	numAvailable := len(availableBSLs)
	numUnavailable := len(unAvailableBSLs)
	numUnknown := len(unknownBSLs)

	if numUnavailable+numUnknown == len(locationList.Items) { // no available BSL
		if len(errs) > 0 {
			log.Errorf("Current backup storage locations available/unavailable/unknown: %v/%v/%v, %s)", numAvailable, numUnavailable, numUnknown, strings.Join(errs, "; "))
		} else {
			log.Errorf("Current backup storage locations available/unavailable/unknown: %v/%v/%v)", numAvailable, numUnavailable, numUnknown)
		}
	} else if numUnavailable > 0 { // some but not all BSL unavailable
		log.Warnf("Invalid backup locations detected: available/unavailable/unknown: %v/%v/%v, %s)", numAvailable, numUnavailable, numUnknown, strings.Join(errs, "; "))
	}

	if !defaultFound {
		log.Warnf("The specified default backup location named %q was not found; for convenience, be sure to create one or make another backup location that is available the default", r.DefaultBackupLocationInfo.StorageLocation)
	}
}

func (r *BackupStorageLocationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.BackupStorageLocation{}).
		Complete(r)
}
