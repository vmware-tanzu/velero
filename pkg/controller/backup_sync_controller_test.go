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
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	core "k8s.io/client-go/testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	cloudprovidermocks "github.com/heptio/ark/pkg/cloudprovider/mocks"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/util/stringslice"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestBackupSyncControllerRun(t *testing.T) {
	tests := []struct {
		name             string
		listBackupsError error
		cloudBackups     []*v1.Backup
		namespace        string
		existingBackups  sets.String
	}{
		{
			name: "no cloud backups",
		},
		{
			name:             "backup lister returns error on ListBackups",
			listBackupsError: errors.New("listBackups"),
		},
		{
			name: "normal case",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			namespace: "ns-1",
		},
		{
			name: "Finalizer gets removed on sync",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers(gcFinalizer).Backup,
			},
			namespace: "ns-1",
		},
		{
			name: "Only target finalizer is removed",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers(gcFinalizer, "blah").Backup,
			},
			namespace: "ns-1",
		},
		{
			name: "backups get created in Ark server's namespace",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-2").WithName("backup-2").Backup,
			},
			namespace: "heptio-ark",
		},
		{
			name: "normal case with backups that already exist in Kubernetes",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			existingBackups: sets.NewString("backup-2", "backup-3"),
			namespace:       "ns-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				backupLister    = &cloudprovidermocks.BackupLister{}
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = arktest.NewLogger()
			)

			c := NewBackupSyncController(
				client.ArkV1(),
				backupLister,
				"bucket",
				time.Duration(0),
				test.namespace,
				sharedInformers.Ark().V1().Backups(),
				logger,
			).(*backupSyncController)

			backupLister.On("ListBackups", "bucket").Return(test.cloudBackups, test.listBackupsError)

			expectedActions := make([]core.Action, 0)

			client.PrependReactor("get", "backups", func(action core.Action) (bool, runtime.Object, error) {
				getAction := action.(core.GetAction)
				if test.existingBackups.Has(getAction.GetName()) {
					return true, nil, nil
				}
				// We return nil in place of the found backup object because
				// we exclusively check for the error and don't use the object
				// returned by the Get / Backups call.
				return true, nil, apierrors.NewNotFound(v1.SchemeGroupVersion.WithResource("backups").GroupResource(), getAction.GetName())
			})

			c.run()

			// we only expect creates for items within the target bucket
			for _, cloudBackup := range test.cloudBackups {
				// Verify that the run function stripped the GC finalizer
				assert.False(t, stringslice.Has(cloudBackup.Finalizers, gcFinalizer))
				assert.Equal(t, test.namespace, cloudBackup.Namespace)

				actionGet := core.NewGetAction(
					v1.SchemeGroupVersion.WithResource("backups"),
					test.namespace,
					cloudBackup.Name,
				)
				expectedActions = append(expectedActions, actionGet)

				if test.existingBackups.Has(cloudBackup.Name) {
					continue
				}
				actionCreate := core.NewCreateAction(
					v1.SchemeGroupVersion.WithResource("backups"),
					test.namespace,
					cloudBackup,
				)
				expectedActions = append(expectedActions, actionCreate)
			}

			assert.Equal(t, expectedActions, client.Actions())
		})
	}
}

func TestDeleteUnused(t *testing.T) {
	tests := []struct {
		name            string
		cloudBackups    []*v1.Backup
		k8sBackups      []*arktest.TestBackup
		namespace       string
		expectedDeletes sets.String
	}{
		{
			name:      "no overlapping backups",
			namespace: "ns-1",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupA").WithPhase(v1.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupB").WithPhase(v1.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupC").WithPhase(v1.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString("backupA", "backupB", "backupC"),
		},
		{
			name:      "some overlapping backups",
			namespace: "ns-1",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithPhase(v1.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").WithPhase(v1.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupC").WithPhase(v1.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString("backupC"),
		},
		{
			name:      "all overlapping backups",
			namespace: "ns-1",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithPhase(v1.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").WithPhase(v1.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").WithPhase(v1.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				backupLister    = &cloudprovidermocks.BackupLister{}
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = arktest.NewLogger()
			)

			c := NewBackupSyncController(
				client.ArkV1(),
				backupLister,
				"bucket",
				time.Duration(0),
				test.namespace,
				sharedInformers.Ark().V1().Backups(),
				logger,
			).(*backupSyncController)

			expectedDeleteActions := make([]core.Action, 0)

			// setup: insert backups into Kubernetes
			for _, backup := range test.k8sBackups {
				if test.expectedDeletes.Has(backup.Name) {
					actionDelete := core.NewDeleteAction(
						v1.SchemeGroupVersion.WithResource("backups"),
						test.namespace,
						backup.Name,
					)
					expectedDeleteActions = append(expectedDeleteActions, actionDelete)
				}

				// add test backup to informer:
				err := sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(backup.Backup)
				assert.NoError(t, err, "Error adding backup to informer")

				// add test backup to kubernetes:
				_, err = client.Ark().Backups(test.namespace).Create(backup.Backup)
				assert.NoError(t, err, "Error deleting from clientset")
			}

			// get names of client backups
			testBackupNames := sets.NewString()
			for _, cloudBackup := range test.cloudBackups {
				testBackupNames.Insert(cloudBackup.Name)
			}

			c.deleteUnused(testBackupNames)

			numBackups, err := numBackups(t, client, c.namespace)
			assert.NoError(t, err)

			expected := len(test.k8sBackups) - len(test.expectedDeletes)
			assert.Equal(t, expected, numBackups)

			arktest.CompareActions(t, expectedDeleteActions, getDeleteActions(client.Actions()))
		})
	}
}

func getDeleteActions(actions []core.Action) []core.Action {
	var deleteActions []core.Action
	for _, action := range actions {
		if action.GetVerb() == "delete" {
			deleteActions = append(deleteActions, action)
		}
	}
	return deleteActions
}

func numBackups(t *testing.T, c *fake.Clientset, ns string) (int, error) {
	t.Helper()
	existingK8SBackups, err := c.ArkV1().Backups(ns).List(metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(existingK8SBackups.Items), nil
}
