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

package encryption

import (
	"fmt"
	"io"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// Encryption defines streaming encryption/decryption.
type Encryption interface {
	Encrypt(io.Writer) (io.WriteCloser, error)
	Decrypt(io.ReadCloser) (io.ReadCloser, error)
}

// NewEncryptionFromConfig returns an Encryption instance based on type and options.
func NewEncryptionFromConfig(t velerov1api.EncryptionType, opts map[string]string) (Encryption, error) {
	switch t {
	case velerov1api.EncryptionTypeNone:
		return nil, nil
	case velerov1api.EncryptionTypeAge:
		return NewAgeEncryption(opts)
	default:
		return nil, fmt.Errorf("unsupported encryption type %q", t)
	}
}
