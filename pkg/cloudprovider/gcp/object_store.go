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
	"encoding/base64"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/vmware-tanzu/velero/pkg/cloudprovider"
)

const (
	credentialsEnvVar    = "GOOGLE_APPLICATION_CREDENTIALS"
	kmsKeyNameConfigKey  = "kmsKeyName"
	serviceAccountConfig = "serviceAccount"
)

// bucketWriter wraps the GCP SDK functions for accessing object store so they can be faked for testing.
type bucketWriter interface {
	// getWriteCloser returns an io.WriteCloser that can be used to upload data to the specified bucket for the specified key.
	getWriteCloser(bucket, key string) io.WriteCloser
	getAttrs(bucket, key string) (*storage.ObjectAttrs, error)
}

type writer struct {
	client     *storage.Client
	kmsKeyName string
}

func (w *writer) getWriteCloser(bucket, key string) io.WriteCloser {
	writer := w.client.Bucket(bucket).Object(key).NewWriter(context.Background())
	writer.KMSKeyName = w.kmsKeyName

	return writer
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
	iamSvc         *iamcredentials.Service
}

func NewObjectStore(logger logrus.FieldLogger) *ObjectStore {
	return &ObjectStore{log: logger}
}

func (o *ObjectStore) Init(config map[string]string) error {
	if err := cloudprovider.ValidateObjectStoreConfigKeys(config, kmsKeyNameConfigKey, serviceAccountConfig); err != nil {
		return err
	}
	// Find default token source to extract the GoogleAccessID
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx)

	if err != nil {
		return errors.WithStack(err)
	}

	if creds.JSON != nil {
		// Using Credentials File
		err = o.initFromKeyFile(creds)
	} else {
		// Using compute engine credentials. Use this if workload identity is enabled.
		err = o.initFromComputeEngine(config)
	}

	if err != nil {
		return errors.WithStack(err)
	}

	client, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadWrite))
	if err != nil {
		return errors.WithStack(err)
	}
	o.client = client

	o.bucketWriter = &writer{
		client:     o.client,
		kmsKeyName: config[kmsKeyNameConfigKey],
	}
	return nil
}

func (o *ObjectStore) initFromKeyFile(creds *google.Credentials) error {
	jwtConfig, err := google.JWTConfigFromJSON(creds.JSON)
	if err != nil {
		return errors.Wrap(err, "error parsing credentials file; should be JSON")
	}
	if jwtConfig.Email == "" {
		return errors.Errorf("credentials file pointed to by %s does not contain an email", "GOOGLE_APPLICATION_CREDENTIALS")
	}
	if len(jwtConfig.PrivateKey) == 0 {
		return errors.Errorf("credentials file pointed to by %s does not contain a private key", "GOOGLE_APPLICATION_CREDENTIALS")
	}

	o.googleAccessID = jwtConfig.Email
	o.privateKey = jwtConfig.PrivateKey
	return nil
}

func (o *ObjectStore) initFromComputeEngine(config map[string]string) error {
	var err error
	var ok bool
	o.googleAccessID, ok = config["serviceAccount"]
	if !ok {
		return errors.Errorf("serviceAccount is expected to be provided as an item in BackupStorageLocation's config")
	}
	o.iamSvc, err = iamcredentials.NewService(context.Background())
	return err
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

/*
 * Use the iamSignBlob api call to sign the url if there is no credentials file to get the key from.
 * https://cloud.google.com/iam/credentials/reference/rest/v1/projects.serviceAccounts/signBlob
 */
func (o *ObjectStore) SignBytes(bytes []byte) ([]byte, error) {
	name := "projects/-/serviceAccounts/" + o.googleAccessID
	resp, err := o.iamSvc.Projects.ServiceAccounts.SignBlob(name, &iamcredentials.SignBlobRequest{
		Payload: base64.StdEncoding.EncodeToString(bytes),
	}).Context(context.Background()).Do()

	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(resp.SignedBlob)
}

func (o *ObjectStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	options := storage.SignedURLOptions{
		GoogleAccessID: o.googleAccessID,
		Method:         "GET",
		Expires:        time.Now().Add(ttl),
	}

	if o.privateKey == nil {
		options.SignBytes = o.SignBytes
	} else {
		options.PrivateKey = o.privateKey
	}

	return storage.SignedURL(bucket, key, &options)
}
