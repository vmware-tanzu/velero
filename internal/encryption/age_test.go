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

package encryption_test

import (
	"bytes"
	"io"
	"testing"

	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/vmware-tanzu/velero/internal/encryption"
)

func TestNewAgeEncryption(t *testing.T) {
	tests := []struct {
		name    string
		opts    map[string]string
		wantErr bool
	}{
		{name: "missing options", opts: map[string]string{}, wantErr: true},
		{name: "invalid options", opts: map[string]string{
			"recipient":  "not-a-recipient",
			"privateKey": "not-a-key",
		}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := tc.wantErr
			_, err := encryption.NewAgeEncryption(tc.opts)
			require.Equal(t, req, err != nil)
		})
	}
}

func TestAgeEncryptionScenarios(t *testing.T) {
	tests := []struct {
		name       string
		optsGenEnc func(*testing.T) map[string]string
		optsGenDec func(*testing.T) map[string]string // nil => reuse enc instance
		plaintext  []byte
		wantErr    bool
	}{
		{
			name:       "decrypt with wrong key",
			optsGenEnc: generateX25519Opts,
			optsGenDec: generateX25519Opts,
			plaintext:  []byte("secret data"),
			wantErr:    true,
		},
		{
			name:       "X25519 encrypt-decrypt",
			optsGenEnc: generateX25519Opts,
			optsGenDec: nil,
			plaintext:  []byte("Hello, Velero encryption test via X25519!"),
			wantErr:    false,
		},
		{
			name:       "SSH encrypt-decrypt",
			optsGenEnc: generateSSHOpts,
			optsGenDec: nil,
			plaintext:  []byte("Hello, Velero encryption test via SSH!"),
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encryptOpts := tc.optsGenEnc(t)
			enc, err := encryption.NewAgeEncryption(encryptOpts)
			require.NoError(t, err)

			var buf bytes.Buffer
			wc, err := enc.Encrypt(&buf)
			require.NoError(t, err)
			_, err = wc.Write(tc.plaintext)
			require.NoError(t, err)
			require.NoError(t, wc.Close())

			var decrypter encryption.Encryption
			if tc.optsGenDec != nil {
				decOpts := tc.optsGenDec(t)
				decrypter, err = encryption.NewAgeEncryption(decOpts)
				require.NoError(t, err)
			} else {
				decrypter = enc
			}

			var decErr error
			rc, err := decrypter.Decrypt(io.NopCloser(bytes.NewReader(buf.Bytes())))
			if err != nil {
				decErr = err
			} else {
				_, decErr = io.ReadAll(rc)
				require.NoError(t, rc.Close())
			}

			require.Equal(t, tc.wantErr, decErr != nil)
		})
	}
}

func generateX25519Opts(t *testing.T) map[string]string {
	t.Helper()

	id, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	return map[string]string{
		"recipient":  id.Recipient().String(),
		"privateKey": id.String(),
	}
}

func generateSSHOpts(t *testing.T) map[string]string {
	t.Helper()

	pubRaw, privRaw, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	pubKey, err := ssh.NewPublicKey(pubRaw)
	require.NoError(t, err)
	pubBytes := ssh.MarshalAuthorizedKey(pubKey)

	der, err := x509.MarshalPKCS8PrivateKey(privRaw)
	require.NoError(t, err)
	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	privPEM := pem.EncodeToMemory(pemBlock)

	return map[string]string{
		"recipient":  string(pubBytes),
		"privateKey": string(privPEM),
	}
}
