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

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ConfigList is a list of Configs.
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Config `json:"items"`
}

// +genclient=true

// Config is an Ark resource that captures configuration information to be
// used for running the Ark server.
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// PersistentVolumeProvider is the configuration information for the cloud where
	// the cluster is running and has PersistentVolumes to snapshot or restore. Optional.
	PersistentVolumeProvider *CloudProviderConfig `json:"persistentVolumeProvider"`

	// BackupStorageProvider is the configuration information for the cloud where
	// Ark backups are stored in object storage. This may be a different cloud than
	// where the cluster is running.
	BackupStorageProvider ObjectStorageProviderConfig `json:"backupStorageProvider"`

	// BackupSyncPeriod is how often the BackupSyncController runs to ensure all
	// Ark backups in object storage exist as Backup API objects in the cluster.
	BackupSyncPeriod metav1.Duration `json:"backupSyncPeriod"`

	// GCSyncPeriod is how often the GCController runs to delete expired backup
	// API objects and corresponding backup files in object storage.
	GCSyncPeriod metav1.Duration `json:"gcSyncPeriod"`

	// ScheduleSyncPeriod is how often the ScheduleController runs to check for
	// new backups that should be triggered based on schedules.
	ScheduleSyncPeriod metav1.Duration `json:"scheduleSyncPeriod"`

	// ResourcePriorities is an ordered slice of resources specifying the desired
	// order of resource restores. Any resources not in the list will be restored
	// alphabetically after the prioritized resources.
	ResourcePriorities []string `json:"resourcePriorities"`

	// RestoreOnlyMode is whether Ark should run in a mode where only restores
	// are allowed; backups, schedules, and garbage-collection are all disabled.
	RestoreOnlyMode bool `json:"restoreOnlyMode"`
}

// CloudProviderConfig is configuration information about how to connect
// to a particular cloud. Only one of the members (AWS, GCP, Azure) may
// be present.
type CloudProviderConfig struct {
	// AWS is configuration information for connecting to AWS.
	AWS *AWSConfig `json:"aws"`

	// GCP is configuration information for connecting to GCP.
	GCP *GCPConfig `json:"gcp"`

	// Azure is configuration information for connecting to Azure.
	Azure *AzureConfig `json:"azure"`
}

// ObjectStorageProviderConfig is configuration information for connecting to
// a particular bucket in object storage to access Ark backups.
type ObjectStorageProviderConfig struct {
	// CloudProviderConfig is the configuration information for the cloud where
	// Ark backups are stored in object storage.
	CloudProviderConfig `json:",inline"`

	// Bucket is the name of the bucket in object storage where Ark backups
	// are stored.
	Bucket string `json:"bucket"`
}

// AWSConfig is configuration information for connecting to AWS.
type AWSConfig struct {
	Region           string `json:"region"`
	DisableSSL       bool   `json:"disableSSL"`
	S3ForcePathStyle bool   `json:"s3ForcePathStyle"`
	S3Url            string `json:"s3Url"`
	KMSKeyID         string `json:"kmsKeyId"`
}

// GCPConfig is configuration information for connecting to GCP.
type GCPConfig struct {
	Project string `json:"project"`
}

// AzureConfig is configuration information for connecting to Azure.
type AzureConfig struct {
	Location   string          `json:"location"`
	APITimeout metav1.Duration `json:"apiTimeout"`
}
