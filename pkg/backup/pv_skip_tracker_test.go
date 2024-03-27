package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummary(t *testing.T) {
	tracker := NewSkipPVTracker()
	tracker.Track("pv5", "", "skipped due to policy")
	tracker.Track("pv3", podVolumeApproach, "it's set to opt-out")
	tracker.Track("pv3", csiSnapshotApproach, "not applicable for CSI ")
	// shouldn't be added
	tracker.Track("", podVolumeApproach, "pvc3 is set to be skipped")
	tracker.Track("pv10", volumeSnapshotApproach, "added by mistake")
	tracker.Untrack("pv10")
	expected := []SkippedPV{
		{
			Name: "pv3",
			Reasons: []PVSkipReason{
				{
					Approach: csiSnapshotApproach,
					Reason:   "not applicable for CSI ",
				},
				{
					Approach: podVolumeApproach,
					Reason:   "it's set to opt-out",
				},
			},
		},
		{
			Name: "pv5",
			Reasons: []PVSkipReason{
				{
					Approach: anyApproach,
					Reason:   "skipped due to policy",
				},
			},
		},
	}
	assert.Equal(t, expected, tracker.Summary())
}

func TestSerializeSkipReasons(t *testing.T) {
	tracker := NewSkipPVTracker()
	//tracker.Track("pv5", "", "skipped due to policy")
	tracker.Track("pv3", podVolumeApproach, "it's set to opt-out")
	tracker.Track("pv3", csiSnapshotApproach, "not applicable for CSI ")

	for _, skippedPV := range tracker.Summary() {
		require.Equal(t, "csiSnapshot: not applicable for CSI ;podvolume: it's set to opt-out;", skippedPV.SerializeSkipReasons())
	}
}

func TestTrackUntrack(t *testing.T) {
	// If a pv is untracked explicitly it can't be Tracked again, b/c the pv is considered backed up already.
	tracker := NewSkipPVTracker()
	tracker.Track("pv3", podVolumeApproach, "it's set to opt-out")
	tracker.Untrack("pv3")
	tracker.Track("pv3", csiSnapshotApproach, "not applicable for CSI ")
	assert.Empty(t, tracker.Summary())
}
