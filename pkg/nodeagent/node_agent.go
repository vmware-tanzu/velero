/*
Copyright The Velero Contributors.

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

package nodeagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	// daemonSet is the name of the Velero node agent daemonset.
	daemonSet = "node-agent"
)

var (
	ErrDaemonSetNotFound      = errors.New("daemonset not found")
	ErrNodeAgentLabelNotFound = errors.New("node-agent label not found")
)

type LoadConcurrency struct {
	// GlobalConfig specifies the concurrency number to all nodes for which per-node config is not specified
	GlobalConfig int `json:"globalConfig,omitempty"`

	// PerNodeConfig specifies the concurrency number to nodes matched by rules
	PerNodeConfig []RuledConfigs `json:"perNodeConfig,omitempty"`
}

type LoadAffinity struct {
	// NodeSelector specifies the label selector to match nodes
	NodeSelector metav1.LabelSelector `json:"nodeSelector"`
}

type RuledConfigs struct {
	// NodeSelector specifies the label selector to match nodes
	NodeSelector metav1.LabelSelector `json:"nodeSelector"`

	// Number specifies the number value associated to the matched nodes
	Number int `json:"number"`
}

type BackupPVC struct {
	// StorageClass is the name of storage class to be used by the backupPVC
	StorageClass string `json:"storageClass,omitempty"`

	// ReadOnly sets the backupPVC's access mode as read only
	ReadOnly bool `json:"readOnly,omitempty"`

	// SPCNoRelabeling sets Spec.SecurityContext.SELinux.Type to "spc_t" for the pod mounting the backupPVC
	// ignored if ReadOnly is false
	SPCNoRelabeling bool `json:"spcNoRelabeling,omitempty"`
}

type Configs struct {
	// LoadConcurrency is the config for data path load concurrency per node.
	LoadConcurrency *LoadConcurrency `json:"loadConcurrency,omitempty"`

	// LoadAffinity is the config for data path load affinity.
	LoadAffinity []*kube.LoadAffinity `json:"loadAffinity,omitempty"`

	// BackupPVCConfig is the config for backupPVC (intermediate PVC) of snapshot data movement
	BackupPVCConfig map[string]BackupPVC `json:"backupPVC,omitempty"`

	// PodResources is the resource config for various types of pods launched by node-agent, i.e., data mover pods.
	PodResources *kube.PodResources `json:"podResources,omitempty"`
}

// IsRunning checks if the node agent daemonset is running properly. If not, return the error found
func IsRunning(ctx context.Context, kubeClient kubernetes.Interface, namespace string) error {
	if _, err := kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, daemonSet, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		return ErrDaemonSetNotFound
	} else if err != nil {
		return err
	} else {
		return nil
	}
}

// IsRunningInNode checks if the node agent pod is running properly in a specified node. If not, return the error found
func IsRunningInNode(ctx context.Context, namespace string, nodeName string, crClient ctrlclient.Client) error {
	if nodeName == "" {
		return errors.New("node name is empty")
	}

	pods := new(v1.PodList)
	parsedSelector, err := labels.Parse(fmt.Sprintf("name=%s", daemonSet))
	if err != nil {
		return errors.Wrap(err, "fail to parse selector")
	}

	err = crClient.List(ctx, pods, &ctrlclient.ListOptions{LabelSelector: parsedSelector})
	if err != nil {
		return errors.Wrap(err, "failed to list daemonset pods")
	}

	for i := range pods.Items {
		if kube.IsPodRunning(&pods.Items[i]) != nil {
			continue
		}

		if pods.Items[i].Spec.NodeName == nodeName {
			return nil
		}
	}

	return errors.Errorf("daemonset pod not found in running state in node %s", nodeName)
}

func GetPodSpec(ctx context.Context, kubeClient kubernetes.Interface, namespace string) (*v1.PodSpec, error) {
	ds, err := kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, daemonSet, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error to get node-agent daemonset")
	}

	return &ds.Spec.Template.Spec, nil
}

func GetConfigs(ctx context.Context, namespace string, kubeClient kubernetes.Interface, configName string) (*Configs, error) {
	cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "error to get node agent configs %s", configName)
	}

	if cm.Data == nil {
		return nil, errors.Errorf("data is not available in config map %s", configName)
	}

	jsonString := ""
	for _, v := range cm.Data {
		jsonString = v
	}

	configs := &Configs{}
	err = json.Unmarshal([]byte(jsonString), configs)
	if err != nil {
		return nil, errors.Wrapf(err, "error to unmarshall configs from %s", configName)
	}

	return configs, nil
}

func GetLabelValue(ctx context.Context, kubeClient kubernetes.Interface, namespace string, key string) (string, error) {
	ds, err := kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, daemonSet, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "error getting node-agent daemonset")
	}

	if ds.Spec.Template.Labels == nil {
		return "", ErrNodeAgentLabelNotFound
	}

	val, found := ds.Spec.Template.Labels[key]
	if !found {
		return "", ErrNodeAgentLabelNotFound
	}

	return val, nil
}
