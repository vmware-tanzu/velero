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

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigList is a list of Configs.
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Config `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
}

// CloudProviderConfig is configuration information about how to connect
// to a particular cloud.
type CloudProviderConfig struct {
	Name string `json:"name"`

	Config map[string]string `json:"config"`
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

	// ResticLocation is the bucket and optional prefix in object storage where
	// Ark stores restic backups of pod volumes, specified either as "bucket" or
	// "bucket/prefix". This bucket must be different than the `Bucket` field.
	// Optional.
	ResticLocation string `json:"resticLocation"`
}
