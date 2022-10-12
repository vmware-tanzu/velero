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

package repository

import "sync"

// RepoLocker manages exclusive/non-exclusive locks for
// operations against backup repositories. The semantics
// of exclusive/non-exclusive locks are the same as for
// a sync.RWMutex, where a non-exclusive lock is equivalent
// to a read lock, and an exclusive lock is equivalent to
// a write lock.
type RepoLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.RWMutex
}

func NewRepoLocker() *RepoLocker {
	return &RepoLocker{
		locks: make(map[string]*sync.RWMutex),
	}
}

// LockExclusive acquires an exclusive lock for the specified
// repository. This function blocks until no other locks exist
// for the repo.
func (rl *RepoLocker) LockExclusive(name string) {
	rl.ensureLock(name).Lock()
}

// Lock acquires a non-exclusive lock for the specified
// repository. This function blocks until no exclusive
// locks exist for the repo.
func (rl *RepoLocker) Lock(name string) {
	rl.ensureLock(name).RLock()
}

// UnlockExclusive releases an exclusive lock for the repo.
func (rl *RepoLocker) UnlockExclusive(name string) {
	rl.ensureLock(name).Unlock()
}

// Unlock releases a non-exclusive lock for the repo.
func (rl *RepoLocker) Unlock(name string) {
	rl.ensureLock(name).RUnlock()
}

func (rl *RepoLocker) ensureLock(name string) *sync.RWMutex {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, ok := rl.locks[name]; !ok {
		rl.locks[name] = new(sync.RWMutex)
	}

	return rl.locks[name]
}
