/*
Copyright 2017 Heptio Inc.

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

package gcp

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	// TODO switch to using newstorage
	newstorage "cloud.google.com/go/storage"
	storage "google.golang.org/api/storage/v1"

	"github.com/heptio/ark/pkg/cloudprovider"
)

const credentialsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

type objectStore struct {
	gcs            *storage.Service
	googleAccessID string
	privateKey     []byte
}

func NewObjectStore() cloudprovider.ObjectStore {
	return &objectStore{}
}

func (o *objectStore) Init(config map[string]string) error {
	credentialsFile := os.Getenv(credentialsEnvVar)
	if credentialsFile == "" {
		return errors.Errorf("%s is undefined", credentialsEnvVar)
	}

	// Get the email and private key from the credentials file so we can pre-sign download URLs
	creds, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		return errors.WithStack(err)
	}
	jwtConfig, err := google.JWTConfigFromJSON(creds)
	if err != nil {
		return errors.WithStack(err)
	}
	if jwtConfig.Email == "" {
		return errors.Errorf("credentials file pointed to by %s does not contain an email", credentialsEnvVar)
	}
	if len(jwtConfig.PrivateKey) == 0 {
		return errors.Errorf("credentials file pointed to by %s does not contain a private key", credentialsEnvVar)
	}

	client, err := google.DefaultClient(oauth2.NoContext, storage.DevstorageReadWriteScope)
	if err != nil {
		return errors.WithStack(err)
	}

	gcs, err := storage.New(client)
	if err != nil {
		return errors.WithStack(err)
	}

	o.gcs = gcs
	o.googleAccessID = jwtConfig.Email
	o.privateKey = jwtConfig.PrivateKey

	return nil
}

func (o *objectStore) PutObject(bucket string, key string, body io.Reader) error {
	obj := &storage.Object{
		Name: key,
	}

	_, err := o.gcs.Objects.Insert(bucket, obj).Media(body).Do()

	return errors.WithStack(err)
}

func (o *objectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	res, err := o.gcs.Objects.Get(bucket, key).Download()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return res.Body, nil
}

func (o *objectStore) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	res, err := o.gcs.Objects.List(bucket).Delimiter(delimiter).Do()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// GCP returns prefixes inclusive of the last delimiter. We need to strip
	// it.
	ret := make([]string, 0, len(res.Prefixes))
	for _, prefix := range res.Prefixes {
		ret = append(ret, prefix[0:strings.LastIndex(prefix, delimiter)])
	}

	return ret, nil
}

func (o *objectStore) ListObjects(bucket, prefix string) ([]string, error) {
	res, err := o.gcs.Objects.List(bucket).Prefix(prefix).Do()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ret := make([]string, 0, len(res.Items))
	for _, item := range res.Items {
		ret = append(ret, item.Name)
	}

	return ret, nil
}

func (o *objectStore) DeleteObject(bucket string, key string) error {
	return errors.Wrapf(o.gcs.Objects.Delete(bucket, key).Do(), "error deleting object %s", key)
}

func (o *objectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return newstorage.SignedURL(bucket, key, &newstorage.SignedURLOptions{
		GoogleAccessID: o.googleAccessID,
		PrivateKey:     o.privateKey,
		Method:         "GET",
		Expires:        time.Now().Add(ttl),
	})
}
