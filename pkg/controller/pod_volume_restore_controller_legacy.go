package controller

import (
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/uploader"
)

func isLegacyPVR(pvr *velerov1api.PodVolumeRestore) bool {
	return pvr.Spec.UploaderType == uploader.ResticType
}
