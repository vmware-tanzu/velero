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

package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/vmware-tanzu/velero/pkg/builder"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func waitUntilServiceAccountCreated(ctx context.Context, client testClient, namespace, serviceAccount string, timeout time.Duration) error {
	return wait.PollImmediate(5*time.Second, timeout,
		func() (bool, error) {
			if _, err := client.clientGo.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccount, metav1.GetOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return false, err
				}
				return false, nil
			}
			return true, nil
		})
}

func patchServiceAccountWithImagePullSecret(ctx context.Context, client testClient, namespace, serviceAccount, dockerCredentialFile string) error {
	credential, err := ioutil.ReadFile(dockerCredentialFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read the docker credential file %q", dockerCredentialFile)
	}
	secretName := "image-pull-secret"
	secret := builder.ForSecret(namespace, secretName).Data(map[string][]byte{".dockerconfigjson": credential}).Result()
	secret.Type = corev1.SecretTypeDockerConfigJson
	if _, err = client.clientGo.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return errors.Wrapf(err, "failed to create secret %q under namespace %q", secretName, namespace)
	}

	if _, err = client.clientGo.CoreV1().ServiceAccounts(namespace).Patch(ctx, serviceAccount, types.StrategicMergePatchType,
		[]byte(fmt.Sprintf(`{"imagePullSecrets": [{"name": "%s"}]}`, secretName)), metav1.PatchOptions{}); err != nil {
		return errors.Wrapf(err, "failed to patch the service account %q under the namespace %q", serviceAccount, namespace)
	}
	return nil
}
