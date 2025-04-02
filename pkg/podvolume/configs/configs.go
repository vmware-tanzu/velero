package configs

const (
	// PVCNameAnnotation is the key for the annotation added to
	// pod volume backups when they're for a PVC.
	PVCNameAnnotation = "velero.io/pvc-name"

	// DefaultVolumesToFsBackup specifies whether pod volume backup should be used, by default, to
	// take backup of all pod volumes.
	DefaultVolumesToFsBackup = false
)
