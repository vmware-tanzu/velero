package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

/*

A BackupStorageProvider is one per provider for storing backups (tarball + metadata).
This could be, e.g., "AWS-S3", "GCS", "GitHub", etc. Within each provider, there's a
default Location. Initially, we'll only allow a single provider to be configured for
an Ark instance. Over time, we may need to enable the concept of a "default" provider.

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
	// Name is a unique name for the location within the provider.
	Name string `json:"name"`

	// AccessMode is whether the storage location is read-write,
	// read-only, etc.
	AccessMode LocationAccessMode `json:"accessMode"`

	// ObjectStorage is the details of the storage location if
	// it's a location within an object store.
	ObjectStorage *ObjectStorageLocation `json:"objectStorage"`
}

// LocationAccessMode is a string representation of whether the location
// is read-write, read-only, etc.
type LocationAccessMode string

const (
	// LocationAccessModeReadWrite means the location can be read from and
	// written to, i.e. restored from and backed up to.
	LocationAccessModeReadWrite LocationAccessMode = "rw"

	// LocationAccessModeReadOnly means the location can only be read from,
	// i.e. restored from.
	LocationAccessModeReadOnly LocationAccessMode = "r"
)

// ObjectStorageLocation contains details of a backup storage location that
// is within object storage.
type ObjectStorageLocation struct {
	// Bucket is the name of the object storage bucket.
	Bucket string `json:"bucket"`

	// Prefix is the prefix that should be prepended to
	// all objects saved in this location. Optional.
	Prefix string `json:"prefix,omitempty"`

	// Config is a map of provider/location-specific
	// configuration information.
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
						Prefix: "ark-backups",
						Config: map[string]string{
							"region": "us-west-1",
						},
					},
				},
				{
					Name: "cluster-a-us-east-1",
					ObjectStorage: &ObjectStorageLocation{
						Bucket: "ark-cluster-a-us-east-1",
						Config: map[string]string{
							"region": "us-east-1",
						},
					},
					AccessMode: LocationAccessModeReadOnly,
				},
			},
			DefaultLocationName: "us-east-1",
		},
	}
}
