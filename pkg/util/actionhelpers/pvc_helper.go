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

package actionhelpers

import (
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

func RelatedItemsForPVC(pvc *corev1api.PersistentVolumeClaim, _ logrus.FieldLogger) []velero.ResourceIdentifier {
	return []velero.ResourceIdentifier{
		{
			GroupResource: kuberesource.PersistentVolumes,
			Name:          pvc.Spec.VolumeName,
		},
	}
}
