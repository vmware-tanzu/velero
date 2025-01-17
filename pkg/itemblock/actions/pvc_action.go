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

package actions

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	plugincommon "github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/actionhelpers"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

// PVCAction inspects a PersistentVolumeClaim for the PersistentVolume
// that it references and backs it up
type PVCAction struct {
	log      logrus.FieldLogger
	crClient crclient.Client
}

func NewPVCAction(f client.Factory) plugincommon.HandlerInitializer {
	return func(logger logrus.FieldLogger) (any, error) {
		crClient, err := f.KubebuilderClient()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return &PVCAction{
			log:      logger,
			crClient: crClient,
		}, nil
	}
}

func (a *PVCAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

func (a *PVCAction) GetRelatedItems(item runtime.Unstructured, _ *v1.Backup) ([]velero.ResourceIdentifier, error) {
	a.log.Info("Executing PVC ItemBlockAction")
	defer a.log.Info("Done executing PVC ItemBlockAction")

	pvc := new(corev1api.PersistentVolumeClaim)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &pvc); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to persistent volume claim")
	}

	if pvc.Status.Phase != corev1api.ClaimBound || pvc.Spec.VolumeName == "" {
		return nil, nil
	}
	// returns the PV for the PVC (shared with BIA additionalItems)
	relatedItems := actionhelpers.RelatedItemsForPVC(pvc, a.log)

	// Adds pods mounting this PVC to ensure that multiple pods mounting the same RWX
	// volume get backed up together.
	pods := new(corev1api.PodList)
	err := a.crClient.List(context.Background(), pods, crclient.InNamespace(pvc.Namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	for i := range pods.Items {
		for _, volume := range pods.Items[i].Spec.Volumes {
			if volume.VolumeSource.PersistentVolumeClaim == nil {
				continue
			}
			if volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				if kube.IsPodRunning(&pods.Items[i]) != nil {
					a.log.Infof("Related pod %s is not running, not adding to ItemBlock for PVC %s", pods.Items[i].Name, pvc.Name)
				} else {
					a.log.Infof("Adding related Pod %s to PVC %s", pods.Items[i].Name, pvc.Name)
					relatedItems = append(relatedItems, velero.ResourceIdentifier{
						GroupResource: kuberesource.Pods,
						Namespace:     pods.Items[i].Namespace,
						Name:          pods.Items[i].Name,
					})
				}
				break
			}
		}
	}

	return relatedItems, nil
}

func (a *PVCAction) Name() string {
	return "PodItemBlockAction"
}
