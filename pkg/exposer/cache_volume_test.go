/*
Copyright The Velero Contributors.

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

package exposer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetCacheVolumeSize(t *testing.T) {
	tests := []struct {
		name     string
		dataSize int64
		info     *CacheConfigs
		expected int64
	}{
		{
			name:     "nil info",
			dataSize: 1024,
			expected: 0,
		},
		{
			name:     "0 data size",
			info:     &CacheConfigs{Limit: 1 << 30, ResidentThreshold: 5120},
			expected: 2 << 30,
		},
		{
			name:     "0 threshold",
			dataSize: 2048,
			info:     &CacheConfigs{Limit: 1 << 30},
			expected: 2 << 30,
		},
		{
			name:     "data size is smaller",
			dataSize: 2048,
			info:     &CacheConfigs{Limit: 1 << 30, ResidentThreshold: 5120},
			expected: 0,
		},
		{
			name:     "data size is lager",
			dataSize: 2048,
			info:     &CacheConfigs{Limit: 1 << 30, ResidentThreshold: 1024},
			expected: 2 << 30,
		},
		{
			name:     "limit smaller than 1G",
			dataSize: 2048,
			info:     &CacheConfigs{Limit: 5120, ResidentThreshold: 1024},
			expected: 1 << 30,
		},
		{
			name:     "larger than 1G after inflate",
			dataSize: 2048,
			info:     &CacheConfigs{Limit: (1 << 30) - 1024, ResidentThreshold: 1024},
			expected: 2 << 30,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			size := getCacheVolumeSize(test.dataSize, test.info)
			require.Equal(t, test.expected, size)
		})
	}
}
