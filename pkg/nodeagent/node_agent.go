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
	ErrDaemonSetNotFound = errors.New("daemonset not found")
)

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
