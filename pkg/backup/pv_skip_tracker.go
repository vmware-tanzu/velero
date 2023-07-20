package backup

import (
	"sort"
	"sync"
)

type SkippedPV struct {
	Name    string         `json:"name"`
	Reasons []PVSkipReason `json:"reasons"`
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
}

const (
	podVolumeApproach      = "podvolume"
	csiSnapshotApproach    = "csiSnapshot"
	volumeSnapshotApproach = "volumeSnapshot"
	anyApproach            = "any"
)

func NewSkipPVTracker() *skipPVTracker {
	return &skipPVTracker{
		RWMutex: &sync.RWMutex{},
		pvs:     make(map[string]map[string]string),
	}
}

// Track tracks the pv with the specified name and the reason why it is skipped
func (pt *skipPVTracker) Track(name, approach, reason string) {
	pt.Lock()
	defer pt.Unlock()
	if name == "" || reason == "" {
		return
	}
	skipReasons := pt.pvs[name]
	if skipReasons == nil {
		skipReasons = make(map[string]string, 0)
		pt.pvs[name] = skipReasons
	}
	if approach == "" {
		approach = anyApproach
	}
	skipReasons[approach] = reason
}

// Untrack removes the pvc with the specified namespace and name.
func (pt *skipPVTracker) Untrack(name string) {
	pt.Lock()
	defer pt.Unlock()
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
