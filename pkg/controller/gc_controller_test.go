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
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/util/kube"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestGCControllerEnqueueAllBackups(t *testing.T) {
	var (
		client          = fake.NewSimpleClientset()
		sharedInformers = informers.NewSharedInformerFactory(client, 0)

		controller = NewGCController(
			arktest.NewLogger(),
			sharedInformers.Ark().V1().Backups(),
			client.ArkV1(),
			1*time.Millisecond,
		).(*gcController)
	)

	// Have to clear this out so the controller doesn't wait
	controller.cacheSyncWaiters = nil

	keys := make(chan string)

	controller.syncHandler = func(key string) error {
		keys <- key
		return nil
	}

	var expected []string

	for i := 0; i < 3; i++ {
		backup := arktest.NewTestBackup().WithName(fmt.Sprintf("backup-%d", i)).Backup
		sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(backup)
		expected = append(expected, kube.NamespaceAndName(backup))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go controller.Run(ctx, 1)

	var received []string

Loop:
	for {
		select {
		case <-ctx.Done():
			t.Fatal("test timed out")
		case key := <-keys:
			received = append(received, key)
			if len(received) == len(expected) {
				break Loop
			}
		}
	}

	sort.Strings(expected)
	sort.Strings(received)
	assert.Equal(t, expected, received)
}

func TestGCControllerHasUpdateFunc(t *testing.T) {
	backup := arktest.NewTestBackup().WithName("backup").Backup
	expected := kube.NamespaceAndName(backup)

	client := fake.NewSimpleClientset(backup)

	fakeWatch := watch.NewFake()
	defer fakeWatch.Stop()
	client.PrependWatchReactor("backups", core.DefaultWatchReactor(fakeWatch, nil))

	sharedInformers := informers.NewSharedInformerFactory(client, 0)

	controller := NewGCController(
		arktest.NewLogger(),
		sharedInformers.Ark().V1().Backups(),
		client.ArkV1(),
		1*time.Millisecond,
	).(*gcController)

	keys := make(chan string)

	controller.syncHandler = func(key string) error {
		keys <- key
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go sharedInformers.Start(ctx.Done())
	go controller.Run(ctx, 1)

	// wait for the AddFunc
	select {
	case <-ctx.Done():
		t.Fatal("test timed out waiting for AddFunc")
	case key := <-keys:
		assert.Equal(t, expected, key)
	}

	backup.Status.Version = 1234
	fakeWatch.Add(backup)

	// wait for the UpdateFunc
	select {
	case <-ctx.Done():
		t.Fatal("test timed out waiting for UpdateFunc")
	case key := <-keys:
		assert.Equal(t, expected, key)
	}
}

func TestGCControllerProcessQueueItem(t *testing.T) {
	fakeClock := clock.NewFakeClock(time.Now())

	tests := []struct {
		name                           string
		backup                         *api.Backup
		expectDeletion                 bool
		createDeleteBackupRequestError bool
		expectError                    bool
	}{
		{
			name: "can't find backup - no error",
		},
		{
			name: "expired backup is deleted",
			backup: arktest.NewTestBackup().WithName("backup-1").
				WithExpiration(fakeClock.Now().Add(-1 * time.Second)).
				Backup,
			expectDeletion: true,
		},
		{
			name: "unexpired backup is not deleted",
			backup: arktest.NewTestBackup().WithName("backup-1").
				WithExpiration(fakeClock.Now().Add(1 * time.Minute)).
				Backup,
			expectDeletion: false,
		},
		{
			name: "create DeleteBackupRequest error returns an error",
			backup: arktest.NewTestBackup().WithName("backup-1").
				WithExpiration(fakeClock.Now().Add(-1 * time.Second)).
				Backup,
			expectDeletion:                 true,
			createDeleteBackupRequestError: true,
			expectError:                    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			controller := NewGCController(
				arktest.NewLogger(),
				sharedInformers.Ark().V1().Backups(),
				client.ArkV1(),
				1*time.Millisecond,
			).(*gcController)
			controller.clock = fakeClock

			var key string
			if test.backup != nil {
				key = kube.NamespaceAndName(test.backup)
				sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup)
			}

			if test.createDeleteBackupRequestError {
				client.PrependReactor("create", "deletebackuprequests", func(action core.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("foo")
				})
			}

			err := controller.processQueueItem(key)
			gotErr := err != nil
			assert.Equal(t, test.expectError, gotErr)

			if test.expectDeletion {
				assert.Len(t, client.Actions(), 1)
			} else {
				assert.Len(t, client.Actions(), 0)
			}
		})
	}
}
