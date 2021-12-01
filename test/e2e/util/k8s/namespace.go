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

package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	waitutil "k8s.io/apimachinery/pkg/util/wait"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

func CreateNamespace(ctx context.Context, client TestClient, namespace string) error {
	ns := builder.ForNamespace(namespace).Result()
	_, err := client.ClientGo.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func CreateNamespaceWithLabel(ctx context.Context, client TestClient, namespace string, label map[string]string) error {
	ns := builder.ForNamespace(namespace).Result()
	ns.Labels = label
	_, err := client.ClientGo.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func GetNamespace(ctx context.Context, client TestClient, namespace string) (*corev1api.Namespace, error) {
	return client.ClientGo.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
}

func DeleteNamespace(ctx context.Context, client TestClient, namespace string, wait bool) error {
	if err := client.ClientGo.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete the namespace %q", namespace))
	}
	if !wait {
		return nil
	}

	return waitutil.PollImmediateInfinite(5*time.Second,
		func() (bool, error) {
			if _, err := client.ClientGo.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			}
			logrus.Debugf("namespace %q is still being deleted...", namespace)
			return false, nil
		})
}

func CleanupNamespaces(ctx context.Context, client TestClient, nsBaseName string) error {
	namespaces, err := client.ClientGo.CoreV1().Namespaces().List(ctx, v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Could not retrieve namespaces")
	}
	for _, checkNamespace := range namespaces.Items {
		if strings.HasPrefix(checkNamespace.Name, nsBaseName) {
			err = client.ClientGo.CoreV1().Namespaces().Delete(ctx, checkNamespace.Name, v1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "Could not delete namespace %s", checkNamespace.Name)
			}
		}
	}
	return nil
}
