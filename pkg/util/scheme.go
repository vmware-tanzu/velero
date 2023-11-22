package util

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
)

var VeleroScheme = runtime.NewScheme()

func init() {
	localSchemeBuilder := runtime.SchemeBuilder{
		v1.AddToScheme,
		v2alpha1.AddToScheme,
	}
	utilruntime.Must(localSchemeBuilder.AddToScheme(VeleroScheme))
}
