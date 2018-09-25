/*
Copyright 2017 the Heptio Ark contributors.

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
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	core "k8s.io/client-go/testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/metrics"
	"github.com/heptio/ark/pkg/persistence"
	persistencemocks "github.com/heptio/ark/pkg/persistence/mocks"
	"github.com/heptio/ark/pkg/plugin"
	pluginmocks "github.com/heptio/ark/pkg/plugin/mocks"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/heptio/ark/pkg/util/logging"
	arktest "github.com/heptio/ark/pkg/util/test"
)

type fakeBackupper struct {
	mock.Mock
}

func (b *fakeBackupper) Backup(logger logrus.FieldLogger, backup *v1.Backup, backupFile io.Writer, actions []backup.ItemAction) error {
	args := b.Called(logger, backup, backupFile, actions)
	return args.Error(0)
}

func TestProcessBackup(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		expectError      bool
		expectedIncludes []string
		expectedExcludes []string
		backup           *arktest.TestBackup
		expectBackup     bool
		allowSnapshots   bool
		defaultLocations map[string]string
	}{
		{
			name:        "bad key",
			key:         "bad/key/here",
			expectError: true,
		},
		{
			name:        "lister failed",
			key:         "heptio-ark/backup1",
			expectError: true,
		},
		{
			name:         "do not process phase FailedValidation",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseFailedValidation),
			expectBackup: false,
		},
		{
			name:         "do not process phase InProgress",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseInProgress),
			expectBackup: false,
		},
		{
			name:         "do not process phase Completed",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseCompleted),
			expectBackup: false,
		},
		{
			name:         "do not process phase Failed",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseFailed),
			expectBackup: false,
		},
		{
			name:         "do not process phase other",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase("arg"),
			expectBackup: false,
		},
		{
			name:         "invalid included/excluded resources fails validation",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithIncludedResources("foo").WithExcludedResources("foo"),
			expectBackup: false,
		},
		{
			name:         "invalid included/excluded namespaces fails validation",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithIncludedNamespaces("foo").WithExcludedNamespaces("foo"),
			expectBackup: false,
		},
		{
			name:             "make sure specified included and excluded resources are honored",
			key:              "heptio-ark/backup1",
			backup:           arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithIncludedResources("i", "j").WithExcludedResources("k", "l"),
			expectedIncludes: []string{"i", "j"},
			expectedExcludes: []string{"k", "l"},
			expectBackup:     true,
		},
		{
			name:         "if includednamespaces are specified, don't default to *",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithIncludedNamespaces("ns-1"),
			expectBackup: true,
		},
		{
			name:         "ttl",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithTTL(10 * time.Minute),
			expectBackup: true,
		},
		{
			name:         "backup with SnapshotVolumes when allowSnapshots=false fails validation",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithSnapshotVolumes(true),
			expectBackup: false,
		},
		{
			name:           "backup with SnapshotVolumes when allowSnapshots=true gets executed",
			key:            "heptio-ark/backup1",
			backup:         arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithSnapshotVolumes(true),
			allowSnapshots: true,
			expectBackup:   true,
		},
		{
			name:         "Backup without a location will have it set to the default",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew),
			expectBackup: true,
		},
		{
			name:         "Backup with a location completes",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithStorageLocation("loc1"),
			expectBackup: true,
		},
		{
			name:         "Backup with non-existent location will fail validation",
			key:          "heptio-ark/backup1",
			backup:       arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithStorageLocation("loc2"),
			expectBackup: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				backupper       = &fakeBackupper{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel)
				clockTime, _    = time.Parse("Mon Jan 2 15:04:05 2006", "Mon Jan 2 15:04:05 2006")
				pluginManager   = &pluginmocks.Manager{}
				backupStore     = &persistencemocks.BackupStore{}
			)
			defer backupper.AssertExpectations(t)
			defer pluginManager.AssertExpectations(t)
			defer backupStore.AssertExpectations(t)

			c := NewBackupController(
				sharedInformers.Ark().V1().Backups(),
				client.ArkV1(),
				backupper,
				test.allowSnapshots,
				logger,
				logrus.InfoLevel,
				func(logrus.FieldLogger) plugin.Manager { return pluginManager },
				NewBackupTracker(),
				sharedInformers.Ark().V1().BackupStorageLocations(),
				"default",
				sharedInformers.Ark().V1().VolumeSnapshotLocations(),
				test.defaultLocations,
				metrics.NewServerMetrics(),
			).(*backupController)

			c.clock = clock.NewFakeClock(clockTime)

			c.newBackupStore = func(*v1.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
				return backupStore, nil
			}

			var expiration, startTime time.Time

			if test.backup != nil {
				// add directly to the informer's store so the lister can function and so we don't have to
				// start the shared informers.
				sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup.Backup)

				startTime = c.clock.Now()

				if test.backup.Spec.TTL.Duration > 0 {
					expiration = c.clock.Now().Add(test.backup.Spec.TTL.Duration)
				}
			}

			if test.expectBackup {
				// set up a Backup object to represent what we expect to be passed to backupper.Backup()
				backup := test.backup.DeepCopy()
				backup.Spec.IncludedResources = test.expectedIncludes
				backup.Spec.ExcludedResources = test.expectedExcludes
				backup.Spec.IncludedNamespaces = test.backup.Spec.IncludedNamespaces
				backup.Spec.SnapshotVolumes = test.backup.Spec.SnapshotVolumes
				backup.Status.Phase = v1.BackupPhaseInProgress
				backup.Status.Expiration.Time = expiration
				backup.Status.StartTimestamp.Time = startTime
				backup.Status.Version = 1
				backupper.On("Backup",
					mock.Anything, // logger
					backup,
					mock.Anything, // backup file
					mock.Anything, // actions
				).Return(nil)

				defaultLocation := &v1.BackupStorageLocation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: backup.Namespace,
						Name:      "default",
					},
					Spec: v1.BackupStorageLocationSpec{
						Provider: "myCloud",
						StorageType: v1.StorageType{
							ObjectStorage: &v1.ObjectStorageLocation{
								Bucket: "bucket",
							},
						},
					},
				}
				loc1 := &v1.BackupStorageLocation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: backup.Namespace,
						Name:      "loc1",
					},
					Spec: v1.BackupStorageLocationSpec{
						Provider: "myCloud",
						StorageType: v1.StorageType{
							ObjectStorage: &v1.ObjectStorageLocation{
								Bucket: "bucket",
							},
						},
					},
				}
				require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(defaultLocation))
				require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(loc1))

				pluginManager.On("GetBackupItemActions").Return(nil, nil)

				// Ensure we have a CompletionTimestamp when uploading.
				// Failures will display the bytes in buf.
				completionTimestampIsPresent := func(buf *bytes.Buffer) bool {
					json := buf.String()
					timeString := `"completionTimestamp": "2006-01-02T15:04:05Z"`

					return strings.Contains(json, timeString)
				}
				backupStore.On("PutBackup", test.backup.Name, mock.MatchedBy(completionTimestampIsPresent), mock.Anything, mock.Anything).Return(nil)
				pluginManager.On("CleanupClients").Return()
			}

			// this is necessary so the Patch() call returns the appropriate object
			client.PrependReactor("patch", "backups", func(action core.Action) (bool, runtime.Object, error) {
				if test.backup == nil {
					return true, nil, nil
				}

				patch := action.(core.PatchAction).GetPatch()
				patchMap := make(map[string]interface{})

				if err := json.Unmarshal(patch, &patchMap); err != nil {
					t.Logf("error unmarshalling patch: %s\n", err)
					return false, nil, err
				}

				phase, err := collections.GetString(patchMap, "status.phase")
				if err != nil {
					t.Logf("error getting status.phase: %s\n", err)
					return false, nil, err
				}

				res := test.backup.DeepCopy()

				// these are the fields that we expect to be set by
				// the controller
				res.Status.Version = 1
				res.Status.Expiration.Time = expiration
				res.Status.Phase = v1.BackupPhase(phase)

				// If there's an error, it's mostly likely that the key wasn't found
				// which is fine since not all patches will have them.
				completionString, err := collections.GetString(patchMap, "status.completionTimestamp")
				if err == nil {
					completionTime, err := time.Parse(time.RFC3339Nano, completionString)
					require.NoError(t, err, "unexpected completionTimestamp parsing error %v", err)
					res.Status.CompletionTimestamp.Time = completionTime
				}
				startString, err := collections.GetString(patchMap, "status.startTimestamp")
				if err == nil {
					startTime, err := time.Parse(time.RFC3339Nano, startString)
					require.NoError(t, err, "unexpected startTimestamp parsing error %v", err)
					res.Status.StartTimestamp.Time = startTime
				}

				return true, res, nil
			})

			// method under test
			err := c.processBackup(test.key)

			if test.expectError {
				require.Error(t, err, "processBackup should error")
				return
			}
			require.NoError(t, err, "processBackup unexpected error: %v", err)

			if !test.expectBackup {
				// the AssertExpectations calls above make sure we aren't running anything we shouldn't be
				return
			}

			actions := client.Actions()
			require.Equal(t, 2, len(actions))

			// structs and func for decoding patch content
			type StatusPatch struct {
				Expiration          time.Time      `json:"expiration"`
				Version             int            `json:"version"`
				Phase               v1.BackupPhase `json:"phase"`
				StartTimestamp      metav1.Time    `json:"startTimestamp"`
				CompletionTimestamp metav1.Time    `json:"completionTimestamp"`
			}
			type SpecPatch struct {
				StorageLocation string `json:"storageLocation"`
			}
			type ObjectMetaPatch struct {
				Labels map[string]string `json:"labels"`
			}

			type Patch struct {
				Status     StatusPatch     `json:"status"`
				Spec       SpecPatch       `json:"spec,omitempty"`
				ObjectMeta ObjectMetaPatch `json:"metadata,omitempty"`
			}

			decode := func(decoder *json.Decoder) (interface{}, error) {
				actual := new(Patch)
				err := decoder.Decode(actual)

				return *actual, err
			}

			// validate Patch call 1 (setting version, expiration, phase, and storage location)
			var expected Patch
			if test.backup.Spec.StorageLocation == "" {
				expected = Patch{
					Status: StatusPatch{
						Version:    1,
						Phase:      v1.BackupPhaseInProgress,
						Expiration: expiration,
					},
					Spec: SpecPatch{
						StorageLocation: "default",
					},
					ObjectMeta: ObjectMetaPatch{
						Labels: map[string]string{
							v1.StorageLocationLabel: "default",
						},
					},
				}
			} else {
				expected = Patch{
					Status: StatusPatch{
						Version:    1,
						Phase:      v1.BackupPhaseInProgress,
						Expiration: expiration,
					},
					ObjectMeta: ObjectMetaPatch{
						Labels: map[string]string{
							v1.StorageLocationLabel: test.backup.Spec.StorageLocation,
						},
					},
				}
			}

			arktest.ValidatePatch(t, actions[0], expected, decode)

			// validate Patch call 2 (setting phase, startTimestamp, completionTimestamp)
			expected = Patch{
				Status: StatusPatch{
					Phase:               v1.BackupPhaseCompleted,
					StartTimestamp:      metav1.Time{Time: c.clock.Now()},
					CompletionTimestamp: metav1.Time{Time: c.clock.Now()},
				},
			}
			arktest.ValidatePatch(t, actions[1], expected, decode)
		})
	}
}

func TestDefaultAndValidateSnapshotLocations(t *testing.T) {
	defaultLocationsAWS := map[string]string{"aws": "aws-us-east-2"}
	defaultLocationsFake := map[string]string{"fake-provider": "some-name"}

	multipleLocationNames := []string{"aws-us-west-1", "aws-us-east-1"}

	multipleLocation1 := arktest.LocationInfo{
		Name:     multipleLocationNames[0],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}
	multipleLocation2 := arktest.LocationInfo{
		Name:     multipleLocationNames[1],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}

	multipleLocationList := []arktest.LocationInfo{multipleLocation1, multipleLocation2}

	dupLocationNames := []string{"aws-us-west-1", "aws-us-west-1"}
	dupLocation1 := arktest.LocationInfo{
		Name:     dupLocationNames[0],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}
	dupLocation2 := arktest.LocationInfo{
		Name:     dupLocationNames[0],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}
	dupLocationList := []arktest.LocationInfo{dupLocation1, dupLocation2}

	tests := []struct {
		name                                string
		backup                              *arktest.TestBackup
		locations                           []*arktest.TestVolumeSnapshotLocation
		defaultLocations                    map[string]string
		expectedVolumeSnapshotLocationNames []string // adding these in the expected order will allow to test with better msgs in case of a test failure
		expectedErrors                      string
		expectedSuccess                     bool
	}{
		{
			name:            "location name does not correspond to any existing location",
			backup:          arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations([]string{"random-name"}),
			locations:       arktest.NewTestVolumeSnapshotLocation().WithName(dupLocationNames[0]).WithProviderConfig(dupLocationList),
			expectedErrors:  "error getting volume snapshot location named random-name: volumesnapshotlocation.ark.heptio.com \"random-name\" not found",
			expectedSuccess: false,
		},
		{
			name:                                "duplicate locationName per provider: should filter out dups",
			backup:                              arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations(dupLocationNames),
			locations:                           arktest.NewTestVolumeSnapshotLocation().WithName(dupLocationNames[0]).WithProviderConfig(dupLocationList),
			expectedVolumeSnapshotLocationNames: []string{dupLocationNames[0]},
			expectedSuccess:                     true,
		},
		{
			name:            "multiple location names per provider",
			backup:          arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations(multipleLocationNames),
			locations:       arktest.NewTestVolumeSnapshotLocation().WithName(multipleLocationNames[0]).WithProviderConfig(multipleLocationList),
			expectedErrors:  "more than one VolumeSnapshotLocation name specified for provider aws: aws-us-east-1; unexpected name was aws-us-west-1",
			expectedSuccess: false,
		},
		{
			name:                                "no location name for the provider exists: the provider's default should be added",
			backup:                              arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew),
			defaultLocations:                    defaultLocationsAWS,
			expectedVolumeSnapshotLocationNames: []string{defaultLocationsAWS["aws"]},
			expectedSuccess:                     true,
		},
		{
			name:            "no existing location name and no default location name given",
			backup:          arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew),
			expectedSuccess: true,
		},
		{
			name:                                "multiple location names for a provider, default location name for another provider",
			backup:                              arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations(dupLocationNames),
			locations:                           arktest.NewTestVolumeSnapshotLocation().WithName(dupLocationNames[0]).WithProviderConfig(dupLocationList),
			defaultLocations:                    defaultLocationsFake,
			expectedVolumeSnapshotLocationNames: []string{dupLocationNames[0], defaultLocationsFake["fake-provider"]},
			expectedSuccess:                     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			c := &backupController{
				snapshotLocationLister: sharedInformers.Ark().V1().VolumeSnapshotLocations().Lister(),
			}

			// set up a Backup object to represent what we expect to be passed to backupper.Backup()
			backup := test.backup.DeepCopy()
			backup.Spec.VolumeSnapshotLocations = test.backup.Spec.VolumeSnapshotLocations
			for _, location := range test.locations {
				require.NoError(t, sharedInformers.Ark().V1().VolumeSnapshotLocations().Informer().GetStore().Add(location.VolumeSnapshotLocation))
			}

			errs := c.defaultAndValidateSnapshotLocations(backup, test.defaultLocations)
			if test.expectedSuccess {
				for _, err := range errs {
					require.NoError(t, errors.New(err), "defaultAndValidateSnapshotLocations unexpected error: %v", err)
				}
				require.Equal(t, test.expectedVolumeSnapshotLocationNames, backup.Spec.VolumeSnapshotLocations)
			} else {
				if len(errs) == 0 {
					require.Error(t, nil, "defaultAndValidateSnapshotLocations expected error")
				}
				require.Contains(t, errs, test.expectedErrors)
			}
		})
	}
}
