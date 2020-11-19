/*
Copyright 2017 the Velero contributors.

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
	"encoding/json"
	"testing"
	"time"

	"github.com/robfig/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestProcessSchedule(t *testing.T) {
	newScheduleBuilder := func(phase velerov1api.SchedulePhase) *builder.ScheduleBuilder {
		return builder.ForSchedule("ns", "name").Phase(phase)
	}

	tests := []struct {
		name                     string
		scheduleKey              string
		schedule                 *velerov1api.Schedule
		fakeClockTime            string
		expectedErr              bool
		expectedPhase            string
		expectedValidationErrors []string
		expectedBackupCreate     *velerov1api.Backup
		expectedLastBackup       string
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
			schedule:    newScheduleBuilder(velerov1api.SchedulePhaseFailedValidation).Result(),
			expectedErr: false,
		},
		{
			name:                     "schedule with phase New gets validated and failed if invalid",
			schedule:                 newScheduleBuilder(velerov1api.SchedulePhaseNew).Result(),
			expectedErr:              false,
			expectedPhase:            string(velerov1api.SchedulePhaseFailedValidation),
			expectedValidationErrors: []string{"Schedule must be a non-empty valid Cron expression"},
		},
		{
			name:                     "schedule with phase <blank> gets validated and failed if invalid",
			schedule:                 newScheduleBuilder(velerov1api.SchedulePhase("")).Result(),
			expectedErr:              false,
			expectedPhase:            string(velerov1api.SchedulePhaseFailedValidation),
			expectedValidationErrors: []string{"Schedule must be a non-empty valid Cron expression"},
		},
		{
			name:                     "schedule with phase Enabled gets re-validated and failed if invalid",
			schedule:                 newScheduleBuilder(velerov1api.SchedulePhaseEnabled).Result(),
			expectedErr:              false,
			expectedPhase:            string(velerov1api.SchedulePhaseFailedValidation),
			expectedValidationErrors: []string{"Schedule must be a non-empty valid Cron expression"},
		},
		{
			name:                 "schedule with phase New gets validated and triggers a backup",
			schedule:             newScheduleBuilder(velerov1api.SchedulePhaseNew).CronSchedule("@every 5m").Result(),
			fakeClockTime:        "2017-01-01 12:00:00",
			expectedErr:          false,
			expectedPhase:        string(velerov1api.SchedulePhaseEnabled),
			expectedBackupCreate: builder.ForBackup("ns", "name-20170101120000").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "name")).Result(),
			expectedLastBackup:   "2017-01-01 12:00:00",
		},
		{
			name:                 "schedule with phase Enabled gets re-validated and triggers a backup if valid",
			schedule:             newScheduleBuilder(velerov1api.SchedulePhaseEnabled).CronSchedule("@every 5m").Result(),
			fakeClockTime:        "2017-01-01 12:00:00",
			expectedErr:          false,
			expectedBackupCreate: builder.ForBackup("ns", "name-20170101120000").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "name")).Result(),
			expectedLastBackup:   "2017-01-01 12:00:00",
		},
		{
			name:                 "schedule that's already run gets LastBackup updated",
			schedule:             newScheduleBuilder(velerov1api.SchedulePhaseEnabled).CronSchedule("@every 5m").LastBackupTime("2000-01-01 00:00:00").Result(),
			fakeClockTime:        "2017-01-01 12:00:00",
			expectedErr:          false,
			expectedBackupCreate: builder.ForBackup("ns", "name-20170101120000").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "name")).Result(),
			expectedLastBackup:   "2017-01-01 12:00:00",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = velerotest.NewLogger()
			)

			c := NewScheduleController(
				"namespace",
				client.VeleroV1(),
				client.VeleroV1(),
				sharedInformers.Velero().V1().Schedules(),
				logger,
				metrics.NewServerMetrics(),
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
				sharedInformers.Velero().V1().Schedules().Informer().GetStore().Add(test.schedule)

				// this is necessary so the Patch() call returns the appropriate object
				client.PrependReactor("patch", "schedules", func(action core.Action) (bool, runtime.Object, error) {
					var (
						patch    = action.(core.PatchAction).GetPatch()
						patchMap = make(map[string]interface{})
						res      = test.schedule.DeepCopy()
					)

					if err := json.Unmarshal(patch, &patchMap); err != nil {
						t.Logf("error unmarshalling patch: %s\n", err)
						return false, nil, err
					}

					// these are the fields that may be updated by the controller
					phase, found, err := unstructured.NestedString(patchMap, "status", "phase")
					if err == nil && found {
						res.Status.Phase = velerov1api.SchedulePhase(phase)
					}

					lastBackupStr, found, err := unstructured.NestedString(patchMap, "status", "lastBackup")
					if err == nil && found {
						parsed, err := time.Parse(time.RFC3339, lastBackupStr)
						if err != nil {
							t.Logf("error parsing status.lastBackup: %s\n", err)
							return false, nil, err
						}
						res.Status.LastBackup = &metav1.Time{Time: parsed}
					}

					return true, res, nil
				})
			}

			key := test.scheduleKey
			if key == "" && test.schedule != nil {
				key, err = cache.MetaNamespaceKeyFunc(test.schedule)
				require.NoError(t, err, "error getting key from test.schedule: %v", err)
			}

			err = c.processSchedule(key)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			actions := client.Actions()
			index := 0

			type PatchStatus struct {
				ValidationErrors []string                  `json:"validationErrors"`
				Phase            velerov1api.SchedulePhase `json:"phase"`
				LastBackup       time.Time                 `json:"lastBackup"`
			}

			type Patch struct {
				Status PatchStatus `json:"status"`
			}

			decode := func(decoder *json.Decoder) (interface{}, error) {
				actual := new(Patch)
				err := decoder.Decode(actual)

				return *actual, err
			}

			if test.expectedPhase != "" {
				require.True(t, len(actions) > index, "len(actions) is too small")

				expected := Patch{
					Status: PatchStatus{
						ValidationErrors: test.expectedValidationErrors,
						Phase:            velerov1api.SchedulePhase(test.expectedPhase),
					},
				}

				velerotest.ValidatePatch(t, actions[index], expected, decode)

				index++
			}

			if created := test.expectedBackupCreate; created != nil {
				require.True(t, len(actions) > index, "len(actions) is too small")

				action := core.NewCreateAction(
					velerov1api.SchemeGroupVersion.WithResource("backups"),
					created.Namespace,
					created)

				assert.Equal(t, action, actions[index])

				index++
			}

			if test.expectedLastBackup != "" {
				require.True(t, len(actions) > index, "len(actions) is too small")

				expected := Patch{
					Status: PatchStatus{
						LastBackup: parseTime(test.expectedLastBackup),
					},
				}

				velerotest.ValidatePatch(t, actions[index], expected, decode)
			}
		})
	}
}

func parseTime(timeString string) time.Time {
	res, _ := time.Parse("2006-01-02 15:04:05", timeString)
	return res
}

func TestGetNextRunTime(t *testing.T) {
	defaultSchedule := func() *velerov1api.Schedule {
		return builder.ForSchedule("velero", "schedule-1").CronSchedule("@every 5m").Result()
	}

	tests := []struct {
		name                      string
		schedule                  *velerov1api.Schedule
		lastRanOffset             string
		expectedDue               bool
		expectedNextRunTimeOffset string
	}{
		{
			name:                      "first run",
			schedule:                  defaultSchedule(),
			expectedDue:               true,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "just ran",
			schedule:                  defaultSchedule(),
			lastRanOffset:             "0s",
			expectedDue:               false,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "almost but not quite time to run",
			schedule:                  defaultSchedule(),
			lastRanOffset:             "4m59s",
			expectedDue:               false,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "time to run again",
			schedule:                  defaultSchedule(),
			lastRanOffset:             "5m",
			expectedDue:               true,
			expectedNextRunTimeOffset: "5m",
		},
		{
			name:                      "several runs missed",
			schedule:                  defaultSchedule(),
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

				test.schedule.Status.LastBackup = &metav1.Time{Time: testClock.Now().Add(-offsetDuration)}
			}

			nextRunTimeOffset, err := time.ParseDuration(test.expectedNextRunTimeOffset)
			if err != nil {
				panic(err)
			}

			// calculate expected next run time (if the schedule hasn't run yet, this
			// will be the zero value which will trigger an immediate backup)
			var baseTime time.Time
			if test.lastRanOffset != "" {
				baseTime = test.schedule.Status.LastBackup.Time
			}
			expectedNextRunTime := baseTime.Add(nextRunTimeOffset)

			due, nextRunTime := getNextRunTime(test.schedule, cronSchedule, testClock.Now())

			assert.Equal(t, test.expectedDue, due)
			// ignore diffs of under a second. the cron library does some rounding.
			assert.WithinDuration(t, expectedNextRunTime, nextRunTime, time.Second)
		})
	}
}

func TestParseCronSchedule(t *testing.T) {
	// From https://github.com/vmware-tanzu/velero/issues/30, where we originally were using cron.Parse(),
	// which treats the first field as seconds, and not minutes. We want to use cron.ParseStandard()
	// instead, which has the first field as minutes.

	now := time.Date(2017, 8, 10, 12, 27, 0, 0, time.UTC)

	// Start with a Schedule with:
	// - schedule: once a day at 9am
	// - last backup: 2017-08-10 12:27:00 (just happened)
	s := builder.ForSchedule("velero", "schedule-1").CronSchedule("0 9 * * *").LastBackupTime(now.Format("2006-01-02 15:04:05")).Result()

	logger := velerotest.NewLogger()

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
	s.Status.LastBackup = &metav1.Time{Time: now}

	// advance clock 1 minute, make sure we're not due and next backup is tomorrow at 9am
	now = time.Date(2017, 8, 11, 9, 2, 0, 0, time.UTC)
	due, next = getNextRunTime(s, c, now)
	assert.False(t, due)
	assert.Equal(t, time.Date(2017, 8, 12, 9, 0, 0, 0, time.UTC), next)
}

func TestGetBackup(t *testing.T) {
	tests := []struct {
		name           string
		schedule       *velerov1api.Schedule
		testClockTime  string
		expectedBackup *velerov1api.Backup
	}{
		{
			name:           "ensure name is formatted correctly (AM time)",
			schedule:       builder.ForSchedule("foo", "bar").Result(),
			testClockTime:  "2017-07-25 09:15:00",
			expectedBackup: builder.ForBackup("foo", "bar-20170725091500").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "bar")).Result(),
		},
		{
			name:           "ensure name is formatted correctly (PM time)",
			schedule:       builder.ForSchedule("foo", "bar").Result(),
			testClockTime:  "2017-07-25 14:15:00",
			expectedBackup: builder.ForBackup("foo", "bar-20170725141500").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "bar")).Result(),
		},
		{
			name: "ensure schedule backup template is copied",
			schedule: builder.ForSchedule("foo", "bar").
				Template(builder.ForBackup("", "").
					IncludedNamespaces("ns-1", "ns-2").
					ExcludedNamespaces("ns-3").
					IncludedResources("foo", "bar").
					ExcludedResources("baz").
					LabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"label": "value"}}).
					TTL(time.Duration(300)).
					Result().
					Spec).
				Result(),
			testClockTime: "2017-07-25 09:15:00",
			expectedBackup: builder.ForBackup("foo", "bar-20170725091500").
				ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "bar")).
				IncludedNamespaces("ns-1", "ns-2").
				ExcludedNamespaces("ns-3").
				IncludedResources("foo", "bar").
				ExcludedResources("baz").
				LabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"label": "value"}}).
				TTL(time.Duration(300)).
				Result(),
		},
		{
			name:           "ensure schedule labels are copied",
			schedule:       builder.ForSchedule("foo", "bar").ObjectMeta(builder.WithLabels("foo", "bar", "bar", "baz")).Result(),
			testClockTime:  "2017-07-25 14:15:00",
			expectedBackup: builder.ForBackup("foo", "bar-20170725141500").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "bar", "bar", "baz", "foo", "bar")).Result(),
		},
		{
			name:           "ensure schedule annotations are copied",
			schedule:       builder.ForSchedule("foo", "bar").ObjectMeta(builder.WithAnnotations("foo", "bar", "bar", "baz")).Result(),
			testClockTime:  "2017-07-25 14:15:00",
			expectedBackup: builder.ForBackup("foo", "bar-20170725141500").ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, "bar"), builder.WithAnnotations("bar", "baz", "foo", "bar")).Result(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testTime, err := time.Parse("2006-01-02 15:04:05", test.testClockTime)
			require.NoError(t, err, "unable to parse test.testClockTime: %v", err)

			backup := getBackup(test.schedule, clock.NewFakeClock(testTime).Now())

			assert.Equal(t, test.expectedBackup.Namespace, backup.Namespace)
			assert.Equal(t, test.expectedBackup.Name, backup.Name)
			assert.Equal(t, test.expectedBackup.Labels, backup.Labels)
			assert.Equal(t, test.expectedBackup.Annotations, backup.Annotations)
			assert.Equal(t, test.expectedBackup.Spec, backup.Spec)
		})
	}
}
