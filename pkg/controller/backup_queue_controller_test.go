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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	//"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestBackupQueueReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	velerov1api.AddToScheme(scheme)

	tests := []struct {
		name                string
		priorBackups        []*velerov1api.Backup
		namespaces          []string
		backup              *velerov1api.Backup
		concurrentBackups   int
		expectError         bool
		expectPhase         velerov1api.BackupPhase
		expectQueuePosition int
	}{
		{
			name:                "New Backup gets queued",
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name:        "InProgress Backup is ignored",
			backup:      builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).Result(),
			expectPhase: velerov1api.BackupPhaseInProgress,
		},
		{
			name: "Second New Backup gets queued with queuePosition 2",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).Result(),
			},
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-12").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 2,
		},
		{
			name:        "Queued Backup moves to ReadyToStart if no others are running",
			backup:      builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseQueued).Result(),
			expectPhase: velerov1api.BackupPhaseReadyToStart,
		},
		{
			name: "Queued Backup remains queued if no spaces available",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).Result(),
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-12").Phase(velerov1api.BackupPhaseInProgress).Result(),
			},
			concurrentBackups:   2,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name: "Queued Backup remains queued if no spaces available including ReadyToStart",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).Result(),
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-12").Phase(velerov1api.BackupPhaseReadyToStart).Result(),
			},
			concurrentBackups:   2,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name: "Queued Backup remains queued if earlier runnable backup is also queued",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).Result(),
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-12").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).Result(),
			},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(2).Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 2,
		},
		{
			name: "Queued Backup remains queued if in conflict with running backup",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).IncludedNamespaces("foo").Result(),
			},
			namespaces:          []string{"foo"},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("foo").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name: "Queued Backup remains queued if in conflict with ReadyToStart backup",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseReadyToStart).IncludedNamespaces("foo").Result(),
			},
			namespaces:          []string{"foo"},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("foo").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name: "Queued Backup remains queued if in conflict with earlier queued backup",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("foo").Result(),
			},
			namespaces:          []string{"foo", "bar"},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(2).IncludedNamespaces("foo", "bar").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 2,
		},
		{
			name: "Queued Backup remains queued if earlier non-ns-conflict backup exists",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).IncludedNamespaces("bar").Result(),
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-12").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("foo").Result(),
			},
			namespaces:          []string{"foo", "bar", "baz"},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(2).IncludedNamespaces("baz").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 2,
		},
		{
			name: "Running all-namespace backup conflicts with queued one-namespace backup ",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).IncludedNamespaces("*").Result(),
			},
			namespaces:          []string{"foo", "bar"},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("foo").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name: "Running one-namespace backup conflicts with queued all-namespace backup ",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).IncludedNamespaces("bar").Result(),
			},
			namespaces:          []string{"foo", "bar"},
			concurrentBackups:   3,
			backup:              builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("*").Result(),
			expectPhase:         velerov1api.BackupPhaseQueued,
			expectQueuePosition: 1,
		},
		{
			name: "Queued Backup moves to ReadyToStart if running count < concurrentBackups",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseInProgress).Result(),
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-12").Phase(velerov1api.BackupPhaseInProgress).Result(),
			},
			concurrentBackups: 3,
			backup:            builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).Result(),
			expectPhase:       velerov1api.BackupPhaseReadyToStart,
		},
		{
			name: "Queued Backup moves to ReadyToStart if running count < concurrentBackups and no ns conflict found",
			priorBackups: []*velerov1api.Backup{
				builder.ForBackup(velerov1api.DefaultNamespace, "backup-11").Phase(velerov1api.BackupPhaseReadyToStart).IncludedNamespaces("foo").Result(),
			},
			namespaces:        []string{"foo", "bar"},
			concurrentBackups: 3,
			backup:            builder.ForBackup(velerov1api.DefaultNamespace, "backup-20").Phase(velerov1api.BackupPhaseQueued).QueuePosition(1).IncludedNamespaces("bar").Result(),
			expectPhase:       velerov1api.BackupPhaseReadyToStart,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.backup == nil {
				return
			}

			initObjs := []runtime.Object{}
			for _, priorBackup := range test.priorBackups {
				initObjs = append(initObjs, priorBackup)
			}
			for _, ns := range test.namespaces {
				initObjs = append(initObjs, builder.ForNamespace(ns).Result())
			}
			initObjs = append(initObjs, test.backup)

			fakeClient := velerotest.NewFakeControllerRuntimeClient(t, initObjs...)
			logger := logrus.New()
			log := logger.WithField("controller", "backup-queue-test")
			r := NewBackupQueueReconciler(fakeClient, scheme, log, test.concurrentBackups)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.backup.Namespace, Name: test.backup.Name}}
			res, err := r.Reconcile(context.Background(), req)
			gotErr := err != nil
			require.NoError(t, err)
			assert.Equal(t, ctrl.Result{}, res)
			assert.Equal(t, test.expectError, gotErr)
			backupAfter := velerov1api.Backup{}
			err = fakeClient.Get(t.Context(), types.NamespacedName{
				Namespace: test.backup.Namespace,
				Name:      test.backup.Name,
			}, &backupAfter)

			require.NoError(t, err)
			assert.Equal(t, test.expectPhase, backupAfter.Status.Phase)
			assert.Equal(t, test.expectQueuePosition, backupAfter.Status.QueuePosition)
		})
	}

}
