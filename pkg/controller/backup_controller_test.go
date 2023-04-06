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
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotfake "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/fake"
	snapshotinformers "github.com/kubernetes-csi/external-snapshotter/client/v4/informers/externalversions"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/clock"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	biav2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/backupitemaction/v2"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type fakeBackupper struct {
	mock.Mock
}

func (b *fakeBackupper) Backup(logger logrus.FieldLogger, backup *pkgbackup.Request, backupFile io.Writer, actions []biav2.BackupItemAction, volumeSnapshotterGetter pkgbackup.VolumeSnapshotterGetter) error {
	args := b.Called(logger, backup, backupFile, actions, volumeSnapshotterGetter)
	return args.Error(0)
}

func (b *fakeBackupper) BackupWithResolvers(logger logrus.FieldLogger, backup *pkgbackup.Request, backupFile io.Writer,
	backupItemActionResolver framework.BackupItemActionResolverV2, volumeSnapshotterGetter pkgbackup.VolumeSnapshotterGetter) error {
	args := b.Called(logger, backup, backupFile, backupItemActionResolver, volumeSnapshotterGetter)
	return args.Error(0)
}

func (b *fakeBackupper) FinalizeBackup(logger logrus.FieldLogger, backup *pkgbackup.Request, inBackupFile io.Reader, outBackupFile io.Writer,
	backupItemActionResolver framework.BackupItemActionResolverV2,
	asyncBIAOperations []*itemoperation.BackupOperation) error {
	args := b.Called(logger, backup, inBackupFile, outBackupFile, backupItemActionResolver, asyncBIAOperations)
	return args.Error(0)
}

func defaultBackup() *builder.BackupBuilder {
	return builder.ForBackup(velerov1api.DefaultNamespace, "backup-1")
}

func TestProcessBackupNonProcessedItems(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		backup *velerov1api.Backup
	}{
		{
			name:   "FailedValidation backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailedValidation).Result(),
		},
		{
			name:   "InProgress backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseInProgress).Result(),
		},
		{
			name:   "Completed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseCompleted).Result(),
		},
		{
			name:   "Failed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailed).Result(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				logger = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			)

			c := &backupReconciler{
				kbClient:   velerotest.NewFakeControllerRuntimeClient(t),
				formatFlag: formatFlag,
				logger:     logger,
			}
			if test.backup != nil {
				require.NoError(t, c.kbClient.Create(context.Background(), test.backup))
			}
			actualResult, err := c.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.backup.Namespace, Name: test.backup.Name}})
			assert.Equal(t, actualResult, ctrl.Result{})
			assert.Nil(t, err)

			// Any backup that would actually proceed to validation will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for validation/etc. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

func TestProcessBackupValidationFailures(t *testing.T) {
	defaultBackupLocation := builder.ForBackupStorageLocation("velero", "loc-1").Result()

	tests := []struct {
		name           string
		backup         *velerov1api.Backup
		backupLocation *velerov1api.BackupStorageLocation
		expectedErrs   []string
	}{
		{
			name:           "invalid included/excluded resources fails validation",
			backup:         defaultBackup().IncludedResources("foo").ExcludedResources("foo").Result(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:           "invalid included/excluded namespaces fails validation",
			backup:         defaultBackup().IncludedNamespaces("foo").ExcludedNamespaces("foo").Result(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:         "non-existent backup location fails validation",
			backup:       defaultBackup().StorageLocation("nonexistent").Result(),
			expectedErrs: []string{"an existing backup storage location wasn't specified at backup creation time and the default 'nonexistent' wasn't found. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: backupstoragelocations.velero.io \"nonexistent\" not found"},
		},
		{
			name:           "backup for read-only backup location fails validation",
			backup:         defaultBackup().StorageLocation("read-only").Result(),
			backupLocation: builder.ForBackupStorageLocation("velero", "read-only").AccessMode(velerov1api.BackupStorageLocationAccessModeReadOnly).Result(),
			expectedErrs:   []string{"backup can't be created because backup storage location read-only is currently in read-only mode"},
		},
		{
			name: "labelSelector as well as orLabelSelectors both are specified in backup request fails validation",
			backup: defaultBackup().LabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}).OrLabelSelector([]*metav1.LabelSelector{{MatchLabels: map[string]string{"a1": "b1"}}, {MatchLabels: map[string]string{"a2": "b2"}},
				{MatchLabels: map[string]string{"a3": "b3"}}, {MatchLabels: map[string]string{"a4": "b4"}}}).Result(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"encountered labelSelector as well as orLabelSelectors in backup spec, only one can be specified"},
		},
		{
			name:           "use old filter parameters and new filter parameters together",
			backup:         defaultBackup().IncludeClusterResources(true).IncludedNamespaceScopedResources("Deployment").IncludedNamespaces("default").Result(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"include-resources, exclude-resources and include-cluster-resources are old filter parameters.\ninclude-cluster-scoped-resources, exclude-cluster-scoped-resources, include-namespace-scoped-resources and exclude-namespace-scoped-resources are new filter parameters.\nThey cannot be used together"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				logger = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			)

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			var fakeClient kbclient.Client
			if test.backupLocation != nil {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t, test.backupLocation)
			} else {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t)
			}

			c := &backupReconciler{
				logger:                logger,
				discoveryHelper:       discoveryHelper,
				kbClient:              fakeClient,
				defaultBackupLocation: defaultBackupLocation.Name,
				clock:                 &clock.RealClock{},
				formatFlag:            formatFlag,
			}

			require.NotNil(t, test.backup)
			require.NoError(t, c.kbClient.Create(context.Background(), test.backup))

			actualResult, err := c.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.backup.Namespace, Name: test.backup.Name}})
			assert.Equal(t, actualResult, ctrl.Result{})
			assert.Nil(t, err)
			res := &velerov1api.Backup{}
			err = c.kbClient.Get(context.Background(), kbclient.ObjectKey{Namespace: test.backup.Namespace, Name: test.backup.Name}, res)
			require.NoError(t, err)

			assert.Equal(t, velerov1api.BackupPhaseFailedValidation, res.Status.Phase)
			assert.Equal(t, test.expectedErrs, res.Status.ValidationErrors)

			// Any backup that would actually proceed to processing will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for running backups. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

