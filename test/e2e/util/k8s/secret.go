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
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

func CreateSecret(c clientset.Interface, ns, name string, labels map[string]string) (*v1.Secret, error) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	return c.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
}

// WaitForSecretsComplete uses c to wait for completions to complete for the Job jobName in namespace ns.
func WaitForSecretsComplete(c clientset.Interface, ns, secretName string) error {
	return wait.Poll(PollInterval, PollTimeout, func() (bool, error) {
		_, err := c.CoreV1().Secrets(ns).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	})
}
