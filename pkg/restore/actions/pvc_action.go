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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util"
)

const (
	AnnBindCompleted          = "pv.kubernetes.io/bind-completed"
	AnnBoundByController      = "pv.kubernetes.io/bound-by-controller"
	AnnStorageProvisioner     = "volume.kubernetes.io/storage-provisioner"
	AnnBetaStorageProvisioner = "volume.beta.kubernetes.io/storage-provisioner"
	AnnSelectedNode           = "volume.kubernetes.io/selected-node"
)

// PVCAction updates/reset PVC's node selector
// if a mapping is found in the plugin's config map.
type PVCAction struct {
	logger          logrus.FieldLogger
	configMapClient corev1client.ConfigMapInterface
	nodeClient      corev1client.NodeInterface
}

// NewPVCAction is the constructor for PVCAction.
func NewPVCAction(
	logger logrus.FieldLogger,
	configMapClient corev1client.ConfigMapInterface,
	nodeClient corev1client.NodeInterface,
) *PVCAction {
	return &PVCAction{
		logger:          logger,
		configMapClient: configMapClient,
		nodeClient:      nodeClient,
	}
}

// AppliesTo returns the resources that PVCAction should be run for
func (p *PVCAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

// PVC actions for restore:
//  1. updates the pvc's selected-node annotation:
//     a) if node mapping found in the config map for the plugin
//     b) if node mentioned in annotation doesn't exist
//  2. removes some additional annotations
//  3. returns bound PV as an additional item
func (p *PVCAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.logger.Info("Executing PVCAction")
	defer p.logger.Info("Done executing PVCAction")

	var pvc, pvcFromBackup corev1api.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &pvc); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.ItemFromBackup.UnstructuredContent(), &pvcFromBackup); err != nil {
		return nil, errors.WithStack(err)
	}

	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}

	log := p.logger.WithFields(map[string]any{
		"kind":      pvc.Kind,
		"namespace": pvc.Namespace,
		"name":      pvc.Name,
	})

	// Handle selected node annotation
	node, ok := pvc.Annotations[AnnSelectedNode]
	if ok {
		// fetch node mapping from configMap
		newNode, err := getNewNodeFromConfigMap(p.configMapClient, node)
		if err != nil {
			return nil, err
		}

		if len(newNode) != 0 {
			// Check whether the mapped node exists first.
			exists, err := isNodeExist(p.nodeClient, newNode)
			if err != nil {
				return nil, errors.Wrapf(err, "error checking %s's mapped node %s existence", node, newNode)
			}
			if !exists {
				log.Warnf("Selected-node's mapped node doesn't exist: source: %s, dest: %s. Please check the ConfigMap with label velero.io/change-pvc-node-selector.", node, newNode)
			}

			// set node selector
			// We assume that node exist for node-mapping
			pvc.Annotations[AnnSelectedNode] = newNode
			log.Infof("Updating selected-node to %s from %s", newNode, node)
		} else {
			// configMap doesn't have node-mapping
			// Let's check if node exists or not
			exists, err := isNodeExist(p.nodeClient, node)
			if err != nil {
				return nil, errors.Wrapf(err, "error checking node %s existence", node)
			}

			if !exists {
				log.Infof("Clearing selected-node because node named %s does not exist", node)
				delete(pvc.Annotations, AnnSelectedNode)
			}
		}
	}

	// Remove other annotations
	removePVCAnnotations(
		&pvc,
		[]string{
			AnnBindCompleted,
			AnnBoundByController,
			AnnStorageProvisioner,
			AnnBetaStorageProvisioner,
			velerov1api.VolumeSnapshotLabel,
			velerov1api.DataUploadNameAnnotation,
		},
	)

	pvcMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pvc)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	output := &velero.RestoreItemActionExecuteOutput{
		UpdatedItem: &unstructured.Unstructured{Object: pvcMap},
	}

	// Add PV as additional item if bound
	// use pvcFromBackup because we need to look at status fields, which have been removed from pvc
	if pvcFromBackup.Status.Phase != corev1api.ClaimBound || pvcFromBackup.Spec.VolumeName == "" {
		log.Info("PVC is not bound or its volume name is empty")
	} else {
		log.Infof("Adding PV %s as an additional item to restore", pvcFromBackup.Spec.VolumeName)
		output.AdditionalItems = []velero.ResourceIdentifier{
			{
				GroupResource: kuberesource.PersistentVolumes,
				Name:          pvcFromBackup.Spec.VolumeName,
			},
		}
	}
	return output, nil
}

func getNewNodeFromConfigMap(client corev1client.ConfigMapInterface, node string) (string, error) {
	// fetch node mapping from configMap
	config, err := common.GetPluginConfig(common.PluginKindRestoreItemAction, "velero.io/change-pvc-node-selector", client)
	if err != nil {
		return "", err
	}

	if config == nil {
		// there is no node mapping defined for change-pvc-node
		// so we will return empty new node
		return "", nil
	}

	return config.Data[node], nil
}

// isNodeExist check if node resource exist or not
func isNodeExist(nodeClient corev1client.NodeInterface, name string) (bool, error) {
	_, err := nodeClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func removePVCAnnotations(pvc *corev1api.PersistentVolumeClaim, remove []string) {
	for k := range pvc.Annotations {
		if util.Contains(remove, k) {
			delete(pvc.Annotations, k)
		}
	}
}