func TestBackupLocationLabel(t *testing.T) {
	tests := []struct {
		name                   string
		backup                 *velerov1api.Backup
		backupLocation         *velerov1api.BackupStorageLocation
		expectedBackupLocation string
	}{
		{
			name:                   "valid backup location name should be used as a label",
			backup:                 defaultBackup().Result(),
			backupLocation:         builder.ForBackupStorageLocation("velero", "loc-1").Result(),
			expectedBackupLocation: "loc-1",
		},
		{
			name:                   "invalid storage location name should be handled while creating label",
			backup:                 defaultBackup().Result(),
			backupLocation:         builder.ForBackupStorageLocation("velero", "defaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultdefault").Result(),
			expectedBackupLocation: "defaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultd58343f",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText

			var (
				logger     = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t)
			)

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupReconciler{
				discoveryHelper:       discoveryHelper,
				kbClient:              fakeClient,
				defaultBackupLocation: test.backupLocation.Name,
				clock:                 &clock.RealClock{},
				formatFlag:            formatFlag,
			}

			res := c.prepareBackupRequest(test.backup, logger)
			assert.NotNil(t, res)
			assert.Equal(t, test.expectedBackupLocation, res.Labels[velerov1api.StorageLocationLabel])
		})
	}
}

func Test_prepareBackupRequest_BackupStorageLocation(t *testing.T) {
	var (
		defaultBackupTTL      = metav1.Duration{Duration: 24 * 30 * time.Hour}
		defaultBackupLocation = "default-location"
	)

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)

	tests := []struct {
		name                             string
		backup                           *velerov1api.Backup
		backupLocationNameInBackup       string
		backupLocationInApiServer        *velerov1api.BackupStorageLocation
		defaultBackupLocationInApiServer *velerov1api.BackupStorageLocation
		expectedBackupLocation           string
		expectedSuccess                  bool
		expectedValidationError          string
	}{
		{
			name:                             "BackupLocation is specified in backup CR'spec and it can be found in ApiServer",
			backup:                           builder.ForBackup("velero", "backup-1").Result(),
			backupLocationNameInBackup:       "test-backup-location",
			backupLocationInApiServer:        builder.ForBackupStorageLocation("velero", "test-backup-location").Result(),
			defaultBackupLocationInApiServer: builder.ForBackupStorageLocation("velero", "default-location").Result(),
			expectedBackupLocation:           "test-backup-location",
			expectedSuccess:                  true,
		},
		{
			name:                             "BackupLocation is specified in backup CR'spec and it can't be found in ApiServer",
			backup:                           builder.ForBackup("velero", "backup-1").Result(),
			backupLocationNameInBackup:       "test-backup-location",
			backupLocationInApiServer:        nil,
			defaultBackupLocationInApiServer: nil,
			expectedSuccess:                  false,
			expectedValidationError:          "an existing backup storage location wasn't specified at backup creation time and the default 'test-backup-location' wasn't found. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: backupstoragelocations.velero.io \"test-backup-location\" not found",
		},
		{
			name:                             "Using default BackupLocation and it can be found in ApiServer",
			backup:                           builder.ForBackup("velero", "backup-1").Result(),
			backupLocationNameInBackup:       "",
			backupLocationInApiServer:        builder.ForBackupStorageLocation("velero", "test-backup-location").Result(),
			defaultBackupLocationInApiServer: builder.ForBackupStorageLocation("velero", "default-location").Result(),
			expectedBackupLocation:           defaultBackupLocation,
			expectedSuccess:                  true,
		},
		{
			name:                             "Using default BackupLocation and it can't be found in ApiServer",
			backup:                           builder.ForBackup("velero", "backup-1").Result(),
			backupLocationNameInBackup:       "",
			backupLocationInApiServer:        nil,
			defaultBackupLocationInApiServer: nil,
			expectedSuccess:                  false,
			expectedValidationError:          fmt.Sprintf("an existing backup storage location wasn't specified at backup creation time and the server default '%s' doesn't exist. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: backupstoragelocations.velero.io \"%s\" not found", defaultBackupLocation, defaultBackupLocation),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			var (
				formatFlag = logging.FormatText
				logger     = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
				apiServer  = velerotest.NewAPIServer(t)
			)

			// objects that should init with client
			objects := make([]runtime.Object, 0)
			if test.backupLocationInApiServer != nil {
				objects = append(objects, test.backupLocationInApiServer)
			}
			if test.defaultBackupLocationInApiServer != nil {
				objects = append(objects, test.defaultBackupLocationInApiServer)
			}
			fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objects...)

			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupReconciler{
				discoveryHelper:       discoveryHelper,
				defaultBackupLocation: defaultBackupLocation,
				kbClient:              fakeClient,
				defaultBackupTTL:      defaultBackupTTL.Duration,
				clock:                 testclocks.NewFakeClock(now),
				formatFlag:            formatFlag,
			}

			test.backup.Spec.StorageLocation = test.backupLocationNameInBackup

			// Run
			res := c.prepareBackupRequest(test.backup, logger)

			// Assert
			if test.expectedSuccess {
				assert.Equal(t, test.expectedBackupLocation, res.Spec.StorageLocation)
				assert.NotNil(t, res)
			} else {
				// in every test case, we only trigger one error at once
				if len(res.Status.ValidationErrors) > 1 {
					assert.Fail(t, "multi error found in request")
				}
				assert.Equal(t, test.expectedValidationError, res.Status.ValidationErrors[0])
			}
		})
	}
}

