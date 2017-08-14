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

	storage "google.golang.org/api/storage/v1"

	"github.com/heptio/ark/pkg/cloudprovider"
)

type objectStorageAdapter struct {
	gcs *storage.Service
}

var _ cloudprovider.ObjectStorageAdapter = &objectStorageAdapter{}

func (op *objectStorageAdapter) PutObject(bucket string, key string, body io.ReadSeeker) error {
	obj := &storage.Object{
		Name: key,
	}

	_, err := op.gcs.Objects.Insert(bucket, obj).Media(body).Do()

	return err
}

func (op *objectStorageAdapter) GetObject(bucket string, key string) (io.ReadCloser, error) {
	res, err := op.gcs.Objects.Get(bucket, key).Download()
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}

func (op *objectStorageAdapter) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	res, err := op.gcs.Objects.List(bucket).Delimiter(delimiter).Do()
	if err != nil {
		return nil, err
	}

	// GCP returns prefixes inclusive of the last delimiter. We need to strip
	// it.
	ret := make([]string, 0, len(res.Prefixes))
	for _, prefix := range res.Prefixes {
		ret = append(ret, prefix[0:strings.LastIndex(prefix, delimiter)])
	}

	return ret, nil
}

func (op *objectStorageAdapter) DeleteObject(bucket string, key string) error {
	return op.gcs.Objects.Delete(bucket, key).Do()
}
