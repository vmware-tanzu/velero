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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHookTracker(t *testing.T) {
	tracker := NewHookTracker()

	assert.NotNil(t, tracker)
	assert.Empty(t, tracker.tracker)
}

func TestHookTracker_Add(t *testing.T) {
	tracker := NewHookTracker()

	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre)

	key := hookTrackerKey{
		podNamespace: "ns1",
		podName:      "pod1",
		container:    "container1",
		hookPhase:    PhasePre,
		hookSource:   HookSourceAnnotation,
		hookName:     "h1",
	}

	_, ok := tracker.tracker[key]
	assert.True(t, ok)
}

func TestHookTracker_Record(t *testing.T) {
	tracker := NewHookTracker()
	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre)
	tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre, true)

	key := hookTrackerKey{
		podNamespace: "ns1",
		podName:      "pod1",
		container:    "container1",
		hookPhase:    PhasePre,
		hookSource:   HookSourceAnnotation,
		hookName:     "h1",
	}

	info := tracker.tracker[key]
	assert.True(t, info.hookFailed)
}

func TestHookTracker_Stat(t *testing.T) {
	tracker := NewHookTracker()

	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre)
	tracker.Add("ns2", "pod2", "container1", HookSourceAnnotation, "h2", PhasePre)
	tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre, true)

	attempted, failed := tracker.Stat()
	assert.Equal(t, 1, attempted)
	assert.Equal(t, 1, failed)
}

func TestHookTracker_Get(t *testing.T) {
	tracker := NewHookTracker()
	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre)

	tr := tracker.GetTracker()
	assert.NotNil(t, tr)

	t.Logf("tracker :%+v", tr)
}