func TestDefaultBackupTTL(t *testing.T) {
	var (
		defaultBackupTTL = metav1.Duration{Duration: 24 * 30 * time.Hour}
	)

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()

	tests := []struct {
		name               string
		backup             *velerov1api.Backup
		backupLocation     *velerov1api.BackupStorageLocation
		expectedTTL        metav1.Duration
		expectedExpiration metav1.Time
	}{
		{
			name:               "backup with no TTL specified",
			backup:             defaultBackup().Result(),
			expectedTTL:        defaultBackupTTL,
			expectedExpiration: metav1.NewTime(now.Add(defaultBackupTTL.Duration)),
		},
		{
			name:               "backup with TTL specified",
			backup:             defaultBackup().TTL(time.Hour).Result(),
			expectedTTL:        metav1.Duration{Duration: 1 * time.Hour},
			expectedExpiration: metav1.NewTime(now.Add(1 * time.Hour)),
		},
	}

	for _, test := range tests {
		formatFlag := logging.FormatText
		var (
			fakeClient kbclient.Client
			logger     = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
		)

		t.Run(test.name, func(t *testing.T) {

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)
			// add the test's backup storage location if it's different than the default
			if test.backupLocation != nil {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t, test.backupLocation)
			} else {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t)
			}
			c := &backupReconciler{
				logger:           logger,
				discoveryHelper:  discoveryHelper,
				kbClient:         fakeClient,
				defaultBackupTTL: defaultBackupTTL.Duration,
				clock:            testclocks.NewFakeClock(now),
				formatFlag:       formatFlag,
			}

			res := c.prepareBackupRequest(test.backup, logger)
			assert.NotNil(t, res)
			assert.Equal(t, test.expectedTTL, res.Spec.TTL)
			assert.Equal(t, test.expectedExpiration, *res.Status.Expiration)
		})
	}
}

