/*
Copyright 2017, 2019 the Velero contributors.

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
	"context"
	"io"
	"io/ioutil"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/heptio/velero/pkg/cloudprovider"
)

const credentialsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

// bucketWriter wraps the GCP SDK functions for accessing object store so they can be faked for testing.
type bucketWriter interface {
	// getWriteCloser returns an io.WriteCloser that can be used to upload data to the specified bucket for the specified key.
	getWriteCloser(bucket, key string) io.WriteCloser
	getAttrs(bucket, key string) (*storage.ObjectAttrs, error)
}

type writer struct {
	client *storage.Client
}

func (w *writer) getWriteCloser(bucket, key string) io.WriteCloser {
	return w.client.Bucket(bucket).Object(key).NewWriter(context.Background())
}

func (w *writer) getAttrs(bucket, key string) (*storage.ObjectAttrs, error) {
	return w.client.Bucket(bucket).Object(key).Attrs(context.Background())
}

type ObjectStore struct {
	log            logrus.FieldLogger
	client         *storage.Client
	googleAccessID string
	privateKey     []byte
	bucketWriter   bucketWriter
}

func NewObjectStore(logger logrus.FieldLogger) *ObjectStore {
	return &ObjectStore{log: logger}
}

func (o *ObjectStore) Init(config map[string]string) error {
	if err := cloudprovider.ValidateObjectStoreConfigKeys(config); err != nil {
		return err
	}

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

	o.googleAccessID = jwtConfig.Email
	o.privateKey = jwtConfig.PrivateKey

	client, err := storage.NewClient(context.Background(), option.WithScopes(storage.ScopeReadWrite))
	if err != nil {
		return errors.WithStack(err)
	}
	o.client = client

	o.bucketWriter = &writer{client: o.client}

	return nil
}

func (o *ObjectStore) PutObject(bucket, key string, body io.Reader) error {
	w := o.bucketWriter.getWriteCloser(bucket, key)

	// The writer returned by NewWriter is asynchronous, so errors aren't guaranteed
	// until Close() is called
	_, copyErr := io.Copy(w, body)

	// Ensure we close w and report errors properly
	closeErr := w.Close()
	if copyErr != nil {
		return copyErr
	}

	return closeErr
}

func (o *ObjectStore) ObjectExists(bucket, key string) (bool, error) {
	if _, err := o.bucketWriter.getAttrs(bucket, key); err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, errors.WithStack(err)
	}

	return true, nil
}

func (o *ObjectStore) GetObject(bucket, key string) (io.ReadCloser, error) {
	r, err := o.client.Bucket(bucket).Object(key).NewReader(context.Background())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return r, nil
}

func (o *ObjectStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	q := &storage.Query{
		Prefix:    prefix,
		Delimiter: delimiter,
	}

	iter := o.client.Bucket(bucket).Objects(context.Background(), q)

	var res []string
	for {
		obj, err := iter.Next()
		if err != nil && err != iterator.Done {
			return nil, errors.WithStack(err)
		}
		if err == iterator.Done {
			break
		}

		if obj.Prefix != "" {
			res = append(res, obj.Prefix)
		}
	}

	return res, nil
}

func (o *ObjectStore) ListObjects(bucket, prefix string) ([]string, error) {
	q := &storage.Query{
		Prefix: prefix,
	}

	var res []string

	iter := o.client.Bucket(bucket).Objects(context.Background(), q)

	for {
		obj, err := iter.Next()
		if err == iterator.Done {
			return res, nil
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}

		res = append(res, obj.Name)
	}
}

func (o *ObjectStore) DeleteObject(bucket, key string) error {
	return errors.Wrapf(o.client.Bucket(bucket).Object(key).Delete(context.Background()), "error deleting object %s", key)
}

func (o *ObjectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return storage.SignedURL(bucket, key, &storage.SignedURLOptions{
		GoogleAccessID: o.googleAccessID,
		PrivateKey:     o.privateKey,
		Method:         "GET",
		Expires:        time.Now().Add(ttl),
	})
}
