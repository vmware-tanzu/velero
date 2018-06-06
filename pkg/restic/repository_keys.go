package restic

import (
	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	CredentialsSecretName = "ark-restic-credentials"
	CredentialsKey        = "ark-restic-credentials"
)

func NewRepositoryKey(secretClient corev1client.SecretsGetter, namespace string, data []byte) error {
	secret := &corev1api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      CredentialsSecretName,
		},
		Type: corev1api.SecretTypeOpaque,
		Data: map[string][]byte{
			CredentialsKey: data,
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

	key, found := secret.Data[CredentialsKey]
	if !found {
		return nil, errors.Errorf("%q secret is missing data for key %q", CredentialsSecretName, CredentialsKey)
	}

	return key, nil
}
