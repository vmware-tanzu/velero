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

package datapath

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager(t *testing.T) {
	m := NewManager(2)

	async_job_1, err := m.CreateFileSystemBR("job-1", "test", context.TODO(), nil, "velero", Callbacks{}, nil)
	assert.NoError(t, err)

	_, err = m.CreateFileSystemBR("job-2", "test", context.TODO(), nil, "velero", Callbacks{}, nil)
	assert.NoError(t, err)

	_, err = m.CreateFileSystemBR("job-3", "test", context.TODO(), nil, "velero", Callbacks{}, nil)
	assert.Equal(t, ConcurrentLimitExceed, err)

	ret := m.GetAsyncBR("job-0")
	assert.Equal(t, nil, ret)

	ret = m.GetAsyncBR("job-1")
	assert.Equal(t, async_job_1, ret)

	m.RemoveAsyncBR("job-0")
	assert.Equal(t, 2, len(m.tracker))

	m.RemoveAsyncBR("job-1")
	assert.Equal(t, 1, len(m.tracker))

	ret = m.GetAsyncBR("job-1")
	assert.Equal(t, nil, ret)
}
