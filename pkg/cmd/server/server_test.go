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

package server

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

func TestApplyConfigDefaults(t *testing.T) {
	var (
		logger, _ = test.NewNullLogger()
		c         = &v1.Config{}
	)

	// test defaulting
	applyConfigDefaults(c, logger)
	assert.Equal(t, defaultGCSyncPeriod, c.GCSyncPeriod.Duration)
	assert.Equal(t, defaultBackupSyncPeriod, c.BackupSyncPeriod.Duration)
	assert.Equal(t, defaultScheduleSyncPeriod, c.ScheduleSyncPeriod.Duration)
	assert.Equal(t, defaultResourcePriorities, c.ResourcePriorities)

	// make sure defaulting doesn't overwrite real values
	c.GCSyncPeriod.Duration = 5 * time.Minute
	c.BackupSyncPeriod.Duration = 4 * time.Minute
	c.ScheduleSyncPeriod.Duration = 3 * time.Minute
	c.ResourcePriorities = []string{"a", "b"}

	applyConfigDefaults(c, logger)
	assert.Equal(t, 5*time.Minute, c.GCSyncPeriod.Duration)
	assert.Equal(t, 4*time.Minute, c.BackupSyncPeriod.Duration)
	assert.Equal(t, 3*time.Minute, c.ScheduleSyncPeriod.Duration)
	assert.Equal(t, []string{"a", "b"}, c.ResourcePriorities)
}
