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

func TestCreateFileSystemBR(t *testing.T) {
	m := NewManager(2)

	async_job_1, err := m.CreateFileSystemBR(context.TODO(), "job-1", "test", nil, "velero", Callbacks{}, nil)
	assert.NoError(t, err)

	_, err = m.CreateFileSystemBR(context.TODO(), "job-2", "test", nil, "velero", Callbacks{}, nil)
	assert.NoError(t, err)

	_, err = m.CreateFileSystemBR(context.TODO(), "job-3", "test", nil, "velero", Callbacks{}, nil)
	assert.Equal(t, ConcurrentLimitExceed, err)

	ret := m.GetAsyncBR("job-0")
	assert.Nil(t, ret)

	ret = m.GetAsyncBR("job-1")
	assert.Equal(t, async_job_1, ret)

	m.RemoveAsyncBR("job-0")
	assert.Len(t, m.tracker, 2)

	m.RemoveAsyncBR("job-1")
	assert.Len(t, m.tracker, 1)

	ret = m.GetAsyncBR("job-1")
	assert.Nil(t, ret)
}

func TestCreateMicroServiceBRWatcher(t *testing.T) {
	m := NewManager(2)

	async_job_1, err := m.CreateMicroServiceBRWatcher(context.TODO(), nil, nil, nil, "test", "job-1", "velero", "pod-1", "container", "du-1", Callbacks{}, false, nil)
	assert.NoError(t, err)

	_, err = m.CreateMicroServiceBRWatcher(context.TODO(), nil, nil, nil, "test", "job-2", "velero", "pod-2", "container", "du-2", Callbacks{}, false, nil)
	assert.NoError(t, err)

	_, err = m.CreateMicroServiceBRWatcher(context.TODO(), nil, nil, nil, "test", "job-3", "velero", "pod-3", "container", "du-3", Callbacks{}, false, nil)
	assert.Equal(t, ConcurrentLimitExceed, err)

	async_job_4, err := m.CreateMicroServiceBRWatcher(context.TODO(), nil, nil, nil, "test", "job-4", "velero", "pod-4", "container", "du-4", Callbacks{}, true, nil)
	assert.NoError(t, err)

	ret := m.GetAsyncBR("job-0")
	assert.Nil(t, ret)

	ret = m.GetAsyncBR("job-1")
	assert.Equal(t, async_job_1, ret)

	ret = m.GetAsyncBR("job-4")
	assert.Equal(t, async_job_4, ret)

	m.RemoveAsyncBR("job-0")
	assert.Len(t, m.tracker, 3)

	m.RemoveAsyncBR("job-1")
	assert.Len(t, m.tracker, 2)

	ret = m.GetAsyncBR("job-1")
	assert.Nil(t, ret)
}
