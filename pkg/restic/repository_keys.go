/*
Copyright 2018 the Velero contributors.

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
	"context"

	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	CredentialsSecretName = "velero-restic-credentials"
	CredentialsKey        = "repository-password"

	encryptionKey = "static-passw0rd"
)

func EnsureCommonRepositoryKey(secretClient corev1client.SecretsGetter, namespace string) error {
	_, err := secretClient.Secrets(namespace).Get(context.TODO(), CredentialsSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.WithStack(err)
	}
	if err == nil {
		return nil
	}

	// if we got here, we got an IsNotFound error, so we need to create the key

	secret := &corev1api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      CredentialsSecretName,
		},
		Type: corev1api.SecretTypeOpaque,
		Data: map[string][]byte{
			CredentialsKey: []byte(encryptionKey),
		},
	}

	if _, err = secretClient.Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return errors.Wrapf(err, "error creating %s secret", CredentialsSecretName)
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
	secret, err := c.client.Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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

	key, found := secret.Data[CredentialsKey]
	if !found {
		return nil, errors.Errorf("%q secret is missing data for key %q", CredentialsSecretName, CredentialsKey)
	}

	return key, nil
}
