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

// hookTrackerKey identifies a backup/restore hook
type hookTrackerKey struct {
	// PodNamespace indicates the namespace of pod where hooks are executed.
	// For hooks specified in the backup/restore spec, this field is the namespace of an applicable pod.
	// For hooks specified in pod annotation, this field is the namespace of pod where hooks are annotated.
	podNamespace string
	// PodName indicates the pod where hooks are executed.
	// For hooks specified in the backup/restore spec, this field is an applicable pod name.
	// For hooks specified in pod annotation, this field is the pod where hooks are annotated.
	podName string
	// HookPhase is only for backup hooks, for restore hooks, this field is empty.
	hookPhase hookPhase
	// HookName is only for hooks specified in the backup/restore spec.
	// For hooks specified in pod annotation, this field is empty or "<from-annotation>".
	hookName string
	// HookSource indicates where hooks come from.
	hookSource string
	// Container indicates the container hooks use.
	// For hooks specified in the backup/restore spec, the container might be the same under different hookName.
	container string
}

// hookTrackerVal records the execution status of a specific hook.
// hookTrackerVal is extensible to accommodate additional fields as needs develop.
type hookTrackerVal struct {
	// HookFailed indicates if hook failed to execute.
	hookFailed bool
	// hookExecuted indicates if hook already execute.
	hookExecuted bool
}

// HookTracker tracks all hooks' execution status
type HookTracker struct {
	lock    *sync.RWMutex
	tracker map[hookTrackerKey]hookTrackerVal
}

// NewHookTracker creates a hookTracker.
func NewHookTracker() *HookTracker {
	return &HookTracker{
		lock:    &sync.RWMutex{},
		tracker: make(map[hookTrackerKey]hookTrackerVal),
	}
}

// Add adds a hook to the tracker
// Add must precede the Record for each individual hook.
// In other words, a hook must be added to the tracker before its execution result is recorded.
func (ht *HookTracker) Add(podNamespace, podName, container, source, hookName string, hookPhase hookPhase) {
	ht.lock.Lock()
	defer ht.lock.Unlock()

	key := hookTrackerKey{
		podNamespace: podNamespace,
		podName:      podName,
		hookSource:   source,
		container:    container,
		hookPhase:    hookPhase,
		hookName:     hookName,
	}

	if _, ok := ht.tracker[key]; !ok {
		ht.tracker[key] = hookTrackerVal{
			hookFailed:   false,
			hookExecuted: false,
		}
	}
}

// Record records the hook's execution status
// Add must precede the Record for each individual hook.
// In other words, a hook must be added to the tracker before its execution result is recorded.
func (ht *HookTracker) Record(podNamespace, podName, container, source, hookName string, hookPhase hookPhase, hookFailed bool) error {
	ht.lock.Lock()
	defer ht.lock.Unlock()

	key := hookTrackerKey{
		podNamespace: podNamespace,
		podName:      podName,
		hookSource:   source,
		container:    container,
		hookPhase:    hookPhase,
		hookName:     hookName,
	}

	var err error
	if _, ok := ht.tracker[key]; ok {
		ht.tracker[key] = hookTrackerVal{
			hookFailed:   hookFailed,
			hookExecuted: true,
		}
	} else {
		err = fmt.Errorf("hook not exist in hooks tracker, hook key: %v", key)
	}
	return err
}

// Stat calculates the number of attempted hooks and failed hooks
func (ht *HookTracker) Stat() (hookAttemptedCnt int, hookFailed int) {
	ht.lock.RLock()
	defer ht.lock.RUnlock()

	for _, hookInfo := range ht.tracker {
		if hookInfo.hookExecuted {
			hookAttemptedCnt++
			if hookInfo.hookFailed {
				hookFailed++
			}
		}
	}
	return
}

// GetTracker gets the tracker inside HookTracker
func (ht *HookTracker) GetTracker() map[hookTrackerKey]hookTrackerVal {
	ht.lock.RLock()
	defer ht.lock.RUnlock()

	return ht.tracker
}
