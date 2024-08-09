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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHookTracker(t *testing.T) {
	tracker := NewHookTracker()

	assert.NotNil(t, tracker)
	assert.Empty(t, tracker.tracker)
}

func TestHookTracker_Add(t *testing.T) {
	tracker := NewHookTracker()

	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")

	key := hookKey{
		podNamespace: "ns1",
		podName:      "pod1",
		container:    "container1",
		hookPhase:    "",
		hookSource:   HookSourceAnnotation,
		hookName:     "h1",
	}

	_, ok := tracker.tracker[key]
	assert.True(t, ok)
}

func TestHookTracker_Record(t *testing.T) {
	tracker := NewHookTracker()
	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	err := tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))

	key := hookKey{
		podNamespace: "ns1",
		podName:      "pod1",
		container:    "container1",
		hookPhase:    "",
		hookSource:   HookSourceAnnotation,
		hookName:     "h1",
	}

	info := tracker.tracker[key]
	assert.True(t, info.hookFailed)
	assert.True(t, info.hookExecuted)
	require.NoError(t, err)

	err = tracker.Record("ns2", "pod2", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))
	require.Error(t, err)

	err = tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", false, nil)
	require.NoError(t, err)
	assert.True(t, info.hookFailed)
}

func TestHookTracker_Stat(t *testing.T) {
	tracker := NewHookTracker()

	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	tracker.Add("ns2", "pod2", "container1", HookSourceAnnotation, "h2", "")
	tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))

	attempted, failed := tracker.Stat()
	assert.Equal(t, 2, attempted)
	assert.Equal(t, 1, failed)
}

func TestHookTracker_IsComplete(t *testing.T) {
	tracker := NewHookTracker()
	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre)
	tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre, true, fmt.Errorf("err"))
	assert.True(t, tracker.IsComplete())

	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	assert.False(t, tracker.IsComplete())
}

func TestHookTracker_HookErrs(t *testing.T) {
	tracker := NewHookTracker()
	tracker.Add("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	tracker.Record("ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))

	hookErrs := tracker.HookErrs()
	assert.Len(t, hookErrs, 1)
}

func TestMultiHookTracker_Add(t *testing.T) {
	mht := NewMultiHookTracker()

	mht.Add("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")

	key := hookKey{
		podNamespace: "ns1",
		podName:      "pod1",
		container:    "container1",
		hookPhase:    "",
		hookSource:   HookSourceAnnotation,
		hookName:     "h1",
	}

	_, ok := mht.trackers["restore1"].tracker[key]
	assert.True(t, ok)
}

func TestMultiHookTracker_Record(t *testing.T) {
	mht := NewMultiHookTracker()
	mht.Add("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	err := mht.Record("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))

	key := hookKey{
		podNamespace: "ns1",
		podName:      "pod1",
		container:    "container1",
		hookPhase:    "",
		hookSource:   HookSourceAnnotation,
		hookName:     "h1",
	}

	info := mht.trackers["restore1"].tracker[key]
	assert.True(t, info.hookFailed)
	assert.True(t, info.hookExecuted)
	require.NoError(t, err)

	err = mht.Record("restore1", "ns2", "pod2", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))
	require.Error(t, err)

	err = mht.Record("restore2", "ns2", "pod2", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))
	require.Error(t, err)
}

func TestMultiHookTracker_Stat(t *testing.T) {
	mht := NewMultiHookTracker()

	mht.Add("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	mht.Add("restore1", "ns2", "pod2", "container1", HookSourceAnnotation, "h2", "")
	mht.Record("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))
	mht.Record("restore1", "ns2", "pod2", "container1", HookSourceAnnotation, "h2", "", false, nil)

	attempted, failed := mht.Stat("restore1")
	assert.Equal(t, 2, attempted)
	assert.Equal(t, 1, failed)
}

func TestMultiHookTracker_Delete(t *testing.T) {
	mht := NewMultiHookTracker()
	mht.Add("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	mht.Delete("restore1")

	_, ok := mht.trackers["restore1"]
	assert.False(t, ok)
}

func TestMultiHookTracker_IsComplete(t *testing.T) {
	mht := NewMultiHookTracker()
	mht.Add("backup1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre)
	mht.Record("backup1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", PhasePre, true, fmt.Errorf("err"))
	assert.True(t, mht.IsComplete("backup1"))

	mht.Add("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	assert.False(t, mht.IsComplete("restore1"))

	assert.True(t, mht.IsComplete("restore2"))
}

func TestMultiHookTracker_HookErrs(t *testing.T) {
	mht := NewMultiHookTracker()
	mht.Add("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "")
	mht.Record("restore1", "ns1", "pod1", "container1", HookSourceAnnotation, "h1", "", true, fmt.Errorf("err"))

	hookErrs := mht.HookErrs("restore1")
	assert.Len(t, hookErrs, 1)

	hookErrs2 := mht.HookErrs("restore2")
	assert.Empty(t, hookErrs2)
}