func TestDefaultVolumesToResticDeprecation(t *testing.T) {
	tests := []struct {
		name         string
		backup       *velerov1api.Backup
		globalVal    bool
		expectGlobal bool
		expectRemap  bool
		expectVal    bool
	}{
		{
			name:         "DefaultVolumesToRestic is not set, DefaultVolumesToFsBackup is not set",
			backup:       defaultBackup().Result(),
			globalVal:    true,
			expectGlobal: true,
			expectVal:    true,
		},
		{
			name:      "DefaultVolumesToRestic is not set, DefaultVolumesToFsBackup is set to false",
			backup:    defaultBackup().DefaultVolumesToFsBackup(false).Result(),
			globalVal: true,
			expectVal: false,
		},
		{
			name:      "DefaultVolumesToRestic is not set, DefaultVolumesToFsBackup is set to true",
			backup:    defaultBackup().DefaultVolumesToFsBackup(true).Result(),
			globalVal: false,
			expectVal: true,
		},
		{
			name:         "DefaultVolumesToRestic is set to false, DefaultVolumesToFsBackup is not set",
			backup:       defaultBackup().DefaultVolumesToRestic(false).Result(),
			globalVal:    false,
			expectGlobal: true,
			expectVal:    false,
		},
		{
			name:      "DefaultVolumesToRestic is set to false, DefaultVolumesToFsBackup is set to true",
			backup:    defaultBackup().DefaultVolumesToRestic(false).DefaultVolumesToFsBackup(true).Result(),
			globalVal: false,
			expectVal: true,
		},
		{
			name:      "DefaultVolumesToRestic is set to false, DefaultVolumesToFsBackup is set to false",
			backup:    defaultBackup().DefaultVolumesToRestic(false).DefaultVolumesToFsBackup(false).Result(),
			globalVal: true,
			expectVal: false,
		},
		{
			name:        "DefaultVolumesToRestic is set to true, DefaultVolumesToFsBackup is not set",
			backup:      defaultBackup().DefaultVolumesToRestic(true).Result(),
			globalVal:   false,
			expectRemap: true,
			expectVal:   true,
		},
		{
			name:        "DefaultVolumesToRestic is set to true, DefaultVolumesToFsBackup is set to false",
			backup:      defaultBackup().DefaultVolumesToRestic(true).DefaultVolumesToFsBackup(false).Result(),
			globalVal:   false,
			expectRemap: true,
			expectVal:   true,
		},
		{
			name:        "DefaultVolumesToRestic is set to true, DefaultVolumesToFsBackup is set to true",
			backup:      defaultBackup().DefaultVolumesToRestic(true).DefaultVolumesToFsBackup(true).Result(),
			globalVal:   false,
			expectRemap: true,
			expectVal:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText

			var (
				logger     = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t)
			)

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupReconciler{
				logger:                   logger,
				discoveryHelper:          discoveryHelper,
				kbClient:                 fakeClient,
				clock:                    &clock.RealClock{},
				formatFlag:               formatFlag,
				defaultVolumesToFsBackup: test.globalVal,
			}

			res := c.prepareBackupRequest(test.backup, logger)
			assert.NotNil(t, res)
			assert.NotNil(t, res.Spec.DefaultVolumesToFsBackup)
			if test.expectRemap {
				assert.Equal(t, res.Spec.DefaultVolumesToRestic, res.Spec.DefaultVolumesToFsBackup)
			} else if test.expectGlobal {
				assert.False(t, res.Spec.DefaultVolumesToRestic == res.Spec.DefaultVolumesToFsBackup)
				assert.Equal(t, &c.defaultVolumesToFsBackup, res.Spec.DefaultVolumesToFsBackup)
			} else {
				assert.False(t, res.Spec.DefaultVolumesToRestic == res.Spec.DefaultVolumesToFsBackup)
				assert.False(t, &c.defaultVolumesToFsBackup == res.Spec.DefaultVolumesToFsBackup)
			}

			assert.Equal(t, test.expectVal, *res.Spec.DefaultVolumesToFsBackup)
		})
	}
}

