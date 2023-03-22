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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	waitutil "k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

func CreateConfigMap(c clientset.Interface, ns, name string, labels, data map[string]string) (*v1.ConfigMap, error) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Data: data,
	}
	return c.CoreV1().ConfigMaps(ns).Create(context.TODO(), cm, metav1.CreateOptions{})
}

// WaitForConfigMapComplete uses c to wait for completions to complete for the Job jobName in namespace ns.
func WaitForConfigMapComplete(c clientset.Interface, ns, configmapName string) error {
	return wait.Poll(PollInterval, PollTimeout, func() (bool, error) {
		_, err := c.CoreV1().ConfigMaps(ns).Get(context.TODO(), configmapName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

func GetConfigmap(c clientset.Interface, ns, secretName string) (*v1.ConfigMap, error) {
	return c.CoreV1().ConfigMaps(ns).Get(context.TODO(), secretName, metav1.GetOptions{})
}

func WaitForConfigmapDelete(c clientset.Interface, ns, name string) error {
	if err := c.CoreV1().ConfigMaps(ns).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete  configmap in namespace %q", ns))
	}
	return waitutil.PollImmediateInfinite(5*time.Second,
		func() (bool, error) {
			if _, err := c.CoreV1().ConfigMaps(ns).Get(context.TODO(), ns, metav1.GetOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			}
			logrus.Debugf("configmap %q in namespace %q is still being deleted...", name, ns)
			return false, nil
		})
}
