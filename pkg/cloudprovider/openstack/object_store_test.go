/*
Copyright 2019 the Heptio Ark contributors.

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

package openstack

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"testing"

	th "github.com/gophercloud/gophercloud/testhelper"
	fake "github.com/gophercloud/gophercloud/testhelper/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func handlePutObject(t *testing.T, bucket, key string, data []byte) {
	th.Mux.HandleFunc(fmt.Sprintf("/%s/%s", bucket, key),
		func(w http.ResponseWriter, r *http.Request) {
			th.TestMethod(t, r, http.MethodPut)
			th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)
			th.TestHeader(t, r, "Accept", "application/json")

			hash := md5.New()
			hash.Write(data)
			localChecksum := hash.Sum(nil)

			w.Header().Set("ETag", fmt.Sprintf("%x", localChecksum))
			w.WriteHeader(http.StatusCreated)
		})
}

func handleGetObject(t *testing.T, bucket, key string, data []byte) {
	th.Mux.HandleFunc(fmt.Sprintf("/%s/%s", bucket, key),
		func(w http.ResponseWriter, r *http.Request) {
			th.TestMethod(t, r, http.MethodGet)
			th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)
			th.TestHeader(t, r, "Accept", "application/json")

			hash := md5.New()
			hash.Write(data)
			localChecksum := hash.Sum(nil)

			w.Header().Set("ETag", fmt.Sprintf("%x", localChecksum))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(data))
		})
}

func handleDeleteObject(t *testing.T, bucket, key string) {
	th.Mux.HandleFunc(fmt.Sprintf("/%s/%s", bucket, key),
		func(w http.ResponseWriter, r *http.Request) {
			th.TestMethod(t, r, http.MethodDelete)
			th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)
			th.TestHeader(t, r, "Accept", "application/json")

			w.WriteHeader(http.StatusAccepted)
		})
}

func TestPutObject(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	bucket := "testBucket"
	key := "testKey"
	content := "All code is guilty until proven innocent"
	handlePutObject(t, bucket, key, []byte(content))

	store := objectStore{
		client: fake.ServiceClient(),
		log:    logrus.New(),
	}
	err := store.PutObject(bucket, key, strings.NewReader(content))
	assert.Nil(t, err)
}

func TestGetObject(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	bucket := "testBucket"
	key := "testKey"
	content := "All code is guilty until proven innocent"
	handleGetObject(t, bucket, key, []byte(content))

	store := objectStore{
		client: fake.ServiceClient(),
		log:    logrus.New(),
	}
	readCloser, err := store.GetObject(bucket, key)

	if !assert.Nil(t, err) {
		t.FailNow()
	}
	defer readCloser.Close()
}

func TestDeleteObject(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	bucket := "testBucket"
	key := "testKey"
	handleDeleteObject(t, bucket, key)

	store := objectStore{
		client: fake.ServiceClient(),
		log:    logrus.New(),
	}
	err := store.DeleteObject(bucket, key)

	if !assert.Nil(t, err) {
		t.FailNow()
	}
}