func TestProcessBackupCompletions(t *testing.T) {
	defaultBackupLocation := builder.ForBackupStorageLocation("velero", "loc-1").Default(true).Bucket("store-1").Result()

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()
	timestamp := metav1.NewTime(now)

	tests := []struct {
		name                     string
		backup                   *velerov1api.Backup
		backupLocation           *velerov1api.BackupStorageLocation
		defaultVolumesToFsBackup bool
		expectedResult           *velerov1api.Backup
		backupExists             bool
		existenceCheckError      error
	}{
		// Finalizing
		{
			name:                     "backup with no backup location gets the default",
			backup:                   defaultBackup().Result(),
			backupLocation:           defaultBackupLocation,
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:                     "backup with a specific backup location keeps it",
			backup:                   defaultBackup().StorageLocation("alt-loc").Result(),
			backupLocation:           builder.ForBackupStorageLocation("velero", "alt-loc").Bucket("store-1").Result(),
			defaultVolumesToFsBackup: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "alt-loc",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          "alt-loc",
					DefaultVolumesToFsBackup: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:   "backup for a location with ReadWrite access mode gets processed",
			backup: defaultBackup().StorageLocation("read-write").Result(),
			backupLocation: builder.ForBackupStorageLocation("velero", "read-write").
				Bucket("store-1").
				AccessMode(velerov1api.BackupStorageLocationAccessModeReadWrite).
				Result(),
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "read-write",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          "read-write",
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:                     "backup with a TTL has expiration set",
			backup:                   defaultBackup().TTL(10 * time.Minute).Result(),
			backupLocation:           defaultBackupLocation,
			defaultVolumesToFsBackup: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					TTL:                      metav1.Duration{Duration: 10 * time.Minute},
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					Expiration:     &metav1.Time{now.Add(10 * time.Minute)},
					StartTimestamp: &timestamp,
				},
			},
		},
		{
			name:                     "backup without an existing backup will succeed",
			backupExists:             false,
			backup:                   defaultBackup().Result(),
			backupLocation:           defaultBackupLocation,
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:           "backup specifying a false value for 'DefaultVolumesToFsBackup' keeps it",
			backupExists:   false,
			backup:         defaultBackup().DefaultVolumesToFsBackup(false).Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:           "backup specifying a true value for 'DefaultVolumesToFsBackup' keeps it",
			backupExists:   false,
			backup:         defaultBackup().DefaultVolumesToFsBackup(true).Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToFsBackup: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:           "backup specifying no value for 'DefaultVolumesToFsBackup' gets the default true value",
			backupExists:   false,
			backup:         defaultBackup().Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},
		{
			name:           "backup specifying no value for 'DefaultVolumesToFsBackup' gets the default false value",
			backupExists:   false,
			backup:         defaultBackup().Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToFsBackup: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:          velerov1api.BackupPhaseFinalizing,
					Version:        1,
					FormatVersion:  "1.1.0",
					StartTimestamp: &timestamp,
					Expiration:     &timestamp,
				},
			},
		},

		// Failed
		{
			name:                     "backup with existing backup will fail",
			backupExists:             true,
			backup:                   defaultBackup().Result(),
			backupLocation:           defaultBackupLocation,
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseFailed,
					FailureReason:       "backup already exists in object storage",
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:                     "error when checking if backup exists will cause backup to fail",
			backup:                   defaultBackup().Result(),
			existenceCheckError:      errors.New("Backup already exists in object storage"),
			backupLocation:           defaultBackupLocation,
			defaultVolumesToFsBackup: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:          defaultBackupLocation.Name,
					DefaultVolumesToFsBackup: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseFailed,
					FailureReason:       "error checking if backup already exists in object storage: Backup already exists in object storage",
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				logger        = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
				pluginManager = new(pluginmocks.Manager)
				backupStore   = new(persistencemocks.BackupStore)
				backupper     = new(fakeBackupper)
			)

			var fakeClient kbclient.Client
			// add the test's backup storage location if it's different than the default
			if test.backupLocation != nil && test.backupLocation != defaultBackupLocation {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t, test.backupLocation)
			} else {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t)
			}

			apiServer := velerotest.NewAPIServer(t)

			apiServer.DiscoveryClient.FakedServerVersion = &version.Info{
				Major:        "1",
				Minor:        "16",
				GitVersion:   "v1.16.4",
				GitCommit:    "FakeTest",
				GitTreeState: "",
				BuildDate:    "",
				GoVersion:    "",
				Compiler:     "",
				Platform:     "",
			}

			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupReconciler{
				logger:                   logger,
				discoveryHelper:          discoveryHelper,
				kbClient:                 fakeClient,
				defaultBackupLocation:    defaultBackupLocation.Name,
				defaultVolumesToFsBackup: test.defaultVolumesToFsBackup,
				backupTracker:            NewBackupTracker(),
				metrics:                  metrics.NewServerMetrics(),
				clock:                    testclocks.NewFakeClock(now),
				newPluginManager:         func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				backupStoreGetter:        NewFakeSingleObjectBackupStoreGetter(backupStore),
				backupper:                backupper,
				formatFlag:               formatFlag,
			}

			pluginManager.On("GetBackupItemActionsV2").Return(nil, nil)
			pluginManager.On("CleanupClients").Return(nil)
			backupper.On("Backup", mock.Anything, mock.Anything, mock.Anything, []biav2.BackupItemAction(nil), pluginManager).Return(nil)
			backupper.On("BackupWithResolvers", mock.Anything, mock.Anything, mock.Anything, framework.BackupItemActionResolverV2{}, pluginManager).Return(nil)
			backupStore.On("BackupExists", test.backupLocation.Spec.StorageType.ObjectStorage.Bucket, test.backup.Name).Return(test.backupExists, test.existenceCheckError)

			// Ensure we have a CompletionTimestamp when uploading and that the backup name matches the backup in the object store.
			// Failures will display the bytes in buf.
			hasNameAndCompletionTimestampIfCompleted := func(info persistence.BackupInfo) bool {
				buf := new(bytes.Buffer)
				buf.ReadFrom(info.Metadata)
				return info.Name == test.backup.Name &&
					(!(strings.Contains(buf.String(), `"phase": "Completed"`) ||
						strings.Contains(buf.String(), `"phase": "Failed"`) ||
						strings.Contains(buf.String(), `"phase": "PartiallyFailed"`)) ||
						strings.Contains(buf.String(), `"completionTimestamp": "2006-01-02T22:04:05Z"`))
			}
			backupStore.On("PutBackup", mock.MatchedBy(hasNameAndCompletionTimestampIfCompleted)).Return(nil)

			// add the test's backup to the informer/lister store
			require.NotNil(t, test.backup)

			require.NoError(t, c.kbClient.Create(context.Background(), test.backup))

			// add the default backup storage location to the clientset and the informer/lister store
			require.NoError(t, fakeClient.Create(context.Background(), defaultBackupLocation))

			actualResult, err := c.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.backup.Namespace, Name: test.backup.Name}})
			assert.Equal(t, actualResult, ctrl.Result{})
			assert.Nil(t, err)

			res := &velerov1api.Backup{}
			err = c.kbClient.Get(context.Background(), kbclient.ObjectKey{Namespace: test.backup.Namespace, Name: test.backup.Name}, res)
			require.NoError(t, err)
			res.ResourceVersion = ""
			assert.Equal(t, test.expectedResult, res)
			// reset defaultBackupLocation resourceVersion
			defaultBackupLocation.ObjectMeta.ResourceVersion = ""
		})
	}
}

