/*
Copyright 2018 the Heptio Ark contributors.

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

package restic

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	CredentialsSecretName = "ark-restic-credentials"
	CurrentCredentialsKey = "current-key"
	NewCredentialsKey     = "new-key"
	OldCredentialsKey     = "old-key"
)

func ChangeRepositoryKey(secretClient corev1client.SecretsGetter, namespace string, newKey []byte) error {
	secret, err := secretClient.Secrets(namespace).Get(CredentialsSecretName, metav1.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	if _, ok := secret.Data[NewCredentialsKey]; ok {
		return errors.Errorf("repository already has an in-progress key change request")
	}

	original, err := json.Marshal(secret)
	if err != nil {
		return errors.Wrapf(err, "error marshalling secret to JSON")
	}

	secret.Data[NewCredentialsKey] = newKey

	updated, err := json.Marshal(secret)
	if err != nil {
		return errors.Wrapf(err, "error marshalling updated secret to JSON")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(original, updated)
	if err != nil {
		return errors.Wrapf(err, "error creating merge patch")
	}

	if _, err := secretClient.Secrets(namespace).Patch(CredentialsSecretName, types.MergePatchType, patchBytes); err != nil {
		return errors.Wrapf(err, "error patching secret")
	}

	return nil
}

func NewRepositoryKey(secretClient corev1client.SecretsGetter, namespace string, data []byte) error {
	secret := &corev1api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      CredentialsSecretName,
		},
		Type: corev1api.SecretTypeOpaque,
		Data: map[string][]byte{
			CurrentCredentialsKey: data,
		},
	}

	_, err := secretClient.Secrets(namespace).Create(secret)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type SecretGetter interface {
	GetSecret(namespace, name string) (*corev1api.Secret, error)
}

type clientSecretGetter struct {
	client corev1client.SecretsGetter
}

func NewClientSecretGetter(client corev1client.SecretsGetter) SecretGetter {
	return &clientSecretGetter{client: client}
}

func (c *clientSecretGetter) GetSecret(namespace, name string) (*corev1api.Secret, error) {
	secret, err := c.client.Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return secret, nil
}

type listerSecretGetter struct {
	lister corev1listers.SecretLister
}

func NewListerSecretGetter(lister corev1listers.SecretLister) SecretGetter {
	return &listerSecretGetter{lister: lister}
}

func (l *listerSecretGetter) GetSecret(namespace, name string) (*corev1api.Secret, error) {
	secret, err := l.lister.Secrets(namespace).Get(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return secret, nil
}

func GetRepositoryKey(secretGetter SecretGetter, namespace string) ([]byte, error) {
	secret, err := secretGetter.GetSecret(namespace, CredentialsSecretName)
	if err != nil {
		return nil, err
	}

	key, found := secret.Data[CurrentCredentialsKey]
	if !found {
		return nil, errors.Errorf("%q secret is missing data for key %q", CredentialsSecretName, CurrentCredentialsKey)
	}

	return key, nil
}
