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

type objectStorageAdapter struct {
	gcs            *storage.Service
	googleAccessID string
	privateKey     []byte
}

var _ cloudprovider.ObjectStorageAdapter = &objectStorageAdapter{}

func NewObjectStorageAdapter(googleAccessID string, privateKey []byte) (cloudprovider.ObjectStorageAdapter, error) {
	client, err := google.DefaultClient(oauth2.NoContext, storage.DevstorageReadWriteScope)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	gcs, err := storage.New(client)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &objectStorageAdapter{
		gcs:            gcs,
		googleAccessID: googleAccessID,
		privateKey:     privateKey,
	}, nil
}

func (op *objectStorageAdapter) PutObject(bucket string, key string, body io.ReadSeeker) error {
	obj := &storage.Object{
		Name: key,
	}

	_, err := op.gcs.Objects.Insert(bucket, obj).Media(body).Do()

	return errors.WithStack(err)
}

func (op *objectStorageAdapter) GetObject(bucket string, key string) (io.ReadCloser, error) {
	res, err := op.gcs.Objects.Get(bucket, key).Download()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return res.Body, nil
}

func (op *objectStorageAdapter) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	res, err := op.gcs.Objects.List(bucket).Delimiter(delimiter).Do()
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

func (op *objectStorageAdapter) ListObjects(bucket, prefix string) ([]string, error) {
	res, err := op.gcs.Objects.List(bucket).Prefix(prefix).Do()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ret := make([]string, 0, len(res.Items))
	for _, item := range res.Items {
		ret = append(ret, item.Name)
	}

	return ret, nil
}

func (op *objectStorageAdapter) DeleteObject(bucket string, key string) error {
	return errors.Wrapf(op.gcs.Objects.Delete(bucket, key).Do(), "error deleting object %s", key)
}

func (op *objectStorageAdapter) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	if op.googleAccessID == "" {
		return "", errors.New("unable to create a pre-signed URL - make sure GOOGLE_APPLICATION_CREDENTIALS points to a valid GCE service account file (missing email address)")
	}
	if len(op.privateKey) == 0 {
		return "", errors.New("unable to create a pre-signed URL - make sure GOOGLE_APPLICATION_CREDENTIALS points to a valid GCE service account file (missing private key)")
	}

	return newstorage.SignedURL(bucket, key, &newstorage.SignedURLOptions{
		GoogleAccessID: op.googleAccessID,
		PrivateKey:     op.privateKey,
		Method:         "GET",
		Expires:        time.Now().Add(ttl),
	})
}
