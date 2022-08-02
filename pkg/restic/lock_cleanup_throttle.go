/*
Copyright 2018 the Velero contributors.

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
package restic

import (
    "time"
    "sync"
)

const (
    cleanupEvery = 30 * time.Minute
)

type lockCleanupThrottle struct {
    mu           sync.Mutex
    last_cleanup map[string]time.Time
}

func newLockCleanupThrottle() *lockCleanupThrottle {
    return &lockCleanupThrottle{
        last_cleanup: make(map[string]time.Time),
    }
}

func (l *lockCleanupThrottle) shouldPerformCleanup(name string) bool {
    l.mu.Lock()
	defer l.mu.Unlock()

    if t, ok := l.last_cleanup[name]; !ok {
        return time.Since(t) > cleanupEvery
    }

    return true
}

func (l *lockCleanupThrottle) locksCleanedUp(name string) {
    l.mu.Lock()
	defer l.mu.Unlock()

    l.last_cleanup[name] = time.Now()
}
