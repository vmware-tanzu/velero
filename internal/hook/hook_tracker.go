/*
Copyright 2020 the Velero contributors.

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

package hook

import (
	"fmt"
	"sync"
)

const (
	HookSourceAnnotation = "annotation"
	HookSourceSpec       = "spec"
)

// hookKey identifies a backup/restore hook
type hookKey struct {
	// PodNamespace indicates the namespace of pod where hooks are executed.
	// For hooks specified in the backup/restore spec, this field is the namespace of an applicable pod.
	// For hooks specified in pod annotation, this field is the namespace of pod where hooks are annotated.
	podNamespace string
	// PodName indicates the pod where hooks are executed.
	// For hooks specified in the backup/restore spec, this field is an applicable pod name.
	// For hooks specified in pod annotation, this field is the pod where hooks are annotated.
	podName string
	// HookPhase is only for backup hooks, for restore hooks, this field is empty.
	hookPhase HookPhase
	// HookName is only for hooks specified in the backup/restore spec.
	// For hooks specified in pod annotation, this field is empty or "<from-annotation>".
	hookName string
	// HookSource indicates where hooks come from.
	hookSource string
	// Container indicates the container hooks use.
	// For hooks specified in the backup/restore spec, the container might be the same under different hookName.
	container string
}

// hookStatus records the execution status of a specific hook.
// hookStatus is extensible to accommodate additional fields as needs develop.
type hookStatus struct {
	// HookFailed indicates if hook failed to execute.
	hookFailed bool
	// hookExecuted indicates if hook already execute.
	hookExecuted bool
}

// HookTracker tracks all hooks' execution status in a single backup/restore.
type HookTracker struct {
	lock *sync.RWMutex
	// tracker records all hook info for a single backup/restore.
	tracker map[hookKey]hookStatus
	// hookAttemptedCnt indicates the number of attempted hooks.
	hookAttemptedCnt int
	// hookFailedCnt indicates the number of failed hooks.
	hookFailedCnt int
	// HookExecutedCnt indicates the number of executed hooks.
	hookExecutedCnt int
	// hookErrs records hook execution errors if any.
	hookErrs        []HookErrInfo
	AsyncItemBlocks *sync.WaitGroup
}

// NewHookTracker creates a hookTracker instance.
func NewHookTracker() *HookTracker {
	return &HookTracker{
		lock:            &sync.RWMutex{},
		tracker:         make(map[hookKey]hookStatus),
		AsyncItemBlocks: &sync.WaitGroup{},
	}
}

// Add adds a hook to the hook tracker
// Add must precede the Record for each individual hook.
// In other words, a hook must be added to the tracker before its execution result is recorded.
func (ht *HookTracker) Add(podNamespace, podName, container, source, hookName string, hookPhase HookPhase) {
	ht.lock.Lock()
	defer ht.lock.Unlock()

	key := hookKey{
		podNamespace: podNamespace,
		podName:      podName,
		hookSource:   source,
		container:    container,
		hookPhase:    hookPhase,
		hookName:     hookName,
	}

	if _, ok := ht.tracker[key]; !ok {
		ht.tracker[key] = hookStatus{
			hookFailed:   false,
			hookExecuted: false,
		}
		ht.hookAttemptedCnt++
	}
}

// Record records the hook's execution status
// Add must precede the Record for each individual hook.
// In other words, a hook must be added to the tracker before its execution result is recorded.
func (ht *HookTracker) Record(podNamespace, podName, container, source, hookName string, hookPhase HookPhase, hookFailed bool, hookErr error) error {
	ht.lock.Lock()
	defer ht.lock.Unlock()

	key := hookKey{
		podNamespace: podNamespace,
		podName:      podName,
		hookSource:   source,
		container:    container,
		hookPhase:    hookPhase,
		hookName:     hookName,
	}

	if _, ok := ht.tracker[key]; !ok {
		return fmt.Errorf("hook not exist in hook tracker, hook: %+v", key)
	}

	if !ht.tracker[key].hookExecuted {
		ht.tracker[key] = hookStatus{
			hookFailed:   hookFailed,
			hookExecuted: true,
		}
		ht.hookExecutedCnt++
		if hookFailed {
			ht.hookFailedCnt++
			ht.hookErrs = append(ht.hookErrs, HookErrInfo{Namespace: key.podNamespace, Err: hookErr})
		}
	}
	return nil
}

// Stat returns the number of attempted hooks and failed hooks
func (ht *HookTracker) Stat() (hookAttemptedCnt int, hookFailedCnt int) {
	ht.AsyncItemBlocks.Wait()

	ht.lock.RLock()
	defer ht.lock.RUnlock()

	return ht.hookAttemptedCnt, ht.hookFailedCnt
}

// IsComplete returns whether the execution of all hooks has finished or not
func (ht *HookTracker) IsComplete() bool {
	ht.lock.RLock()
	defer ht.lock.RUnlock()

	return ht.hookAttemptedCnt == ht.hookExecutedCnt
}

// HooksErr returns hook execution errors
func (ht *HookTracker) HookErrs() []HookErrInfo {
	ht.lock.RLock()
	defer ht.lock.RUnlock()

	return ht.hookErrs
}

// MultiHookTrackers tracks all hooks' execution status for multiple backups/restores.
type MultiHookTracker struct {
	lock *sync.RWMutex
	// trackers is a map that uses the backup/restore name as the key and stores a HookTracker as value.
	trackers map[string]*HookTracker
}

// NewMultiHookTracker creates a multiHookTracker instance.
func NewMultiHookTracker() *MultiHookTracker {
	return &MultiHookTracker{
		lock:     &sync.RWMutex{},
		trackers: make(map[string]*HookTracker),
	}
}

// Add adds a backup/restore hook to the tracker
func (mht *MultiHookTracker) Add(name, podNamespace, podName, container, source, hookName string, hookPhase HookPhase) {
	mht.lock.Lock()
	defer mht.lock.Unlock()

	if _, ok := mht.trackers[name]; !ok {
		mht.trackers[name] = NewHookTracker()
	}
	mht.trackers[name].Add(podNamespace, podName, container, source, hookName, hookPhase)
}

// Record records a backup/restore hook execution status
func (mht *MultiHookTracker) Record(name, podNamespace, podName, container, source, hookName string, hookPhase HookPhase, hookFailed bool, hookErr error) error {
	mht.lock.RLock()
	defer mht.lock.RUnlock()

	var err error
	if _, ok := mht.trackers[name]; ok {
		err = mht.trackers[name].Record(podNamespace, podName, container, source, hookName, hookPhase, hookFailed, hookErr)
	} else {
		err = fmt.Errorf("the backup/restore not exist in hook tracker, backup/restore name: %s", name)
	}
	return err
}

// Stat returns the number of attempted hooks and failed hooks for a particular backup/restore
func (mht *MultiHookTracker) Stat(name string) (hookAttemptedCnt int, hookFailedCnt int) {
	mht.lock.RLock()
	defer mht.lock.RUnlock()

	if _, ok := mht.trackers[name]; ok {
		return mht.trackers[name].Stat()
	}
	return
}

// Delete removes the hook data for a particular backup/restore
func (mht *MultiHookTracker) Delete(name string) {
	mht.lock.Lock()
	defer mht.lock.Unlock()

	delete(mht.trackers, name)
}

// IsComplete returns whether the execution of all hooks for a particular backup/restore has finished or not
func (mht *MultiHookTracker) IsComplete(name string) bool {
	mht.lock.RLock()
	defer mht.lock.RUnlock()

	if _, ok := mht.trackers[name]; ok {
		return mht.trackers[name].IsComplete()
	}
	return true
}

// HooksErr returns hook execution errors for a particular backup/restore
func (mht *MultiHookTracker) HookErrs(name string) []HookErrInfo {
	mht.lock.RLock()
	defer mht.lock.RUnlock()

	if _, ok := mht.trackers[name]; ok {
		return mht.trackers[name].HookErrs()
	}
	return nil
}
