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
	// Targets is a list of the backup storage targets
	// for this provider
	Targets []BackupStorageTarget `json:"targets"`

	// DefaultTargetName is the name of the default backup
	// storage target for this provider
	DefaultTargetName string `json:"defaultTargetName"`
}

type BackupStorageTarget struct {
	Name string `json:"name"`

	ObjectStorage *ObjectStorageTarget `json:"objectStorage"`
}

type ObjectStorageTarget struct {
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
			Targets: []BackupStorageTarget{
				{
					Name: "us-east-1",
					ObjectStorage: &ObjectStorageTarget{
						Bucket: "ark-us-east-1",
						Config: map[string]string{
							"region": "us-east-1",
						},
					},
				},
				{
					Name: "us-west-1",
					ObjectStorage: &ObjectStorageTarget{
						Bucket: "ark-us-west-1",
						Config: map[string]string{
							"region": "us-west-1",
						},
					},
				},
			},
			DefaultTargetName: "us-east-1",
		},
	}
}
