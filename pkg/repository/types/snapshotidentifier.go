package types

// SnapshotIdentifier uniquely identifies a snapshot
// taken by Velero.
type SnapshotIdentifier struct {
	// VolumeNamespace is the namespace of the pod/volume that
	// the snapshot is for.
	VolumeNamespace string `json:"volumeNamespace"`

	// BackupStorageLocation is the backup's storage location
	// name.
	BackupStorageLocation string `json:"backupStorageLocation"`

	// SnapshotID is the short ID of the snapshot.
	SnapshotID string `json:"snapshotID"`

	// RepositoryType is the type of the repository where the
	// snapshot is stored
	RepositoryType string `json:"repositoryType"`

	// Source is the source of the data saved in the repo by the snapshot
	Source string `json:"source"`

	// UploaderType is the type of uploader which saved the snapshot data
	UploaderType string `json:"uploaderType"`

	// RepoIdentifier is the identifier of the repository where the
	// snapshot is stored
	RepoIdentifier string `json:"repoIdentifier"`
}
