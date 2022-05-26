/*
Copyright the Velero contributors.

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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	backupStorageLocationSyncPeriod = 1 * time.Minute
)

// BackupStorageLocationReconciler reconciles a BackupStorageLocation object
type BackupStorageLocationReconciler struct {
	Ctx                       context.Context
	Client                    client.Client
	Scheme                    *runtime.Scheme
	DefaultBackupLocationInfo storage.DefaultBackupLocationInfo
	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	NewPluginManager  func(logrus.FieldLogger) clientmgmt.Manager
	BackupStoreGetter persistence.ObjectBackupStoreGetter

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=backupstoragelocations/status,verbs=get;update;patch
func (r *BackupStorageLocationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var unavailableErrors []string
	var location velerov1api.BackupStorageLocation

	log := r.Log.WithField("controller", BackupStorageLocation).WithField(BackupStorageLocation, req.NamespacedName.String())
	log.Debug("Validating availability of BackupStorageLocation")

	locationList, err := storage.ListBackupStorageLocations(r.Ctx, r.Client, req.Namespace)
	if err != nil {
		log.WithError(err).Error("No BackupStorageLocations found, at least one is required")
		return ctrl.Result{}, nil
	}

	pluginManager := r.NewPluginManager(log)
	defer pluginManager.CleanupClients()

	var defaultFound bool
	for _, bsl := range locationList.Items {
		if bsl.Spec.Default {
			defaultFound = true
		}
		if bsl.Name == req.Name && bsl.Namespace == req.Namespace {
			location = bsl
		}
	}

	if location.Name == "" || location.Namespace == "" {
		log.WithError(err).Error("BackupStorageLocation is not found")
		return ctrl.Result{}, nil
	}

	isDefault := location.Spec.Default

	// TODO(2.0) remove this check since the server default will be deprecated
	if !defaultFound && location.Name == r.DefaultBackupLocationInfo.StorageLocation {
		// For backward-compatible, to configure the backup storage location as the default if
		// none of the BSLs be marked as the default and the BSL name matches against the
		// "velero server --default-backup-storage-location".
		isDefault = true
		defaultFound = true
	}

	func() {
		// Initialize the patch helper.
		patchHelper, err := patch.NewHelper(&location, r.Client)
		if err != nil {
			log.WithError(err).Error("Error getting a patch helper to update BackupStorageLocation")
			return
		}
		defer func() {
			location.Status.LastValidationTime = &metav1.Time{Time: time.Now().UTC()}
			if err != nil {
				log.Info("BackupStorageLocation is invalid, marking as unavailable")
				err = errors.Wrapf(err, "BackupStorageLocation %q is unavailable", location.Name)
				unavailableErrors = append(unavailableErrors, err.Error())
				location.Status.Phase = velerov1api.BackupStorageLocationPhaseUnavailable
				location.Status.Message = err.Error()
			} else {
				log.Info("BackupStorageLocations is valid, marking as available")
				location.Status.Phase = velerov1api.BackupStorageLocationPhaseAvailable
				location.Status.Message = ""
			}
			if err := patchHelper.Patch(r.Ctx, &location); err != nil {
				log.WithError(err).Error("Error updating BackupStorageLocation phase")
			}
		}()

		backupStore, err := r.BackupStoreGetter.Get(&location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting a backup store")
			return
		}

		log.Info("Validating BackupStorageLocation")
		err = backupStore.IsValid()
		if err != nil {
			log.WithError(err).Error("fail to validate backup store")
			return
		}

		// updates the default backup location
		location.Spec.Default = isDefault
	}()

	r.logReconciledPhase(defaultFound, locationList, unavailableErrors)

	return ctrl.Result{}, nil
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
			log.Errorf("Current BackupStorageLocations available/unavailable/unknown: %v/%v/%v, %s)", numAvailable, numUnavailable, numUnknown, strings.Join(errs, "; "))
		} else {
			log.Errorf("Current BackupStorageLocations available/unavailable/unknown: %v/%v/%v)", numAvailable, numUnavailable, numUnknown)
		}
	} else if numUnavailable > 0 { // some but not all BSL unavailable
		log.Warnf("Unavailable BackupStorageLocations detected: available/unavailable/unknown: %v/%v/%v, %s)", numAvailable, numUnavailable, numUnknown, strings.Join(errs, "; "))
	}

	if !defaultFound {
		log.Warn("There is no existing BackupStorageLocation set as default. Please see `velero backup-location -h` for options.")
	}
}

func (r *BackupStorageLocationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	g := kube.NewPeriodicalEnqueueSource(
		r.Log,
		mgr.GetClient(),
		&velerov1api.BackupStorageLocationList{},
		backupStorageLocationSyncPeriod,
		// Add filter function to enqueue BSL per ValidationFrequency setting.
		func(object client.Object) bool {
			location := object.(*velerov1api.BackupStorageLocation)
			return storage.IsReadyToValidate(location.Spec.ValidationFrequency, location.Status.LastValidationTime, r.DefaultBackupLocationInfo.ServerValidationFrequency, r.Log.WithField("controller", BackupStorageLocation))
		},
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.BackupStorageLocation{}).
		// Handle BSL's creation event and spec update event to let changed BSL got validation immediately.
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(ce event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(ue event.UpdateEvent) bool {
				return ue.ObjectNew.GetGeneration() != ue.ObjectOld.GetGeneration()
			},
			DeleteFunc: func(de event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(ge event.GenericEvent) bool {
				return false
			},
		}).
		Watches(g, nil).
		Complete(r)
}
