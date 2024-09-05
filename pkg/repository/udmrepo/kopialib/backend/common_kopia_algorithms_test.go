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

package backend

import (
	"context"
	"testing"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/encryption"
	"github.com/kopia/kopia/repo/format"
	"github.com/kopia/kopia/repo/hashing"
	"github.com/kopia/kopia/repo/splitter"
	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
)

func TestSetupNewRepoAlgorithms(t *testing.T) {
	testCases := []struct {
		name     string
		envVars  map[string]string
		flags    map[string]string
		expected repo.NewRepositoryOptions
	}{
		{
			name: "with valid non-default hash algo from env",
			envVars: map[string]string{
				"KOPIA_HASHING_ALGORITHM": "HMAC-SHA3-224",
			},
			flags: map[string]string{},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       "HMAC-SHA3-224",
					Encryption: encryption.DefaultAlgorithm,
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: splitter.DefaultAlgorithm,
				},
			},
		},
		{
			name: "with valid non-default encryption algo from env",
			envVars: map[string]string{
				"KOPIA_HASHING_ALGORITHM":    "",
				"KOPIA_SPLITTER_ALGORITHM":   "",
				"KOPIA_ENCRYPTION_ALGORITHM": "CHACHA20-POLY1305-HMAC-SHA256",
			},
			flags: map[string]string{},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       hashing.DefaultAlgorithm,
					Encryption: "CHACHA20-POLY1305-HMAC-SHA256",
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: splitter.DefaultAlgorithm,
				},
			},
		},
		{
			name:    "with valid non-default splitter algo from env",
			envVars: map[string]string{"KOPIA_SPLITTER_ALGORITHM": "FIXED-512K"},
			flags:   map[string]string{},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       hashing.DefaultAlgorithm,
					Encryption: encryption.DefaultAlgorithm,
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: "FIXED-512K",
				},
			},
		},
		{
			name:    "with valid non-default splitter and hashing algo from env, invalid encryption from env",
			envVars: map[string]string{"KOPIA_SPLITTER_ALGORITHM": "FIXED-512K", "KOPIA_HASHING_ALGORITHM": "HMAC-SHA3-224", "KOPIA_ENCRYPTION_ALGORITHM": "NON-EXISTING-SHA256"},
			flags:   map[string]string{},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       "HMAC-SHA3-224",
					Encryption: encryption.DefaultAlgorithm,
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: "FIXED-512K",
				},
			},
		},
		{
			name:    "with unsupported hash algo in env, fallback to default",
			envVars: map[string]string{"KOPIA_HASHING_ALGORITHM": "unsupported-hash"},
			flags:   map[string]string{},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       hashing.DefaultAlgorithm,
					Encryption: encryption.DefaultAlgorithm,
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: splitter.DefaultAlgorithm,
				},
			},
		},
		{
			name:    "hash in StoreOptionGenHashAlgo and env, env wins",
			envVars: map[string]string{"KOPIA_HASHING_ALGORITHM": "HMAC-SHA3-224"},
			flags: map[string]string{
				udmrepo.StoreOptionGenHashAlgo: "HMAC-SHA3-256",
			},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       "HMAC-SHA3-224",
					Encryption: encryption.DefaultAlgorithm,
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: splitter.DefaultAlgorithm,
				},
			},
		},
		{
			name:    "hash in StoreOptionGenHashAlgo and invalid in env, StoreOptionGenHashAlgo takes precedence",
			envVars: map[string]string{"KOPIA_HASHING_ALGORITHM": "INVALID"},
			flags: map[string]string{
				udmrepo.StoreOptionGenHashAlgo: "HMAC-SHA3-256",
			},
			expected: repo.NewRepositoryOptions{
				BlockFormat: format.ContentFormat{
					Hash:       "HMAC-SHA3-256",
					Encryption: encryption.DefaultAlgorithm,
				},
				ObjectFormat: format.ObjectFormat{
					Splitter: splitter.DefaultAlgorithm,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range tc.envVars {
				t.Setenv(key, value)
			}
			ret := SetupNewRepositoryOptions(context.Background(), tc.flags)
			assert.Equal(t, tc.expected, ret)
		})
	}
}
