/*
Copyright the Velero contributors.

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

package persistence

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

const (
	testBucket = "bucket"
	testKey    = "key"
)

// globalOpts initializes encryption options once for all tests
var globalOpts map[string]string

func TestEncryptedStore_PutGet_Success(t *testing.T) {
	tests := []struct {
		name    string
		encType v1.EncryptionType
	}{
		{"AgeEncryption", v1.EncryptionTypeAge},
		{"NoneEncryption", v1.EncryptionTypeNone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newInMemoryObjectStore(testBucket)
			es := mustNewEncryptedStore(t, s, tc.encType)

			original := []byte("confidential data bytes")

			err := es.PutObject(testBucket, testKey, bytes.NewReader(original))
			require.NoError(t, err)

			raw := s.Data[testBucket][testKey]
			if tc.encType == v1.EncryptionTypeAge {
				assert.NotEqual(t, original, raw)
			} else {
				assert.Equal(t, original, raw)
			}

			rc, err := es.GetObject(testBucket, testKey)
			require.NoError(t, err)
			defer func() { assert.NoError(t, rc.Close()) }()

			decrypted, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Equal(t, original, decrypted)
		})
	}
}

func TestEncryptedStore_GetObject_CorruptedData(t *testing.T) {
	s := newInMemoryObjectStore(testBucket)
	s.Data[testBucket][testKey] = []byte("invalid encrypted content")

	es := mustNewEncryptedStore(t, s, v1.EncryptionTypeAge)

	rc, err := es.GetObject(testBucket, testKey)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDecryptObject)

	assert.Nil(t, rc)
}

func TestEncryptedStore_PutObject_Errors(t *testing.T) {
	tests := []struct {
		name       string
		setupStore func() velero.ObjectStore
		setupEnc   func(s velero.ObjectStore) velero.ObjectStore
		reader     io.Reader
		wantErr    error
	}{
		{
			"EncryptError",
			func() velero.ObjectStore { return newInMemoryObjectStore(testBucket) },
			func(s velero.ObjectStore) velero.ObjectStore {
				return &encryptedStore{
					inner:  s,
					enc:    &fakeEncryption{},
					logger: logrus.NewEntry(logrus.New()),
				}
			},
			bytes.NewReader([]byte("data")),
			ErrEncryptWriter,
		},
		{
			"CopyError",
			func() velero.ObjectStore { return newInMemoryObjectStore(testBucket) },
			func(s velero.ObjectStore) velero.ObjectStore {
				return mustNewEncryptedStore(t, s, v1.EncryptionTypeAge)
			},
			&simulatedErrorReader{data: []byte("abcdefghijklmnopqrstuvwxyz"), max: 5},
			ErrEncryptionCopy,
		},
		{
			"InnerStoreError",
			func() velero.ObjectStore { return &errStore{} },
			func(s velero.ObjectStore) velero.ObjectStore {
				return mustNewEncryptedStore(t, s, v1.EncryptionTypeAge)
			},
			bytes.NewReader([]byte("data")),
			ErrInnerPutObject,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := tc.setupStore()
			es := tc.setupEnc(s)

			err := es.PutObject(testBucket, testKey, tc.reader)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.wantErr)

			if tc.name == "CopyError" {
				exists, _ := s.ObjectExists(testBucket, testKey)
				assert.False(t, exists)
			}
		})
	}
}

func mustNewEncryptedStore(t *testing.T, s velero.ObjectStore, et v1.EncryptionType) velero.ObjectStore {
	t.Helper()
	es, err := NewEncryptedStore(s, et, logrus.NewEntry(logrus.New()), globalOpts)
	require.NoError(t, err)
	return es
}

// fakeEncryption always fails on Encrypt
type fakeEncryption struct{}

func (f *fakeEncryption) Encrypt(writer io.Writer) (io.WriteCloser, error) {
	return nil, errors.New("simulated encrypt error")
}
func (f *fakeEncryption) Decrypt(reader io.ReadCloser) (io.ReadCloser, error) { return reader, nil }

// errStore returns an error on PutObject
type errStore struct{}

func (e *errStore) PutObject(bucket, key string, data io.Reader) error {
	return errors.New("store failure")
}
func (e *errStore) GetObject(bucket, key string) (io.ReadCloser, error) { return nil, nil }
func (e *errStore) DeleteObject(bucket, key string) error               { return nil }
func (e *errStore) ListObjects(bucket, prefix string) ([]string, error) { return nil, nil }
func (e *errStore) Init(config map[string]string) error                 { return nil }
func (e *errStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return "", nil
}
func (e *errStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	return nil, nil
}
func (e *errStore) ObjectExists(bucket, key string) (bool, error) { return false, nil }

type simulatedErrorReader struct {
	data   []byte
	max    int
	offset int
}

func (r *simulatedErrorReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	if r.offset+n > r.max {
		n = r.max - r.offset
		r.offset += n
		return n, errors.New("simulated read error")
	}
	r.offset += n
	return n, nil
}

func init() {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		panic("failed to generate age identity: " + err.Error())
	}
	recipient := id.Recipient().String()
	privKey := id.String()
	globalOpts = map[string]string{
		"recipient":  recipient,
		"privateKey": privKey,
	}
}
