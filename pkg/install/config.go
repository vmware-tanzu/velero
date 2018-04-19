package install

import (
	"time"

	arkv1 "github.com/heptio/ark/pkg/apis/ark/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Config(
	namespace string,
	pvCloudProviderName string,
	pvCloudProviderConfig map[string]string,
	backupCloudProviderName string,
	backupCloudProviderConfig map[string]string,
	bucket string,
) *arkv1.Config {
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
			Duration: 30 * time.Minute,
		},
		GCSyncPeriod: metav1.Duration{
			Duration: 30 * time.Minute,
		},
		ScheduleSyncPeriod: metav1.Duration{
			Duration: time.Minute,
		},
		RestoreOnlyMode: false,
	}
}
