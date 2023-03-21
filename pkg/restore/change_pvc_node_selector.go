/*
Copyright 2020 the Velero contributors.

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

package restore

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// ChangePVCNodeSelectorAction updates/reset PVC's node selector
// if a mapping is found in the plugin's config map.
type ChangePVCNodeSelectorAction struct {
	logger          logrus.FieldLogger
	configMapClient corev1client.ConfigMapInterface
	nodeClient      corev1client.NodeInterface
}

// NewChangePVCNodeSelectorAction is the constructor for ChangePVCNodeSelectorAction.
func NewChangePVCNodeSelectorAction(
	logger logrus.FieldLogger,
	configMapClient corev1client.ConfigMapInterface,
	nodeClient corev1client.NodeInterface,
) *ChangePVCNodeSelectorAction {
	return &ChangePVCNodeSelectorAction{
		logger:          logger,
		configMapClient: configMapClient,
		nodeClient:      nodeClient,
	}
}

// AppliesTo returns the resources that ChangePVCNodeSelectorAction should be run for
func (p *ChangePVCNodeSelectorAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

// Execute updates the pvc's selected-node annotation:
//
//	a) if node mapping found in the config map for the plugin
//	b) if node mentioned in annotation doesn't exist
func (p *ChangePVCNodeSelectorAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.logger.Info("Executing ChangePVCNodeSelectorAction")
	defer p.logger.Info("Done executing ChangePVCNodeSelectorAction")

	typeAcc, err := meta.TypeAccessor(input.Item)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}

	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	log := p.logger.WithFields(map[string]interface{}{
		"kind":      typeAcc.GetKind(),
		"namespace": metadata.GetNamespace(),
		"name":      metadata.GetName(),
	})

	// let's check if PVC has annotation of the selected node
	node, ok := annotations["volume.kubernetes.io/selected-node"]
	if !ok {
		log.Debug("PVC doesn't have node selector")
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

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
		annotations["volume.kubernetes.io/selected-node"] = newNode
		metadata.SetAnnotations(annotations)
		log.Infof("Updating selected-node to %s from %s", newNode, node)
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	// configMap doesn't have node-mapping
	// Let's check if node exists or not
	exists, err := isNodeExist(p.nodeClient, node)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking node %s existence", node)
	}

	if !exists {
		log.Infof("Clearing selected-node because node named %s does not exist", node)
		delete(annotations, "volume.kubernetes.io/selected-node")
		if len(annotations) == 0 {
			metadata.SetAnnotations(nil)
		} else {
			metadata.SetAnnotations(annotations)
		}
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
}

func getNewNodeFromConfigMap(client corev1client.ConfigMapInterface, node string) (string, error) {
	// fetch node mapping from configMap
	config, err := getPluginConfig(common.PluginKindRestoreItemAction, "velero.io/change-pvc-node-selector", client)
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
