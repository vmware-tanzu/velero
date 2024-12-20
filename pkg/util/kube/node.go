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
package kube

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsLinuxNode(ctx context.Context, nodeName string, client client.Client) error {
	node := &corev1api.Node{}
	if err := client.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		return errors.Wrapf(err, "error getting node %s", nodeName)
	}

	if os, found := node.Labels["kubernetes.io/os"]; !found {
		return errors.Errorf("no os type label for node %s", nodeName)
	} else if os != "linux" {
		return errors.Errorf("os type %s for node %s is not linux", os, nodeName)
	} else {
		return nil
	}
}

func WithLinuxNode(ctx context.Context, client client.Client, log logrus.FieldLogger) bool {
	return withOSNode(ctx, client, "linux", log)
}

func WithWindowsNode(ctx context.Context, client client.Client, log logrus.FieldLogger) bool {
	return withOSNode(ctx, client, "windows", log)
}

func withOSNode(ctx context.Context, client client.Client, osType string, log logrus.FieldLogger) bool {
	nodeList := new(corev1api.NodeList)
	if err := client.List(ctx, nodeList); err != nil {
		log.Warn("Failed to list nodes, cannot decide existence of windows nodes")
		return false
	}

	for _, node := range nodeList.Items {
		if os, found := node.Labels["kubernetes.io/os"]; !found {
			log.Warnf("Node %s doesn't have os type label, cannot decide existence of windows nodes")
		} else if os == osType {
			return true
		}
	}

	return false
}
