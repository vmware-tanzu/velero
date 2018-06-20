/*
Copyright 2018 the Heptio Ark contributors.

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

package install

import (
	"time"

	arkv1 "github.com/heptio/ark/pkg/apis/ark/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type arkConfigOption func(*arkConfig)

type arkConfig struct {
	backupSyncPeriod          time.Duration
	gcSyncPeriod              time.Duration
	podVolumeOperationTimeout time.Duration
	restoreOnly               bool
	resticLocation            string
}

func WithBackupSyncPeriod(t time.Duration) arkConfigOption {
	return func(c *arkConfig) {
		c.backupSyncPeriod = t
	}
}

func WithGCSyncPeriod(t time.Duration) arkConfigOption {
	return func(c *arkConfig) {
		c.gcSyncPeriod = t
	}
}

func WithPodVolumeOperationTimeout(t time.Duration) arkConfigOption {
	return func(c *arkConfig) {
		c.podVolumeOperationTimeout = t
	}
}

func WithRestoreOnly() arkConfigOption {
	return func(c *arkConfig) {
		c.restoreOnly = true
	}
}

func WithResticLocation(location string) arkConfigOption {
	return func(c *arkConfig) {
		c.resticLocation = location
	}
}

func Config(
	namespace string,
	pvCloudProviderName string,
	pvCloudProviderConfig map[string]string,
	backupCloudProviderName string,
	backupCloudProviderConfig map[string]string,
	bucket string,
	opts ...arkConfigOption,
) *arkv1.Config {
	c := &arkConfig{
		backupSyncPeriod:          30 * time.Minute,
		gcSyncPeriod:              30 * time.Minute,
		podVolumeOperationTimeout: 60 * time.Minute,
	}

	for _, opt := range opts {
		opt(c)
	}

	return &arkv1.Config{
		ObjectMeta: objectMeta(namespace, "default"),
		PersistentVolumeProvider: &arkv1.CloudProviderConfig{
			Name:   pvCloudProviderName,
			Config: pvCloudProviderConfig,
		},
		BackupStorageProvider: arkv1.ObjectStorageProviderConfig{
			CloudProviderConfig: arkv1.CloudProviderConfig{
				Name:   backupCloudProviderName,
				Config: backupCloudProviderConfig,
			},
			Bucket:         bucket,
			ResticLocation: c.resticLocation,
		},
		BackupSyncPeriod: metav1.Duration{
			Duration: c.backupSyncPeriod,
		},
		GCSyncPeriod: metav1.Duration{
			Duration: c.gcSyncPeriod,
		},
		ScheduleSyncPeriod: metav1.Duration{
			Duration: time.Minute,
		},
		PodVolumeOperationTimeout: metav1.Duration{
			Duration: c.podVolumeOperationTimeout,
		},
		RestoreOnlyMode: c.restoreOnly,
	}
}
