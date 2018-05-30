package backup

import (
	"github.com/heptio/ark/pkg/apis/ark/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Writer interface {
	PrepareBackup(backup *v1.Backup) error
	WriteResource(id ResourceIdentifier, obj runtime.Unstructured) error
	FinalizeBackup(backup *v1.Backup) error
}

type ResourceFormat string

const (
	ResourceFormatJSON ResourceFormat = "json"
	ResourceFormatYAML ResourceFormat = "yaml"
)
