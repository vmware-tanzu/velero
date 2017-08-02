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

package test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

type TestSchedule struct {
	*api.Schedule
}

func NewTestSchedule(namespace, name string) *TestSchedule {
	return &TestSchedule{
		Schedule: &api.Schedule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (s *TestSchedule) WithPhase(phase api.SchedulePhase) *TestSchedule {
	s.Status.Phase = phase
	return s
}

func (s *TestSchedule) WithValidationError(msg string) *TestSchedule {
	s.Status.ValidationErrors = append(s.Status.ValidationErrors, msg)
	return s
}

func (s *TestSchedule) WithCronSchedule(cronExpression string) *TestSchedule {
	s.Spec.Schedule = cronExpression
	return s
}

func (s *TestSchedule) WithLastBackupTime(timeString string) *TestSchedule {
	t, _ := time.Parse("2006-01-02 15:04:05", timeString)
	s.Status.LastBackup = metav1.Time{Time: t}
	return s
}
