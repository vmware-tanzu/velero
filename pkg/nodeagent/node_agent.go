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
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/pkg/util/kube"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// daemonSet is the name of the Velero node agent daemonset.
	daemonSet = "node-agent"
)

var (
	DaemonsetNotFound = errors.New("daemonset not found")
)

// IsRunning checks if the node agent daemonset is running properly. If not, return the error found
func IsRunning(ctx context.Context, kubeClient kubernetes.Interface, namespace string) error {
	if _, err := kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, daemonSet, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		return DaemonsetNotFound
	} else if err != nil {
		return err
	} else {
		return nil
	}
}

// IsRunningInNode checks if the node agent pod is running properly in a specified node. If not, return the error found
func IsRunningInNode(ctx context.Context, namespace string, nodeName string, podClient corev1client.PodsGetter) error {
	if nodeName == "" {
		return errors.New("node name is empty")
	}

	pods, err := podClient.Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("name=%s", daemonSet)})
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
