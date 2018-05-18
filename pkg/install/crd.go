package install

import (
	"fmt"

	arkv1 "github.com/heptio/ark/pkg/apis/ark/v1"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CRDs returns a list of the CRD types for all of the required Ark CRDs
func CRDs() []*apiextv1beta1.CustomResourceDefinition {
	return []*apiextv1beta1.CustomResourceDefinition{
		crd("Backup", "backups"),
		crd("Schedule", "schedules"),
		crd("Restore", "restores"),
		crd("Config", "configs"),
		crd("DownloadRequest", "downloadrequests"),
		crd("DeleteBackupRequest", "deletebackuprequests"),
	}
}

func crd(kind, plural string) *apiextv1beta1.CustomResourceDefinition {
	return &apiextv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", plural, arkv1.GroupName),
		},
		Spec: apiextv1beta1.CustomResourceDefinitionSpec{
			Group:   arkv1.GroupName,
			Version: arkv1.SchemeGroupVersion.Version,
			Scope:   apiextv1beta1.NamespaceScoped,
			Names: apiextv1beta1.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   kind,
			},
		},
	}
}