func TestValidateAndGetSnapshotLocations(t *testing.T) {
	tests := []struct {
		name                                string
		backup                              *velerov1api.Backup
		locations                           []*velerov1api.VolumeSnapshotLocation
		defaultLocations                    map[string]string
		expectedVolumeSnapshotLocationNames []string // adding these in the expected order will allow to test with better msgs in case of a test failure
		expectedErrors                      string
		expectedSuccess                     bool
	}{
		{
			name:   "location name does not correspond to any existing location",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("random-name").Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedErrors: "a VolumeSnapshotLocation CRD for the location random-name with the name specified in the backup spec needs to be created before this snapshot can be executed. Error: volumesnapshotlocations.velero.io \"random-name\" not found", expectedSuccess: false,
		},
		{
			name:   "duplicate locationName per provider: should filter out dups",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-west-1"},
			expectedSuccess:                     true,
		},
		{
			name:   "multiple non-dupe location names per provider should error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-east-1", "aws-us-west-1").Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedErrors:  "more than one VolumeSnapshotLocation name specified for provider aws: aws-us-west-1; unexpected name was aws-us-east-1",
			expectedSuccess: false,
		},
		{
			name:   "no location name for the provider exists, only one VSL for the provider: use it",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-east-1"},
			expectedSuccess:                     true,
		},
		{
			name:   "no location name for the provider exists, no default, more than one VSL for the provider: error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedErrors: "provider aws has more than one possible volume snapshot location, and none were specified explicitly or as a default",
		},
		{
			name:             "no location name for the provider exists, more than one VSL for the provider: the provider's default should be added",
			backup:           defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			defaultLocations: map[string]string{"aws": "aws-us-east-1"},
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-east-1"},
			expectedSuccess:                     true,
		},
		{
			name:            "no existing location name and no default location name given",
			backup:          defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			expectedSuccess: true,
		},
		{
			name:             "multiple location names for a provider, default location name for another provider",
			backup:           defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").Result(),
			defaultLocations: map[string]string{"fake-provider": "some-name"},
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-west-1", "some-name"},
			expectedSuccess:                     true,
		},
		{
			name:   "location name does not correspond to any existing location and snapshotvolume disabled; should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("random-name").SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
		{
			name:   "duplicate locationName per provider and snapshotvolume disabled; should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
		{
			name:   "no location name for the provider exists, only one VSL created and snapshotvolume disabled; should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
		{
			name:   "multiple location names for a provider, no default location and backup has no location defined, but snapshotvolume disabled, should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				logger = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			)

			c := &backupReconciler{
				logger:                   logger,
				defaultSnapshotLocations: test.defaultLocations,
				kbClient:                 velerotest.NewFakeControllerRuntimeClient(t),
			}

			// set up a Backup object to represent what we expect to be passed to backupper.Backup()
			backup := test.backup.DeepCopy()
			backup.Spec.VolumeSnapshotLocations = test.backup.Spec.VolumeSnapshotLocations
			for _, location := range test.locations {
				require.NoError(t, c.kbClient.Create(context.Background(), location))
			}

			providerLocations, errs := c.validateAndGetSnapshotLocations(backup)
			if test.expectedSuccess {
				for _, err := range errs {
					require.NoError(t, errors.New(err), "validateAndGetSnapshotLocations unexpected error: %v", err)
				}

				var locations []string
				for _, loc := range providerLocations {
					locations = append(locations, loc.Name)
				}

				sort.Strings(test.expectedVolumeSnapshotLocationNames)
				sort.Strings(locations)
				require.Equal(t, test.expectedVolumeSnapshotLocationNames, locations)
			} else {
				if len(errs) == 0 {
					require.Error(t, nil, "validateAndGetSnapshotLocations expected error")
				}
				require.Contains(t, errs, test.expectedErrors)
			}
		})
	}
}

// Test_getLastSuccessBySchedule verifies that the getLastSuccessBySchedule helper function correctly returns
// the completion timestamp of the most recent completed backup for each schedule, including an entry for ad-hoc
// or non-scheduled backups.
func Test_getLastSuccessBySchedule(t *testing.T) {
	buildBackup := func(phase velerov1api.BackupPhase, completion time.Time, schedule string) velerov1api.Backup {
		b := builder.ForBackup("", "").
			ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, schedule)).
			Phase(phase)

		if !completion.IsZero() {
			b.CompletionTimestamp(completion)
		}

		return *b.Result()
	}

	// create a static "base time" that can be used to easily construct completion timestamps
	// by using the .Add(...) method.
	baseTime, err := time.Parse(time.RFC1123, time.RFC1123)
	require.NoError(t, err)

	tests := []struct {
		name    string
		backups []velerov1api.Backup
		want    map[string]time.Time
	}{
		{
			name:    "when backups is nil, an empty map is returned",
			backups: nil,
			want:    map[string]time.Time{},
		},
		{
			name:    "when backups is empty, an empty map is returned",
			backups: []velerov1api.Backup{},
			want:    map[string]time.Time{},
		},
		{
			name: "when multiple completed backups for a schedule exist, the latest one is returned",
			backups: []velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{
				"schedule-1": baseTime.Add(time.Second),
			},
		},
		{
			name: "when the most recent backup for a schedule is Failed, the timestamp of the most recent Completed one is returned",
			backups: []velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{
				"schedule-1": baseTime,
			},
		},
		{
			name: "when there are no Completed backups for a schedule, it's not returned",
			backups: []velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseInProgress, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhasePartiallyFailed, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{},
		},
		{
			name: "when backups exist without a schedule, the most recent Completed one is returned",
			backups: []velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, ""),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), ""),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), ""),
			},
			want: map[string]time.Time{
				"": baseTime,
			},
		},
		{
			name: "when backups exist for multiple schedules, the most recent Completed timestamp for each schedule is returned",
			backups: []velerov1api.Backup{
				// ad-hoc backups (no schedule)
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(30*time.Minute), ""),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Hour), ""),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), ""),

				// schedule-1
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),

				// schedule-2
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(24*time.Hour), "schedule-2"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(48*time.Hour), "schedule-2"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(72*time.Hour), "schedule-2"),

				// schedule-3
				buildBackup(velerov1api.BackupPhaseNew, baseTime, "schedule-3"),
				buildBackup(velerov1api.BackupPhaseInProgress, baseTime.Add(time.Minute), "schedule-3"),
				buildBackup(velerov1api.BackupPhasePartiallyFailed, baseTime.Add(2*time.Minute), "schedule-3"),
			},
			want: map[string]time.Time{
				"":           baseTime.Add(30 * time.Minute),
				"schedule-1": baseTime,
				"schedule-2": baseTime.Add(72 * time.Hour),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, getLastSuccessBySchedule(tc.backups))
		})
	}
}

