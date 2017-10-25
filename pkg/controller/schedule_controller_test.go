/*
Copyright 2017 Heptio Inc.

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

	"github.com/robfig/cron"
	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/scheme"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	. "github.com/heptio/ark/pkg/util/test"
)

func TestProcessSchedule(t *testing.T) {
	tests := []struct {
		name                             string
		scheduleKey                      string
		schedule                         *api.Schedule
		fakeClockTime                    string
		expectedErr                      bool
		expectedSchedulePhaseUpdate      *api.Schedule
		expectedScheduleLastBackupUpdate *api.Schedule
		expectedBackupCreate             *api.Backup
	}{
		{
			name:        "invalid key returns error",
			scheduleKey: "invalid/key/value",
			expectedErr: true,
		},
		{
			name:        "missing schedule returns early without an error",
			scheduleKey: "foo/bar",
			expectedErr: false,
		},
		{
			name:        "schedule with phase FailedValidation does not get processed",
			schedule:    NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseFailedValidation).Schedule,
			expectedErr: false,
		},
		{
			name:        "schedule with phase New gets validated and failed if invalid",
			schedule:    NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseNew).Schedule,
			expectedErr: false,
			expectedSchedulePhaseUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseFailedValidation).
				WithValidationError("Schedule must be a non-empty valid Cron expression").Schedule,
		},
		{
			name:        "schedule with phase <blank> gets validated and failed if invalid",
			schedule:    NewTestSchedule("ns", "name").Schedule,
			expectedErr: false,
			expectedSchedulePhaseUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseFailedValidation).
				WithValidationError("Schedule must be a non-empty valid Cron expression").Schedule,
		},
		{
			name:        "schedule with phase Enabled gets re-validated and failed if invalid",
			schedule:    NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).Schedule,
			expectedErr: false,
			expectedSchedulePhaseUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseFailedValidation).
				WithValidationError("Schedule must be a non-empty valid Cron expression").Schedule,
		},
		{
			name:                        "schedule with phase New gets validated and triggers a backup",
			schedule:                    NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseNew).WithCronSchedule("@every 5m").Schedule,
			fakeClockTime:               "2017-01-01 12:00:00",
			expectedErr:                 false,
			expectedSchedulePhaseUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).WithCronSchedule("@every 5m").Schedule,
			expectedBackupCreate:        NewTestBackup().WithNamespace("ns").WithName("name-20170101120000").WithLabel("ark-schedule", "name").Backup,
			expectedScheduleLastBackupUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).
				WithCronSchedule("@every 5m").WithLastBackupTime("2017-01-01 12:00:00").Schedule,
		},
		{
			name:                 "schedule with phase Enabled gets re-validated and triggers a backup if valid",
			schedule:             NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).WithCronSchedule("@every 5m").Schedule,
			fakeClockTime:        "2017-01-01 12:00:00",
			expectedErr:          false,
			expectedBackupCreate: NewTestBackup().WithNamespace("ns").WithName("name-20170101120000").WithLabel("ark-schedule", "name").Backup,
			expectedScheduleLastBackupUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).
				WithCronSchedule("@every 5m").WithLastBackupTime("2017-01-01 12:00:00").Schedule,
		},
		{
			name: "schedule that's already run gets LastBackup updated",
			schedule: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).
				WithCronSchedule("@every 5m").WithLastBackupTime("2000-01-01 00:00:00").Schedule,
			fakeClockTime:        "2017-01-01 12:00:00",
			expectedErr:          false,
			expectedBackupCreate: NewTestBackup().WithNamespace("ns").WithName("name-20170101120000").WithLabel("ark-schedule", "name").Backup,
			expectedScheduleLastBackupUpdate: NewTestSchedule("ns", "name").WithPhase(api.SchedulePhaseEnabled).
				WithCronSchedule("@every 5m").WithLastBackupTime("2017-01-01 12:00:00").Schedule,
		},
	}

	// flag.Set("logtostderr", "true")
	// flag.Set("v", "4")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger, _       = testlogger.NewNullLogger()
			)

			c := NewScheduleController(
				client.ArkV1(),
				client.ArkV1(),
				sharedInformers.Ark().V1().Schedules(),
				time.Duration(0),
				logger,
			)

			var (
				testTime time.Time
				err      error
			)
			if test.fakeClockTime != "" {
				testTime, err = time.Parse("2006-01-02 15:04:05", test.fakeClockTime)
				require.NoError(t, err, "unable to parse test.fakeClockTime: %v", err)
			}
			c.clock = clock.NewFakeClock(testTime)

			if test.schedule != nil {
				sharedInformers.Ark().V1().Schedules().Informer().GetStore().Add(test.schedule)

				// this is necessary so the Update() call returns the appropriate object
				client.PrependReactor("update", "schedules", func(action core.Action) (bool, runtime.Object, error) {
					obj := action.(core.UpdateAction).GetObject()
					// need to deep copy so we can test the schedule state for each call to update
					copy, err := scheme.Scheme.DeepCopy(obj)
					if err != nil {
						return false, nil, err
					}
					ret := copy.(runtime.Object)
					return true, ret, nil
				})
			}

			key := test.scheduleKey
			if key == "" && test.schedule != nil {
				key, err = cache.MetaNamespaceKeyFunc(test.schedule)
				require.NoError(t, err, "error getting key from test.schedule: %v", err)
			}

			err = c.processSchedule(key)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			expectedActions := make([]core.Action, 0)

			if upd := test.expectedSchedulePhaseUpdate; upd != nil {
				action := core.NewUpdateAction(
					api.SchemeGroupVersion.WithResource("schedules"),
					upd.Namespace,
					upd)
				expectedActions = append(expectedActions, action)
			}

			if created := test.expectedBackupCreate; created != nil {
				action := core.NewCreateAction(
					api.SchemeGroupVersion.WithResource("backups"),
					created.Namespace,
					created)
				expectedActions = append(expectedActions, action)
			}

			if upd := test.expectedScheduleLastBackupUpdate; upd != nil {
				action := core.NewUpdateAction(
					api.SchemeGroupVersion.WithResource("schedules"),
					upd.Namespace,
					upd)
				expectedActions = append(expectedActions, action)
			}

			assert.Equal(t, expectedActions, client.Actions())
		})
	}
}

func TestGetNextRunTime(t *testing.T) {
	tests := []struct {
		name                      string
		schedule                  *api.Schedule
		lastRanOffset             string
		expectedDue               bool
		expectedNextRunTimeOffset string
	}{
		{
			name:                      "first run",
			schedule:                  &api.Schedule{Spec: api.ScheduleSpec{Schedule: "@every 5m"}},
			expectedDue:               true,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "just ran",
			schedule:                  &api.Schedule{Spec: api.ScheduleSpec{Schedule: "@every 5m"}},
			lastRanOffset:             "0s",
			expectedDue:               false,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "almost but not quite time to run",
			schedule:                  &api.Schedule{Spec: api.ScheduleSpec{Schedule: "@every 5m"}},
			lastRanOffset:             "4m59s",
			expectedDue:               false,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "time to run again",
			schedule:                  &api.Schedule{Spec: api.ScheduleSpec{Schedule: "@every 5m"}},
			lastRanOffset:             "5m",
			expectedDue:               true,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "several runs missed",
			schedule:                  &api.Schedule{Spec: api.ScheduleSpec{Schedule: "@every 5m"}},
			lastRanOffset:             "5h",
			expectedDue:               true,
			expectedNextRunTimeOffset: "5m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cronSchedule, err := cron.Parse(test.schedule.Spec.Schedule)
			require.NoError(t, err, "unable to parse test.schedule.Spec.Schedule: %v", err)

			testClock := clock.NewFakeClock(time.Now())

			if test.lastRanOffset != "" {
				offsetDuration, err := time.ParseDuration(test.lastRanOffset)
				require.NoError(t, err, "unable to parse test.lastRanOffset: %v", err)

				test.schedule.Status.LastBackup = metav1.Time{Time: testClock.Now().Add(-offsetDuration)}
			}

			nextRunTimeOffset, err := time.ParseDuration(test.expectedNextRunTimeOffset)
			if err != nil {
				panic(err)
			}
			expectedNextRunTime := test.schedule.Status.LastBackup.Add(nextRunTimeOffset)

			due, nextRunTime := getNextRunTime(test.schedule, cronSchedule, testClock.Now())

			assert.Equal(t, test.expectedDue, due)
			// ignore diffs of under a second. the cron library does some rounding.
			assert.WithinDuration(t, expectedNextRunTime, nextRunTime, time.Second)
		})
	}
}

func TestParseCronSchedule(t *testing.T) {
	// From https://github.com/heptio/ark/issues/30, where we originally were using cron.Parse(),
	// which treats the first field as seconds, and not minutes. We want to use cron.ParseStandard()
	// instead, which has the first field as minutes.

	now := time.Date(2017, 8, 10, 12, 27, 0, 0, time.UTC)

	// Start with a Schedule with:
	// - schedule: once a day at 9am
	// - last backup: 2017-08-10 12:27:00 (just happened)
	s := &api.Schedule{
		Spec: api.ScheduleSpec{
			Schedule: "0 9 * * *",
		},
		Status: api.ScheduleStatus{
			LastBackup: metav1.NewTime(now),
		},
	}

	logger, _ := testlogger.NewNullLogger()

	c, errs := parseCronSchedule(s, logger)
	require.Empty(t, errs)

	// make sure we're not due and next backup is tomorrow at 9am
	due, next := getNextRunTime(s, c, now)
	assert.False(t, due)
	assert.Equal(t, time.Date(2017, 8, 11, 9, 0, 0, 0, time.UTC), next)

	// advance the clock a couple of hours and make sure nothing has changed
	now = now.Add(2 * time.Hour)
	due, next = getNextRunTime(s, c, now)
	assert.False(t, due)
	assert.Equal(t, time.Date(2017, 8, 11, 9, 0, 0, 0, time.UTC), next)

	// advance clock to 1 minute after due time, make sure due=true
	now = time.Date(2017, 8, 11, 9, 1, 0, 0, time.UTC)
	due, next = getNextRunTime(s, c, now)
	assert.True(t, due)
	assert.Equal(t, time.Date(2017, 8, 11, 9, 0, 0, 0, time.UTC), next)

	// record backup time
	s.Status.LastBackup = metav1.NewTime(now)

	// advance clock 1 minute, make sure we're not due and next backup is tomorrow at 9am
	now = time.Date(2017, 8, 11, 9, 2, 0, 0, time.UTC)
	due, next = getNextRunTime(s, c, now)
	assert.False(t, due)
	assert.Equal(t, time.Date(2017, 8, 12, 9, 0, 0, 0, time.UTC), next)
}

func TestGetBackup(t *testing.T) {
	tests := []struct {
		name           string
		schedule       *api.Schedule
		testClockTime  string
		expectedBackup *api.Backup
	}{
		{
			name: "ensure name is formatted correctly (AM time)",
			schedule: &api.Schedule{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: api.ScheduleSpec{
					Template: api.BackupSpec{},
				},
			},
			testClockTime: "2017-07-25 09:15:00",
			expectedBackup: &api.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar-20170725091500",
				},
				Spec: api.BackupSpec{},
			},
		},
		{
			name: "ensure name is formatted correctly (PM time)",
			schedule: &api.Schedule{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: api.ScheduleSpec{
					Template: api.BackupSpec{},
				},
			},
			testClockTime: "2017-07-25 14:15:00",
			expectedBackup: &api.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar-20170725141500",
				},
				Spec: api.BackupSpec{},
			},
		},
		{
			name: "ensure schedule backup template is copied",
			schedule: &api.Schedule{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: api.ScheduleSpec{
					Template: api.BackupSpec{
						IncludedNamespaces: []string{"ns-1", "ns-2"},
						ExcludedNamespaces: []string{"ns-3"},
						IncludedResources:  []string{"foo", "bar"},
						ExcludedResources:  []string{"baz"},
						LabelSelector:      &metav1.LabelSelector{MatchLabels: map[string]string{"label": "value"}},
						TTL:                metav1.Duration{Duration: time.Duration(300)},
					},
				},
			},
			testClockTime: "2017-07-25 09:15:00",
			expectedBackup: &api.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar-20170725091500",
				},
				Spec: api.BackupSpec{
					IncludedNamespaces: []string{"ns-1", "ns-2"},
					ExcludedNamespaces: []string{"ns-3"},
					IncludedResources:  []string{"foo", "bar"},
					ExcludedResources:  []string{"baz"},
					LabelSelector:      &metav1.LabelSelector{MatchLabels: map[string]string{"label": "value"}},
					TTL:                metav1.Duration{Duration: time.Duration(300)},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testTime, err := time.Parse("2006-01-02 15:04:05", test.testClockTime)
			require.NoError(t, err, "unable to parse test.testClockTime: %v", err)

			backup := getBackup(test.schedule, clock.NewFakeClock(testTime).Now())

			assert.Equal(t, test.expectedBackup.Namespace, backup.Namespace)
			assert.Equal(t, test.expectedBackup.Name, backup.Name)
			assert.Equal(t, test.expectedBackup.Spec, backup.Spec)
		})
	}
}
