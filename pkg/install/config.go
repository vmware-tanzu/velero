package install

import (
	"time"

	arkv1 "github.com/heptio/ark/pkg/apis/ark/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type arkConfigOption func(*arkConfig)

type arkConfig struct {
	backupSyncPeriod time.Duration
	gcSyncPeriod     time.Duration
	restoreOnly      bool
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

func WithRestoreOnly() arkConfigOption {
	return func(c *arkConfig) {
		c.restoreOnly = true
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
		backupSyncPeriod: 30 * time.Minute,
		gcSyncPeriod:     30 * time.Minute,
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
			Bucket: bucket,
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
		RestoreOnlyMode: c.restoreOnly,
	}
}