func TestDeleteVolumeSnapshots(t *testing.T) {
	tests := []struct {
		name             string
		vsArray          []snapshotv1api.VolumeSnapshot
		vscArray         []snapshotv1api.VolumeSnapshotContent
		expectedVSArray  []snapshotv1api.VolumeSnapshot
		expectedVSCArray []snapshotv1api.VolumeSnapshotContent
	}{
		{
			name: "VS is ReadyToUse, and VS has corresponding VSC. VS should be deleted.",
			vsArray: []snapshotv1api.VolumeSnapshot{
				*builder.ForVolumeSnapshot("velero", "vs1").ObjectMeta(builder.WithLabels("testing-vs", "vs1")).Status().BoundVolumeSnapshotContentName("vsc1").Result(),
			},
			vscArray: []snapshotv1api.VolumeSnapshotContent{
				*builder.ForVolumeSnapshotContent("vsc1").DeletionPolicy(snapshotv1api.VolumeSnapshotContentDelete).Status().Result(),
			},
			expectedVSArray: []snapshotv1api.VolumeSnapshot{},
			expectedVSCArray: []snapshotv1api.VolumeSnapshotContent{
				*builder.ForVolumeSnapshotContent("vsc1").DeletionPolicy(snapshotv1api.VolumeSnapshotContentRetain).VolumeSnapshotRef("ns-", "name-").Status().Result(),
			},
		},
		{
			name: "VS is ReadyToUse, and VS has corresponding VSC. Concurrent test.",
			vsArray: []snapshotv1api.VolumeSnapshot{
				*builder.ForVolumeSnapshot("velero", "vs1").ObjectMeta(builder.WithLabels("testing-vs", "vs1")).Status().BoundVolumeSnapshotContentName("vsc1").Result(),
				*builder.ForVolumeSnapshot("velero", "vs2").ObjectMeta(builder.WithLabels("testing-vs", "vs2")).Status().BoundVolumeSnapshotContentName("vsc2").Result(),
				*builder.ForVolumeSnapshot("velero", "vs3").ObjectMeta(builder.WithLabels("testing-vs", "vs3")).Status().BoundVolumeSnapshotContentName("vsc3").Result(),
			},
			vscArray: []snapshotv1api.VolumeSnapshotContent{
				*builder.ForVolumeSnapshotContent("vsc1").DeletionPolicy(snapshotv1api.VolumeSnapshotContentDelete).Status().Result(),
				*builder.ForVolumeSnapshotContent("vsc2").DeletionPolicy(snapshotv1api.VolumeSnapshotContentDelete).Status().Result(),
				*builder.ForVolumeSnapshotContent("vsc3").DeletionPolicy(snapshotv1api.VolumeSnapshotContentDelete).Status().Result(),
			},
			expectedVSArray: []snapshotv1api.VolumeSnapshot{},
			expectedVSCArray: []snapshotv1api.VolumeSnapshotContent{
				*builder.ForVolumeSnapshotContent("vsc1").DeletionPolicy(snapshotv1api.VolumeSnapshotContentRetain).VolumeSnapshotRef("ns-", "name-").Status().Result(),
				*builder.ForVolumeSnapshotContent("vsc2").DeletionPolicy(snapshotv1api.VolumeSnapshotContentRetain).VolumeSnapshotRef("ns-", "name-").Status().Result(),
				*builder.ForVolumeSnapshotContent("vsc3").DeletionPolicy(snapshotv1api.VolumeSnapshotContentRetain).VolumeSnapshotRef("ns-", "name-").Status().Result(),
			},
		},
		{
			name: "Corresponding VSC not found for VS. VS is not deleted.",
			vsArray: []snapshotv1api.VolumeSnapshot{
				*builder.ForVolumeSnapshot("velero", "vs1").ObjectMeta(builder.WithLabels("testing-vs", "vs1")).Status().BoundVolumeSnapshotContentName("vsc1").Result(),
			},
			vscArray: []snapshotv1api.VolumeSnapshotContent{},
			expectedVSArray: []snapshotv1api.VolumeSnapshot{
				*builder.ForVolumeSnapshot("velero", "vs1").Status().BoundVolumeSnapshotContentName("vsc1").Result(),
			},
			expectedVSCArray: []snapshotv1api.VolumeSnapshotContent{},
		},
		{
			name: "VS status is nil. VSC should not be modified.",
			vsArray: []snapshotv1api.VolumeSnapshot{
				*builder.ForVolumeSnapshot("velero", "vs1").ObjectMeta(builder.WithLabels("testing-vs", "vs1")).Result(),
			},
			vscArray: []snapshotv1api.VolumeSnapshotContent{
				*builder.ForVolumeSnapshotContent("vsc1").DeletionPolicy(snapshotv1api.VolumeSnapshotContentDelete).Status().Result(),
			},
			expectedVSArray: []snapshotv1api.VolumeSnapshot{},
			expectedVSCArray: []snapshotv1api.VolumeSnapshotContent{
				*builder.ForVolumeSnapshotContent("vsc1").DeletionPolicy(snapshotv1api.VolumeSnapshotContentDelete).Status().Result(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				fakeClient = velerotest.NewFakeControllerRuntimeClientBuilder(t).WithLists(
					&snapshotv1api.VolumeSnapshotContentList{Items: tc.vscArray},
				).Build()
				vsClient        = snapshotfake.NewSimpleClientset()
				sharedInformers = snapshotinformers.NewSharedInformerFactory(vsClient, 0)
			)
			c := &backupReconciler{
				kbClient:             fakeClient,
				volumeSnapshotLister: sharedInformers.Snapshot().V1().VolumeSnapshots().Lister(),
				volumeSnapshotClient: vsClient,
			}

			for _, vs := range tc.vsArray {
				_, err := c.volumeSnapshotClient.SnapshotV1().VolumeSnapshots(vs.Namespace).Create(context.Background(), &vs, metav1.CreateOptions{})
				require.NoError(t, err)
				require.NoError(t, sharedInformers.Snapshot().V1().VolumeSnapshots().Informer().GetStore().Add(&vs))
			}
			logger := logging.DefaultLogger(logrus.DebugLevel, logging.FormatText)

			c.deleteVolumeSnapshots(tc.vsArray, tc.vscArray, logger, 30)

			vsList, err := c.volumeSnapshotClient.SnapshotV1().VolumeSnapshots("velero").List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)
			assert.Equal(t, len(tc.expectedVSArray), len(vsList.Items))
			for index := range tc.expectedVSArray {
				assert.Equal(t, tc.expectedVSArray[index].Status, vsList.Items[index].Status)
				assert.Equal(t, tc.expectedVSArray[index].Spec, vsList.Items[index].Spec)
			}

			vscList := &snapshotv1api.VolumeSnapshotContentList{}
			require.NoError(t, c.kbClient.List(context.Background(), vscList))
			assert.Equal(t, len(tc.expectedVSCArray), len(vscList.Items))
			for index := range tc.expectedVSCArray {
				assert.Equal(t, tc.expectedVSCArray[index].Spec, vscList.Items[index].Spec)
			}
		})
	}
}
