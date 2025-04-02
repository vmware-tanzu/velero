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

package backup

import (
	"sort"
	"sync"
)

type SkippedPV struct {
	Name    string         `json:"name"`
	Reasons []PVSkipReason `json:"reasons"`
}

func (s *SkippedPV) SerializeSkipReasons() string {
	ret := ""
	for _, reason := range s.Reasons {
		ret = ret + reason.Approach + ": " + reason.Reason + ";"
	}
	return ret
}

type PVSkipReason struct {
	Approach string `json:"approach"`
	Reason   string `json:"reason"`
}

// skipPVTracker keeps track of persistent volumes that have been skipped and the reason why they are skipped.
type skipPVTracker struct {
	*sync.RWMutex
	// pvs is a map of name of the pv to the list of reasons why it is skipped.
	// The reasons are stored in a map each key of the map is the backup approach, each approach can have one reason
	pvs map[string]map[string]string
	// includedPVs is a set of pv to be included in the backup, the element in this set should not be in the "pvs" map
	includedPVs map[string]struct{}
}

const (
	podVolumeApproach       = "podvolume"
	csiSnapshotApproach     = "csiSnapshot"
	volumeSnapshotApproach  = "volumeSnapshot"
	vsphereSnapshotApproach = "vsphereSnapshot"
	anyApproach             = "any"
)

func NewSkipPVTracker() *skipPVTracker {
	return &skipPVTracker{
		RWMutex:     &sync.RWMutex{},
		pvs:         make(map[string]map[string]string),
		includedPVs: make(map[string]struct{}),
	}
}

// Track tracks the pv with the specified name and the reason why it is skipped
func (pt *skipPVTracker) Track(name, approach, reason string) {
	pt.Lock()
	defer pt.Unlock()
	if name == "" || reason == "" {
		return
	}
	if _, ok := pt.includedPVs[name]; ok {
		return
	}
	skipReasons := pt.pvs[name]
	if skipReasons == nil {
		skipReasons = make(map[string]string)
		pt.pvs[name] = skipReasons
	}
	if approach == "" {
		approach = anyApproach
	}
	skipReasons[approach] = reason
}

// Untrack removes the pvc with the specified namespace and name.
// This func should be called when the PV is taken for snapshot, regardless native snapshot, CSI snapshot or fsb backup
// therefore, in one backup processed if a PV is Untracked once, it will not be tracked again.
func (pt *skipPVTracker) Untrack(name string) {
	pt.Lock()
	defer pt.Unlock()
	pt.includedPVs[name] = struct{}{}
	delete(pt.pvs, name)
}

// Summary returns the summary of the tracked pvcs.
func (pt *skipPVTracker) Summary() []SkippedPV {
	pt.RLock()
	defer pt.RUnlock()
	keys := make([]string, 0, len(pt.pvs))
	for key := range pt.pvs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	res := make([]SkippedPV, 0, len(keys))
	for _, key := range keys {
		if skipReasons := pt.pvs[key]; len(skipReasons) > 0 {
			entry := SkippedPV{
				Name:    key,
				Reasons: make([]PVSkipReason, 0, len(skipReasons)),
			}
			approaches := make([]string, 0, len(skipReasons))
			for a := range skipReasons {
				approaches = append(approaches, a)
			}
			sort.Strings(approaches)
			for _, a := range approaches {
				entry.Reasons = append(entry.Reasons, PVSkipReason{
					Approach: a,
					Reason:   skipReasons[a],
				})
			}
			res = append(res, entry)
		}
	}
	return res
}
