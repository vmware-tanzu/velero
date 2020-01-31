/*
Copyright 2019 the Velero contributors.

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

package logging

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// LogCounterHook is a logrus hook that counts the number of log
// statements that have been written at each logrus level.
type LogCounterHook struct {
	mu     sync.RWMutex
	counts map[logrus.Level]int
}

// NewLogCounterHook returns a pointer to an initialized LogCounterHook.
func NewLogCounterHook() *LogCounterHook {
	return &LogCounterHook{
		counts: make(map[logrus.Level]int),
	}
}

// Levels returns the logrus levels that the hook should be fired for.
func (h *LogCounterHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire executes the hook's logic.
func (h *LogCounterHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.counts[entry.Level]++

	return nil
}

// GetCount returns the number of log statements that have been
// written at the specific level provided.
func (h *LogCounterHook) GetCount(level logrus.Level) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.counts[level]
}
