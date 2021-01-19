/*
Copyright 2021 the Velero contributors.

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

package credentials

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Getter interface {
	Get(selector *corev1.SecretKeySelector) ([]byte, error)

	// GetAsFile gets the secret data specified by the selector and returns a filepath
	// where the contents have been written to
	GetAsFile(selector *corev1.SecretKeySelector) (string, error)
}

type credentialsGetter struct {
	client           kubernetes.Interface
	namespace        string
	credentialsFiles map[corev1.SecretKeySelector]string
}

func (c *credentialsGetter) Get(selector *corev1.SecretKeySelector) ([]byte, error) {
	secret, err := c.client.CoreV1().Secrets(c.namespace).Get(context.TODO(), selector.Name, metav1.GetOptions{})
	if err != nil {
		return []byte{}, err
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return []byte{}, fmt.Errorf("invalid key selector, key %q does not exist in secret %q", selector.Key, selector.Name)
	}
	return data, nil
}

func (c *credentialsGetter) GetAsFile(selector *corev1.SecretKeySelector) (string, error) {
	creds, err := c.Get(selector)
	if err != nil {
		return "", err
	}

	filePath, ok := c.credentialsFiles[*selector]
	if !ok {
		tmpFile, err := ioutil.TempFile("", "creds")
		if err != nil {
			return "", errors.Wrap(err, "unable to create temp file to write credentials to")
		}
		filePath = tmpFile.Name()
	}

	if err := ioutil.WriteFile(filePath, creds, 0644); err != nil {
		return "", errors.Wrap(err, "unable to write credentials to temp file")
	}
	return filePath, nil
}

func NewCredentialsGetter(client kubernetes.Interface, namespace string) Getter {
	return &credentialsGetter{
		client:    client,
		namespace: namespace,
	}
}
