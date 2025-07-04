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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/internal/encryption"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

var (
	ErrEncryptWriter  = errors.New("failed to create encrypt writer")
	ErrEncryptionCopy = errors.New("encryption copy failed")
	ErrEncryptClose   = errors.New("encryptWriter close error")
	ErrInnerPutObject = errors.New("inner PutObject failed")
	ErrDecryptObject  = errors.New("failed to decrypt object")
)

// encryptedStore wraps an ObjectStore applying Encryption if provided.
type encryptedStore struct {
	enc    encryption.Encryption
	inner  velero.ObjectStore
	logger logrus.FieldLogger
}

// NewEncryptedStore wraps inner store with enc. If enc is nil, returns inner unchanged.
func NewEncryptedStore(inner velero.ObjectStore, et velerov1api.EncryptionType, l logrus.FieldLogger, opts map[string]string) (velero.ObjectStore, error) {
	enc, err := encryption.NewEncryptionFromConfig(et, opts)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return inner, err
	}
	return &encryptedStore{
		inner:  inner,
		enc:    enc,
		logger: l,
	}, nil
}

func (e *encryptedStore) PutObject(bucket, key string, data io.Reader) error {
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		encWriter, err := e.enc.Encrypt(pw)
		if err != nil {
			err = fmt.Errorf("%w: %v", ErrEncryptWriter, err)
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		if _, copyErr := io.Copy(encWriter, data); copyErr != nil {
			copyErr = fmt.Errorf("%w: %v", ErrEncryptionCopy, copyErr)
			_ = pw.CloseWithError(copyErr)
			e.logger.Errorf("Encrypt copy failed: %v", copyErr)
			errCh <- copyErr
			return
		}
		if cerr := encWriter.Close(); cerr != nil {
			cerr = fmt.Errorf("%w: %v", ErrEncryptClose, cerr)
			_ = pw.CloseWithError(cerr)
			errCh <- cerr
			return
		}
		_ = pw.Close()
		errCh <- nil
	}()

	innerErr := e.inner.PutObject(bucket, key, pr)

	if innerErr != nil {
		_ = pw.CloseWithError(innerErr)
		if isEncryptionPipelineError(innerErr) {
			return innerErr
		}
		return fmt.Errorf("%w: %v", ErrInnerPutObject, innerErr)
	}

	if gorErr := <-errCh; gorErr != nil {
		return gorErr
	}

	e.logger.Debugf("Encrypt successful")
	return nil
}

type readCloser struct {
	plain       io.Reader
	closeBehind io.Closer
}

func (rc *readCloser) Read(p []byte) (int, error) {
	return rc.plain.Read(p)
}

func (rc *readCloser) Close() error {
	return rc.closeBehind.Close()
}

func (e *encryptedStore) GetObject(bucket, key string) (io.ReadCloser, error) {
	rc, err := e.inner.GetObject(bucket, key)
	if err != nil {
		return nil, err
	}
	plain, err := e.enc.Decrypt(rc)
	if err != nil {
		rc.Close()
		return nil, fmt.Errorf("%w: %v", ErrDecryptObject, err)
	}
	return &readCloser{
		plain:       plain,
		closeBehind: rc,
	}, nil
}

func (e *encryptedStore) DeleteObject(bucket, key string) error {
	return e.inner.DeleteObject(bucket, key)
}

func (e *encryptedStore) ListObjects(bucket, prefix string) ([]string, error) {
	return e.inner.ListObjects(bucket, prefix)
}

func (e *encryptedStore) Init(config map[string]string) error {
	return e.inner.Init(config)
}

func (e *encryptedStore) CreateSignedURL(bucket, key string, ttl time.Duration) (string, error) {
	return e.inner.CreateSignedURL(bucket, key, ttl)
}

func (e *encryptedStore) ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error) {
	return e.inner.ListCommonPrefixes(bucket, prefix, delimiter)
}

func (e *encryptedStore) ObjectExists(bucket, key string) (bool, error) {
	return e.inner.ObjectExists(bucket, key)
}

func isEncryptionPipelineError(err error) bool {
	for _, ee := range []error{ErrEncryptWriter, ErrEncryptionCopy, ErrEncryptClose} {
		if errors.Is(err, ee) {
			return true
		}
	}
	return false
}
