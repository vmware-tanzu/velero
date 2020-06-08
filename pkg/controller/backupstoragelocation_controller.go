/*


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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/velero"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// BackupStorageLocationReconciler reconciles a BackupStorageLocation object
type BackupStorageLocationReconciler struct {
	client.Client
	Log             logrus.FieldLogger
	Scheme          *runtime.Scheme
	StorageLocation velero.StorageLocation
}

// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations/status,verbs=get;update;patch
func (r *BackupStorageLocationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithField("controller", "backupstoragelocation")

	log.Info("Checking for existing backup locations ready to be verified; there needs to be at least 1 backup location available")

	locationList, err := velero.BackupStorageLocationsExist(r.Client, ctx, req.Namespace)
	if err != nil {
		log.WithError(err).Error("No backup storage locations found, at least one is required")
	}

	if r.StorageLocation.DefaultStoreValidationFrequency <= 0 {
		r.StorageLocation.DefaultStoreValidationFrequency = time.Minute
	}

	var defaultFound bool
	var unavailableErrors []string
	var someVerified bool
	for _, location := range locationList.Items {
		log := log.WithField("backupstoragelocation", location.Name)

		if location.Name == r.StorageLocation.DefaultStorageLocation {
			defaultFound = true
		}

		ready := r.StorageLocation.IsReadyToValidate(&location, log)
		if !ready {
			continue
		}
		someVerified = true

		log.Debug("Backup location ready to be verified")

		if err := r.StorageLocation.IsValidFor(&location, log); err != nil {
			log.Debug("Backup location verified, not valid")
			unavailableErrors = append(unavailableErrors, errors.Wrapf(err, "Backup location %q is unavailable", location.Name).Error())

			if location.Name == r.StorageLocation.DefaultStorageLocation {
				log.Warnf("The specified default backup location named %q is unavailable; for convenience, be sure to configure it properly or make another backup location that is available the default", r.StorageLocation.DefaultStorageLocation)
			}

			if err2 := r.update(ctx, &location, velerov1api.BackupStorageLocationPhaseUnavailable); err2 != nil {
				log.WithError(err).Errorf("Error updating backup location phase to %s", velerov1api.BackupStorageLocationPhaseUnavailable)
				continue
			}
		} else {
			log.Debug("Backup location verified and it is valid")
			if err := r.update(ctx, &location, velerov1api.BackupStorageLocationPhaseAvailable); err != nil {
				log.WithError(err).Errorf("Error updating backup location phase to %s", velerov1api.BackupStorageLocationPhaseAvailable)
				continue
			}
		}
	}

	if !someVerified {
		log.Info("No backup locations were ready to be verified")
	}

	r.logReconciledPhase(defaultFound, locationList, unavailableErrors)

	return ctrl.Result{Requeue: true}, nil
}

func (r *BackupStorageLocationReconciler) update(ctx context.Context, location *velerov1api.BackupStorageLocation, phase velerov1api.BackupStorageLocationPhase) error {
	statusPatch := client.MergeFrom(location.DeepCopyObject())
	location.Status.Phase = phase
	location.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
	if err := r.Status().Patch(ctx, location, statusPatch); err != nil {
		return err
	}

	return nil
}

func (r *BackupStorageLocationReconciler) logReconciledPhase(defaultFound bool, locationList velerov1api.BackupStorageLocationList, errs []string) {
	var availableBSLs []*velerov1api.BackupStorageLocation
	var unAvailableBSLs []*velerov1api.BackupStorageLocation
	var unknownBSLs []*velerov1api.BackupStorageLocation
	log := r.Log.WithField("controller", "backupstoragelocation")

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

	numUnavailable := len(unAvailableBSLs)
	numUnknown := len(unknownBSLs)
	total := numUnavailable + numUnknown + len(availableBSLs)

	if numUnavailable+numUnknown == len(locationList.Items) { // no available BSL, all are unavailable
		if len(errs) > 0 {
			log.Errorf("Amongst the backup locations that were ready to be verified, none were valid (total: %v - unavailable: %v, unknown: %v); at least one valid location is required: %v", total, numUnavailable, numUnknown, strings.Join(errs, "; "))
		} else {
			log.Errorf("Amongst the backup locations that were ready to be verified, none were valid (total: %v - unavailable: %v, unknown: %v); at least one valid location is required", total, numUnavailable, numUnknown)
		}
	} else if numUnavailable > 0 { // some but not all BSL unavailable
		log.Warnf("Invalid backup locations detected: (total: %v - unavailable: %v, unknown: %v); at least one valid location is required: %v", total, numUnavailable, numUnknown, strings.Join(errs, "; "))
	}

	if !defaultFound {
		log.Warnf("The specified default backup location named %q was not found; for convenience, be sure to create one or make another backup location that is available the default", r.StorageLocation.DefaultStorageLocation)
	}
}

func (r *BackupStorageLocationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.BackupStorageLocation{}).
		Complete(r)
}
