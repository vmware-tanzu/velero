/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package volumehelper

import (
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	"github.com/vmware-tanzu/velero/internal/volumehelper"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

// ShouldPerformSnapshotWithBackup is used for third-party plugins.
// It supports to check whether the PVC or PodVolume should be backed
// up on demand. On the other hand, the volumeHelperImpl assume there
// is a VolumeHelper instance initialized before calling the
// ShouldPerformXXX functions.
func ShouldPerformSnapshotWithBackup(
	unstructured runtime.Unstructured,
	groupResource schema.GroupResource,
	backup velerov1api.Backup,
	crClient crclient.Client,
	logger logrus.FieldLogger,
) (bool, error) {
	resourcePolicies, err := resourcepolicies.GetResourcePoliciesFromBackup(
		backup,
		crClient,
		logger,
	)
	if err != nil {
		return false, err
	}

	volumeHelperImpl := volumehelper.NewVolumeHelperImpl(
		resourcePolicies,
		backup.Spec.SnapshotVolumes,
		logger,
		crClient,
		boolptr.IsSetToTrue(backup.Spec.DefaultVolumesToFsBackup),
		true,
	)

	return volumeHelperImpl.ShouldPerformSnapshot(unstructured, groupResource)
}
