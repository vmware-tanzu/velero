package backup

import "github.com/heptio/ark/pkg/apis/ark/v1"

type Writer interface {
	PrepareBackup(backup *v1.Backup) error
	WriteResource(groupResource, namespace, name string, resource []byte) error
	FinalizeBackup(backup *v1.Backup) error
}
