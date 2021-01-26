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
func (r *BackupStorageLocationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	for _, location := range locationList.Items {
		if location.Spec.Default {
			defaultFound = true
			break
		}
	}

	var unavailableErrors []string
	var anyVerified bool
	for i := range locationList.Items {
		location := &locationList.Items[i]
		isDefault := location.Spec.Default
		log := r.Log.WithField("controller", BackupStorageLocation).WithField(BackupStorageLocation, location.Name)

		// TODO(2.0) remove this check since the server default will be deprecated
		if !defaultFound && location.Name == r.DefaultBackupLocationInfo.StorageLocation {
			// For backward-compatible, to configure the backup storage location as the default if
			// none of the BSLs be marked as the default and the BSL name matches against the
			// "velero server --default-backup-storage-location".
			isDefault = true
			defaultFound = true
		}

		if !storage.IsReadyToValidate(location.Spec.ValidationFrequency, location.Status.LastValidationTime, r.DefaultBackupLocationInfo.ServerValidationFrequency, log) {
			log.Debug("Validation not required, skipping...")
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

		// updates the default backup location
		location.Spec.Default = isDefault

		log.Info("Validating backup storage location")
		anyVerified = true
		if err := backupStore.IsValid(); err != nil {
			log.Info("Backup storage location is invalid, marking as unavailable")
			unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "Backup storage location %q is unavailable", location.Name).Error())
			location.Status.Phase = velerov1api.BackupStorageLocationPhaseUnavailable
		} else {
			log.Info("Backup storage location valid, marking as available")
			location.Status.Phase = velerov1api.BackupStorageLocationPhaseAvailable
		}
		location.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}

		if err := patchHelper.Patch(r.Ctx, location); err != nil {
			log.WithError(err).Error("Error updating backup storage location phase")
		}
	}

	if !anyVerified {
		log.Debug("No backup storage locations needed to be validated")
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
		log.Warnf("Unavailable backup storage locations detected: available/unavailable/unknown: %v/%v/%v, %s)", numAvailable, numUnavailable, numUnknown, strings.Join(errs, "; "))
	}

	if !defaultFound {
		log.Warn("There is no existing backup storage location set as default. Please see `velero backup-location -h` for options.")
	}
}

func (r *BackupStorageLocationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.BackupStorageLocation{}).
		Complete(r)
}
