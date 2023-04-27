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
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ConcurrentLimitExceed error = errors.New("Concurrent number exceeds")
var FSBRCreator = newFileSystemBR

type Manager struct {
	cocurrentNum int
	trackerLock  sync.Mutex
	tracker      map[string]AsyncBR
}

// NewManager creates the data path manager to manage concurrent data path instances
func NewManager(cocurrentNum int) *Manager {
	return &Manager{
		cocurrentNum: cocurrentNum,
		tracker:      map[string]AsyncBR{},
	}
}

// CreateFileSystemBR creates a new file system backup/restore data path instance
func (m *Manager) CreateFileSystemBR(jobName string, requestorType string, ctx context.Context, client client.Client, namespace string, callbacks Callbacks, log logrus.FieldLogger) (AsyncBR, error) {
	m.trackerLock.Lock()
	defer m.trackerLock.Unlock()

	if len(m.tracker) == m.cocurrentNum {
		return nil, ConcurrentLimitExceed
	}

	m.tracker[jobName] = FSBRCreator(jobName, requestorType, client, namespace, callbacks, log)

	return m.tracker[jobName], nil
}

// RemoveAsyncBR removes a file system backup/restore data path instance
func (m *Manager) RemoveAsyncBR(jobName string) {
	m.trackerLock.Lock()
	defer m.trackerLock.Unlock()

	delete(m.tracker, jobName)
}

// GetAsyncBR returns the file system backup/restore data path instance for the specified job name
func (m *Manager) GetAsyncBR(jobName string) AsyncBR {
	m.trackerLock.Lock()
	defer m.trackerLock.Unlock()

	if async, exist := m.tracker[jobName]; exist {
		return async
	} else {
		return nil
	}
}
