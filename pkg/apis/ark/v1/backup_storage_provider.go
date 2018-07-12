package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

/*

- A BackupStorageProvider is one per provider for storing backups (tarball + metadata).
  This could be, e.g., "AWS-S3", "GCS", "GitHub", etc. Initially, we'll only allow a
  single provider to be configured for an Ark instance. Over time, we may need to enable
  the concept of a "default" provider. Within each provider, there's a default *target*.

*/

type BackupStorageProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   BackupStorageProviderSpec   `json:"spec"`
	Status BackupStorageProviderStatus `json:"status,omitempty"`
}

type BackupStorageProviderSpec struct {
	// Locations is a list of the backup storage locations
	// for this provider
	Locations []BackupStorageLocation `json:"locations"`

	// DefaultLocationName is the name of the default backup
	// storage location for this provider
	DefaultLocationName string `json:"defaultLocationName"`
}

type BackupStorageLocation struct {
	Name string `json:"name"`

	ObjectStorage *ObjectStorageLocation `json:"objectStorage"`
}

type ObjectStorageLocation struct {
	Bucket string            `json:"bucket"`
	Prefix string            `json:"prefix,omitempty"`
	Config map[string]string `json:"config,omitempty"`
}

type BackupStorageProviderStatus struct {
}

// NewBackupStorageProvider is an example of what constructing a
// BackupStorageProvider looks like. To be removed.
func NewBackupStorageProvider() *BackupStorageProvider {
	return &BackupStorageProvider{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: DefaultNamespace,
			Name:      "aws",
		},
		Spec: BackupStorageProviderSpec{
			Locations: []BackupStorageLocation{
				{
					Name: "us-east-1",
					ObjectStorage: &ObjectStorageLocation{
						Bucket: "ark-us-east-1",
						Config: map[string]string{
							"region": "us-east-1",
						},
					},
				},
				{
					Name: "us-west-1",
					ObjectStorage: &ObjectStorageLocation{
						Bucket: "ark-us-west-1",
						Config: map[string]string{
							"region": "us-west-1",
						},
					},
				},
			},
			DefaultLocationName: "us-east-1",
		},
	}
}
